package httpapi

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/risulongmemory/archive-center-go/internal/config"
	"github.com/risulongmemory/archive-center-go/internal/store"
	"github.com/risulongmemory/archive-center-go/internal/vector"
)

// turnRecordingStore implements store.Store and records all save/read calls.
type turnRecordingStore struct {
	memoryFakeStore
	savedChatLogs          []*store.ChatLog
	savedEffectiveInputs   []*store.EffectiveInput
	savedAuditLogs         []*store.AuditLog
	savedCriticFeedback    []*store.CriticFeedback
	returnMemories         []store.Memory
	savedMemories          []*store.Memory
	updatedImportance      map[int64]float64
	savedEvidence          []*store.DirectEvidence
	savedKGTriples         []*store.KGTriple
	savedStorylines        []*store.Storyline
	savedWorldRules        []*store.WorldRule
	savedEntities          []*store.Entity
	savedTrusts            []*store.Trust
	savedCharacterEvents   []*store.CharacterEvent
	savedCharacterStates   []*store.CharacterState
	savedPendingThreads    []*store.PendingThread
	savedActiveStates      []*store.ActiveState
	savedCanonicalLayers   []*store.CanonicalStateLayer
	returnKGTriples        []store.KGTriple
	returnEvidence         []store.DirectEvidence
	returnChatLogs         []store.ChatLog
	returnResumePack       *store.ResumePack
	returnStorylines       []store.Storyline
	returnWorldRules       []store.WorldRule
	returnCharStates       []store.CharacterState
	returnPendingThreads   []store.PendingThread
	returnActiveStates     []store.ActiveState
	returnCanonicalLayers  []store.CanonicalStateLayer
	returnEpisodeSums      []store.EpisodeSummary
	returnPersonaEntries   []store.PersonaMemoryEntry
	returnEntityMemories   []store.ProtagonistEntityMemory
	lastEpisodeLimit       int
	lastPersonaLimit       int
	lastEntityMemoryLimit  int
	savedEntityMemories    []*store.ProtagonistEntityMemory
	createdPersonaCapsules []*store.PersonaMemoryCapsule
	createdPersonaEntries  []store.PersonaMemoryEntry
	deletedStorylineIDs    []int64
	deletedWorldRuleIDs    []int64
}

type turnRecordingVectorStore struct {
	docs               []vector.VectorDocument
	deletedDocumentIDs []string
}

func (f *turnRecordingVectorStore) Search(ctx context.Context, sessionID string, query []float32, limit int, filter string) ([]vector.VectorDocument, error) {
	return nil, vector.ErrNotFound
}

func (f *turnRecordingVectorStore) Upsert(ctx context.Context, sessionID string, docs []vector.VectorDocument) error {
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

// ---------------------------------------------------------------------------
// /complete-turn tests
// ---------------------------------------------------------------------------

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

	// writeback_plan bundle (noop mode)
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

	// writeback_plan bundle (dual_shadow mode)
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

func TestCompleteTurnMariaDBAuthorityStartsAtTurnOneAndAdvances(t *testing.T) {
	fake := &turnRecordingStore{}
	cfg := config.Default()
	cfg.StoreMode = config.StoreModeMariaDBAuthority
	srv := NewServer(cfg)
	srv.Store = fake
	srv.StoreOpenError = nil

	mux := http.NewServeMux()
	srv.RegisterRoutes(mux)

	postCompleteTurn := func(body map[string]any) map[string]any {
		raw, _ := json.Marshal(body)
		req := httptest.NewRequest(http.MethodPost, "/complete-turn", bytes.NewReader(raw))
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
		return resp
	}

	first := postCompleteTurn(map[string]any{
		"chat_session_id":   "sess-turn-index",
		"user_input":        "first user",
		"assistant_content": "first assistant",
	})
	if first["turn_index"] != float64(1) {
		t.Fatalf("first turn_index = %v, want 1", first["turn_index"])
	}
	if len(fake.savedChatLogs) != 2 {
		t.Fatalf("after first turn savedChatLogs = %d, want 2", len(fake.savedChatLogs))
	}
	for _, log := range fake.savedChatLogs {
		if log.ChatSessionID != "sess-turn-index" || log.TurnIndex != 1 {
			t.Fatalf("first turn log mismatch: %#v", log)
		}
	}
	if len(fake.savedEffectiveInputs) != 1 || fake.savedEffectiveInputs[0].TurnIndex != 1 {
		t.Fatalf("first effective input mismatch: %#v", fake.savedEffectiveInputs)
	}

	fake.returnChatLogs = make([]store.ChatLog, 0, len(fake.savedChatLogs))
	for _, log := range fake.savedChatLogs {
		fake.returnChatLogs = append(fake.returnChatLogs, *log)
	}

	second := postCompleteTurn(map[string]any{
		"chat_session_id":   "sess-turn-index",
		"user_input":        "second user",
		"assistant_content": "second assistant",
	})
	if second["turn_index"] != float64(2) {
		t.Fatalf("second turn_index = %v, want 2", second["turn_index"])
	}
	if len(fake.savedChatLogs) != 4 {
		t.Fatalf("after second turn savedChatLogs = %d, want 4", len(fake.savedChatLogs))
	}
	wantTurns := []int{1, 1, 2, 2}
	wantRoles := []string{"user", "assistant", "user", "assistant"}
	for i, log := range fake.savedChatLogs {
		if log.ChatSessionID != "sess-turn-index" || log.TurnIndex != wantTurns[i] || log.Role != wantRoles[i] {
			t.Fatalf("chat log[%d] mismatch: %#v", i, log)
		}
	}
	if len(fake.savedEffectiveInputs) != 2 {
		t.Fatalf("savedEffectiveInputs = %d, want 2", len(fake.savedEffectiveInputs))
	}
	if fake.savedEffectiveInputs[0].TurnIndex != 1 || fake.savedEffectiveInputs[1].TurnIndex != 2 {
		t.Fatalf("effective input turn indexes = %#v, want 1 then 2", fake.savedEffectiveInputs)
	}
}

func TestCompleteTurnExplicitStaleTurnIndexWithoutPreserveDoesNotAutoAdvance(t *testing.T) {
	fake := &turnRecordingStore{
		returnChatLogs: []store.ChatLog{
			{ID: 1, ChatSessionID: "sess-stale-explicit", TurnIndex: 2, Role: "user", Content: "previous user"},
			{ID: 2, ChatSessionID: "sess-stale-explicit", TurnIndex: 2, Role: "assistant", Content: "previous assistant"},
		},
	}
	cfg := config.Default()
	cfg.StoreMode = config.StoreModeMariaDBAuthority
	srv := NewServer(cfg)
	srv.Store = fake
	srv.StoreOpenError = nil

	mux := http.NewServeMux()
	srv.RegisterRoutes(mux)
	body := `{"chat_session_id":"sess-stale-explicit","turn_index":2,"user_input":"new user","assistant_content":"new assistant","client_meta":{"critic":{"api_key":"","endpoint":"","model":""}}}`
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
	if resp["turn_index"] != float64(2) || resp["chat_logs_saved"] != float64(0) || resp["derived_artifacts_saved"] != float64(0) {
		t.Fatalf("stale explicit turn must fail closed without auto-advance: %+v", resp)
	}
	if len(fake.savedChatLogs) != 0 || len(fake.savedMemories) != 0 || len(fake.savedEvidence) != 0 || len(fake.savedKGTriples) != 0 {
		t.Fatalf("stale explicit turn must not save duplicate rows, logs=%d memories=%d evidence=%d kg=%d", len(fake.savedChatLogs), len(fake.savedMemories), len(fake.savedEvidence), len(fake.savedKGTriples))
	}
	failReasons, _ := resp["fail_reasons"].([]any)
	if len(failReasons) == 0 || failReasons[0] != "raw_turn_content_conflict" {
		t.Fatalf("fail_reasons = %#v, want raw_turn_content_conflict", resp["fail_reasons"])
	}
}

func TestCompleteTurnCriticWaitsForAssistantOutput(t *testing.T) {
	fake := &turnRecordingStore{}
	cfg := config.Default()
	cfg.StoreMode = config.StoreModeMariaDBAuthority
	srv := NewServer(cfg)
	srv.Store = fake
	srv.StoreOpenError = nil

	called := 0
	oldClient := proxyHTTPClient
	proxyHTTPClient = &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		called++
		return &http.Response{
			StatusCode: http.StatusOK,
			Header:     make(http.Header),
			Body:       io.NopCloser(strings.NewReader(`{"choices":[{"message":{"content":"{}"}}]}`)),
		}, nil
	})}
	defer func() { proxyHTTPClient = oldClient }()

	mux := http.NewServeMux()
	srv.RegisterRoutes(mux)

	body := map[string]any{
		"chat_session_id": "sess-critic-waits",
		"turn_index":      1,
		"user_input":      "user typed, but assistant has not answered yet",
		"client_meta":     map[string]any{"critic": map[string]any{"api_key": "sk-test", "endpoint": "https://api.example.com/v1", "model": "critic-model", "provider": "openai"}},
		"request_type":    "model",
	}
	raw, _ := json.Marshal(body)
	req := httptest.NewRequest(http.MethodPost, "/complete-turn", bytes.NewReader(raw))
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
	if called != 0 {
		t.Fatalf("critic HTTP calls = %d, want 0 before assistant output", called)
	}
	if resp["critic_triggered"] != false {
		t.Fatalf("critic_triggered = %v, want false before assistant output", resp["critic_triggered"])
	}
	failReasons, _ := resp["fail_reasons"].([]any)
	if len(failReasons) != 1 || failReasons[0] != "critic_skipped: assistant_content_missing" {
		t.Fatalf("fail_reasons = %#v, want assistant_content_missing skip", failReasons)
	}
	if len(fake.savedChatLogs) != 2 || fake.savedChatLogs[0].TurnIndex != 1 || fake.savedChatLogs[1].TurnIndex != 1 {
		t.Fatalf("chat logs should still save turn 1, got %#v", fake.savedChatLogs)
	}
	if len(fake.savedMemories) != 0 || len(fake.savedEvidence) != 0 || len(fake.savedKGTriples) != 0 {
		t.Fatalf("critic artifacts should not save without assistant output, memories=%d evidence=%d kg=%d", len(fake.savedMemories), len(fake.savedEvidence), len(fake.savedKGTriples))
	}
}

func TestCompleteTurnRejectsAssistantOnlyWithoutUserInput(t *testing.T) {
	fake := &turnRecordingStore{}
	cfg := config.Default()
	cfg.StoreMode = config.StoreModeMariaDBAuthority
	srv := NewServer(cfg)
	srv.Store = fake
	srv.StoreOpenError = nil

	mux := http.NewServeMux()
	srv.RegisterRoutes(mux)

	body := map[string]any{
		"chat_session_id":   "sess-assistant-only",
		"turn_index":        3,
		"user_input":        "",
		"assistant_content": "assistant text arrived without the matching user input",
		"request_type":      "model",
	}
	raw, _ := json.Marshal(body)
	req := httptest.NewRequest(http.MethodPost, "/complete-turn", bytes.NewReader(raw))
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
	if resp["status"] != "error" || resp["save_ok"] != false {
		t.Fatalf("assistant-only complete-turn should be rejected, resp=%+v", resp)
	}
	failReasons, _ := resp["fail_reasons"].([]any)
	if len(failReasons) != 1 || failReasons[0] != "user_input_missing" {
		t.Fatalf("fail_reasons = %#v, want user_input_missing", resp["fail_reasons"])
	}
	if len(fake.savedChatLogs) != 0 || len(fake.savedEffectiveInputs) != 0 || len(fake.savedMemories) != 0 || len(fake.savedEvidence) != 0 || len(fake.savedKGTriples) != 0 {
		t.Fatalf("assistant-only turn must not write artifacts, logs=%d effective=%d memories=%d evidence=%d kg=%d",
			len(fake.savedChatLogs), len(fake.savedEffectiveInputs), len(fake.savedMemories), len(fake.savedEvidence), len(fake.savedKGTriples))
	}
}

func TestCompleteTurnAllowsExplicitAutoContinueEmptyInput(t *testing.T) {
	fake := &turnRecordingStore{}
	cfg := config.Default()
	cfg.StoreMode = config.StoreModeMariaDBAuthority
	srv := NewServer(cfg)
	srv.Store = fake
	srv.StoreOpenError = nil

	mux := http.NewServeMux()
	srv.RegisterRoutes(mux)

	body := map[string]any{
		"chat_session_id":   "sess-auto-continue",
		"turn_index":        2,
		"user_input":        "",
		"assistant_content": "assistant continued the current scene from context",
		"request_type":      "model",
		"client_meta": map[string]any{
			"actual_empty_user_input": true,
			"logical_user_turn_key":   "[auto-continue]",
			"user_input_kind":         "auto_continue",
		},
	}
	raw, _ := json.Marshal(body)
	req := httptest.NewRequest(http.MethodPost, "/complete-turn", bytes.NewReader(raw))
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
	if resp["status"] != "ok" || resp["save_ok"] != true {
		t.Fatalf("explicit auto-continue empty input should save, resp=%+v", resp)
	}
	if len(fake.savedChatLogs) != 2 {
		t.Fatalf("chat logs saved = %d, want user+assistant rows", len(fake.savedChatLogs))
	}
	if fake.savedChatLogs[0].Role != "user" || fake.savedChatLogs[0].Content != "[auto-continue]" {
		t.Fatalf("auto-continue user row = %#v, want [auto-continue]", fake.savedChatLogs[0])
	}
	failReasons, _ := resp["fail_reasons"].([]any)
	for _, reason := range failReasons {
		if reason == "user_input_missing" {
			t.Fatalf("explicit auto-continue must not report user_input_missing: %#v", failReasons)
		}
	}
}

func TestCompleteTurnSanitizesThoughtTagsBeforeSave(t *testing.T) {
	fake := &turnRecordingStore{}
	cfg := config.Default()
	cfg.StoreMode = config.StoreModeMariaDBAuthority
	srv := NewServer(cfg)
	srv.Store = fake
	srv.StoreOpenError = nil
	mux := http.NewServeMux()
	srv.RegisterRoutes(mux)

	body := map[string]any{
		"chat_session_id":   "sess-sanitize",
		"turn_index":        1,
		"user_input":        "Visible user text. <thinking>hidden user chain</thinking> Still visible.",
		"assistant_content": "Visible assistant text. <filter>hidden assistant trace",
		"request_type":      "model",
	}
	raw, _ := json.Marshal(body)
	req := httptest.NewRequest(http.MethodPost, "/complete-turn", bytes.NewReader(raw))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200: %s", rec.Code, rec.Body.String())
	}
	if len(fake.savedChatLogs) != 2 {
		t.Fatalf("savedChatLogs count = %d, want 2", len(fake.savedChatLogs))
	}
	for _, log := range fake.savedChatLogs {
		lower := strings.ToLower(log.Content)
		for _, blocked := range []string{"hidden", "thinking", "filter"} {
			if strings.Contains(lower, blocked) {
				t.Fatalf("saved chat log leaked %q in %#v", blocked, log)
			}
		}
	}
	if len(fake.savedEffectiveInputs) != 1 {
		t.Fatalf("savedEffectiveInputs count = %d, want 1", len(fake.savedEffectiveInputs))
	}
	if strings.Contains(strings.ToLower(fake.savedEffectiveInputs[0].EffectiveInput), "hidden") {
		t.Fatalf("effective input leaked hidden text: %#v", fake.savedEffectiveInputs[0])
	}
}

func TestCompleteTurnWithCriticConfigWritesExtractedArtifacts(t *testing.T) {
	fake := &turnRecordingStore{
		returnCharStates: []store.CharacterState{{
			ChatSessionID:     "sess-live",
			CharacterName:     "Alice",
			RelationshipsJSON: `{"Carol":{"affection":20}}`,
		}},
		returnEvidence: []store.DirectEvidence{{EvidenceKind: "turn_excerpt", EvidenceText: "Alice previously accepted Bob's help.", SourceTurnStart: 1, SourceTurnEnd: 1, TurnAnchor: 1}},
	}
	vec := &turnRecordingVectorStore{}
	cfg := config.Default()
	cfg.StoreMode = config.StoreModeMariaDBAuthority
	cfg.ChromaEndpoint = "http://127.0.0.1:8000"
	cfg.Readiness.ChromaConfigured = true
	srv := NewServer(cfg)
	srv.Store = fake
	srv.StoreOpenError = nil
	srv.Vector = vec
	srv.VectorOpenError = nil

	extraction := map[string]any{
		"turn_summary":           "Alice decided to trust Bob after the rescue.",
		"importance_score":       8,
		"relationship_memory":    map[string]any{"bond_and_distance": "Alice trusts Bob more after he helped her.", "trust": 0.8},
		"entities":               map[string]any{"characters": []any{map[string]any{"name": "Alicee", "role": "protagonist", "status_emotion": "relieved"}}},
		"kg_triples":             []any{map[string]any{"subject": "Alicee", "predicate": "trusts", "object": "Bob", "valid_from": 2}},
		"archive_hint":           map[string]any{"wing": "wing_general", "room": "hall_relationships"},
		"evidence_excerpts":      []any{"I trust Bob."},
		"emotional_intensity":    0.7,
		"narrative_significance": 0.9,
		"state_deltas":           map[string]any{"scene_state": map[string]any{"mood": "warm"}},
		"character_deltas": []any{map[string]any{
			"name":          "Alicee",
			"status":        map[string]any{"emotion": "relieved"},
			"relationships": map[string]any{"Bob": map[string]any{"affection": 70, "tension": 15}},
			"events":        []any{map[string]any{"type": "relationship_shift", "detail": "Alice's trust in Bob increased."}},
		}},
		"pending_threads": []any{
			map[string]any{"thread_type": "promise", "title": "Alice thanks Bob later", "confidence": 0.85},
			map[string]any{"thread_type": "misc", "title": "Invalid hook should be skipped", "confidence": 0.9},
			map[string]any{"thread_type": "open_question", "title": "Too weak to keep", "confidence": 0.2},
		},
		"world_rules": []any{map[string]any{"scope": "session", "category": "relationship", "key": "trust_changes_need_evidence", "value": "Trust shifts should be grounded in visible actions."}},
	}
	extractionBytes, _ := json.Marshal(extraction)
	chatResp, _ := json.Marshal(map[string]any{
		"model":   "critic-model",
		"choices": []any{map[string]any{"message": map[string]any{"content": string(extractionBytes)}}},
	})
	oldClient := proxyHTTPClient
	proxyHTTPClient = &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		if strings.HasSuffix(r.URL.Path, "/embeddings") {
			return &http.Response{
				StatusCode: http.StatusOK,
				Header:     make(http.Header),
				Body:       io.NopCloser(strings.NewReader(`{"model":"embed-model","data":[{"embedding":[0.1,0.2,0.3]}]}`)),
			}, nil
		}
		return &http.Response{
			StatusCode: http.StatusOK,
			Header:     make(http.Header),
			Body:       io.NopCloser(bytes.NewReader(chatResp)),
		}, nil
	})}
	defer func() { proxyHTTPClient = oldClient }()

	mux := http.NewServeMux()
	srv.RegisterRoutes(mux)

	body := map[string]any{
		"chat_session_id":   "sess-live",
		"turn_index":        2,
		"user_input":        "I trust Bob.",
		"assistant_content": "Alice relaxed after Bob helped her.",
		"context_messages":  []map[string]any{{"role": "user", "content": "Alice hesitated before trusting Bob."}, {"role": "assistant", "content": "Bob helped Alice escape."}},
		"improvement_trace": map[string]any{"score": 9},
		"client_meta":       map[string]any{"critic": map[string]any{"api_key": "sk-test", "endpoint": "https://api.example.com/v1", "model": "critic-model", "provider": "openai", "max_tokens": 1200}, "embedding": map[string]any{"api_key": "sk-test", "endpoint": "https://api.example.com/v1", "model": "embed-model", "provider": "openai"}},
		"request_type":      "model",
	}
	raw, _ := json.Marshal(body)
	req := httptest.NewRequest(http.MethodPost, "/complete-turn", bytes.NewReader(raw))
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
		t.Fatalf("critic_triggered = %v, want true", resp["critic_triggered"])
	}
	wantCounts := map[string]float64{
		"memories_saved":         1,
		"evidence_saved":         1,
		"kg_triples_saved":       1,
		"entities_saved":         1,
		"trust_states_saved":     1,
		"world_rules_saved":      1,
		"storylines_saved":       1,
		"character_states_saved": 1,
		"character_events_saved": 1,
		"pending_threads_saved":  1,
		"active_states_saved":    5,
	}
	for key, want := range wantCounts {
		if resp[key] != want {
			t.Fatalf("%s = %v, want %.0f", key, resp[key], want)
		}
	}
	if len(fake.savedMemories) != 1 || fake.savedMemories[0].EmbeddingModel == "fake" {
		t.Fatalf("expected one non-fake memory, got %#v", fake.savedMemories)
	}
	if fake.savedMemories[0].Importance != 0.9 || fake.savedMemories[0].EmotionalBoost != 1.0 || fake.savedMemories[0].EmotionalIntensity != 0.7 || fake.savedMemories[0].NarrativeSignificance != 0.9 {
		t.Fatalf("expected emotional boost memory fields, got %#v", fake.savedMemories[0])
	}
	if len(fake.savedEvidence) != 1 || fake.savedEvidence[0].EvidenceText != "I trust Bob." {
		t.Fatalf("expected excerpt evidence only, got %#v", fake.savedEvidence)
	}
	if ev := fake.savedEvidence[0]; ev.SourceTurnStart != 2 || ev.SourceTurnEnd != 2 || ev.TurnAnchor != 2 || !strings.Contains(ev.SourceMessageIDsJSON, "turn:2") || !strings.Contains(ev.LineageJSON, "critic.evidence_excerpts") {
		t.Fatalf("expected evidence source lineage for turn 2, got %#v", ev)
	}
	if len(fake.savedKGTriples) != 1 || fake.savedKGTriples[0].Subject != "Alice" || fake.savedKGTriples[0].Object != "Bob" {
		t.Fatalf("expected extracted KG triple, got %#v", fake.savedKGTriples)
	}
	if len(fake.savedEntities) != 1 || fake.savedEntities[0].Name != "Alice" {
		t.Fatalf("expected extracted entity, got %#v", fake.savedEntities)
	}
	if len(fake.savedTrusts) != 1 {
		t.Fatalf("expected trust state, got %#v", fake.savedTrusts)
	}
	if fake.savedTrusts[0].TargetName == "relationship_memory" || fake.savedTrusts[0].TargetName == "" {
		t.Fatalf("expected trust to target an extracted entity, got %#v", fake.savedTrusts[0])
	}
	if fake.savedTrusts[0].Score != 0.8 {
		t.Fatalf("expected normalized trust score 0.8, got %#v", fake.savedTrusts[0])
	}
	if len(fake.savedCharacterEvents) != 1 || len(fake.savedCharacterStates) != 1 || len(fake.savedPendingThreads) != 1 || len(fake.savedActiveStates) != 5 {
		t.Fatalf("expected character/state/thread artifacts, events=%d states=%d threads=%d active=%d", len(fake.savedCharacterEvents), len(fake.savedCharacterStates), len(fake.savedPendingThreads), len(fake.savedActiveStates))
	}
	if rel := fake.savedCharacterStates[0].RelationshipsJSON; !strings.Contains(rel, "Carol") || !strings.Contains(rel, "Bob") || !strings.Contains(rel, "affection") || !strings.Contains(rel, "tension") {
		t.Fatalf("expected character relationships to merge existing and incoming values, got %s", rel)
	}
	if len(fake.savedWorldRules) != 1 || len(fake.savedStorylines) != 1 {
		t.Fatalf("expected world/story artifacts, world=%d story=%d", len(fake.savedWorldRules), len(fake.savedStorylines))
	}
	if wr := fake.savedWorldRules[0]; wr.Scope != "session" || wr.Category != "relationship" || wr.Key != "trust_changes_need_evidence" || !strings.Contains(wr.ValueJSON, "Trust shifts") {
		t.Fatalf("expected normalized world rule fields, got %#v", wr)
	}
	if sl := fake.savedStorylines[0]; sl.Name != "Alice thanks Bob later" || sl.Status != "active" || !strings.Contains(sl.KeyPointsJSON, "Alice thanks Bob later") || !strings.Contains(sl.OngoingTensionsJSON, "promise") {
		t.Fatalf("expected normalized storyline fields, got %#v", sl)
	}
	if len(vec.docs) != 3 {
		t.Fatalf("expected memory/evidence/world-rule vector upserts, got %#v", vec.docs)
	}
	if vec.docs[0].Tier != "memory" || vec.docs[0].ChatSessionID != "sess-live" || len(vec.docs[0].Embedding) != 3 {
		t.Fatalf("unexpected vector doc: %#v", vec.docs[0])
	}
	trace := resp["trace_handoff"].(map[string]any)
	if trace["vector_status"] != "ok" || resp["vectors_upserted"] != float64(3) || resp["vectors_evidence_upserted"] != float64(1) || resp["vectors_world_rule_upserted"] != float64(1) {
		t.Fatalf("vector status/count mismatch: trace=%+v resp=%+v", trace, resp)
	}
	if resp["maintenance_enqueued"] != true {
		t.Fatalf("maintenance_enqueued = %v, want true", resp["maintenance_enqueued"])
	}
	if trace["critic_pipeline_version"] != "ea1j.v1" || trace["critic_pipeline_split_enabled"] != true || trace["critic_pipeline_all_in_single_call"] != false {
		t.Fatalf("critic pipeline handoff mismatch: %+v", trace)
	}
	if trace["critic_preview_pass_version"] != "ea1k.v1" || trace["direct_evidence_retention_policy_version"] != "ea1l.v1" {
		t.Fatalf("preview/retention handoff mismatch: %+v", trace)
	}
	criticTrace, ok := trace["critic_trace"].(map[string]any)
	if !ok {
		t.Fatalf("critic_trace missing: %+v", trace)
	}
	pipeline, ok := criticTrace["pipeline"].(map[string]any)
	if !ok || pipeline["policy_version"] != "ea1j.v1" {
		t.Fatalf("critic pipeline trace missing: %+v", criticTrace)
	}
	stages, ok := pipeline["stages"].(map[string]any)
	if !ok || stages["evidence_extractor"] == nil || stages["deterministic_reducer"] == nil || stages["summary_compactor_background"] == nil {
		t.Fatalf("critic split stages missing: %+v", pipeline)
	}
	previewPass, ok := criticTrace["preview_pass"].(map[string]any)
	if !ok || previewPass["policy_version"] != "ea1k.v1" {
		t.Fatalf("preview_pass trace missing: %+v", criticTrace)
	}
	rawPreview, ok := previewPass["recent_raw_preview"].([]any)
	if !ok || len(rawPreview) == 0 {
		t.Fatalf("preview_pass recent_raw_preview missing: %+v", previewPass)
	}
	switch directSeed := previewPass["recent_verified_direct_evidence_seed"].(type) {
	case []map[string]any:
		if len(directSeed) == 0 {
			t.Fatalf("preview_pass direct evidence seed empty: %+v", previewPass)
		}
	case []any:
		if len(directSeed) == 0 {
			t.Fatalf("preview_pass direct evidence seed empty: %+v", previewPass)
		}
	default:
		t.Fatalf("preview_pass direct evidence seed missing: %+v", previewPass)
	}
	if _, ok := previewPass["triage"].(map[string]any); !ok {
		t.Fatalf("preview_pass triage missing: %+v", previewPass)
	}
	if _, ok := previewPass["compaction_hint"].(map[string]any); !ok {
		t.Fatalf("preview_pass compaction_hint missing: %+v", previewPass)
	}
	if trace["maintenance_queue_status"] != "audit_shadow_enqueued" || trace["maintenance_queue_depth"] != float64(1) {
		t.Fatalf("maintenance handoff mismatch: %+v", trace)
	}
	maintenance, ok := trace["maintenance_handoff"].(map[string]any)
	if !ok || maintenance["owner"] != "complete_turn" || maintenance["worker_enabled"] != false {
		t.Fatalf("maintenance_handoff owner/worker flag mismatch: %#v", trace["maintenance_handoff"])
	}
	foundMaintenanceAudit := false
	for _, item := range fake.savedAuditLogs {
		if item.EventType == "maintenance_enqueued" && item.TargetID == 2 {
			foundMaintenanceAudit = true
			break
		}
	}
	if !foundMaintenanceAudit {
		t.Fatalf("expected maintenance_enqueued audit log, got %#v", fake.savedAuditLogs)
	}
}

func TestCompleteTurnEpisodeCheckpointGeneratesAtIntervalBoundary(t *testing.T) {
	fake := &adminRegeneratedArtifactStore{
		adminEpisodeBackfillStore: &adminEpisodeBackfillStore{
			turnRecordingStore: &turnRecordingStore{
				returnChatLogs: []store.ChatLog{
					{ChatSessionID: "sess-live-episode", TurnIndex: 1, Role: "user", Content: "Luka draws the bridge plan."},
					{ChatSessionID: "sess-live-episode", TurnIndex: 1, Role: "assistant", Content: "Hank studies the marked routes."},
					{ChatSessionID: "sess-live-episode", TurnIndex: 2, Role: "user", Content: "Wren checks the oxygen tanks."},
					{ChatSessionID: "sess-live-episode", TurnIndex: 2, Role: "assistant", Content: "The supply crew marks the tanks."},
					{ChatSessionID: "sess-live-episode", TurnIndex: 3, Role: "user", Content: "Luka confirms the convoy route."},
					{ChatSessionID: "sess-live-episode", TurnIndex: 3, Role: "assistant", Content: "Hank approves the north approach."},
					{ChatSessionID: "sess-live-episode", TurnIndex: 4, Role: "user", Content: "The demolition team prepares the first charge."},
					{ChatSessionID: "sess-live-episode", TurnIndex: 4, Role: "assistant", Content: "The bridge plan is ready for final timing."},
				},
			},
		},
	}
	cfg := config.Default()
	cfg.StoreMode = config.StoreModeMariaDBAuthority
	srv := NewServer(cfg)
	srv.Store = fake
	srv.StoreOpenError = nil

	extractionBytes, _ := json.Marshal(map[string]any{
		"turn_summary":     "Wren loads oxygen tanks for Operation Ice Wedge.",
		"importance_score": 7,
		"evidence_excerpts": []any{
			"Wren loads oxygen tanks",
		},
	})
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
	body := map[string]any{
		"chat_session_id":   "sess-live-episode",
		"turn_index":        5,
		"user_input":        "Wren starts loading oxygen tanks.",
		"assistant_content": "Wren loads oxygen tanks while Hank confirms Operation Ice Wedge.",
		"request_type":      "model",
		"client_meta": map[string]any{
			"episode_interval_turns": 5,
			"critic": map[string]any{
				"api_key":  "sk-test",
				"endpoint": "https://api.example.com/v1",
				"model":    "critic-model",
				"provider": "openai",
			},
		},
	}
	raw, _ := json.Marshal(body)
	req := httptest.NewRequest(http.MethodPost, "/complete-turn", bytes.NewReader(raw))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200: %s", rec.Code, rec.Body.String())
	}
	if len(fake.savedEpisodes) != 1 {
		t.Fatalf("saved episodes = %d, want 1: %#v", len(fake.savedEpisodes), fake.savedEpisodes)
	}
	if !strings.Contains(fake.savedEpisodes[0].SummaryText, "Operation Ice Wedge") {
		t.Fatalf("episode did not use checkpoint turn memory: %q", fake.savedEpisodes[0].SummaryText)
	}
	var resp map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	episodeResult := resp["episode_result"].(map[string]any)
	if episodeResult["triggered"] != true || episodeResult["generated"] != float64(1) {
		t.Fatalf("episode_result mismatch: %+v", episodeResult)
	}
}

func TestCompleteTurnAccumulatesCharacterRelationshipAcrossThreeTurns(t *testing.T) {
	const sid = "sess-rel-accumulate"
	fake := newRelationshipAccumulatingTurnStore([]store.CharacterState{{
		ChatSessionID:     sid,
		CharacterName:     "Alice",
		RelationshipsJSON: `{"Bob":{"affection":10,"tension":60,"last_change":"uneasy truce"}}`,
		TurnIndex:         1,
	}})
	cfg := config.Default()
	cfg.StoreMode = config.StoreModeMariaDBAuthority
	srv := NewServer(cfg)
	srv.Store = fake
	srv.StoreOpenError = nil

	extractions := []map[string]any{
		{
			"turn_summary":     "Alice accepts Bob's help but remains guarded.",
			"importance_score": 6,
			"character_deltas": []any{map[string]any{
				"name":          "Alice",
				"relationships": map[string]any{"Bob": map[string]any{"affection": 25, "tension": 50}},
				"events":        []any{map[string]any{"type": "relationship_shift", "detail": "Alice accepts Bob's help."}},
			}},
		},
		{
			"turn_summary":     "Alice warms to Bob after he keeps watch.",
			"importance_score": 6,
			"character_deltas": []any{map[string]any{
				"name":          "Alice",
				"relationships": map[string]any{"Bob": map[string]any{"affection": 35}},
				"events":        []any{map[string]any{"type": "relationship_shift", "detail": "Alice warms to Bob."}},
			}},
		},
		{
			"turn_summary":     "Alice accepts Bob's apology and the tension drops.",
			"importance_score": 7,
			"character_deltas": []any{map[string]any{
				"name":          "Alice",
				"relationships": map[string]any{"Bob": map[string]any{"tension": 12, "last_change": "Alice accepted Bob's apology."}},
				"events":        []any{map[string]any{"type": "relationship_shift", "detail": "Alice accepted Bob's apology."}},
			}},
		},
	}
	criticCall := 0
	oldClient := proxyHTTPClient
	proxyHTTPClient = &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		if criticCall >= len(extractions) {
			t.Fatalf("unexpected extra critic call %d", criticCall+1)
		}
		extractionBytes, _ := json.Marshal(extractions[criticCall])
		criticCall++
		resp, _ := json.Marshal(map[string]any{
			"model":   "critic-model",
			"choices": []any{map[string]any{"message": map[string]any{"content": string(extractionBytes)}}},
		})
		return &http.Response{
			StatusCode: http.StatusOK,
			Header:     make(http.Header),
			Body:       io.NopCloser(bytes.NewReader(resp)),
		}, nil
	})}
	defer func() { proxyHTTPClient = oldClient }()

	mux := http.NewServeMux()
	srv.RegisterRoutes(mux)
	for i := range extractions {
		turnIndex := i + 2
		body := map[string]any{
			"chat_session_id":   sid,
			"turn_index":        turnIndex,
			"user_input":        fmt.Sprintf("relationship check turn %d", turnIndex),
			"assistant_content": fmt.Sprintf("Alice and Bob relationship beat %d.", turnIndex),
			"client_meta": map[string]any{
				"critic": map[string]any{
					"api_key":  "sk-critic-test",
					"endpoint": "https://api.example.com/v1",
					"model":    "critic-model",
					"provider": "openai",
				},
			},
		}
		raw, _ := json.Marshal(body)
		req := httptest.NewRequest(http.MethodPost, "/complete-turn", bytes.NewReader(raw))
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()
		mux.ServeHTTP(rec, req)
		if rec.Code != http.StatusOK {
			t.Fatalf("turn %d status = %d, want 200: %s", turnIndex, rec.Code, rec.Body.String())
		}
		var resp map[string]any
		if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
			t.Fatalf("turn %d decode response: %v", turnIndex, err)
		}
		if resp["critic_triggered"] != true || resp["character_states_saved"] != float64(1) || resp["character_events_saved"] != float64(1) {
			t.Fatalf("turn %d did not save one character relationship update: %+v", turnIndex, resp)
		}
	}
	if criticCall != 3 || len(fake.savedCharacterStates) != 3 || len(fake.savedCharacterEvents) != 3 {
		t.Fatalf("expected 3 critic/state/event calls, critic=%d states=%d events=%d", criticCall, len(fake.savedCharacterStates), len(fake.savedCharacterEvents))
	}

	finalState, err := fake.GetCharacterState(context.Background(), sid, "Alice")
	if err != nil {
		t.Fatalf("GetCharacterState final: %v", err)
	}
	if finalState.TurnIndex != 4 {
		t.Fatalf("final turn_index = %d, want 4", finalState.TurnIndex)
	}
	var relationships map[string]any
	if err := json.Unmarshal([]byte(finalState.RelationshipsJSON), &relationships); err != nil {
		t.Fatalf("decode relationships_json %q: %v", finalState.RelationshipsJSON, err)
	}
	if _, ok := relationships["affection"]; ok {
		t.Fatalf("relationship fields leaked to top level instead of target key: %+v", relationships)
	}
	bob, ok := relationships["Bob"].(map[string]any)
	if !ok {
		t.Fatalf("missing Bob relationship target in %+v", relationships)
	}
	if got := extractionFloatFromAny(bob["affection"], 0); got != 35 {
		t.Fatalf("Bob affection = %v, want latest preserved 35 in %+v", got, bob)
	}
	if got := extractionFloatFromAny(bob["tension"], 0); got != 12 {
		t.Fatalf("Bob tension = %v, want latest 12 in %+v", got, bob)
	}
	if got := extractionStringFromAny(bob["last_change"]); got != "Alice accepted Bob's apology." {
		t.Fatalf("Bob last_change = %q, want final turn change in %+v", got, bob)
	}
}

func TestCompleteTurnCriticGuardsEvidenceKGAndEntityTypes(t *testing.T) {
	fake := &turnRecordingStore{}
	cfg := config.Default()
	cfg.StoreMode = config.StoreModeMariaDBAuthority
	srv := NewServer(cfg)
	srv.Store = fake
	srv.StoreOpenError = nil

	userText := "Mina searches the old library."
	assistantText := "Mina found a brass key. Rowan promised to help her open the cellar."
	fullTurn := strings.TrimSpace(userText + "\n" + assistantText)
	extraction := map[string]any{
		"turn_summary":        "Mina found a brass key and Rowan offered help.",
		"importance_score":    7,
		"evidence_excerpts":   []any{fullTurn, "Mina found a brass key."},
		"relationship_memory": map[string]any{"bond_and_distance": "Mina trusts Rowan more after the promise.", "target_name": "Rowan", "trust": 0.6},
		"entities": map[string]any{
			"characters": []any{map[string]any{"name": "Mina", "role": "protagonist"}},
			"locations":  []any{map[string]any{"name": "old library", "description": "quiet archive room"}},
			"items":      []any{map[string]any{"name": "brass key", "description": "cellar key"}},
		},
		"kg_triples": []any{
			map[string]any{"subject": "char_59_cid_fb179fa9-3a73-496e-8df5-35c621338f9f", "predicate": "has_turn", "object": "turn_1"},
			map[string]any{"subject": "Mina", "predicate": "found", "object": "brass key"},
		},
		"world_rules": []any{map[string]any{"scope": "location", "scope_name": "old library", "category": "access", "key": "cellar_needs_key", "value": "The cellar can be opened with the brass key."}},
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
	body := map[string]any{
		"chat_session_id":   "sess-guard",
		"turn_index":        1,
		"user_input":        userText,
		"assistant_content": assistantText,
		"request_type":      "model",
		"client_meta":       map[string]any{"critic": map[string]any{"api_key": "sk-test", "endpoint": "https://api.example.com/v1", "model": "critic-model", "provider": "openai"}},
	}
	raw, _ := json.Marshal(body)
	req := httptest.NewRequest(http.MethodPost, "/complete-turn", bytes.NewReader(raw))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200: %s", rec.Code, rec.Body.String())
	}

	if len(fake.savedEvidence) != 1 || fake.savedEvidence[0].EvidenceText != "Mina found a brass key." {
		t.Fatalf("expected only grounded short evidence, got %#v", fake.savedEvidence)
	}
	if fake.savedEvidence[0].ArchiveState != "verified_direct" || fake.savedEvidence[0].CaptureVerification != "verified" || fake.savedEvidence[0].CommittedGate != "auto_grounded_excerpt" {
		t.Fatalf("grounded direct evidence was not auto-verified: %#v", fake.savedEvidence[0])
	}
	if len(fake.savedKGTriples) != 1 || fake.savedKGTriples[0].Subject != "Mina" || fake.savedKGTriples[0].Predicate == "has_turn" || fake.savedKGTriples[0].Object == "turn_1" {
		t.Fatalf("expected placeholder KG to be dropped, got %#v", fake.savedKGTriples)
	}
	if len(fake.savedEntities) != 3 {
		t.Fatalf("expected character/location/item entities, got %#v", fake.savedEntities)
	}
	types := map[string]bool{}
	for _, item := range fake.savedEntities {
		types[item.EntityType] = true
	}
	for _, want := range []string{"protagonist", "location", "item"} {
		if !types[want] {
			t.Fatalf("missing entity type %q in %#v", want, fake.savedEntities)
		}
	}
	if len(fake.savedTrusts) != 1 || fake.savedTrusts[0].TargetName != "Rowan" {
		t.Fatalf("expected trust target Rowan, got %#v", fake.savedTrusts)
	}
	if len(fake.savedWorldRules) != 1 {
		t.Fatalf("expected world rule, got %#v", fake.savedWorldRules)
	}
}

func TestCompleteTurnLocationTimeGroundingSeparatesSceneResidenceAndSeason(t *testing.T) {
	t.Run("critic_prompt_names_location_time_lanes", func(t *testing.T) {
		prompt := buildCompleteTurnCriticPrompt(
			"sess-loc-time", 8,
			"Rowan lives in London.",
			"The current scene stays on the school rooftop as summer vacation begins.",
			nil, nil, nil,
		)
		for _, needle := range []string{
			"current scene location or current scene time belongs in state_deltas.scene_state",
			"durable residence, hometown, birthplace, workplace, or affiliation belongs in character_deltas.status",
			"Do not treat 'X lives in London' as 'the current scene is London'",
			"summer vacation has started",
			"Do not infer an immediate return to school",
		} {
			if !strings.Contains(prompt, needle) {
				t.Fatalf("critic prompt missing location/time grounding marker %q", needle)
			}
		}
	})

	fake := &turnRecordingStore{}
	srv := NewServer(config.Default())
	srv.Store = fake
	srv.StoreOpenError = nil

	content := `Rowan told Mina, "I live in London, not at the academy." The current scene stayed on the school rooftop. Summer vacation had just begun, so classes were over.`
	extraction := normalizeCriticExtraction(map[string]any{
		"turn_summary":     "Rowan's residence is London, while the current scene remains on the school rooftop as summer vacation begins.",
		"importance_score": 7,
		"evidence_excerpts": []any{
			"I live in London",
			"current scene stayed on the school rooftop",
			"Summer vacation had just begun",
		},
		"entities": map[string]any{
			"characters": []any{map[string]any{"name": "Rowan"}, map[string]any{"name": "Mina"}},
			"locations":  []any{map[string]any{"name": "London"}, map[string]any{"name": "school rooftop"}},
		},
		"kg_triples": []any{
			map[string]any{"subject": "Rowan", "predicate": "residence", "object": "London", "valid_from": 8},
		},
		"character_deltas": []any{map[string]any{
			"name":   "Rowan",
			"status": map[string]any{"residence": "London"},
		}},
		"state_deltas": map[string]any{
			"scene_state":  map[string]any{"location": "school rooftop", "time_state": "summer_vacation_started", "school_status": "classes_over"},
			"confidence":   0.86,
			"verification": "verified",
		},
		"world_rules": []any{map[string]any{
			"scope":      "session",
			"category":   "time",
			"key":        "summer_vacation_started",
			"value":      "Summer vacation has started; classes are over until direct evidence says otherwise.",
			"confidence": 0.85,
		}},
		"world_state": map[string]any{
			"time_state":    "summer_vacation_started",
			"season":        "summer",
			"school_status": "classes_over",
			"confidence":    0.85,
			"verification":  "verified",
		},
	})

	result := srv.saveCriticExtractionArtifacts(context.Background(), "sess-loc-time", 8, extraction, content, completeTurnEmbeddingConfig{}, time.Unix(800, 0))
	if result.Errors != 0 {
		t.Fatalf("saveCriticExtractionArtifacts errors=%d details=%#v", result.Errors, result.ErrorDetails)
	}
	if result.Evidence != 3 || len(fake.savedEvidence) != 3 {
		t.Fatalf("expected 3 grounded evidence excerpts, got result=%d saved=%d skip=%#v", result.Evidence, len(fake.savedEvidence), result.SkipReasons)
	}
	if len(fake.savedKGTriples) != 1 {
		t.Fatalf("expected one residence KG triple, got %#v", fake.savedKGTriples)
	}
	kg := fake.savedKGTriples[0]
	if kg.Subject != "Rowan" || kg.Predicate != "residence" || kg.Object != "London" || kg.ValidFrom != 8 || kg.SourceTurn != 8 {
		t.Fatalf("residence KG triple mismatch: %#v", kg)
	}
	if len(fake.savedCharacterStates) != 1 {
		t.Fatalf("expected one character state, got %#v", fake.savedCharacterStates)
	}
	statusJSON := fake.savedCharacterStates[0].StatusJSON
	if !strings.Contains(statusJSON, "London") || !strings.Contains(statusJSON, "residence") {
		t.Fatalf("character status should carry durable residence, got %s", statusJSON)
	}
	if strings.Contains(statusJSON, "school rooftop") {
		t.Fatalf("current scene location leaked into durable character status: %s", statusJSON)
	}

	var sceneState, worldState string
	for _, item := range fake.savedActiveStates {
		switch item.StateType {
		case "state_deltas":
			sceneState = item.Content
		case "world_state":
			worldState = item.Content
		}
	}
	if !strings.Contains(sceneState, "school rooftop") || !strings.Contains(sceneState, "summer_vacation_started") {
		t.Fatalf("scene state should carry current location/time, got %s", sceneState)
	}
	if strings.Contains(sceneState, `"residence"`) {
		t.Fatalf("durable residence leaked into current scene state: %s", sceneState)
	}
	if !strings.Contains(worldState, "summer_vacation_started") || !strings.Contains(worldState, "classes_over") {
		t.Fatalf("world_state should carry verified story calendar state, got %s", worldState)
	}
	if len(fake.savedWorldRules) != 1 || fake.savedWorldRules[0].Category != "time" || fake.savedWorldRules[0].Key != "summer_vacation_started" {
		t.Fatalf("expected time world rule for summer vacation, got %#v", fake.savedWorldRules)
	}

	layerByType := map[string]string{}
	for _, item := range fake.savedCanonicalLayers {
		layerByType[item.LayerType] = item.Content
	}
	if !strings.Contains(layerByType["scene_state"], "school rooftop") {
		t.Fatalf("canonical scene_state missing current scene location: %#v", layerByType)
	}
	if strings.Contains(layerByType["scene_state"], `"residence"`) || strings.Contains(layerByType["scene_state"], "London") {
		t.Fatalf("canonical scene_state should not promote durable residence: %s", layerByType["scene_state"])
	}
	if !strings.Contains(layerByType["world_state"], "summer_vacation_started") {
		t.Fatalf("canonical world_state missing summer vacation state: %#v", layerByType)
	}
}

func TestCompleteTurnCriticIngestTraceRecordsSkipReasons(t *testing.T) {
	fake := &turnRecordingStore{}
	srv := NewServer(config.Default())
	srv.Store = fake
	srv.StoreOpenError = nil

	extraction := normalizeCriticExtraction(map[string]any{
		"turn_summary":      "Mina found the brass key.",
		"importance_score":  6,
		"evidence_excerpts": []any{"prompt template says remember everything"},
		"kg_triples": []any{
			map[string]any{"subject": "char_59", "predicate": "has_turn", "object": "turn_1"},
		},
		"pending_threads": []any{
			map[string]any{"thread_type": "promise", "title": "Mina will test the lock", "confidence": 0.1},
			map[string]any{"thread_type": "style_rule", "title": "Write in a poetic style", "confidence": 0.9},
		},
	})
	result := srv.saveCriticExtractionArtifacts(context.Background(), "sess-trace", 9, extraction, "Mina found the brass key.", completeTurnEmbeddingConfig{}, time.Unix(900, 0))
	if result.Errors != 0 {
		t.Fatalf("critic ingest trace should not error, result=%#v", result)
	}
	if result.Evidence != 0 || result.KGTriples != 0 || result.PendingThreads != 0 {
		t.Fatalf("expected unsafe derived rows to be skipped, evidence=%d kg=%d threads=%d", result.Evidence, result.KGTriples, result.PendingThreads)
	}
	if len(result.SkipReasons) < 3 {
		t.Fatalf("expected direct evidence/KG/pending skip reasons, got %#v", result.SkipReasons)
	}
	var trace string
	for _, item := range fake.savedAuditLogs {
		if item.EventType == "critic_ingest_trace" {
			trace = item.DetailsJSON
			break
		}
	}
	if trace == "" {
		t.Fatalf("expected critic_ingest_trace audit, got %#v", fake.savedAuditLogs)
	}
	for _, needle := range []string{"critic_ingest_trace.v1", "not_grounded_in_current_turn", "placeholder_or_control_edge", "low_confidence", "invalid_thread_type"} {
		if !strings.Contains(trace, needle) {
			t.Fatalf("critic_ingest_trace missing %q: %s", needle, trace)
		}
	}
}

func TestCompleteTurnPersonaCapsuleCandidatesRemainOptIn(t *testing.T) {
	fake := &turnRecordingStore{}
	srv := NewServer(config.Default())
	srv.Store = fake
	srv.StoreOpenError = nil

	extraction := normalizeCriticExtraction(map[string]any{
		"turn_summary":      "",
		"importance_score":  8,
		"evidence_excerpts": []any{},
		"persona_capsule_candidates": []any{
			map[string]any{
				"memory_text":       "The protagonist remembers dying before the loop reset.",
				"source_turn_index": 11,
				"importance_10":     9,
				"emotional_weight":  0.8,
				"portability":       "cross_world",
				"mode":              "full_loop_memory",
				"secret_guard":      "true",
				"tags":              []any{"loop", "protagonist_private"},
				"evidence_excerpt":  "I remember dying before everything reset.",
			},
		},
	})

	result := srv.saveCriticExtractionArtifacts(context.Background(), "sess-pmc5", 11, extraction, "I remember dying before everything reset.", completeTurnEmbeddingConfig{}, time.Unix(1100, 0))
	if result.PersonaCapsuleCandidates != 1 {
		t.Fatalf("PersonaCapsuleCandidates = %d, want 1", result.PersonaCapsuleCandidates)
	}
	if len(fake.createdPersonaCapsules) != 0 || len(fake.createdPersonaEntries) != 0 {
		t.Fatalf("persona capsule candidates must not auto-create capsules, capsules=%d entries=%d", len(fake.createdPersonaCapsules), len(fake.createdPersonaEntries))
	}
	if len(fake.savedMemories) != 0 || len(fake.savedEvidence) != 0 || len(fake.savedKGTriples) != 0 {
		t.Fatalf("persona capsule candidates must not auto-promote to canonical rows, memories=%d evidence=%d kg=%d", len(fake.savedMemories), len(fake.savedEvidence), len(fake.savedKGTriples))
	}
	warnings := strings.Join(result.Warnings, "\n")
	if !strings.Contains(warnings, "persona_capsule_candidates_detected:auto_create_disabled") {
		t.Fatalf("missing opt-in warning, got %#v", result.Warnings)
	}
	var foundSkip bool
	for _, skip := range result.SkipReasons {
		if skip["surface"] == "persona_capsule_candidates" && skip["reason"] == "requires_explicit_user_or_operator_approval" {
			foundSkip = true
			break
		}
	}
	if !foundSkip {
		t.Fatalf("missing persona capsule approval skip reason: %#v", result.SkipReasons)
	}
	var trace string
	for _, item := range fake.savedAuditLogs {
		if item.EventType == "critic_ingest_trace" {
			trace = item.DetailsJSON
			break
		}
	}
	for _, needle := range []string{"persona_capsule_candidates", "auto_create_disabled", "requires_explicit_user_or_operator_approval"} {
		if !strings.Contains(trace, needle) {
			t.Fatalf("critic ingest trace missing %q: %s", needle, trace)
		}
	}
}

func TestCompleteTurnSubjectiveEntityMemoriesAutoSaveByOwner(t *testing.T) {
	fake := &turnRecordingStore{}
	srv := NewServer(config.Default())
	srv.Store = fake
	srv.StoreOpenError = nil

	content := "Siwoo noticed the warning sign in Exit 2. Asuna thought Siwoo was hiding fear from her."
	extraction := normalizeCriticExtraction(map[string]any{
		"turn_summary":      "",
		"importance_score":  7,
		"evidence_excerpts": []any{},
		"subjective_entity_memories": []any{
			map[string]any{
				"owner_entity_key":     "siwoo",
				"owner_entity_name":    "Siwoo",
				"owner_entity_role":    "protagonist",
				"owner_visibility":     "player_known",
				"memory_text":          "Siwoo remembers the Exit 2 hallway as a warning sign.",
				"source_turn_index":    2,
				"importance_10":        7,
				"emotional_weight":     0.6,
				"evidence_excerpt":     "Exit 2",
				"target_reveal_policy": "requires_explicit_attachment",
			},
			map[string]any{
				"owner_entity_key":  "asuna",
				"owner_entity_name": "Asuna",
				"owner_entity_role": "npc",
				"memory_text":       "Asuna privately believes Siwoo is hiding fear from her.",
				"importance_10":     6,
				"emotional_weight":  0.7,
				"evidence_excerpt":  "hiding fear",
				"secret_guard":      true,
			},
		},
		"persona_capsule_candidates": []any{},
	})

	result := srv.saveCriticExtractionArtifacts(context.Background(), "sess-subjective", 2, extraction, content, completeTurnEmbeddingConfig{}, time.Unix(1200, 0))
	if result.SubjectiveEntityMemories != 2 {
		t.Fatalf("SubjectiveEntityMemories = %d, want 2; result=%#v", result.SubjectiveEntityMemories, result)
	}
	if len(fake.savedEntityMemories) != 2 {
		t.Fatalf("savedEntityMemories = %d, want 2: %#v", len(fake.savedEntityMemories), fake.savedEntityMemories)
	}
	siwoo := fake.savedEntityMemories[0]
	asuna := fake.savedEntityMemories[1]
	if siwoo.OwnerEntityKey != "siwoo" || siwoo.OwnerEntityName != "Siwoo" || siwoo.OwnerEntityRole != "protagonist" || siwoo.OwnerVisibility != "player_known" {
		t.Fatalf("Siwoo subjective memory owner mismatch: %+v", siwoo)
	}
	if siwoo.SourceChatSessionID != "sess-subjective" || siwoo.SourceTurn != 2 || siwoo.Importance10 != 7 || siwoo.EmotionalWeight != 0.6 {
		t.Fatalf("Siwoo subjective memory source/score mismatch: %+v", siwoo)
	}
	if asuna.OwnerEntityKey != "asuna" || asuna.OwnerEntityName != "Asuna" || asuna.OwnerEntityRole != "npc" || asuna.OwnerVisibility != "owner_private" {
		t.Fatalf("Asuna subjective memory owner/private mismatch: %+v", asuna)
	}
	if !asuna.SecretGuard || asuna.TargetRevealPolicy != "owner_private_until_revealed" || asuna.Portability != "npc_private_recollection" {
		t.Fatalf("Asuna private policy mismatch: %+v", asuna)
	}
	if len(fake.savedMemories) != 0 || len(fake.savedKGTriples) != 0 || len(fake.savedEvidence) != 0 {
		t.Fatalf("subjective memories must not create canonical artifacts: mem=%d kg=%d evi=%d", len(fake.savedMemories), len(fake.savedKGTriples), len(fake.savedEvidence))
	}
}

func TestCompleteTurnSubjectiveEntityMemoryDuplicateSkipped(t *testing.T) {
	memoryText := "Siwoo remembers that the same hallway becomes dangerous when revisited."
	fake := &turnRecordingStore{
		returnEntityMemories: []store.ProtagonistEntityMemory{{
			OwnerEntityKey:      "siwoo",
			OwnerEntityName:     "Siwoo",
			OwnerEntityRole:     "protagonist",
			OwnerVisibility:     "player_known",
			SourceChatSessionID: "sess-subjective",
			SourceTurn:          3,
			MemoryText:          memoryText,
		}},
	}
	srv := NewServer(config.Default())
	srv.Store = fake
	srv.StoreOpenError = nil

	extraction := normalizeCriticExtraction(map[string]any{
		"subjective_entity_memories": []any{
			map[string]any{
				"owner_entity_key":  "siwoo",
				"owner_entity_name": "Siwoo",
				"memory_text":       memoryText,
				"source_turn_index": 3,
			},
		},
	})
	result := srv.saveCriticExtractionArtifacts(context.Background(), "sess-subjective", 3, extraction, "The same hallway becomes dangerous when revisited.", completeTurnEmbeddingConfig{}, time.Unix(1300, 0))
	if result.SubjectiveEntityMemories != 0 || len(fake.savedEntityMemories) != 0 {
		t.Fatalf("duplicate subjective memory should be skipped, result=%#v saved=%#v", result, fake.savedEntityMemories)
	}
	var foundDuplicate bool
	for _, skip := range result.SkipReasons {
		if skip["surface"] == "subjective_entity_memories" && skip["reason"] == "duplicate_source_turn_owner_memory" {
			foundDuplicate = true
			break
		}
	}
	if !foundDuplicate {
		t.Fatalf("missing duplicate skip reason: %#v", result.SkipReasons)
	}
}

func TestCompleteTurnSubjectiveEntityMemoryCanonicalizesOwnerAliases(t *testing.T) {
	fake := &turnRecordingStore{
		returnCharStates: []store.CharacterState{{CharacterName: "\uc774\uc2dc\uc6b0"}},
	}
	srv := NewServer(config.Default())
	srv.Store = fake
	srv.StoreOpenError = nil

	extraction := normalizeCriticExtraction(map[string]any{
		"subjective_entity_memories": []any{
			map[string]any{
				"owner_entity_key":  "siwoo",
				"owner_entity_name": "Siwoo",
				"memory_text":       "Siwoo privately remembers that Exit 2 felt unsafe.",
				"source_turn_index": 4,
			},
		},
	})
	result := srv.saveCriticExtractionArtifacts(context.Background(), "sess-subjective", 4, extraction, "Exit 2 felt unsafe.", completeTurnEmbeddingConfig{}, time.Unix(1400, 0))
	if result.SubjectiveEntityMemories != 1 || len(fake.savedEntityMemories) != 1 {
		t.Fatalf("expected one canonical subjective memory, result=%#v saved=%#v", result, fake.savedEntityMemories)
	}
	saved := fake.savedEntityMemories[0]
	if saved.OwnerEntityKey != "siwoo" || saved.PersonaEntityKey != "siwoo" {
		t.Fatalf("owner alias key was not canonicalized: %+v", saved)
	}
	if saved.OwnerEntityName != "\uc774\uc2dc\uc6b0" || saved.PersonaEntityName != "\uc774\uc2dc\uc6b0" {
		t.Fatalf("owner display name should use canonical character state: %+v", saved)
	}
	for _, needle := range []string{"owner_entity_alias:Siwoo", "entity_alias_canonicalized", "raw_owner_entity_name:Siwoo"} {
		if !strings.Contains(saved.TagsJSON, needle) {
			t.Fatalf("canonical alias tag %q missing from %s", needle, saved.TagsJSON)
		}
	}
}

func TestSaveCriticExtractionArtifactsNormalizesMultilingualEntityAliases(t *testing.T) {
	fake := &turnRecordingStore{
		returnCharStates: []store.CharacterState{{CharacterName: "Mina"}},
	}
	srv := NewServer(config.Default())
	srv.Store = fake
	srv.StoreOpenError = nil

	extraction := normalizeCriticExtraction(map[string]any{
		"turn_summary":      "Mina promised Rowan she would return.",
		"importance_score":  6,
		"evidence_excerpts": []any{"Mina promised Rowan she would return."},
		"entities": map[string]any{
			"characters": []any{map[string]any{
				"name":        "\uBBFC\uC544",
				"aliases":     []any{"Mina", "\uBBFC\uC544"},
				"description": "returning ally",
			}},
		},
		"kg_triples": []any{map[string]any{"subject": "\uBBFC\uC544", "predicate": "promised", "object": "Rowan"}},
	})

	result := srv.saveCriticExtractionArtifacts(context.Background(), "sess-multi", 4, extraction, "Mina promised Rowan she would return.", completeTurnEmbeddingConfig{}, time.Unix(100, 0))
	if result.Entities != 1 || result.KGTriples != 1 {
		t.Fatalf("expected entity and KG saves, result=%#v entities=%#v kg=%#v", result, fake.savedEntities, fake.savedKGTriples)
	}
	if len(fake.savedEntities) != 1 || fake.savedEntities[0].Name != "Mina" {
		t.Fatalf("expected Korean entity name to canonicalize to Mina, got %#v", fake.savedEntities)
	}
	if !strings.Contains(fake.savedEntities[0].AliasesJSON, "Mina") || !strings.Contains(fake.savedEntities[0].AliasesJSON, "\uBBFC\uC544") {
		t.Fatalf("expected aliases to preserve display variants, got %#v", fake.savedEntities[0])
	}
	if len(fake.savedKGTriples) != 1 || fake.savedKGTriples[0].Subject != "Mina" || fake.savedKGTriples[0].Object != "Rowan" {
		t.Fatalf("expected KG subject to use canonical display name, got %#v", fake.savedKGTriples)
	}
}

func TestSaveCriticExtractionArtifactsGuardsTransientDescriptorsAndParticipants(t *testing.T) {
	fake := &turnRecordingStore{}
	srv := NewServer(config.Default())
	srv.Store = fake
	srv.StoreOpenError = nil

	extraction := normalizeCriticExtraction(map[string]any{
		"turn_summary":      "Mina and Rowan made a plan while a red-haired girl watched.",
		"importance_score":  5,
		"evidence_excerpts": []any{"Mina and Rowan made a plan while a red-haired girl watched."},
		"state_deltas": map[string]any{
			"relationship_changes": []any{
				map[string]any{"pair": "Mina <-> Rowan", "detail": "They trust each other more."},
				map[string]any{"pair": "Mina <-> {{user}}", "detail": "The participant placeholder should not persist."},
			},
		},
		"character_deltas": []any{
			map[string]any{"name": "red-haired girl", "status": map[string]any{"emotion": "curious"}, "events": []any{map[string]any{"type": "sighting", "detail": "She watched from the door."}}},
			map[string]any{"name": "Mina", "events": []any{map[string]any{"type": "relationship_shift", "detail": "Mina trusted Rowan more."}}},
		},
		"pending_threads": []any{map[string]any{"thread_type": "promise", "title": "Mina asks Rowan about the plan", "owner": "{{user}}", "target": "Mina"}},
	})

	result := srv.saveCriticExtractionArtifacts(context.Background(), "sess-scrub", 5, extraction, "Mina and Rowan made a plan while a red-haired girl watched.", completeTurnEmbeddingConfig{}, time.Unix(200, 0))
	if result.CharacterStates != 1 || len(fake.savedCharacterStates) != 1 || fake.savedCharacterStates[0].CharacterName != "Mina" {
		t.Fatalf("expected only anchored Mina character state, result=%#v states=%#v", result, fake.savedCharacterStates)
	}
	if result.CharacterEvents != 1 || len(fake.savedCharacterEvents) != 1 || fake.savedCharacterEvents[0].CharacterName != "Mina" {
		t.Fatalf("expected only anchored Mina character event, result=%#v events=%#v", result, fake.savedCharacterEvents)
	}
	var stateDeltas string
	for _, item := range fake.savedActiveStates {
		if item.StateType == "state_deltas" {
			stateDeltas = item.Content
			break
		}
	}
	if !strings.Contains(stateDeltas, "Mina") || !strings.Contains(stateDeltas, "Rowan") || strings.Contains(stateDeltas, "{{user}}") {
		t.Fatalf("expected safe pair-only relationship delta and no participant placeholder, state_deltas=%s", stateDeltas)
	}
	if len(fake.savedPendingThreads) != 1 || fake.savedPendingThreads[0].Owner != "" || fake.savedPendingThreads[0].Target != "Mina" {
		t.Fatalf("expected pending thread participant owner scrubbed and safe target kept, got %#v", fake.savedPendingThreads)
	}
}

func TestSaveCriticExtractionArtifactsAppliesSoftPrune(t *testing.T) {
	fake := &turnRecordingStore{
		returnMemories: []store.Memory{
			{ID: 11, ChatSessionID: "sess-prune", TurnIndex: 1, SummaryJSON: `{"turn_summary":"obsolete clue should fade"}`, Importance: 0.7},
			{ID: 12, ChatSessionID: "sess-prune", TurnIndex: 1, SummaryJSON: `{"turn_summary":"fresh clue remains"}`, Importance: 0.8},
		},
	}
	srv := NewServer(config.Default())
	srv.Store = fake
	srv.StoreOpenError = nil

	extraction := normalizeCriticExtraction(map[string]any{
		"turn_summary":      "The obsolete clue was superseded.",
		"importance_score":  5,
		"evidence_excerpts": []any{"The obsolete clue was superseded."},
		"prune_targets":     []any{"obsolete clue"},
	})
	result := srv.saveCriticExtractionArtifacts(context.Background(), "sess-prune", 6, extraction, "The obsolete clue was superseded.", completeTurnEmbeddingConfig{}, time.Unix(300, 0))
	if result.Errors != 0 {
		t.Fatalf("soft prune should not error, result=%#v", result)
	}
	if got := fake.updatedImportance[11]; got < 0.499 || got > 0.501 {
		t.Fatalf("expected memory 11 importance to be softly pruned to 0.5, got %.2f updates=%#v", got, fake.updatedImportance)
	}
	if _, exists := fake.updatedImportance[12]; exists {
		t.Fatalf("memory 12 should not be pruned, updates=%#v", fake.updatedImportance)
	}
	foundAudit := false
	for _, item := range fake.savedAuditLogs {
		if item.EventType == "soft_prune" && item.Source == "critic" && strings.Contains(item.DetailsJSON, "obsolete clue") {
			foundAudit = true
			break
		}
	}
	if !foundAudit {
		t.Fatalf("expected soft_prune audit log, got %#v", fake.savedAuditLogs)
	}
	foundResolution := false
	for _, item := range fake.savedAuditLogs {
		if item.EventType == "supersession_resolution" && item.TargetType == "memory" && item.TargetID == 11 && strings.Contains(item.DetailsJSON, "stale_demote") {
			foundResolution = true
			break
		}
	}
	if !foundResolution {
		t.Fatalf("expected supersession_resolution stale_demote audit log, got %#v", fake.savedAuditLogs)
	}
}

func TestCompleteTurnMemorySemanticDedupSkipsInsertAndReinforcesExisting(t *testing.T) {
	fake := &turnRecordingStore{
		returnMemories: []store.Memory{
			{ID: 42, ChatSessionID: "sess-dedup", TurnIndex: 2, SummaryJSON: `{"turn_summary":"Mina promised Rowan she would return with the brass key."}`, Importance: 0.4},
		},
	}
	srv := NewServer(config.Default())
	srv.Store = fake
	srv.StoreOpenError = nil

	extraction := normalizeCriticExtraction(map[string]any{
		"turn_summary":      "Mina promised Rowan she would return with the brass key.",
		"importance_score":  8,
		"evidence_excerpts": []any{"Mina promised Rowan she would return with the brass key."},
	})
	result := srv.saveCriticExtractionArtifacts(context.Background(), "sess-dedup", 6, extraction, "Mina promised Rowan she would return with the brass key.", completeTurnEmbeddingConfig{}, time.Unix(310, 0))
	if result.Errors != 0 {
		t.Fatalf("semantic dedup should not error, result=%#v", result)
	}
	if len(fake.savedMemories) != 0 || result.Memories != 0 {
		t.Fatalf("expected duplicate memory insert to be skipped, result=%#v saved=%#v", result, fake.savedMemories)
	}
	if got := fake.updatedImportance[42]; got < 0.79 || got > 0.81 {
		t.Fatalf("expected existing memory importance reinforced to 0.8, got %.2f updates=%#v", got, fake.updatedImportance)
	}
	if !containsString(result.Warnings, "memory_semantic_dedup_merged") {
		t.Fatalf("expected memory_semantic_dedup_merged warning, got %#v", result.Warnings)
	}
	foundAudit := false
	for _, item := range fake.savedAuditLogs {
		if item.EventType == "memory_semantic_dedup" && item.Source == "critic" && strings.Contains(item.DetailsJSON, `"merged_memory_id":42`) {
			foundAudit = true
			break
		}
	}
	if !foundAudit {
		t.Fatalf("expected memory_semantic_dedup audit log, got %#v", fake.savedAuditLogs)
	}
}

func TestCompleteTurnMemorySameTurnRegenerationSkipsDuplicateInsert(t *testing.T) {
	fake := &turnRecordingStore{
		returnMemories: []store.Memory{
			{ID: 77, ChatSessionID: "sess-turn-dupe", TurnIndex: 5, SummaryJSON: `{"turn_summary":"Luna explains that apostles borrow divine power to defeat monsters."}`, Importance: 0.7},
		},
	}
	srv := NewServer(config.Default())
	srv.Store = fake
	srv.StoreOpenError = nil

	extraction := normalizeCriticExtraction(map[string]any{
		"turn_summary":      "Luna recounts the church doctrine about apostles and monsters.",
		"importance_score":  8,
		"evidence_excerpts": []any{"Apostles borrow divine power to defeat monsters."},
	})
	result := srv.saveCriticExtractionArtifacts(context.Background(), "sess-turn-dupe", 5, extraction, "Luna explains apostles and monsters.", completeTurnEmbeddingConfig{}, time.Unix(315, 0))
	if result.Errors != 0 {
		t.Fatalf("same-turn duplicate guard should not error, result=%#v", result)
	}
	if len(fake.savedMemories) != 0 || result.Memories != 0 {
		t.Fatalf("same-turn regeneration inserted duplicate memory, result=%#v saved=%#v", result, fake.savedMemories)
	}
	foundSkip := false
	for _, item := range result.SkipReasons {
		row := mapFromAny(item)
		if stringFromMap(row, "surface") == "memories" && stringFromMap(row, "reason") == "duplicate_source_turn_memory" {
			foundSkip = true
			break
		}
	}
	if !foundSkip {
		t.Fatalf("duplicate_source_turn_memory skip reason missing: %#v", result.SkipReasons)
	}
}

func TestSaveCriticExtractionArtifactsRespectsPrunePolicyOff(t *testing.T) {
	fake := &turnRecordingStore{
		returnMemories: []store.Memory{
			{ID: 11, ChatSessionID: "sess-prune", TurnIndex: 1, SummaryJSON: `{"turn_summary":"obsolete clue should fade"}`, Importance: 0.7},
		},
	}
	cfg := config.Default()
	cfg.PrunePolicy = "off"
	srv := NewServer(cfg)
	srv.Store = fake

	extraction := map[string]any{
		"turn_summary":      "The obsolete clue was superseded.",
		"importance_score":  5,
		"evidence_excerpts": []any{"The obsolete clue was superseded."},
		"prune_targets":     []any{"obsolete clue"},
	}
	result := srv.saveCriticExtractionArtifacts(context.Background(), "sess-prune", 6, extraction, "The obsolete clue was superseded.", completeTurnEmbeddingConfig{}, time.Unix(300, 0))
	if result.Errors != 0 {
		t.Fatalf("prune policy off should not error, result=%#v", result)
	}
	if len(fake.updatedImportance) != 0 {
		t.Fatalf("expected no importance update when prune policy is off, got %#v", fake.updatedImportance)
	}
	if !containsString(result.Warnings, "soft_prune_disabled") {
		t.Fatalf("expected soft_prune_disabled warning, got %#v", result.Warnings)
	}
}

func TestConfigUpdateRuntimeSettingsFeedCompleteTurnCritic(t *testing.T) {
	fake := &turnRecordingStore{}
	cfg := config.Default()
	cfg.StoreMode = config.StoreModeMariaDBAuthority
	srv := NewServer(cfg)
	srv.Store = fake
	srv.StoreOpenError = nil

	extractionBytes, _ := json.Marshal(map[string]any{
		"turn_summary":      "Runtime config critic extracted one durable memory.",
		"importance_score":  7,
		"evidence_excerpts": []any{"The runtime-configured critic is active."},
		"kg_triples":        []any{map[string]any{"subject": "Critic", "predicate": "extracts", "object": "Memory"}},
	})
	chatResp, _ := json.Marshal(map[string]any{
		"model":   "runtime-critic",
		"choices": []any{map[string]any{"message": map[string]any{"content": string(extractionBytes)}}},
	})
	oldClient := proxyHTTPClient
	var capturedUpstreamReq map[string]any
	proxyHTTPClient = &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		if err := json.NewDecoder(r.Body).Decode(&capturedUpstreamReq); err != nil {
			t.Fatalf("decode upstream critic request: %v", err)
		}
		return &http.Response{
			StatusCode: http.StatusOK,
			Header:     make(http.Header),
			Body:       io.NopCloser(bytes.NewReader(chatResp)),
		}, nil
	})}
	defer func() { proxyHTTPClient = oldClient }()

	mux := http.NewServeMux()
	srv.RegisterRoutes(mux)

	updateBody := `{"criticApiKey":"sk-runtime","criticEndpoint":"https://api.example.com/v1","criticModel":"runtime-critic","criticProvider":"openai","criticTimeout":45,"criticReasoningPreset":"glm","criticReasoningEffort":"enable","criticReasoningBudgetTokens":2048}`
	updateReq := httptest.NewRequest(http.MethodPost, "/config/update", bytes.NewReader([]byte(updateBody)))
	updateReq.Header.Set("Content-Type", "application/json")
	updateRec := httptest.NewRecorder()
	mux.ServeHTTP(updateRec, updateReq)
	if updateRec.Code != http.StatusOK {
		t.Fatalf("config/update status = %d, want 200: %s", updateRec.Code, updateRec.Body.String())
	}
	var updateResp map[string]any
	if err := json.Unmarshal(updateRec.Body.Bytes(), &updateResp); err != nil {
		t.Fatalf("decode config/update: %v", err)
	}
	runtimeTrace, ok := updateResp["runtime_config_trace"].(map[string]any)
	if !ok {
		t.Fatalf("runtime_config_trace missing from config/update: %+v", updateResp)
	}
	criticConfigTrace, ok := runtimeTrace["critic"].(map[string]any)
	if !ok || criticConfigTrace["configured"] != true {
		t.Fatalf("critic runtime config trace not configured: %+v", runtimeTrace["critic"])
	}
	if criticConfigTrace["reasoning_preset"] != "glm" || criticConfigTrace["reasoning_effort"] != "enable" || criticConfigTrace["reasoning_budget_tokens"] != float64(2048) || criticConfigTrace["glm_thinking_type"] != "enabled" {
		t.Fatalf("critic runtime config trace missing reasoning settings: %+v", criticConfigTrace)
	}

	turnBody := `{"chat_session_id":"sess-runtime-config","turn_index":1,"user_input":"critic should run","assistant_content":"The runtime-configured critic is active.","request_type":"model"}`
	req := httptest.NewRequest(http.MethodPost, "/complete-turn", bytes.NewReader([]byte(turnBody)))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("complete-turn status = %d, want 200: %s", rec.Code, rec.Body.String())
	}
	var resp map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if resp["critic_triggered"] != true {
		t.Fatalf("critic_triggered = %v, want true with runtime config", resp["critic_triggered"])
	}
	llmTrace, ok := resp["llm_config_trace"].(map[string]any)
	if !ok {
		t.Fatalf("llm_config_trace missing from complete-turn response: %+v", resp)
	}
	criticTrace, ok := llmTrace["critic"].(map[string]any)
	if !ok || criticTrace["configured"] != true {
		t.Fatalf("critic complete-turn trace not configured: %+v", llmTrace["critic"])
	}
	if criticTrace["reasoning_preset"] != "glm" || criticTrace["reasoning_effort"] != "enable" || criticTrace["reasoning_budget_tokens"] != float64(2048) || criticTrace["glm_thinking_type"] != "enabled" {
		t.Fatalf("critic complete-turn trace missing reasoning settings: %+v", criticTrace)
	}
	thinking, _ := capturedUpstreamReq["thinking"].(map[string]any)
	if thinking["type"] != "enabled" {
		t.Fatalf("upstream critic request missing GLM thinking payload: %+v", capturedUpstreamReq)
	}
	if got, _ := resp["derived_artifacts_saved"].(float64); got < 3 {
		t.Fatalf("derived_artifacts_saved = %v, want at least memory/evidence/KG", resp["derived_artifacts_saved"])
	}
	if len(fake.savedMemories) != 1 || len(fake.savedEvidence) != 1 || len(fake.savedKGTriples) != 1 {
		t.Fatalf("expected runtime-config critic artifacts, memories=%d evidence=%d kg=%d", len(fake.savedMemories), len(fake.savedEvidence), len(fake.savedKGTriples))
	}
}

func TestFocusedRecallFallbackAddsGroundedSummaryAndEvidence(t *testing.T) {
	extraction := enrichNormalizedCriticExtractionForFocusedRecall(normalizeCriticExtraction(map[string]any{}),
		"Mina promises Rowan she will return with the brass key.",
		"Rowan accepts the promise and marks the cellar route as the only safe path.",
		12,
	)
	if strings.TrimSpace(extractionStringFromAny(extraction["turn_summary"])) == "" {
		t.Fatalf("turn_summary was not backfilled: %+v", extraction)
	}
	excerpts := stringsFromAny(extraction["evidence_excerpts"])
	if len(excerpts) == 0 {
		t.Fatalf("evidence_excerpts were not backfilled: %+v", extraction)
	}
	if !strings.Contains(strings.Join(excerpts, "\n"), "Mina promises Rowan") {
		t.Fatalf("fallback evidence is not grounded in latest turn: %#v", excerpts)
	}
}

func TestSaveCriticExtractionArtifactsSkipsDuplicateEvidenceAndKGForSameTurn(t *testing.T) {
	fake := &turnRecordingStore{
		returnEvidence: []store.DirectEvidence{{
			ChatSessionID:   "sess-dupe-artifacts",
			EvidenceText:    "Mina promises Rowan she will return.",
			SourceTurnStart: 7,
			SourceTurnEnd:   7,
			TurnAnchor:      7,
		}},
		returnKGTriples: []store.KGTriple{{
			ChatSessionID: "sess-dupe-artifacts",
			Subject:       "Mina",
			Predicate:     "promises",
			Object:        "Rowan",
			SourceTurn:    7,
			ValidFrom:     7,
		}},
	}
	srv := NewServer(config.Default())
	srv.Store = fake
	extraction := normalizeCriticExtraction(map[string]any{
		"turn_summary":      "Mina promises Rowan she will return.",
		"importance_score":  6,
		"evidence_excerpts": []any{"Mina promises Rowan she will return."},
		"kg_triples":        []any{map[string]any{"subject": "Mina", "predicate": "promises", "object": "Rowan", "valid_from": 7}},
	})
	result := srv.saveCriticExtractionArtifacts(context.Background(), "sess-dupe-artifacts", 7, extraction, "Mina promises Rowan she will return.", completeTurnEmbeddingConfig{}, time.Unix(700, 0))
	if result.Evidence != 0 || result.KGTriples != 0 {
		t.Fatalf("duplicate artifacts were saved, evidence=%d kg=%d", result.Evidence, result.KGTriples)
	}
	if len(fake.savedEvidence) != 0 || len(fake.savedKGTriples) != 0 {
		t.Fatalf("duplicate save calls occurred, evidence=%d kg=%d", len(fake.savedEvidence), len(fake.savedKGTriples))
	}
}

func TestSaveCriticExtractionArtifactsSkipsFuzzyEvidenceAndActiveKGDuplicates(t *testing.T) {
	fake := &turnRecordingStore{
		returnEvidence: []store.DirectEvidence{{
			ChatSessionID:   "sess-dupe-artifacts-fuzzy",
			EvidenceText:    "Mina promises Rowan she will return before dawn.",
			SourceTurnStart: 5,
			SourceTurnEnd:   9,
			TurnAnchor:      7,
		}},
		returnKGTriples: []store.KGTriple{{
			ChatSessionID: "sess-dupe-artifacts-fuzzy",
			Subject:       "Mina",
			Predicate:     "protects",
			Object:        "Rowan",
			SourceTurn:    3,
			ValidFrom:     3,
			ValidTo:       0,
		}},
	}
	srv := NewServer(config.Default())
	srv.Store = fake
	extraction := normalizeCriticExtraction(map[string]any{
		"turn_summary":      "Mina repeats her promise to return and protect Rowan.",
		"importance_score":  6,
		"evidence_excerpts": []any{"Mina promises Rowan she will return."},
		"kg_triples":        []any{map[string]any{"subject": "Mina", "predicate": "protects", "object": "Rowan", "valid_from": 7}},
	})
	result := srv.saveCriticExtractionArtifacts(context.Background(), "sess-dupe-artifacts-fuzzy", 7, extraction, "Mina promises Rowan she will return. Mina protects Rowan.", completeTurnEmbeddingConfig{}, time.Unix(701, 0))
	if result.Evidence != 0 || result.KGTriples != 0 {
		t.Fatalf("fuzzy/active duplicate artifacts were saved, evidence=%d kg=%d skip=%#v", result.Evidence, result.KGTriples, result.SkipReasons)
	}
	if len(fake.savedEvidence) != 0 || len(fake.savedKGTriples) != 0 {
		t.Fatalf("duplicate save calls occurred, evidence=%d kg=%d", len(fake.savedEvidence), len(fake.savedKGTriples))
	}
}

func TestCriticJudgedWorldRulesPersistKoreanInstitutionalRules(t *testing.T) {
	fake := &turnRecordingStore{}
	srv := NewServer(config.Default())
	srv.Store = fake
	extraction := normalizeCriticExtraction(map[string]any{
		"turn_summary":      "학생회는 기숙사 통금 시간을 밤 10시로 확정했고, 지각자는 다음 날 청소 당번을 맡아야 한다.",
		"importance_score":  7,
		"evidence_excerpts": []any{"학생회는 기숙사 통금 시간을 밤 10시로 확정했고, 지각자는 다음 날 청소 당번을 맡아야 한다."},
	})
	extraction["world_rules"] = []any{map[string]any{
		"scope":      "location",
		"scope_name": "dormitory",
		"category":   "institution",
		"key":        "dormitory_curfew_and_cleanup_penalty",
		"value":      "Dormitory curfew is 10 PM, and late students must clean the common room the next day.",
		"confidence": 0.86,
	}}
	result := srv.saveCriticExtractionArtifacts(context.Background(), "sess-kr-world", 9, extraction, "학생회는 기숙사 통금 시간을 밤 10시로 확정했고, 지각자는 다음 날 청소 당번을 맡아야 한다.", completeTurnEmbeddingConfig{}, time.Unix(900, 0))
	if result.WorldRules == 0 || len(fake.savedWorldRules) == 0 {
		t.Fatalf("expected critic-judged institutional world rule to persist, result=%+v", result)
	}
}

func TestExplorerRegenerateMemoryUsesCompleteTurnArtifactPipeline(t *testing.T) {
	fake := &turnRecordingStore{
		returnChatLogs: []store.ChatLog{
			{ChatSessionID: "sess-explorer-regen", TurnIndex: 4, Role: "user", Content: "Mina asks Rowan to guard the cellar route."},
			{ChatSessionID: "sess-explorer-regen", TurnIndex: 4, Role: "assistant", Content: "Rowan accepts and confirms the cellar route is the only safe path."},
		},
	}
	cfg := config.Default()
	cfg.StoreMode = config.StoreModeMariaDBAuthority
	srv := NewServer(cfg)
	srv.Store = fake
	srv.StoreOpenError = nil

	extractionBytes, _ := json.Marshal(map[string]any{
		"turn_summary":      "Rowan accepts Mina's request and confirms the cellar route.",
		"importance_score":  7,
		"evidence_excerpts": []any{"Rowan accepts and confirms the cellar route is the only safe path."},
		"kg_triples":        []any{map[string]any{"subject": "Rowan", "predicate": "guards", "object": "cellar route"}},
		"world_rules":       []any{map[string]any{"scope": "location", "scope_name": "cellar", "category": "access", "key": "cellar_route_only_safe_path", "value": "The cellar route is the only safe path."}},
	})
	chatResp, _ := json.Marshal(map[string]any{
		"model":   "critic-test",
		"choices": []any{map[string]any{"message": map[string]any{"content": string(extractionBytes)}}},
	})
	oldClient := proxyHTTPClient
	proxyHTTPClient = &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		return &http.Response{StatusCode: http.StatusOK, Header: make(http.Header), Body: io.NopCloser(bytes.NewReader(chatResp))}, nil
	})}
	defer func() { proxyHTTPClient = oldClient }()

	mux := http.NewServeMux()
	srv.RegisterRoutes(mux)
	body := `{"chat_session_id":"sess-explorer-regen","turn_index":4,"client_meta":{"critic":{"api_key":"sk-test","endpoint":"https://api.example.com/v1","model":"critic-test","provider":"openai"}}}`
	req := httptest.NewRequest(http.MethodPost, "/explorer/memories/regenerate", strings.NewReader(body))
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
		t.Fatalf("status = %v, response: %+v", resp["status"], resp)
	}
	if len(fake.savedMemories) != 1 || len(fake.savedEvidence) != 1 || len(fake.savedKGTriples) != 1 || len(fake.savedWorldRules) != 1 {
		t.Fatalf("regenerate did not save full artifact set: memories=%d evidence=%d kg=%d world=%d", len(fake.savedMemories), len(fake.savedEvidence), len(fake.savedKGTriples), len(fake.savedWorldRules))
	}
}

func TestCompleteTurnCriticLedgerWiringBehindFeatureFlag(t *testing.T) {
	run := func(t *testing.T, enabled bool) (string, map[string]any) {
		t.Helper()
		fake := &turnRecordingStore{
			returnMemories: []store.Memory{
				{ID: 77, ChatSessionID: "sess-ledger-wiring", TurnIndex: 3, SummaryJSON: `{"summary":"Mina already trusts Rowan after the bridge scene."}`, Importance: 8.5},
			},
			returnEvidence: []store.DirectEvidence{
				{ID: 88, ChatSessionID: "sess-ledger-wiring", EvidenceText: "<thinking>private draft</thinking>Rowan protected Mina at the bridge.", SourceTurnStart: 3, SourceTurnEnd: 3, CaptureVerification: "verified"},
			},
		}
		cfg := config.Default()
		cfg.StoreMode = config.StoreModeMariaDBAuthority
		cfg.CriticLedgerEnabled = enabled
		srv := NewServer(cfg)
		srv.Store = fake
		srv.StoreOpenError = nil

		extractionBytes, _ := json.Marshal(map[string]any{
			"turn_summary":      "Latest turn produced a small durable memory.",
			"importance_score":  5,
			"evidence_excerpts": []any{"Latest turn produced a small durable memory."},
		})
		chatResp, _ := json.Marshal(map[string]any{
			"model":   "critic-ledger-test",
			"choices": []any{map[string]any{"message": map[string]any{"content": string(extractionBytes)}}},
		})

		oldClient := proxyHTTPClient
		var capturedPrompt string
		proxyHTTPClient = &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
			var captured map[string]any
			if err := json.NewDecoder(r.Body).Decode(&captured); err != nil {
				t.Fatalf("decode upstream critic request: %v", err)
			}
			messages, _ := captured["messages"].([]any)
			if len(messages) < 2 {
				t.Fatalf("upstream messages missing: %+v", captured)
			}
			userMessage, _ := messages[1].(map[string]any)
			capturedPrompt, _ = userMessage["content"].(string)
			return &http.Response{
				StatusCode: http.StatusOK,
				Header:     make(http.Header),
				Body:       io.NopCloser(bytes.NewReader(chatResp)),
			}, nil
		})}
		defer func() { proxyHTTPClient = oldClient }()

		mux := http.NewServeMux()
		srv.RegisterRoutes(mux)

		body := `{"chat_session_id":"sess-ledger-wiring","turn_index":6,"user_input":"Mina asks what Rowan remembers.","assistant_content":"Rowan answers with a new visible reply.","request_type":"model","output_language_override":{"language":"ko"},"client_meta":{"critic":{"api_key":"sk-test","endpoint":"https://api.example.com/v1","model":"critic-ledger-test","provider":"openai"}}}`
		req := httptest.NewRequest(http.MethodPost, "/complete-turn", bytes.NewReader([]byte(body)))
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()
		mux.ServeHTTP(rec, req)
		if rec.Code != http.StatusOK {
			t.Fatalf("complete-turn status = %d, want 200: %s", rec.Code, rec.Body.String())
		}
		var resp map[string]any
		if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
			t.Fatalf("decode response: %v", err)
		}
		traceHandoff, _ := resp["trace_handoff"].(map[string]any)
		criticTrace, _ := traceHandoff["critic_trace"].(map[string]any)
		return capturedPrompt, criticTrace
	}

	disabledPrompt, disabledTrace := run(t, false)
	if !strings.Contains(disabledPrompt, "<Critic_Archive_Ledger_JSON>") || !strings.Contains(disabledPrompt, "null") {
		t.Fatalf("disabled prompt should carry a null ledger section, prompt=%s", disabledPrompt)
	}
	if strings.Contains(disabledPrompt, "Mina already trusts Rowan") {
		t.Fatalf("disabled prompt leaked archive ledger content: %s", disabledPrompt)
	}
	if strings.Contains(disabledPrompt, "private draft") || strings.Contains(disabledPrompt, "<thinking>") {
		t.Fatalf("disabled prompt leaked private reasoning: %s", disabledPrompt)
	}
	disabledLedgerTrace, _ := disabledTrace["critic_archive_ledger"].(map[string]any)
	if disabledLedgerTrace["enabled"] != false || disabledLedgerTrace["included"] != false {
		t.Fatalf("disabled ledger trace mismatch: %+v", disabledLedgerTrace)
	}

	enabledPrompt, enabledTrace := run(t, true)
	if !strings.Contains(enabledPrompt, "<Critic_Archive_Ledger_JSON>") {
		t.Fatalf("enabled prompt missing ledger section: %s", enabledPrompt)
	}
	for _, expected := range []string{"critic_archive_ledger.v1", "Mina already trusts Rowan after the bridge scene.", "Rowan protected Mina at the bridge.", "raw_archive_dump_blocked"} {
		if !strings.Contains(enabledPrompt, expected) {
			t.Fatalf("enabled prompt missing %q: %s", expected, enabledPrompt)
		}
	}
	if strings.Contains(enabledPrompt, "private draft") || strings.Contains(enabledPrompt, "<thinking>") {
		t.Fatalf("enabled prompt leaked private reasoning: %s", enabledPrompt)
	}
	enabledLedgerTrace, _ := enabledTrace["critic_archive_ledger"].(map[string]any)
	if enabledLedgerTrace["enabled"] != true || enabledLedgerTrace["included"] != true || enabledLedgerTrace["llm_call_attempted"] != false || enabledLedgerTrace["write_attempted"] != false {
		t.Fatalf("enabled ledger trace mismatch: %+v", enabledLedgerTrace)
	}
	language, _ := enabledLedgerTrace["language"].(map[string]any)
	if language["assistant_final_language"] != "ko" {
		t.Fatalf("enabled ledger language mismatch: %+v", language)
	}
}

func TestImportHypamemoryRequiresCriticConfig(t *testing.T) {
	fake := &turnRecordingStore{}
	cfg := config.Default()
	cfg.StoreMode = config.StoreModeMariaDBAuthority
	srv := NewServer(cfg)
	srv.Store = fake
	srv.StoreOpenError = nil

	mux := http.NewServeMux()
	srv.RegisterRoutes(mux)

	body := `{"chat_session_id":"sess-hypa-missing","summaries":[{"text":"Chloe remembers the rooftop promise.","is_important":true}]}`
	req := httptest.NewRequest(http.MethodPost, "/import/hypamemory", bytes.NewReader([]byte(body)))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("import/hypamemory status = %d, want 200 fail-closed response: %s", rec.Code, rec.Body.String())
	}
	var resp map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode import/hypamemory response: %v", err)
	}
	if resp["status"] != "error" || resp["code"] != "critic_config_missing" {
		t.Fatalf("expected critic_config_missing without runtime critic config, got %+v", resp)
	}
	if len(fake.savedMemories) != 0 || len(fake.savedEvidence) != 0 || len(fake.savedKGTriples) != 0 || len(fake.savedAuditLogs) != 0 {
		t.Fatalf("HypaMemory import must not fake writes without critic config, memories=%d evidence=%d kg=%d audit=%d", len(fake.savedMemories), len(fake.savedEvidence), len(fake.savedKGTriples), len(fake.savedAuditLogs))
	}
}

func TestImportHypamemoryWithRuntimeCriticSavesArtifacts(t *testing.T) {
	fake := &turnRecordingStore{}
	vec := &turnRecordingVectorStore{}
	cfg := config.Default()
	cfg.StoreMode = config.StoreModeMariaDBAuthority
	cfg.ChromaEndpoint = "http://127.0.0.1:8000"
	srv := NewServer(cfg)
	srv.Store = fake
	srv.StoreOpenError = nil
	srv.Vector = vec

	extractionBytes, _ := json.Marshal(map[string]any{
		"turn_summary":      "Imported HypaMemory says Chloe trusts Hero after the rooftop promise.",
		"importance_score":  8,
		"evidence_excerpts": []any{"Chloe trusts Hero after the rooftop promise."},
		"kg_triples":        []any{map[string]any{"subject": "Chloe", "predicate": "trusts", "object": "Hero"}},
	})
	chatResp, _ := json.Marshal(map[string]any{
		"model":   "runtime-critic",
		"choices": []any{map[string]any{"message": map[string]any{"content": string(extractionBytes)}}},
	})
	oldClient := proxyHTTPClient
	proxyHTTPClient = &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		if strings.Contains(r.URL.Path, "/embeddings") {
			return &http.Response{
				StatusCode: http.StatusOK,
				Header:     make(http.Header),
				Body:       io.NopCloser(strings.NewReader(`{"model":"embed-model","data":[{"embedding":[0.1,0.2,0.3]}]}`)),
			}, nil
		}
		return &http.Response{
			StatusCode: http.StatusOK,
			Header:     make(http.Header),
			Body:       io.NopCloser(bytes.NewReader(chatResp)),
		}, nil
	})}
	defer func() { proxyHTTPClient = oldClient }()

	mux := http.NewServeMux()
	srv.RegisterRoutes(mux)

	updateBody := `{"criticApiKey":"sk-runtime","criticEndpoint":"https://api.example.com/v1","criticModel":"runtime-critic","criticProvider":"openai","embeddingApiKey":"sk-embed","embeddingEndpoint":"https://api.example.com/v1/embeddings","embeddingModel":"embed-model","embeddingProvider":"openai"}`
	updateReq := httptest.NewRequest(http.MethodPost, "/config/update", bytes.NewReader([]byte(updateBody)))
	updateReq.Header.Set("Content-Type", "application/json")
	updateRec := httptest.NewRecorder()
	mux.ServeHTTP(updateRec, updateReq)
	if updateRec.Code != http.StatusOK {
		t.Fatalf("config/update status = %d, want 200: %s", updateRec.Code, updateRec.Body.String())
	}

	body := `{"chat_session_id":"sess-hypa-runtime","summaries":[{"text":"Chloe trusts Hero after the rooftop promise.","is_important":true,"category":"relationship","tags":["trust","rooftop"],"source_turn_index":-42}]}`
	req := httptest.NewRequest(http.MethodPost, "/import/hypamemory", bytes.NewReader([]byte(body)))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("import/hypamemory status = %d, want 200: %s", rec.Code, rec.Body.String())
	}
	var resp map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode import/hypamemory response: %v", err)
	}
	if resp["status"] != "ok" || resp["succeeded"] != float64(1) || resp["failed"] != float64(0) {
		t.Fatalf("expected successful HypaMemory import, got %+v", resp)
	}
	if len(fake.savedMemories) != 1 || len(fake.savedEvidence) != 1 || len(fake.savedKGTriples) != 1 {
		t.Fatalf("expected HypaMemory Critic artifacts, memories=%d evidence=%d kg=%d", len(fake.savedMemories), len(fake.savedEvidence), len(fake.savedKGTriples))
	}
	if fake.savedMemories[0].TurnIndex != -42 || fake.savedEvidence[0].SourceTurnStart != -42 || fake.savedKGTriples[0].SourceTurn != -42 {
		t.Fatalf("expected HypaMemory import artifacts to use negative import turn index, memory=%d evidence=%d kg=%d", fake.savedMemories[0].TurnIndex, fake.savedEvidence[0].SourceTurnStart, fake.savedKGTriples[0].SourceTurn)
	}
	if len(vec.docs) != 2 {
		t.Fatalf("expected memory and evidence vector upserts for imported memory, got %d", len(vec.docs))
	}
	if !hasAuditEvent(fake.savedAuditLogs, "hypamemory_import") {
		t.Fatalf("expected hypamemory_import audit log, got %#v", fake.savedAuditLogs)
	}
}

func TestImportHypamemoryScoringPassRaisesLowCriticImportance(t *testing.T) {
	fake := &turnRecordingStore{}
	cfg := config.Default()
	cfg.StoreMode = config.StoreModeMariaDBAuthority
	srv := NewServer(cfg)
	srv.Store = fake
	srv.StoreOpenError = nil

	scoringBytes, _ := json.Marshal(map[string]any{
		"importance_10":               8.4,
		"retrieval_priority":          0.91,
		"continuity_weight":           0.88,
		"dialogue_or_sensory_density": 0.42,
		"memory_kind":                 "injury_continuity",
		"entity_relevance":            []any{"Hero", "Chloe"},
		"time_anchor_quality":         "summary_level",
		"keep_reason":                 "The imported memory changes how future danger should be interpreted.",
	})
	extractionBytes, _ := json.Marshal(map[string]any{
		"turn_summary":      "Imported HypaMemory says Hero was shot before and Chloe took him to hospital.",
		"importance_score":  2,
		"evidence_excerpts": []any{"Hero was shot before and Chloe took him to hospital."},
		"kg_triples":        []any{map[string]any{"subject": "Hero", "predicate": "was_taken_to", "object": "hospital"}},
	})
	scoringResp, _ := json.Marshal(map[string]any{
		"model":   "runtime-critic",
		"choices": []any{map[string]any{"message": map[string]any{"content": string(scoringBytes)}}},
	})
	criticResp, _ := json.Marshal(map[string]any{
		"model":   "runtime-critic",
		"choices": []any{map[string]any{"message": map[string]any{"content": string(extractionBytes)}}},
	})
	oldClient := proxyHTTPClient
	chatCalls := 0
	proxyHTTPClient = &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		if strings.Contains(r.URL.Path, "/embeddings") {
			return &http.Response{
				StatusCode: http.StatusOK,
				Header:     make(http.Header),
				Body:       io.NopCloser(strings.NewReader(`{"model":"embed-model","data":[{"embedding":[0.1,0.2,0.3]}]}`)),
			}, nil
		}
		chatCalls++
		body := scoringResp
		if chatCalls >= 2 {
			body = criticResp
		}
		return &http.Response{
			StatusCode: http.StatusOK,
			Header:     make(http.Header),
			Body:       io.NopCloser(bytes.NewReader(body)),
		}, nil
	})}
	defer func() { proxyHTTPClient = oldClient }()

	mux := http.NewServeMux()
	srv.RegisterRoutes(mux)

	updateBody := `{"criticApiKey":"sk-runtime","criticEndpoint":"https://api.example.com/v1","criticModel":"runtime-critic","criticProvider":"openai","embeddingApiKey":"sk-embed","embeddingEndpoint":"https://api.example.com/v1/embeddings","embeddingModel":"embed-model","embeddingProvider":"openai"}`
	updateReq := httptest.NewRequest(http.MethodPost, "/config/update", bytes.NewReader([]byte(updateBody)))
	updateReq.Header.Set("Content-Type", "application/json")
	updateRec := httptest.NewRecorder()
	mux.ServeHTTP(updateRec, updateReq)
	if updateRec.Code != http.StatusOK {
		t.Fatalf("config/update status = %d, want 200: %s", updateRec.Code, updateRec.Body.String())
	}

	body := `{"chat_session_id":"sess-hypa-score","summaries":[{"text":"Hero was shot before and Chloe took him to hospital.","is_important":false,"category":"injury","source_turn_index":12}]}`
	req := httptest.NewRequest(http.MethodPost, "/import/hypamemory", bytes.NewReader([]byte(body)))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("import/hypamemory status = %d, want 200: %s", rec.Code, rec.Body.String())
	}
	var resp map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode import/hypamemory response: %v", err)
	}
	if resp["status"] != "ok" || resp["scoring_policy"] != hypaMemoryImportScoringPolicyVersion {
		t.Fatalf("expected scored HypaMemory import response, got %+v", resp)
	}
	if chatCalls != 2 {
		t.Fatalf("expected scoring + critic chat calls, got %d", chatCalls)
	}
	if len(fake.savedMemories) != 1 {
		t.Fatalf("expected one memory, got %d", len(fake.savedMemories))
	}
	if fake.savedMemories[0].TurnIndex != -12 {
		t.Fatalf("expected positive source_turn_index to normalize to -12, got %d", fake.savedMemories[0].TurnIndex)
	}
	if fake.savedMemories[0].Importance < 0.84 {
		t.Fatalf("expected scoring pass to raise low critic importance to >=0.84, got %.3f", fake.savedMemories[0].Importance)
	}
	saved := map[string]any{}
	if err := json.Unmarshal([]byte(fake.savedMemories[0].SummaryJSON), &saved); err != nil {
		t.Fatalf("decode saved summary: %v", err)
	}
	if saved["hypamemory_import_score"] == nil || saved["importance_score"] != float64(8.4) {
		t.Fatalf("saved extraction missing scoring payload or raised importance: %+v", saved)
	}
}

func TestCompleteTurnCriticProviderFailurePreservesRawTurnAndReportsReason(t *testing.T) {
	fake := &turnRecordingStore{}
	cfg := config.Default()
	cfg.StoreMode = config.StoreModeMariaDBAuthority
	srv := NewServer(cfg)
	srv.Store = fake
	srv.StoreOpenError = nil

	const apiKey = "sk-critic-fail"
	oldClient := proxyHTTPClient
	proxyHTTPClient = &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		return &http.Response{
			StatusCode: http.StatusUnauthorized,
			Header:     make(http.Header),
			Body:       io.NopCloser(strings.NewReader(`{"error":{"message":"bad key sk-critic-fail"}}`)),
		}, nil
	})}
	defer func() { proxyHTTPClient = oldClient }()

	mux := http.NewServeMux()
	srv.RegisterRoutes(mux)
	body := map[string]any{
		"chat_session_id":   "sess-critic-provider-fail",
		"turn_index":        1,
		"user_input":        "Mina asks Rowan to remember the blue key.",
		"assistant_content": "Rowan promises to keep the blue key safe.",
		"client_meta": map[string]any{"critic": map[string]any{
			"api_key":  apiKey,
			"endpoint": "https://api.example.com/v1",
			"model":    "critic-model",
			"provider": "openai",
		}},
	}
	raw, _ := json.Marshal(body)
	req := httptest.NewRequest(http.MethodPost, "/complete-turn", bytes.NewReader(raw))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("complete-turn status = %d, want 200 fail-open persistence: %s", rec.Code, rec.Body.String())
	}
	if strings.Contains(rec.Body.String(), apiKey) {
		t.Fatalf("complete-turn response leaked API key: %s", rec.Body.String())
	}

	var resp map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if resp["save_ok"] != true || resp["critic_triggered"] != false {
		t.Fatalf("expected raw save with critic failure, got %+v", resp)
	}
	if resp["chat_logs_saved"] != float64(2) || resp["derived_artifacts_saved"] != float64(0) {
		t.Fatalf("unexpected save counters: %+v", resp)
	}
	if len(fake.savedChatLogs) != 2 || len(fake.savedMemories) != 0 || len(fake.savedEvidence) != 0 || len(fake.savedKGTriples) != 0 {
		t.Fatalf("critic failure should preserve raw turn only, logs=%d memories=%d evidence=%d kg=%d", len(fake.savedChatLogs), len(fake.savedMemories), len(fake.savedEvidence), len(fake.savedKGTriples))
	}
	reasons, _ := resp["fail_reasons"].([]any)
	found := false
	for _, item := range reasons {
		if strings.Contains(fmt.Sprint(item), "critic_extract_failed") {
			found = true
		}
	}
	if !found {
		t.Fatalf("expected critic_extract_failed reason, got %+v", resp["fail_reasons"])
	}
	if !hasAuditEvent(fake.savedAuditLogs, "critic_extract_failed") {
		t.Fatalf("expected critic_extract_failed audit log, got %#v", fake.savedAuditLogs)
	}
}

func TestCompleteTurnCriticProviderFailureRetriesWithRedactedInput(t *testing.T) {
	fake := &turnRecordingStore{}
	cfg := config.Default()
	cfg.StoreMode = config.StoreModeMariaDBAuthority
	srv := NewServer(cfg)
	srv.Store = fake
	srv.StoreOpenError = nil

	extractionBytes, _ := json.Marshal(map[string]any{
		"turn_summary":      "Mina and Rowan crossed an intimate threshold while Rowan stayed reassuring.",
		"importance_score":  7,
		"evidence_excerpts": []any{"Rowan stayed reassuring."},
		"kg_triples":        []any{map[string]any{"subject": "Rowan", "predicate": "reassures", "object": "Mina"}},
	})
	chatResp, _ := json.Marshal(map[string]any{
		"model":   "critic-model",
		"choices": []any{map[string]any{"message": map[string]any{"content": string(extractionBytes)}}},
	})
	oldClient := proxyHTTPClient
	callCount := 0
	secondPrompt := ""
	proxyHTTPClient = &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		callCount++
		var upstreamReq map[string]any
		if err := json.NewDecoder(r.Body).Decode(&upstreamReq); err != nil {
			t.Fatalf("decode upstream request: %v", err)
		}
		if callCount == 1 {
			return &http.Response{
				StatusCode: http.StatusInternalServerError,
				Header:     make(http.Header),
				Body:       io.NopCloser(strings.NewReader(`{"error":{"message":"upstream rejected"}}`)),
			}, nil
		}
		msgs, _ := upstreamReq["messages"].([]any)
		if len(msgs) >= 2 {
			if msg, _ := msgs[1].(map[string]any); msg != nil {
				secondPrompt = fmt.Sprint(msg["content"])
			}
		}
		return &http.Response{
			StatusCode: http.StatusOK,
			Header:     make(http.Header),
			Body:       io.NopCloser(bytes.NewReader(chatResp)),
		}, nil
	})}
	defer func() { proxyHTTPClient = oldClient }()

	mux := http.NewServeMux()
	srv.RegisterRoutes(mux)
	body := map[string]any{
		"chat_session_id":   "sess-critic-redacted-retry",
		"turn_index":        1,
		"user_input":        "Mina asks Rowan to be gentle.",
		"assistant_content": "Rowan stayed reassuring. The intimate scene involved penetration.",
		"client_meta": map[string]any{"critic": map[string]any{
			"api_key":  "sk-redacted-retry",
			"endpoint": "https://api.example.com/v1",
			"model":    "critic-model",
			"provider": "openai",
		}},
	}
	raw, _ := json.Marshal(body)
	req := httptest.NewRequest(http.MethodPost, "/complete-turn", bytes.NewReader(raw))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("complete-turn status = %d, want 200: %s", rec.Code, rec.Body.String())
	}
	var resp map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if callCount != 2 {
		t.Fatalf("critic call count = %d, want 2", callCount)
	}
	if strings.Contains(strings.ToLower(secondPrompt), "penetration") || !strings.Contains(secondPrompt, "redacted for critic retry") {
		t.Fatalf("second critic prompt was not redacted as expected: %s", secondPrompt)
	}
	if resp["critic_triggered"] != true {
		t.Fatalf("critic_triggered = %v, want true after redacted retry: %+v", resp["critic_triggered"], resp)
	}
	if resp["derived_artifacts_saved"].(float64) < 3 {
		t.Fatalf("derived_artifacts_saved = %v, want memory/evidence/KG after retry: %+v", resp["derived_artifacts_saved"], resp)
	}
	trace, _ := resp["trace_handoff"].(map[string]any)
	criticTrace, _ := trace["critic_trace"].(map[string]any)
	if _, ok := criticTrace["provider_retry"].(map[string]any); !ok {
		t.Fatalf("provider_retry trace missing: %+v", criticTrace)
	}
}

func TestCompleteTurnEmbeddingProviderFailureReportsWarning(t *testing.T) {
	fake := &turnRecordingStore{}
	cfg := config.Default()
	cfg.StoreMode = config.StoreModeMariaDBAuthority
	cfg.ChromaEndpoint = "http://127.0.0.1:8000"
	cfg.Readiness.ChromaConfigured = true
	srv := NewServer(cfg)
	srv.Store = fake
	srv.StoreOpenError = nil
	srv.Vector = &turnRecordingVectorStore{}

	oldClient := proxyHTTPClient
	proxyHTTPClient = &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		if strings.Contains(r.URL.Path, "/embeddings") {
			return &http.Response{
				StatusCode: http.StatusUnauthorized,
				Header:     make(http.Header),
				Body:       io.NopCloser(strings.NewReader(`{"error":{"message":"embedding unauthorized"}}`)),
			}, nil
		}
		return &http.Response{
			StatusCode: http.StatusOK,
			Header:     make(http.Header),
			Body: io.NopCloser(strings.NewReader(`{
				"model":"critic-model",
				"choices":[{"message":{"content":"{\"turn_summary\":\"Mina and Rowan commit to the blue key.\",\"importance_score\":7,\"evidence_excerpts\":[\"blue key safe\"],\"kg_triples\":[{\"subject\":\"Rowan\",\"predicate\":\"protects\",\"object\":\"blue key\"}],\"entities\":{\"characters\":[{\"name\":\"Rowan\"}],\"items\":[{\"name\":\"blue key\"}]}}"}}]
			}`)),
		}, nil
	})}
	defer func() { proxyHTTPClient = oldClient }()

	mux := http.NewServeMux()
	srv.RegisterRoutes(mux)
	body := map[string]any{
		"chat_session_id":   "sess-embedding-provider-fail",
		"turn_index":        1,
		"user_input":        "Mina asks Rowan to remember the blue key.",
		"assistant_content": "Rowan promises to keep the blue key safe.",
		"client_meta": map[string]any{
			"critic": map[string]any{
				"api_key":  "sk-critic-ok",
				"endpoint": "https://api.example.com/v1",
				"model":    "critic-model",
				"provider": "openai",
			},
			"embedding": map[string]any{
				"api_key":  "sk-embedding-fail",
				"endpoint": "https://api.example.com/v1/embeddings",
				"model":    "embedding-model",
				"provider": "openai",
			},
		},
	}
	raw, _ := json.Marshal(body)
	req := httptest.NewRequest(http.MethodPost, "/complete-turn", bytes.NewReader(raw))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("complete-turn status = %d, want 200 with embedding warning: %s", rec.Code, rec.Body.String())
	}

	var resp map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if resp["critic_triggered"] != true || resp["memories_saved"] != float64(1) || resp["vectors_upserted"] != float64(0) {
		t.Fatalf("expected critic artifacts but no vector upsert, got %+v", resp)
	}
	trace, _ := resp["trace_handoff"].(map[string]any)
	if !strings.Contains(fmt.Sprint(trace["embedding_status"]), "error:") || !strings.Contains(fmt.Sprint(trace["vector_status"]), "error:") {
		t.Fatalf("embedding/vector failure was not visible in trace: %+v", trace)
	}
	warnings, _ := resp["warnings"].([]any)
	found := false
	for _, item := range warnings {
		if fmt.Sprint(item) == "embedding_call_failed" {
			found = true
		}
	}
	if !found {
		t.Fatalf("expected embedding_call_failed warning, got %+v", resp["warnings"])
	}
}

func TestCompleteTurnOOCGuardSkipsWrites(t *testing.T) {
	fake := &turnRecordingStore{}
	cfg := config.Default()
	cfg.StoreMode = config.StoreModeMariaDBAuthority
	srv := NewServer(cfg)
	srv.Store = fake
	srv.StoreOpenError = nil

	mux := http.NewServeMux()
	srv.RegisterRoutes(mux)

	body := `{"chat_session_id":"sess-ooc","turn_index":1,"user_input":"OOC: please change the plugin setting","assistant_content":"Sure, I will help.","context_messages":[]}`
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
	if resp["save_error"] != "skipped_by_ooc_guard" || resp["critic_triggered"] != false {
		t.Fatalf("unexpected OOC response: %+v", resp)
	}
	if len(fake.savedChatLogs) != 0 || len(fake.savedMemories) != 0 || len(fake.savedEvidence) != 0 || len(fake.savedKGTriples) != 0 {
		t.Fatalf("OOC guard should skip all writes, logs=%d memories=%d evidence=%d kg=%d", len(fake.savedChatLogs), len(fake.savedMemories), len(fake.savedEvidence), len(fake.savedKGTriples))
	}
}

func TestCompleteTurnKoreanOOCGuardSkipsWrites(t *testing.T) {
	fake := &turnRecordingStore{}
	cfg := config.Default()
	cfg.StoreMode = config.StoreModeMariaDBAuthority
	srv := NewServer(cfg)
	srv.Store = fake
	srv.StoreOpenError = nil

	mux := http.NewServeMux()
	srv.RegisterRoutes(mux)

	body := `{"chat_session_id":"sess-ooc-ko","turn_index":1,"user_input":"OOC: this is a setting change request, not story dialogue.","assistant_content":"Understood.","context_messages":[]}`
	req := httptest.NewRequest(http.MethodPost, "/complete-turn", bytes.NewReader([]byte(body)))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200: %s", rec.Code, rec.Body.String())
	}
	if len(fake.savedChatLogs) != 0 || len(fake.savedMemories) != 0 || len(fake.savedEvidence) != 0 || len(fake.savedKGTriples) != 0 {
		t.Fatalf("Korean OOC guard should skip all writes, logs=%d memories=%d evidence=%d kg=%d", len(fake.savedChatLogs), len(fake.savedMemories), len(fake.savedEvidence), len(fake.savedKGTriples))
	}
}

func TestCompleteTurnSourceAwareGuardSkipsDerivedIngest(t *testing.T) {
	fake := &turnRecordingStore{}
	cfg := config.Default()
	cfg.StoreMode = config.StoreModeMariaDBAuthority
	srv := NewServer(cfg)
	srv.Store = fake
	srv.StoreOpenError = nil

	calls := 0
	oldClient := proxyHTTPClient
	proxyHTTPClient = &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		calls++
		return &http.Response{StatusCode: http.StatusInternalServerError, Header: make(http.Header), Body: io.NopCloser(strings.NewReader(`{}`))}, nil
	})}
	defer func() { proxyHTTPClient = oldClient }()

	mux := http.NewServeMux()
	srv.RegisterRoutes(mux)
	body := map[string]any{
		"chat_session_id":   "sess-source-aware",
		"turn_index":        1,
		"user_input":        "[Narrative Guide]\nScene Mandate: keep the mood stable\nForbidden Moves:\n- sudden battle",
		"assistant_content": "Response Template\n{{char}} should answer in the requested style.",
		"client_meta":       map[string]any{"critic": map[string]any{"api_key": "sk-test", "endpoint": "https://api.example.com/v1", "model": "critic-model", "provider": "openai"}},
	}
	raw, _ := json.Marshal(body)
	req := httptest.NewRequest(http.MethodPost, "/complete-turn", bytes.NewReader(raw))
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
	if resp["critic_triggered"] != false || calls != 0 {
		t.Fatalf("source-aware guard should skip critic call, triggered=%v calls=%d resp=%+v", resp["critic_triggered"], calls, resp)
	}
	if len(fake.savedChatLogs) != 2 || len(fake.savedMemories) != 0 || len(fake.savedEvidence) != 0 || len(fake.savedKGTriples) != 0 || len(fake.savedPendingThreads) != 0 {
		t.Fatalf("source-aware guard should keep raw save but skip derived artifacts, logs=%d memories=%d evidence=%d kg=%d threads=%d", len(fake.savedChatLogs), len(fake.savedMemories), len(fake.savedEvidence), len(fake.savedKGTriples), len(fake.savedPendingThreads))
	}
	reasons, _ := resp["fail_reasons"].([]any)
	found := false
	for _, item := range reasons {
		if strings.Contains(fmt.Sprint(item), "source_aware_ingest_guard") {
			found = true
		}
	}
	if !found {
		t.Fatalf("expected source_aware_ingest_guard fail reason, got %+v", resp["fail_reasons"])
	}
}

// ---------------------------------------------------------------------------
// ---------------------------------------------------------------------------
// /effective-inputs tests
// ---------------------------------------------------------------------------

func TestEffectiveInputsNoopModeHasTransparency(t *testing.T) {
	fake := &turnRecordingStore{}
	cfg := config.Default()
	srv := NewServer(cfg)
	srv.Store = fake

	mux := http.NewServeMux()
	srv.RegisterRoutes(mux)

	body := `{"chat_session_id":"sess-ei-noop","turn_index":2,"effective_input":"user refined intent here"}`
	req := httptest.NewRequest(http.MethodPost, "/effective-inputs", bytes.NewReader([]byte(body)))
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

	it, ok := resp["input_transparency"].(map[string]any)
	if !ok {
		t.Fatalf("input_transparency is not an object")
	}
	if it["status"] != "ready" {
		t.Errorf("input_transparency.status = %v, want ready", it["status"])
	}
	if it["store_write_enabled"] != false {
		t.Errorf("input_transparency.store_write_enabled = %v, want false", it["store_write_enabled"])
	}
	if it["would_write"] != false {
		t.Errorf("input_transparency.would_write = %v, want false", it["would_write"])
	}
	if it["effective_input_chars"] != float64(24) {
		t.Errorf("input_transparency.effective_input_chars = %v, want 24", it["effective_input_chars"])
	}
	preview, _ := it["preview"].(string)
	if !strings.Contains(preview, "refined intent") {
		t.Errorf("input_transparency.preview missing expected text: %q", preview)
	}
	notes, _ := it["notes"].(string)
	if !strings.Contains(notes, "R1 read-shadow") {
		t.Errorf("input_transparency.notes missing R1 marker: %q", notes)
	}
	if resp["save_ok"] != false {
		t.Errorf("save_ok = %v, want false", resp["save_ok"])
	}
}

func TestEffectiveInputsDualShadowHasTransparency(t *testing.T) {
	fake := &turnRecordingStore{}
	cfg := config.Default()
	cfg.StoreMode = config.StoreModeDualShadow
	srv := NewServer(cfg)
	srv.Store = fake

	mux := http.NewServeMux()
	srv.RegisterRoutes(mux)

	body := `{"chat_session_id":"sess-ei-dual","turn_index":5,"effective_input":"dual shadow text"}`
	req := httptest.NewRequest(http.MethodPost, "/effective-inputs", bytes.NewReader([]byte(body)))
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

	it, ok := resp["input_transparency"].(map[string]any)
	if !ok {
		t.Fatalf("input_transparency is not an object")
	}
	if it["status"] != "ready" {
		t.Errorf("input_transparency.status = %v, want ready", it["status"])
	}
	if it["store_write_enabled"] != true {
		t.Errorf("input_transparency.store_write_enabled = %v, want true", it["store_write_enabled"])
	}
	if it["would_write"] != true {
		t.Errorf("input_transparency.would_write = %v, want true", it["would_write"])
	}
	if it["effective_input_chars"] != float64(16) {
		t.Errorf("input_transparency.effective_input_chars = %v, want 16", it["effective_input_chars"])
	}
	preview, _ := it["preview"].(string)
	if !strings.Contains(preview, "dual shadow") {
		t.Errorf("input_transparency.preview missing expected text: %q", preview)
	}
	if resp["save_ok"] != true {
		t.Errorf("save_ok = %v, want true", resp["save_ok"])
	}
}

// /prepare-turn tests
// ---------------------------------------------------------------------------

func TestPrepareTurnStoreBackedAssembly(t *testing.T) {
	fake := &turnRecordingStore{
		returnMemories: []store.Memory{
			{ID: 1, ChatSessionID: "sess-prep", TurnIndex: 2, SummaryJSON: `{"turn_summary":"A warm evening in the garden"}`, Importance: 0.9},
			{ID: 2, ChatSessionID: "sess-prep", TurnIndex: 3, SummaryJSON: `{"turn_summary":"A confrontation at the gate"}`, Importance: 0.8},
		},
		returnKGTriples: []store.KGTriple{
			{ID: 10, ChatSessionID: "sess-prep", Subject: "Alice", Predicate: "knows", Object: "Bob"},
		},
		returnEvidence: []store.DirectEvidence{
			{ID: 20, ChatSessionID: "sess-prep", EvidenceText: "The letter was sealed with red wax.", EvidenceKind: "verbatim", SourceTurnStart: 5, SourceTurnEnd: 5, TurnAnchor: 5},
		},
		returnChatLogs: []store.ChatLog{
			{ID: 100, ChatSessionID: "sess-prep", TurnIndex: 5, Role: "user", Content: "What happens next?"},
			{ID: 101, ChatSessionID: "sess-prep", TurnIndex: 5, Role: "assistant", Content: "The door creaks open."},
		},
		returnResumePack: &store.ResumePack{
			PackStatus:    "ready",
			AssembledText: "Session resume: Alice and Bob are investigating the old manor.",
		},
		returnStorylines: []store.Storyline{
			{ID: 1, ChatSessionID: "sess-prep", Name: "The Manor Mystery", CurrentContext: "Alice investigates the old manor"},
		},
		returnWorldRules: []store.WorldRule{
			{ID: 1, ChatSessionID: "sess-prep", Key: "magic_requires_blood", Scope: "session"},
		},
		returnCharStates: []store.CharacterState{
			{ID: 1, ChatSessionID: "sess-prep", CharacterName: "Alice", StatusJSON: `{"health":"injured","mood":"determined"}`},
		},
		returnPendingThreads: []store.PendingThread{
			{ID: 1, ChatSessionID: "sess-prep", Description: "Who sent the letter?"},
		},
		returnActiveStates: []store.ActiveState{
			{ID: 1, ChatSessionID: "sess-prep", StateType: "location", Content: "Old manor library"},
		},
		returnCanonicalLayers: []store.CanonicalStateLayer{
			{ID: 1, ChatSessionID: "sess-prep", LayerType: "location", Content: "The library smells of old books"},
		},
		returnEpisodeSums: []store.EpisodeSummary{
			{ID: 1, ChatSessionID: "sess-prep", FromTurn: 1, ToTurn: 3, SummaryText: "Alice arrives at the manor and finds clues"},
		},
	}

	cfg := config.Default()
	cfg.StoreMode = config.StoreModeDualShadow
	srv := NewServer(cfg)
	srv.Store = fake

	mux := http.NewServeMux()
	srv.RegisterRoutes(mux)

	body := `{"chat_session_id":"sess-prep","turn_index":6,"raw_user_input":"Open the door","settings":{"max_injection_chars":500,"max_input_context_chars":400,"injection_enabled":true,"input_context_enabled":true,"top_k":2}}`
	req := httptest.NewRequest(http.MethodPost, "/prepare-turn", bytes.NewReader([]byte(body)))
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
		t.Errorf("status = %v, want ok", resp["status"])
	}

	gp, ok := resp["generation_packet"].(map[string]any)
	if !ok {
		t.Fatalf("generation_packet is not an object")
	}

	if gp["packet_mode"] != "store_backed_shadow" {
		t.Errorf("packet_mode = %v, want store_backed_shadow", gp["packet_mode"])
	}
	if gp["degraded"] != false {
		t.Errorf("degraded = %v, want false", gp["degraded"])
	}

	injection, _ := gp["injection_text"].(string)
	if injection == "" {
		t.Error("injection_text is empty")
	}
	if len(injection) > 500 {
		t.Errorf("injection_text length %d exceeds cap 500", len(injection))
	}

	ict, _ := resp["input_context_text"].(string)
	if ict == "" {
		t.Error("input_context_text is empty")
	}
	if len(ict) > 400 {
		t.Errorf("input_context_text length %d exceeds cap 400", len(ict))
	}

	trace, ok := gp["trace_summary"].(map[string]any)
	if !ok {
		t.Fatalf("trace_summary is not an object")
	}
	// 5 original reads + 7 narrative reads + persona capsule read = 13 total.
	if trace["reads_ok"] != float64(13) {
		t.Errorf("reads_ok = %v, want 13", trace["reads_ok"])
	}
	if trace["memory_count"] != float64(2) {
		t.Errorf("memory_count = %v, want 2", trace["memory_count"])
	}
	if trace["storyline_count"] != float64(1) {
		t.Errorf("storyline_count = %v, want 1", trace["storyline_count"])
	}
	if trace["world_rule_count"] != float64(1) {
		t.Errorf("world_rule_count = %v, want 1", trace["world_rule_count"])
	}
	if trace["character_state_count"] != float64(1) {
		t.Errorf("character_state_count = %v, want 1", trace["character_state_count"])
	}
	if trace["pending_thread_count"] != float64(1) {
		t.Errorf("pending_thread_count = %v, want 1", trace["pending_thread_count"])
	}
	if trace["active_state_count"] != float64(1) {
		t.Errorf("active_state_count = %v, want 1", trace["active_state_count"])
	}
	if trace["canonical_layer_count"] != float64(1) {
		t.Errorf("canonical_layer_count = %v, want 1", trace["canonical_layer_count"])
	}
	if trace["episode_summary_count"] != float64(1) {
		t.Errorf("episode_summary_count = %v, want 1", trace["episode_summary_count"])
	}
	if trace["scoped_verbatim_support_count"] != float64(1) {
		t.Errorf("scoped_verbatim_support_count = %v, want 1", trace["scoped_verbatim_support_count"])
	}
	verbatimSupport, ok := trace["verbatim_support"].(map[string]any)
	if !ok {
		t.Fatalf("verbatim_support is not an object")
	}
	if verbatimSupport["active"] != true {
		t.Errorf("verbatim_support.active = %v, want true", verbatimSupport["active"])
	}
	if verbatimSupport["policy_version"] != "vr18a.v1" {
		t.Errorf("verbatim_support.policy_version = %v, want vr18a.v1", verbatimSupport["policy_version"])
	}
	earlyInjectionPack, ok := resp["injection_pack"].(map[string]any)
	if !ok {
		t.Fatalf("injection_pack is not an object")
	}
	if earlyInjectionPack["scoped_verbatim_support_count"] != float64(1) {
		t.Errorf("injection_pack scoped count = %v, want 1", earlyInjectionPack["scoped_verbatim_support_count"])
	}
	if strings.Contains(injection, "Scoped Verbatim Recall") {
		t.Errorf("injection text should not inline support surface: %q", injection)
	}

	// Verify section labels in injection_text
	if !strings.Contains(injection, "[Memory]") {
		t.Error("injection_text missing [Memory] section")
	}
	if !strings.Contains(injection, "[Knowledge Graph]") {
		t.Error("injection_text missing [Knowledge Graph] section")
	}
	if !strings.Contains(injection, "[Storylines]") {
		t.Error("injection_text missing [Storylines] section")
	}
	if !strings.Contains(injection, "[World Rules]") {
		t.Error("injection_text missing [World Rules] section")
	}
	if !strings.Contains(injection, "[Characters]") {
		t.Error("injection_text missing [Characters] section")
	}
	if !strings.Contains(injection, "[Pending Threads]") {
		t.Error("injection_text missing [Pending Threads] section")
	}

	// Verify section labels in input_context_text
	if !strings.Contains(ict, "[Resume Pack]") {
		t.Error("input_context_text missing [Resume Pack] section")
	}
	if !strings.Contains(ict, "[Direct Evidence]") {
		t.Error("input_context_text missing [Direct Evidence] section")
	}
	if !strings.Contains(ict, "[Recent Chat]") {
		t.Error("input_context_text missing [Recent Chat] section")
	}
	if !strings.Contains(ict, "[Active States]") {
		t.Error("input_context_text missing [Active States] section")
	}
	if !strings.Contains(ict, "[Canonical State Layers]") {
		t.Error("input_context_text missing [Canonical State Layers] section")
	}
	if !strings.Contains(ict, "[Episode Summaries]") {
		t.Error("input_context_text missing [Episode Summaries] section")
	}

	supervisorPack, ok := resp["supervisor_input_pack"].(map[string]any)
	if !ok {
		t.Fatalf("supervisor_input_pack is not an object")
	}
	if supervisorPack["status"] != "ready" {
		t.Errorf("supervisor_input_pack.status = %v, want ready", supervisorPack["status"])
	}
	if supervisorPack["would_call_llm"] != false {
		t.Errorf("supervisor_input_pack.would_call_llm = %v, want false", supervisorPack["would_call_llm"])
	}
	if supervisorPack["would_write"] != false {
		t.Errorf("supervisor_input_pack.would_write = %v, want false", supervisorPack["would_write"])
	}
	if supervisorPack["prompt_source"] != "not_configured" {
		t.Errorf("supervisor_input_pack.prompt_source = %v, want not_configured", supervisorPack["prompt_source"])
	}
	if suffix, _ := supervisorPack["final_guidance_suffix"].(string); !strings.Contains(suffix, "Go R1 Supervisor Read Shadow") {
		t.Errorf("final_guidance_suffix missing read-shadow marker: %q", suffix)
	}

	criticPack, ok := resp["critic_input_pack"].(map[string]any)
	if !ok {
		t.Fatalf("critic_input_pack is not an object")
	}
	if criticPack["status"] != "ready" {
		t.Errorf("critic_input_pack.status = %v, want ready", criticPack["status"])
	}
	if criticPack["would_call_llm"] != false {
		t.Errorf("critic_input_pack.would_call_llm = %v, want false", criticPack["would_call_llm"])
	}
	if criticPack["verdict"] != "not_executed" {
		t.Errorf("critic_input_pack.verdict = %v, want not_executed", criticPack["verdict"])
	}

	injectionPack, ok := resp["injection_pack"].(map[string]any)
	if !ok {
		t.Fatalf("injection_pack is not an object")
	}
	if injectionPack["would_inject"] != true {
		t.Errorf("injection_pack.would_inject = %v, want true", injectionPack["would_inject"])
	}
	if injectionPack["would_write"] != false {
		t.Errorf("injection_pack.would_write = %v, want false", injectionPack["would_write"])
	}
	if injectionPack["final_budget_owner"] != "archive_center_js_assembleInjectionWithBudget" {
		t.Errorf("final_budget_owner = %v, want JS budget owner", injectionPack["final_budget_owner"])
	}
	if _, ok := injectionPack["budget_decisions"].(map[string]any); !ok {
		t.Fatalf("injection_pack.budget_decisions is not an object")
	}
	if memoryText, _ := injectionPack["memory_text"].(string); !strings.Contains(memoryText, "A warm evening in the garden") {
		t.Errorf("memory_text missing readable memory summary: %q", memoryText)
	}
	if kgText, _ := injectionPack["kg_text"].(string); !strings.Contains(kgText, "Alice --knows--> Bob") {
		t.Errorf("kg_text missing KG triple: %q", kgText)
	}
	if fallback := injectionPack["fallback_text"]; fallback != nil {
		t.Errorf("fallback_text = %v, want nil when two memories are available", fallback)
	}
	if deText, _ := injectionPack["latest_direct_evidence_text"].(string); !strings.Contains(deText, "red wax") {
		t.Errorf("latest_direct_evidence_text missing direct evidence: %q", deText)
	}
	if rawText, _ := injectionPack["recent_raw_turn_text"].(string); !strings.Contains(rawText, "The door creaks open") {
		t.Errorf("recent_raw_turn_text missing recent raw turn: %q", rawText)
	}
	if canonText, _ := injectionPack["canon_text"].(string); !strings.Contains(canonText, "library smells") {
		t.Errorf("canon_text missing canonical layer: %q", canonText)
	}

	recall, ok := resp["recall_result"].(map[string]any)
	if !ok {
		t.Fatalf("recall_result is not an object")
	}
	if recall["status"] != "ready" {
		t.Errorf("recall_result.status = %v, want ready", recall["status"])
	}
	items, ok := recall["items"].([]any)
	if !ok {
		t.Fatalf("recall_result.items is not an array")
	}
	if len(items) != 3 {
		t.Errorf("recall_result.items length = %d, want 3", len(items))
	}
	kgRecall, ok := recall["kg_triples"].([]any)
	if !ok {
		t.Fatalf("recall_result.kg_triples is not an array")
	}
	if len(kgRecall) != 1 {
		t.Errorf("recall_result.kg_triples length = %d, want 1", len(kgRecall))
	}
	episodeRecall, ok := recall["episodes"].([]any)
	if !ok {
		t.Fatalf("recall_result.episodes is not an array")
	}
	if len(episodeRecall) != 1 {
		t.Errorf("recall_result.episodes length = %d, want 1", len(episodeRecall))
	}
	counts, ok := recall["counts"].(map[string]any)
	if !ok {
		t.Fatalf("recall_result.counts is not an object")
	}
	if counts["memories_total"] != float64(2) {
		t.Errorf("recall_result.counts.memories_total = %v, want 2", counts["memories_total"])
	}
	if counts["kg_total"] != float64(1) {
		t.Errorf("recall_result.counts.kg_total = %v, want 1", counts["kg_total"])
	}
	vectorShadow, ok := recall["vector_shadow"].(map[string]any)
	if !ok {
		t.Fatalf("recall_result.vector_shadow is not an object")
	}
	if vectorShadow["status"] != "shadow" {
		t.Errorf("recall_result.vector_shadow.status = %v, want shadow", vectorShadow["status"])
	}
	if recall["would_call_vector"] != false {
		t.Errorf("recall_result.would_call_vector = %v, want false", recall["would_call_vector"])
	}
	if recall["would_write"] != false {
		t.Errorf("recall_result.would_write = %v, want false", recall["would_write"])
	}

	// session_state bundle
	ss, ok := resp["session_state"].(map[string]any)
	if !ok {
		t.Fatalf("session_state is not an object")
	}
	if ss["snapshot_status"] != "ready" {
		t.Errorf("session_state.snapshot_status = %v, want ready", ss["snapshot_status"])
	}
	if ss["fetched"] != true {
		t.Errorf("session_state.fetched = %v, want true", ss["fetched"])
	}
	ssMeta, _ := ss["section_meta"].(map[string]any)
	if ssMeta["storyline_count"] != float64(1) {
		t.Errorf("session_state.section_meta.storyline_count = %v, want 1", ssMeta["storyline_count"])
	}
	if ssMeta["character_count"] != float64(1) {
		t.Errorf("session_state.section_meta.character_count = %v, want 1", ssMeta["character_count"])
	}
	if ssMeta["world_rule_count"] != float64(1) {
		t.Errorf("session_state.section_meta.world_rule_count = %v, want 1", ssMeta["world_rule_count"])
	}
	if ssMeta["pending_thread_count"] != float64(1) {
		t.Errorf("session_state.section_meta.pending_thread_count = %v, want 1", ssMeta["pending_thread_count"])
	}
	if ssMeta["active_state_count"] != float64(1) {
		t.Errorf("session_state.section_meta.active_state_count = %v, want 1", ssMeta["active_state_count"])
	}

	// narrative_control bundle
	nc, ok := resp["narrative_control"].(map[string]any)
	if !ok {
		t.Fatalf("narrative_control is not an object")
	}
	if nc["state_status"] != "shadow_evidence" {
		t.Errorf("narrative_control.state_status = %v, want shadow_evidence", nc["state_status"])
	}
	if nc["storyline_count"] != float64(1) {
		t.Errorf("narrative_control.storyline_count = %v, want 1", nc["storyline_count"])
	}
	if nc["world_rule_count"] != float64(1) {
		t.Errorf("narrative_control.world_rule_count = %v, want 1", nc["world_rule_count"])
	}
	if nc["pending_thread_count"] != float64(1) {
		t.Errorf("narrative_control.pending_thread_count = %v, want 1", nc["pending_thread_count"])
	}
	if nc["character_count"] != float64(1) {
		t.Errorf("narrative_control.character_count = %v, want 1", nc["character_count"])
	}
	if nc["would_call_llm"] != false {
		t.Errorf("narrative_control.would_call_llm = %v, want false", nc["would_call_llm"])
	}
	if nc["would_write"] != false {
		t.Errorf("narrative_control.would_write = %v, want false", nc["would_write"])
	}

	// continuity_pack bundle
	cp, ok := resp["continuity_pack"].(map[string]any)
	if !ok {
		t.Fatalf("continuity_pack is not an object")
	}
	if cp["status"] != "ready" {
		t.Errorf("continuity_pack.status = %v, want ready", cp["status"])
	}
	if cp["resume_pack_present"] != true {
		t.Errorf("continuity_pack.resume_pack_present = %v, want true", cp["resume_pack_present"])
	}
	if cp["episode_count"] != float64(1) {
		t.Errorf("continuity_pack.episode_count = %v, want 1", cp["episode_count"])
	}
	if cp["chat_log_count"] != float64(2) {
		t.Errorf("continuity_pack.chat_log_count = %v, want 2", cp["chat_log_count"])
	}
	if cp["active_state_count"] != float64(1) {
		t.Errorf("continuity_pack.active_state_count = %v, want 1", cp["active_state_count"])
	}
	if cp["canonical_layer_count"] != float64(1) {
		t.Errorf("continuity_pack.canonical_layer_count = %v, want 1", cp["canonical_layer_count"])
	}
	cpItems, _ := cp["items"].([]any)
	if len(cpItems) == 0 {
		t.Errorf("continuity_pack.items is empty")
	}
	if cp["would_call_llm"] != false {
		t.Errorf("continuity_pack.would_call_llm = %v, want false", cp["would_call_llm"])
	}
	if cp["would_write"] != false {
		t.Errorf("continuity_pack.would_write = %v, want false", cp["would_write"])
	}

	progressionLedger, ok := resp["progression_ledger"].(map[string]any)
	if !ok {
		t.Fatalf("progression_ledger is not an object")
	}
	if progressionLedger["status"] != "ready" {
		t.Errorf("progression_ledger.status = %v, want ready", progressionLedger["status"])
	}
	if progressionLedger["chat_session_id"] != "sess-prep" {
		t.Errorf("progression_ledger.chat_session_id = %v, want sess-prep", progressionLedger["chat_session_id"])
	}
	if progressionLedger["storyline_count"] != float64(1) {
		t.Errorf("progression_ledger.storyline_count = %v, want 1", progressionLedger["storyline_count"])
	}
	if progressionLedger["world_rule_count"] != float64(1) {
		t.Errorf("progression_ledger.world_rule_count = %v, want 1", progressionLedger["world_rule_count"])
	}
	if progressionLedger["pending_thread_count"] != float64(1) {
		t.Errorf("progression_ledger.pending_thread_count = %v, want 1", progressionLedger["pending_thread_count"])
	}
	if progressionLedger["episode_count"] != float64(1) {
		t.Errorf("progression_ledger.episode_count = %v, want 1", progressionLedger["episode_count"])
	}
	if progressionLedger["would_write"] != false {
		t.Errorf("progression_ledger.would_write = %v, want false", progressionLedger["would_write"])
	}
	if progressionLedger["ledger_policy_version"] != "lw1h.v1" || progressionLedger["ledger_mode"] != "deterministic_no_llm" {
		t.Fatalf("progression_ledger LW policy/mode mismatch: %#v", progressionLedger)
	}
	for _, key := range []string{"unresolved_tensions", "consequences", "payoffs", "scene_deltas"} {
		items, ok := progressionLedger[key].([]any)
		if !ok || len(items) == 0 {
			t.Fatalf("progression_ledger.%s missing generated items: %#v", key, progressionLedger[key])
		}
		first, ok := items[0].(map[string]any)
		if !ok {
			t.Fatalf("progression_ledger.%s first item shape: %#v", key, items[0])
		}
		for _, field := range []string{"entry_type", "label", "source", "status", "lifecycle_state", "pressure_score", "decay_turns", "source_record_id", "source_message_ids", "affected_relations", "affected_world"} {
			if _, exists := first[field]; !exists {
				t.Fatalf("progression_ledger.%s first item missing %s: %#v", key, field, first)
			}
		}
	}
	payoffs, _ := progressionLedger["payoffs"].([]any)
	payoff := payoffs[0].(map[string]any)
	if payoff["payoff_state"] != "pending" || payoff["do_not_resolve_yet"] != true || payoff["resolve_guard_reason"] != "long_horizon_candidate" {
		t.Fatalf("progression_ledger payoff guard mismatch: %#v", payoff)
	}
	worldPressure, ok := progressionLedger["world_pressure"].(map[string]any)
	if !ok || worldPressure["status"] != "structured_support" {
		t.Fatalf("progression_ledger world_pressure missing: %#v", progressionLedger["world_pressure"])
	}
	for _, key := range []string{"factions", "regions", "offscreen_threads", "public_pressure", "timeline"} {
		if _, exists := worldPressure[key]; !exists {
			t.Fatalf("world_pressure missing %s: %#v", key, worldPressure)
		}
	}
	guard, ok := progressionLedger["supporting_precedence_guard"].(map[string]any)
	if !ok || guard["cannot_override_current_user_input"] != true || guard["cannot_override_verified_direct_evidence"] != true {
		t.Fatalf("supporting precedence guard mismatch: %#v", progressionLedger["supporting_precedence_guard"])
	}
	compat, ok := progressionLedger["compatibility_contract"].(map[string]any)
	if !ok || compat["shape_mode"] != "additive_non_breaking" || compat["consumer_safe"] != true {
		t.Fatalf("compatibility contract mismatch: %#v", progressionLedger["compatibility_contract"])
	}
	lifecycle, ok := progressionLedger["lifecycle_model"].(map[string]any)
	if !ok || lifecycle["status"] != "active" || lenAnySlice(lifecycle["states"]) != 6 || lenAnyMap(lifecycle["decay_rules"]) == 0 {
		t.Fatalf("lifecycle model mismatch: %#v", progressionLedger["lifecycle_model"])
	}
	doNotResolveGuard, ok := progressionLedger["do_not_resolve_guard"].(map[string]any)
	if !ok || doNotResolveGuard["status"] != "active" || doNotResolveGuard["mode"] != "deterministic_no_llm" {
		t.Fatalf("do_not_resolve_guard mismatch: %#v", progressionLedger["do_not_resolve_guard"])
	}
	tracePreview, ok := resp["trace_preview"].(map[string]any)
	if !ok {
		t.Fatalf("trace_preview is not an object")
	}
	for _, key := range []string{
		"story_ledger_policy_version", "unresolved_tensions_count", "consequences_count", "payoffs_count", "scene_deltas_count",
		"payoff_pending_count", "world_pressure_ready", "world_pressure_policy_version", "continuity_precedence_policy_version",
		"supporting_precedence_guard_ready", "compatibility_policy_version", "compatibility_ready", "lifecycle_policy_version",
		"lifecycle_ready", "lifecycle_entry_count", "do_not_resolve_policy_version", "do_not_resolve_guard_ready", "do_not_resolve_protected_count",
	} {
		if _, exists := tracePreview[key]; !exists {
			t.Fatalf("trace_preview missing LW field %s: %#v", key, tracePreview)
		}
	}
	if tracePreview["story_ledger_policy_version"] != "lw1h.v1" || tracePreview["world_pressure_policy_version"] != "lw1d.v1" || tracePreview["do_not_resolve_policy_version"] != "lw1h.v1" {
		t.Fatalf("trace_preview LW policy mismatch: %#v", tracePreview)
	}

	autonomyPlan, ok := resp["autonomy_plan"].(map[string]any)
	if !ok {
		t.Fatalf("autonomy_plan is not an object")
	}
	if autonomyPlan["status"] != "ready" {
		t.Errorf("autonomy_plan.status = %v, want ready", autonomyPlan["status"])
	}
	if autonomyPlan["guide_mode"] != "off" {
		t.Errorf("autonomy_plan.guide_mode = %v, want off", autonomyPlan["guide_mode"])
	}
	if autonomyPlan["narrative_stance"] != "balanced" {
		t.Errorf("autonomy_plan.narrative_stance = %v, want balanced", autonomyPlan["narrative_stance"])
	}
	if autonomyPlan["would_call_llm"] != false {
		t.Errorf("autonomy_plan.would_call_llm = %v, want false", autonomyPlan["would_call_llm"])
	}

	microBeat, ok := resp["micro_beat_proposal"].(map[string]any)
	if !ok {
		t.Fatalf("micro_beat_proposal is not an object")
	}
	if microBeat["status"] != "ready" {
		t.Errorf("micro_beat_proposal.status = %v, want ready", microBeat["status"])
	}
	beats, _ := microBeat["beats"].([]any)
	if len(beats) != 2 {
		t.Errorf("micro_beat_proposal.beats len = %d, want 2", len(beats))
	}
	if microBeat["would_call_llm"] != false {
		t.Errorf("micro_beat_proposal.would_call_llm = %v, want false", microBeat["would_call_llm"])
	}
	if microBeat["would_write"] != false {
		t.Errorf("micro_beat_proposal.would_write = %v, want false", microBeat["would_write"])
	}

	sceneStep, ok := resp["scene_step_proposal"].(map[string]any)
	if !ok {
		t.Fatalf("scene_step_proposal is not an object")
	}
	if sceneStep["status"] != "ready" {
		t.Errorf("scene_step_proposal.status = %v, want ready", sceneStep["status"])
	}
	steps, _ := sceneStep["steps"].([]any)
	if len(steps) != 2 {
		t.Errorf("scene_step_proposal.steps len = %d, want topK 2", len(steps))
	}
	if sceneStep["would_call_llm"] != false {
		t.Errorf("scene_step_proposal.would_call_llm = %v, want false", sceneStep["would_call_llm"])
	}
	if sceneStep["would_write"] != false {
		t.Errorf("scene_step_proposal.would_write = %v, want false", sceneStep["would_write"])
	}

	combinedProposal, ok := resp["combined_proposal"].(map[string]any)
	if !ok {
		t.Fatalf("combined_proposal is not an object")
	}
	if combinedProposal["status"] != "ready" {
		t.Errorf("combined_proposal.status = %v, want ready", combinedProposal["status"])
	}
	if combinedProposal["micro_beat_count"] != float64(2) {
		t.Errorf("combined_proposal.micro_beat_count = %v, want 2", combinedProposal["micro_beat_count"])
	}
	if combinedProposal["scene_step_count"] != float64(2) {
		t.Errorf("combined_proposal.scene_step_count = %v, want topK 2", combinedProposal["scene_step_count"])
	}
	if combinedProposal["source"] != "go_r1_read_shadow" {
		t.Errorf("combined_proposal.source = %v, want go_r1_read_shadow", combinedProposal["source"])
	}
	if combinedProposal["would_write"] != false {
		t.Errorf("combined_proposal.would_write = %v, want false", combinedProposal["would_write"])
	}

	writebackPreview, ok := resp["writeback_preview"].(map[string]any)
	if !ok {
		t.Fatalf("writeback_preview is not an object")
	}
	if writebackPreview["status"] != "ready" {
		t.Errorf("writeback_preview.status = %v, want ready", writebackPreview["status"])
	}
	targets, _ := writebackPreview["targets"].([]any)
	if len(targets) != 6 {
		t.Errorf("writeback_preview.targets len = %d, want 6", len(targets))
	}
	if writebackPreview["would_write"] != false {
		t.Errorf("writeback_preview.would_write = %v, want false", writebackPreview["would_write"])
	}
}

func TestPrepareTurnPersonaRecollectionSupportLane(t *testing.T) {
	fake := &turnRecordingStore{
		returnChatLogs: []store.ChatLog{
			{ID: 1, ChatSessionID: "target-loop", TurnIndex: 1, Role: "user", Content: "Why does this feel familiar?"},
			{ID: 2, ChatSessionID: "target-loop", TurnIndex: 1, Role: "assistant", Content: "Chloe pauses near the mirror."},
		},
		returnPersonaEntries: []store.PersonaMemoryEntry{
			{
				ID:              7,
				CapsuleID:       3,
				SourceTurn:      12,
				MemoryText:      "Siwoo remembers that Chloe hid the brass key behind the cracked mirror in the previous loop.",
				Importance10:    8.5,
				EmotionalWeight: 0.7,
				Portability:     "cross_world",
				InjectionPolicy: "support_only",
			},
		},
	}
	cfg := config.Default()
	cfg.StoreMode = config.StoreModeDualShadow
	srv := NewServer(cfg)
	srv.Store = fake
	mux := http.NewServeMux()
	srv.RegisterRoutes(mux)

	body := `{"chat_session_id":"target-loop","turn_index":2,"raw_user_input":"Look around the room.","settings":{"max_injection_chars":1200,"max_input_context_chars":800,"injection_enabled":true,"input_context_enabled":true,"top_k":2}}`
	req := httptest.NewRequest(http.MethodPost, "/prepare-turn", strings.NewReader(body))
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
	injectionText, _ := resp["injection_text"].(string)
	if !strings.Contains(injectionText, "[Persona Recollection]") {
		t.Fatalf("injection_text missing persona recollection: %q", injectionText)
	}
	if strings.Contains(injectionText, "brass key") {
		t.Fatalf("injection_text leaked protected persona recollection content: %q", injectionText)
	}
	if !strings.Contains(injectionText, "Secret Guard") || !strings.Contains(injectionText, "protagonist-only private intuition") || !strings.Contains(injectionText, "Never reveal its origin") {
		t.Fatalf("injection_text missing persona secret guard: %q", injectionText)
	}
	if strings.Contains(injectionText, "previous loop") || strings.Contains(injectionText, "regressor") || strings.Contains(injectionText, "regression") || strings.Contains(injectionText, "loop") {
		t.Fatalf("injection_text leaked explicit loop secret instead of masked protagonist-private hint: %q", injectionText)
	}
	if !strings.Contains(injectionText, "Protected hint") {
		t.Fatalf("injection_text missing masked persona secret hint: %q", injectionText)
	}
	inputContextText, _ := resp["input_context_text"].(string)
	if !strings.Contains(inputContextText, "[Persona Recollection]") || !strings.Contains(inputContextText, "support-only private recollection") {
		t.Fatalf("input_context_text missing support-only persona lane: %q", inputContextText)
	}
	ip, ok := resp["injection_pack"].(map[string]any)
	if !ok {
		t.Fatalf("injection_pack is not an object")
	}
	if ip["persona_recollection_active"] != true {
		t.Fatalf("persona_recollection_active = %v, want true", ip["persona_recollection_active"])
	}
	policy, ok := ip["persona_recollection_policy"].(map[string]any)
	if !ok {
		t.Fatalf("persona_recollection_policy is not an object")
	}
	if policy["truth_authority"] != false || policy["canonical_write"] != false {
		t.Fatalf("persona policy must be support-only, got %+v", policy)
	}
	if policy["secret_guard_active"] != true {
		t.Fatalf("persona policy missing active secret guard: %+v", policy)
	}
	secretGuard, ok := policy["secret_guard"].(map[string]any)
	if !ok || secretGuard["active"] != true {
		t.Fatalf("persona policy secret_guard mismatch: %+v", policy["secret_guard"])
	}
	surface, ok := resp["persona_recollection"].(map[string]any)
	if !ok {
		t.Fatalf("persona_recollection surface missing")
	}
	if surface["status"] != "ready" || surface["count"] != float64(1) || surface["would_write"] != false {
		t.Fatalf("persona surface mismatch: %+v", surface)
	}
	if surface["secret_guard_active"] != true {
		t.Fatalf("persona surface missing active secret guard: %+v", surface)
	}
	if len(fake.savedMemories) != 0 || len(fake.savedKGTriples) != 0 || len(fake.savedEvidence) != 0 {
		t.Fatalf("prepare-turn persona recollection must not write canonical rows: mem=%d kg=%d evi=%d", len(fake.savedMemories), len(fake.savedKGTriples), len(fake.savedEvidence))
	}
}

func TestPrepareTurnCharacterPrivateRecollectionLane(t *testing.T) {
	fake := &turnRecordingStore{
		returnChatLogs: []store.ChatLog{
			{ID: 1, ChatSessionID: "target-npc-loop", TurnIndex: 1, Role: "user", Content: "Siwoo enters the kitchen."},
			{ID: 2, ChatSessionID: "target-npc-loop", TurnIndex: 1, Role: "assistant", Content: "Chloe quietly watches him."},
		},
		returnEntityMemories: []store.ProtagonistEntityMemory{
			{
				ID:                  21,
				OwnerEntityKey:      "chloe",
				OwnerEntityName:     "Chloe",
				OwnerEntityRole:     "npc",
				OwnerVisibility:     "owner_private",
				SourceChatSessionID: "target-npc-loop",
				SourceTurn:          5,
				MemoryText:          "Chloe remembers from a previous loop that Siwoo avoided the broken bridge.",
				SecretGuard:         true,
				TargetRevealPolicy:  "owner_private_until_revealed",
				Portability:         "npc_private_recollection",
				TagsJSON:            `["loop","npc_private"]`,
				Importance10:        9,
				EmotionalWeight:     0.8,
			},
		},
	}
	cfg := config.Default()
	cfg.StoreMode = config.StoreModeDualShadow
	srv := NewServer(cfg)
	srv.Store = fake
	mux := http.NewServeMux()
	srv.RegisterRoutes(mux)

	body := `{"chat_session_id":"target-npc-loop","turn_index":2,"raw_user_input":"He asks Chloe where to go.","settings":{"max_injection_chars":1400,"max_input_context_chars":900,"injection_enabled":true,"input_context_enabled":true,"top_k":2}}`
	req := httptest.NewRequest(http.MethodPost, "/prepare-turn", strings.NewReader(body))
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
	injectionText, _ := resp["injection_text"].(string)
	if !strings.Contains(injectionText, "[Character Private Recollection]") || !strings.Contains(injectionText, "owner Chloe") {
		t.Fatalf("injection_text missing character private recollection: %q", injectionText)
	}
	if strings.Contains(injectionText, "[Persona Recollection]") {
		t.Fatalf("NPC private recollection leaked into persona lane: %q", injectionText)
	}
	if strings.Contains(injectionText, "previous loop") {
		t.Fatalf("NPC private recollection leaked explicit loop wording: %q", injectionText)
	}
	if !strings.Contains(injectionText, "Protected NPC-private hint") {
		t.Fatalf("NPC private recollection missing protected hint wording: %q", injectionText)
	}
	inputContextText, _ := resp["input_context_text"].(string)
	if !strings.Contains(inputContextText, "[Character Private Recollection]") || !strings.Contains(inputContextText, "not player knowledge") {
		t.Fatalf("input_context_text missing NPC private lane guard: %q", inputContextText)
	}
	ip, ok := resp["injection_pack"].(map[string]any)
	if !ok {
		t.Fatalf("injection_pack is not an object")
	}
	if ip["character_private_recollection_active"] != true {
		t.Fatalf("character private lane inactive: %+v", ip)
	}
	policy, ok := ip["character_private_recollection_policy"].(map[string]any)
	if !ok {
		t.Fatalf("character_private_recollection_policy missing: %+v", ip)
	}
	if policy["visible_to_player"] != false || policy["narrator_reveal_blocked"] != true || policy["canonical_write"] != false {
		t.Fatalf("character private policy must block reveal/write: %+v", policy)
	}
	surface, ok := resp["character_private_recollection"].(map[string]any)
	if !ok {
		t.Fatalf("character_private_recollection surface missing")
	}
	if surface["status"] != "ready" || surface["count"] != float64(1) || surface["visible_to_player"] != false || surface["narrator_reveal_blocked"] != true {
		t.Fatalf("character private surface mismatch: %+v", surface)
	}
	if len(fake.savedMemories) != 0 || len(fake.savedKGTriples) != 0 || len(fake.savedEvidence) != 0 {
		t.Fatalf("prepare-turn character private recollection must not write canonical rows: mem=%d kg=%d evi=%d", len(fake.savedMemories), len(fake.savedKGTriples), len(fake.savedEvidence))
	}
}

func TestPrepareTurnCharacterPrivateRecollectionTreatsMisunderstandingAsInterpretation(t *testing.T) {
	fake := &turnRecordingStore{
		returnChatLogs: []store.ChatLog{
			{ID: 1, ChatSessionID: "target-private-conflict", TurnIndex: 2, Role: "user", Content: "Siwoo stays quiet beside Chloe."},
			{ID: 2, ChatSessionID: "target-private-conflict", TurnIndex: 2, Role: "assistant", Content: "Chloe watches his silence and hesitates."},
		},
		returnEntityMemories: []store.ProtagonistEntityMemory{
			{
				ID:                  41,
				OwnerEntityKey:      "chloe",
				OwnerEntityName:     "Chloe",
				OwnerEntityRole:     "npc",
				OwnerVisibility:     "owner_private",
				SourceChatSessionID: "target-private-conflict",
				SourceTurn:          9,
				MemoryText:          "misunderstanding: Chloe privately interpreted Siwoo's silence as rejection.",
				TargetRevealPolicy:  "owner_private_until_revealed",
				Portability:         "npc_private_recollection",
				TagsJSON:            `["npc_private","misunderstanding","conflict"]`,
				Importance10:        7,
				EmotionalWeight:     0.6,
			},
		},
	}
	cfg := config.Default()
	cfg.StoreMode = config.StoreModeDualShadow
	srv := NewServer(cfg)
	srv.Store = fake
	mux := http.NewServeMux()
	srv.RegisterRoutes(mux)

	body := `{"chat_session_id":"target-private-conflict","turn_index":3,"raw_user_input":"Chloe asks why Siwoo went quiet.","settings":{"max_injection_chars":2600,"max_input_context_chars":1400,"injection_enabled":true,"input_context_enabled":true,"top_k":2}}`
	req := httptest.NewRequest(http.MethodPost, "/prepare-turn", strings.NewReader(body))
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
	injectionText, _ := resp["injection_text"].(string)
	if !strings.Contains(injectionText, "[Character Private Recollection]") || !strings.Contains(injectionText, "Private interpretation") {
		t.Fatalf("injection_text missing private interpretation lane: %q", injectionText)
	}
	if !strings.Contains(injectionText, "owning NPC's interpretation") || !strings.Contains(injectionText, "do not present it as objective fact") {
		t.Fatalf("injection_text missing interpretation-not-fact guard: %q", injectionText)
	}
	if strings.Contains(injectionText, "[Persona Recollection]") {
		t.Fatalf("NPC private recollection leaked into persona lane: %q", injectionText)
	}
	ip, ok := resp["injection_pack"].(map[string]any)
	if !ok {
		t.Fatalf("injection_pack is not an object")
	}
	policy, ok := ip["character_private_recollection_policy"].(map[string]any)
	if !ok {
		t.Fatalf("character_private_recollection_policy missing: %+v", ip)
	}
	if policy["interpretation_not_fact"] != true || policy["private_conflict_guard"] != true || policy["narrator_must_not_confirm_private_memory"] != true {
		t.Fatalf("character private policy missing PMC-20 guard fields: %+v", policy)
	}
	allowedExpression, _ := policy["allowed_expression"].([]any)
	hasMisunderstandingExpression := false
	for _, item := range allowedExpression {
		if item == "misunderstanding" {
			hasMisunderstandingExpression = true
			break
		}
	}
	if !hasMisunderstandingExpression {
		t.Fatalf("character private policy must allow misunderstanding as private expression: %+v", policy)
	}
	if policy["truth_authority"] != false || policy["canonical_write"] != false || policy["current_world_fact"] != false {
		t.Fatalf("character private policy must remain support-only: %+v", policy)
	}
	surface, ok := resp["character_private_recollection"].(map[string]any)
	if !ok {
		t.Fatalf("character_private_recollection surface missing")
	}
	if surface["interpretation_not_fact"] != true || surface["private_conflict_guard"] != true || surface["narrator_fact_reveal_blocked"] != true {
		t.Fatalf("character private surface missing PMC-20 guard fields: %+v", surface)
	}
	if surface["visible_to_player"] != false || surface["narrator_reveal_blocked"] != true {
		t.Fatalf("character private surface must block reveal: %+v", surface)
	}
	if len(fake.savedMemories) != 0 || len(fake.savedKGTriples) != 0 || len(fake.savedEvidence) != 0 {
		t.Fatalf("prepare-turn character private recollection must not write canonical rows: mem=%d kg=%d evi=%d", len(fake.savedMemories), len(fake.savedKGTriples), len(fake.savedEvidence))
	}
}

func TestPrepareTurnEntityRecollectionRelevanceFiltersUnrelatedNPCMemory(t *testing.T) {
	fake := &turnRecordingStore{
		returnChatLogs: []store.ChatLog{
			{ID: 1, ChatSessionID: "target-pmc19", TurnIndex: 3, Role: "user", Content: "Siwoo reaches the second exit corridor."},
			{ID: 2, ChatSessionID: "target-pmc19", TurnIndex: 3, Role: "assistant", Content: "Chloe stands beside him and studies the flickering sign."},
		},
		returnActiveStates: []store.ActiveState{
			{ID: 1, ChatSessionID: "target-pmc19", StateType: "scene", Content: `{"location":"second exit corridor","present_entities":["Siwoo","Chloe"]}`, TurnIndex: 3},
		},
		returnEntityMemories: []store.ProtagonistEntityMemory{
			{
				ID:                  31,
				OwnerEntityKey:      "chloe",
				OwnerEntityName:     "Chloe",
				OwnerEntityRole:     "npc",
				OwnerVisibility:     "owner_private",
				SourceChatSessionID: "target-pmc19",
				SourceTurn:          2,
				MemoryText:          "Chloe privately remembers that Siwoo noticed the exit sign changed.",
				TargetRevealPolicy:  "owner_private_until_revealed",
				Portability:         "npc_private_recollection",
				Importance10:        8,
			},
			{
				ID:                  32,
				OwnerEntityKey:      "saori",
				OwnerEntityName:     "Saori",
				OwnerEntityRole:     "npc",
				OwnerVisibility:     "owner_private",
				SourceChatSessionID: "target-pmc19",
				SourceTurn:          2,
				MemoryText:          "Saori privately remembers the kitchen confession from another scene.",
				TargetRevealPolicy:  "owner_private_until_revealed",
				Portability:         "npc_private_recollection",
				Importance10:        8,
			},
		},
		returnPersonaEntries: []store.PersonaMemoryEntry{
			{
				ID:              71,
				CapsuleID:       12,
				SourceTurn:      7,
				MemoryText:      "Saori remembers a private kitchen promise.",
				Importance10:    8,
				Portability:     "npc_private_recollection",
				TagsJSON:        `["owner_entity_key:saori","owner_entity_name:Saori","owner_entity_role:npc","owner_visibility:owner_private","source_chat_session_id:old-saori-session","npc_private"]`,
				InjectionPolicy: "support_only_npc_private_recollection",
			},
		},
	}
	cfg := config.Default()
	cfg.StoreMode = config.StoreModeDualShadow
	srv := NewServer(cfg)
	srv.Store = fake
	mux := http.NewServeMux()
	srv.RegisterRoutes(mux)

	body := `{"chat_session_id":"target-pmc19","turn_index":4,"raw_user_input":"Chloe quietly checks the second exit sign with Siwoo.","settings":{"max_injection_chars":1800,"max_input_context_chars":900,"injection_enabled":true,"input_context_enabled":true,"top_k":2}}`
	req := httptest.NewRequest(http.MethodPost, "/prepare-turn", strings.NewReader(body))
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
	injectionText, _ := resp["injection_text"].(string)
	if !strings.Contains(injectionText, "[Character Private Recollection]") || !strings.Contains(injectionText, "owner Chloe") {
		t.Fatalf("expected Chloe private recollection in injection text: %q", injectionText)
	}
	if strings.Contains(injectionText, "Saori") || strings.Contains(injectionText, "kitchen promise") || strings.Contains(injectionText, "kitchen confession") {
		t.Fatalf("unrelated Saori memory leaked into prepare-turn injection: %q", injectionText)
	}
	surface, ok := resp["character_private_recollection"].(map[string]any)
	if !ok {
		t.Fatalf("character_private_recollection surface missing")
	}
	if surface["status"] != "ready" || surface["count"] != float64(1) {
		t.Fatalf("expected exactly one relevant private recollection, got %+v", surface)
	}
	relevance, ok := resp["entity_recollection_relevance"].(map[string]any)
	if !ok {
		t.Fatalf("entity_recollection_relevance surface missing")
	}
	if relevance["character_private_before_filter"] != float64(3) || relevance["character_private_after_filter"] != float64(1) || relevance["character_private_dropped_count"] != float64(2) {
		t.Fatalf("unexpected relevance counts: %+v", relevance)
	}
	if relevance["blocks_unrelated_session_memory"] != true || relevance["blocks_unrelated_entity_memory"] != true {
		t.Fatalf("relevance surface must expose unrelated memory guards: %+v", relevance)
	}
}

func TestPrepareTurnAttachedNPCPrivateCapsuleUsesCharacterPrivateLane(t *testing.T) {
	fake := &turnRecordingStore{
		returnChatLogs: []store.ChatLog{
			{ID: 1, ChatSessionID: "target-attached-npc", TurnIndex: 1, Role: "user", Content: "Siwoo enters the old classroom."},
			{ID: 2, ChatSessionID: "target-attached-npc", TurnIndex: 1, Role: "assistant", Content: "Chloe becomes unusually careful."},
		},
		returnPersonaEntries: []store.PersonaMemoryEntry{
			{
				ID:              42,
				CapsuleID:       9,
				SourceTurn:      6,
				MemoryText:      "Chloe remembers from a previous loop that Siwoo should avoid the broken bridge.",
				Importance10:    8.5,
				EmotionalWeight: 0.7,
				Portability:     "npc_private_recollection",
				TagsJSON:        `["owner_entity_key:chloe","owner_entity_name:Chloe","owner_entity_role:npc","owner_visibility:owner_private","source_chat_session_id:old-chloe-session","target_reveal_policy:owner_private_until_revealed","npc_private","secret_guard"]`,
				EvidenceExcerpt: "avoid the broken bridge",
				InjectionPolicy: "support_only_npc_private_recollection",
			},
		},
	}
	cfg := config.Default()
	cfg.StoreMode = config.StoreModeDualShadow
	srv := NewServer(cfg)
	srv.Store = fake
	mux := http.NewServeMux()
	srv.RegisterRoutes(mux)

	body := `{"chat_session_id":"target-attached-npc","turn_index":2,"raw_user_input":"He asks Chloe if they should cross the bridge.","settings":{"max_injection_chars":1400,"max_input_context_chars":900,"injection_enabled":true,"input_context_enabled":true,"top_k":2}}`
	req := httptest.NewRequest(http.MethodPost, "/prepare-turn", strings.NewReader(body))
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
	injectionText, _ := resp["injection_text"].(string)
	if !strings.Contains(injectionText, "[Character Private Recollection]") || !strings.Contains(injectionText, "owner Chloe") {
		t.Fatalf("attached NPC capsule did not enter character private lane: %q", injectionText)
	}
	if strings.Contains(injectionText, "[Persona Recollection]") {
		t.Fatalf("attached NPC capsule leaked into persona lane: %q", injectionText)
	}
	if strings.Contains(injectionText, "previous loop") {
		t.Fatalf("attached NPC capsule leaked explicit loop wording: %q", injectionText)
	}
	surface, ok := resp["character_private_recollection"].(map[string]any)
	if !ok {
		t.Fatalf("character_private_recollection surface missing")
	}
	if surface["status"] != "ready" || surface["count"] != float64(1) || surface["visible_to_player"] != false {
		t.Fatalf("character private attached capsule surface mismatch: %+v", surface)
	}
	items, ok := surface["items"].([]any)
	if !ok || len(items) != 1 {
		t.Fatalf("character private items missing: %+v", surface["items"])
	}
	first, ok := items[0].(map[string]any)
	if !ok || first["owner_entity_key"] != "chloe" || first["owner_visibility"] != "owner_private" {
		t.Fatalf("attached NPC capsule owner metadata not restored: %+v", first)
	}
	personaSurface, ok := resp["persona_recollection"].(map[string]any)
	if !ok || personaSurface["status"] != "empty" || personaSurface["count"] != float64(0) {
		t.Fatalf("persona surface should remain empty for NPC-private capsule: %+v", personaSurface)
	}
}

func TestPrepareTurnEpisodeDenseAnchorsSurviveSummaryText(t *testing.T) {
	episodes := []store.EpisodeSummary{
		{
			ID:                      42,
			ChatSessionID:           "sess-ds1a",
			FromTurn:                1,
			ToTurn:                  2,
			SummaryText:             "A short episode summary.",
			KeyEvents:               `["Alice opens the sealed gate"]`,
			RelationshipChangesJSON: `["Alice trusts Bob"]`,
			OpenLoopsJSON:           `["sealed gate remains unresolved"]`,
			CreatedAt:               time.Unix(20, 0),
		},
	}
	assembly := buildPrepareTurnInjectionAssembly(nil, nil, nil, nil, nil, nil, nil, nil, nil, episodes, nil, nil, nil, 5, 1200, "", "wide_context_700k", nil, nil, nil)
	if !strings.Contains(assembly.EpisodeText, "key_event=Alice opens the sealed gate") {
		t.Fatalf("episode_text missing key event anchor: %s", assembly.EpisodeText)
	}
	if !strings.Contains(assembly.EpisodeText, "rel=Alice trusts Bob") {
		t.Fatalf("episode_text missing relationship anchor: %s", assembly.EpisodeText)
	}
	if !strings.Contains(assembly.EpisodeText, "open_loop=sealed gate remains unresolved") {
		t.Fatalf("episode_text missing open-loop anchor: %s", assembly.EpisodeText)
	}
}

func TestUnifiedRetrievalDocumentsEpisodeDenseAnchors(t *testing.T) {
	episodes := []store.EpisodeSummary{
		{
			ID:                      42,
			ChatSessionID:           "sess-ds1a",
			FromTurn:                1,
			ToTurn:                  2,
			SummaryText:             "A short episode summary.",
			KeyEvents:               `["Alice opens the sealed gate"]`,
			RelationshipChangesJSON: `["Alice trusts Bob"]`,
			OpenLoopsJSON:           `["sealed gate remains unresolved"]`,
			CreatedAt:               time.Unix(20, 0),
		},
	}
	docs := buildUnifiedRetrievalDocuments("sess-ds1a", nil, nil, nil, episodes, nil, nil)
	if len(docs) != 1 {
		t.Fatalf("docs = %d, want 1", len(docs))
	}
	text := extractionStringFromAny(docs[0]["text"])
	if !strings.Contains(text, "Alice opens the sealed gate") || !strings.Contains(text, "Alice trusts Bob") || !strings.Contains(text, "sealed gate remains unresolved") {
		t.Fatalf("episode retrieval document text missing dense anchors: %s", text)
	}
	meta, _ := docs[0]["metadata"].(map[string]any)
	if meta["key_events"] == "" || meta["relationship_changes_json"] == "" || meta["open_loops_json"] == "" {
		t.Fatalf("episode retrieval metadata missing dense anchors: %+v", meta)
	}
}

func TestUnifiedRetrievalDocumentsDenseSummaryMetadataSurfaces(t *testing.T) {
	now := time.Unix(30, 0)
	evidence := []store.DirectEvidence{
		{
			ID:              9,
			ChatSessionID:   "sess-ds1f-docs",
			EvidenceKind:    "relationship_world_promise",
			EvidenceText:    "Alice and Bob promise to obey the gate law.",
			SourceTurnStart: 10,
			SourceTurnEnd:   80,
			TurnAnchor:      40,
		},
	}
	episodes := []store.EpisodeSummary{
		{
			ID:                      42,
			ChatSessionID:           "sess-ds1f-docs",
			FromTurn:                10,
			ToTurn:                  20,
			SummaryText:             "A short episode summary.",
			KeyEvents:               `["Alice opens the sealed gate"]`,
			RelationshipChangesJSON: `["Alice trusts Bob"]`,
			OpenLoopsJSON:           `["sealed gate remains unresolved"]`,
			CreatedAt:               now,
		},
	}
	resumePack := &store.ResumePack{
		Chapter: &store.ChapterSummary{
			ID:                      50,
			ChatSessionID:           "sess-ds1f-docs",
			FromTurn:                21,
			ToTurn:                  40,
			ChapterIndex:            2,
			ChapterTitle:            "Gate Promise",
			SummaryText:             "The gate promise becomes explicit.",
			ResumeText:              "The gate promise remains unresolved.",
			RelationshipChangesJSON: `["Alice and Bob form an alliance"]`,
			WorldChangesJSON:        `["gate law changes the archive"]`,
			CallbackCandidatesJSON:  `["repay the promise"]`,
			CreatedAt:               &now,
		},
		Arc: &store.ArcSummary{
			ID:                     60,
			ChatSessionID:          "sess-ds1f-docs",
			FromTurn:               41,
			ToTurn:                 70,
			ArcName:                "Gate Arc",
			CoreConflict:           "The gate law binds the group.",
			ActivePromisesJSON:     `["protect the gate"]`,
			RelationshipPivotsJSON: `["Alice trusts Bob"]`,
			CreatedAt:              &now,
		},
		Saga: &store.SagaDigest{
			ID:                      70,
			ChatSessionID:           "sess-ds1f-docs",
			FromTurn:                1,
			ToTurn:                  80,
			EraLabel:                "Gate Era",
			SagaSummary:             "The gate law shapes the era.",
			PersistentFactsJSON:     `["gate law persists"]`,
			NeverDropCandidatesJSON: `["Alice and Bob promise remains"]`,
			CreatedAt:               &now,
		},
	}
	docs := buildUnifiedRetrievalDocuments("sess-ds1f-docs", nil, evidence, nil, episodes, resumePack, nil)
	byTier := map[string]map[string]any{}
	for _, doc := range docs {
		tier, _ := doc["tier"].(string)
		byTier[tier] = doc
	}
	for _, tier := range []string{"episode", "chapter", "arc", "saga"} {
		doc, ok := byTier[tier]
		if !ok {
			t.Fatalf("missing %s doc in %+v", tier, byTier)
		}
		meta, ok := doc["metadata"].(map[string]any)
		if !ok {
			t.Fatalf("%s metadata missing: %+v", tier, doc)
		}
		if meta["dense_source_anchor_policy_version"] != denseSourceAnchorPolicyVersion {
			t.Fatalf("%s DS-1f source anchor missing: %+v", tier, meta)
		}
		if meta["dense_role_split_policy_version"] != denseRoleSplitPolicyVersion || meta["dense_structured_usage"] != "adjudication_retrieval" {
			t.Fatalf("%s DS-1h role split missing: %+v", tier, meta)
		}
		if meta["dense_retention_policy_version"] != denseRetentionPolicyVersion || meta["dense_retention_applied"] != true {
			t.Fatalf("%s DS-1g retention missing: %+v", tier, meta)
		}
		if meta["dense_direct_evidence_promotion_policy_version"] != denseEvidencePromotionPolicy || meta["dense_structured_precedence_applied"] != true {
			t.Fatalf("%s DS-1i promotion missing: %+v", tier, meta)
		}
	}
}

func TestPrepareTurnNarrativeGuideAutoModeBundle(t *testing.T) {
	mux := http.NewServeMux()
	srv := setupTestServer()
	srv.RegisterRoutes(mux)

	body := `{"chat_session_id":"sess-guide","turn_index":1,"raw_user_input":"If combat starts, immediately engage and defend.","settings":{"guide_mode":"auto","guide_strength":"strong","injection_enabled":false,"input_context_enabled":false}}`
	req := httptest.NewRequest(http.MethodPost, "/prepare-turn", bytes.NewReader([]byte(body)))
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
	pack, ok := resp["supervisor_input_pack"].(map[string]any)
	if !ok {
		t.Fatalf("supervisor_input_pack is not an object: %+v", resp)
	}
	if pack["guide_mode"] != "action" {
		t.Fatalf("guide_mode = %v, want action", pack["guide_mode"])
	}
	if pack["guide_strength"] != "strong" {
		t.Fatalf("guide_strength = %v, want strong", pack["guide_strength"])
	}
	if suffix, _ := pack["guide_suffix"].(string); !strings.Contains(suffix, "Narrative Guide") || !strings.Contains(suffix, "Action") || !strings.Contains(suffix, "Strength: strong") {
		t.Fatalf("guide_suffix missing action suffix: %q", suffix)
	}
	if guidance, _ := pack["persistent_guidance"].(string); !strings.Contains(guidance, "Narrative Guide") || !strings.Contains(guidance, "combat/chase") {
		t.Fatalf("persistent_guidance missing guide suffix: %q", guidance)
	}
	overrides, ok := pack["director_overrides"].(map[string]any)
	if !ok {
		t.Fatalf("director_overrides is not an object: %+v", pack["director_overrides"])
	}
	emphasis, _ := overrides["emphasis"].([]any)
	if len(emphasis) == 0 {
		t.Fatalf("director_overrides.emphasis is empty: %+v", overrides)
	}

	autonomyPlan, ok := resp["autonomy_plan"].(map[string]any)
	if !ok {
		t.Fatalf("autonomy_plan is not an object")
	}
	if autonomyPlan["guide_mode"] != "action" {
		t.Fatalf("autonomy_plan.guide_mode = %v, want action", autonomyPlan["guide_mode"])
	}
}

func TestNarrativeGuideModeSuffixAndDirectorOverrides(t *testing.T) {
	if got := resolveNarrativeGuideMode("auto", []map[string]any{{"role": "user", "content": "The romantic mood deepens and the scene moves closer."}}, "", ""); got != "romantic" {
		t.Fatalf("resolveNarrativeGuideMode(auto romantic) = %q, want romantic", got)
	}
	if suffix := buildGuideModeSuffix("mature_soft", "medium"); !strings.Contains(suffix, "Mature (Sensual)") || !strings.Contains(suffix, "story-appropriate") || !strings.Contains(suffix, "Strength: medium") {
		t.Fatalf("mature_soft suffix mismatch: %q", suffix)
	}
	overrides := buildGuideModeDirectorOverrides("mature_direct")
	forbidden, _ := overrides["forbidden_moves"].([]string)
	if len(forbidden) == 0 || forbidden[0] != "dehumanizing portrayals" {
		t.Fatalf("mature_direct forbidden overrides mismatch: %+v", overrides)
	}
	if suffix := buildGuideModeSuffix("strict"); suffix != "" {
		t.Fatalf("unknown guide mode should not create suffix, got %q", suffix)
	}
}

func TestPrepareTurnCharacterBlockIncludesSpeechStyle(t *testing.T) {
	assembly := buildPrepareTurnInjectionAssembly(
		nil, nil, nil, nil, nil, nil,
		[]store.CharacterState{{
			ChatSessionID:     "sess-speech",
			CharacterName:     "Chloe",
			StatusJSON:        `{"mood":"guarded"}`,
			SpeechStyleJSON:   `{"default_tone":"dry","speech_notes":"short replies"}`,
			RelationshipsJSON: `{"Hero":{"affection":35}}`,
		}},
		nil, nil, nil,
		nil,
		nil,
		nil,
		3, 1000,
		"",
		"default",
		nil,
		nil,
		nil,
	)
	if !strings.Contains(assembly.CharacterText, "speech_style") || !strings.Contains(assembly.CharacterText, "dry") || !strings.Contains(assembly.CharacterText, "short replies") {
		t.Fatalf("character block should include speech style guidance, got %q", assembly.CharacterText)
	}
	if !strings.Contains(assembly.Text, "[Characters]") || !strings.Contains(assembly.Text, "speech_style") {
		t.Fatalf("injection text should carry speech style inside character block, got %q", assembly.Text)
	}
}

func TestPrepareTurnVectorHitsHydrateIntoMemoryLane(t *testing.T) {
	memories := []store.Memory{
		{
			ID:          1,
			TurnIndex:   1,
			SummaryJSON: `{"turn_summary":"Mina hid the brass key under the old shrine.","language_context":{"contract_version":"language_memory.v1","session_output_language":"en","raw_user_language":"ko","summary_language":"en","search_text_policy":"summary_plus_raw_plus_aliases"},"entities":[{"name":"Mina","aliases":["미나"]}],"archive_hint":{"wing":"North Wing"}}`,
			Evidence:    `{"evidence_excerpts":["RAW-KO: Mina hid the brass key."]}`,
			Importance:  5,
		},
		{
			ID:          2,
			TurnIndex:   9,
			SummaryJSON: `{"turn_summary":"Unrelated recent market conversation."}`,
			Importance:  9,
		},
	}
	vectorShadow := map[string]any{
		"search_result": "ok",
		"search_results": []map[string]any{
			{
				"id":                      "memory:sess-vector:1",
				"source_table":            "memories",
				"source_row_id":           "1",
				"search_text_policy":      "summary_plus_raw_plus_aliases",
				"raw_language":            "ko",
				"summary_language":        "en",
				"session_output_language": "en",
				"alias_count":             3,
			},
		},
	}

	selection := selectPrepareTurnMemoryLanesWithVector(memories, "Where is the key?", 1, vectorShadow)
	if len(selection.VectorRelevant) != 1 {
		t.Fatalf("vector relevant count = %d, want 1; trace=%#v", len(selection.VectorRelevant), selection.Trace)
	}
	if selection.VectorRelevant[0].ID != 1 {
		t.Fatalf("vector relevant id = %d, want old semantic memory id 1", selection.VectorRelevant[0].ID)
	}
	if got := prepareTurnSelectedMemoryCount(selection); got != 1 {
		t.Fatalf("selected count = %d, want topK-limited 1", got)
	}
	if len(selection.Recent)+len(selection.Relevant)+len(selection.Deep) != 0 {
		t.Fatalf("vector-selected memory should consume topK slot without duplicate fallback lanes: %#v", selection)
	}
	assembly := buildPrepareTurnInjectionAssembly(memories, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, 1, 2000, "Where is the key?", "default", nil, vectorShadow, nil)
	if !strings.Contains(assembly.MemoryText, "[vector_relevant, turn 1") || !strings.Contains(assembly.MemoryText, "Mina hid the brass key") {
		t.Fatalf("memory_text should expose vector_relevant lane and hydrated memory: %q", assembly.MemoryText)
	}
	for key, want := range map[string]int{
		"vector_memory_hit_count":                       1,
		"vector_memory_hydrated_count":                  1,
		"vector_memory_selected_count":                  1,
		"vector_memory_injected_count":                  1,
		"vector_memory_hit_language_context_count":      1,
		"vector_memory_hit_alias_indexed_count":         1,
		"vector_memory_hydrated_language_context_count": 1,
		"vector_memory_hydrated_alias_ready_count":      1,
		"vector_relevant_memory_count":                  1,
		"selected_memory_total_count":                   1,
	} {
		if got := intFromAny(assembly.Counts[key], 0); got != want {
			t.Fatalf("assembly.Counts[%s] = %d, want %d; counts=%#v", key, got, want, assembly.Counts)
		}
	}
	if got := stringFromMap(assembly.Counts, "vector_memory_search_text_policy"); got != "summary_plus_raw_plus_aliases" {
		t.Fatalf("vector_memory_search_text_policy = %q, counts=%#v", got, assembly.Counts)
	}
}

func TestPrepareTurnVectorReadyDoesNotFillTopKWithLexicalRecent(t *testing.T) {
	memories := []store.Memory{
		{ID: 1, TurnIndex: 1, SummaryJSON: `{"turn_summary":"Mina hid the brass key under the old shrine."}`, Importance: 5},
		{ID: 2, TurnIndex: 9, SummaryJSON: `{"turn_summary":"Recent unrelated market conversation."}`, Importance: 9},
		{ID: 3, TurnIndex: 10, SummaryJSON: `{"turn_summary":"Another unrelated recent tail."}`, Importance: 8},
	}
	vectorShadow := map[string]any{
		"search_result": "ok",
		"search_results": []map[string]any{
			{"id": "memory:sess-vector:1", "source_table": "memories", "source_row_id": "1"},
		},
	}

	selection := selectPrepareTurnMemoryLanesWithVector(memories, "Where is the key?", 3, vectorShadow)
	if len(selection.VectorRelevant) != 1 {
		t.Fatalf("vector relevant count = %d, want 1", len(selection.VectorRelevant))
	}
	if len(selection.Relevant)+len(selection.Deep)+len(selection.Recent) != 0 {
		t.Fatalf("Chroma-ready recall must not fill topK with lexical/recent lanes: %#v", selection)
	}
	if got := prepareTurnSelectedMemoryCount(selection); got != 1 {
		t.Fatalf("selected count = %d, want only hydrated Chroma hit", got)
	}
	if selection.Trace["lexical_fill_enabled"] != false || selection.Trace["vector_recall_ready"] != true {
		t.Fatalf("vector-ready trace mismatch: %#v", selection.Trace)
	}
}

func TestStep29RegressionPrepareTurnVectorIsInjectedNotShadowOnly(t *testing.T) {
	fake := &turnRecordingStore{
		returnMemories: []store.Memory{
			{ID: 1, ChatSessionID: "sess-29-shadow-only", TurnIndex: 1, SummaryJSON: `{"turn_summary":"Old semantic oath belongs in the next prompt."}`, Importance: 0.1},
			{ID: 2, ChatSessionID: "sess-29-shadow-only", TurnIndex: 20, SummaryJSON: `{"turn_summary":"Recent unrelated weather note."}`, Importance: 0.9},
		},
	}
	cfg := config.Default()
	cfg.StoreMode = config.StoreModeDualShadow
	cfg.ChromaEndpoint = "http://127.0.0.1:8000"
	cfg.Readiness.ChromaConfigured = true
	srv := NewServer(cfg)
	srv.Store = fake
	srv.Vector = &fakeVectorStore{
		healthSnapshot: vector.HealthSnapshot{Status: "ok", TotalCount: 2, ModelReady: true},
		searchResults: []vector.VectorDocument{
			{ID: "memory:sess-29-shadow-only:1", Tier: "memory", ChatSessionID: "sess-29-shadow-only", SourceTable: "memories", SourceRowID: "1", DocumentText: "old semantic oath"},
		},
	}
	srv.VectorOpenError = nil

	mux := http.NewServeMux()
	srv.RegisterRoutes(mux)

	body := `{
		"chat_session_id":"sess-29-shadow-only",
		"turn_index":21,
		"raw_user_input":"Continue the old oath.",
		"client_meta":{"chroma_query_vector":[0.1,0.2]},
		"settings":{"max_injection_chars":1200,"injection_enabled":true,"input_context_enabled":false,"top_k":1}
	}`
	req := httptest.NewRequest(http.MethodPost, "/prepare-turn", bytes.NewReader([]byte(body)))
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
	injectionPack, ok := resp["injection_pack"].(map[string]any)
	if !ok {
		t.Fatalf("injection_pack missing: %#v", resp["injection_pack"])
	}
	memoryText, _ := injectionPack["memory_text"].(string)
	if !strings.Contains(memoryText, "[vector_relevant, turn 1") || !strings.Contains(memoryText, "Old semantic oath") {
		t.Fatalf("memory_text did not inject hydrated vector memory: %q", memoryText)
	}
	if strings.Contains(memoryText, "Recent unrelated weather note") {
		t.Fatalf("recent unrelated memory should not replace vector topK slot: %q", memoryText)
	}
	counts := injectionPack["counts"].(map[string]any)
	for key, want := range map[string]float64{
		"vector_memory_hit_count":      1,
		"vector_memory_hydrated_count": 1,
		"vector_memory_selected_count": 1,
		"vector_memory_injected_count": 1,
		"selected_memory_total_count":  1,
	} {
		if got := counts[key]; got != want {
			t.Fatalf("injection_pack.counts[%s] = %v, want %v; counts=%#v", key, got, want, counts)
		}
	}
	recall := resp["recall_result"].(map[string]any)
	searchBundle := recall["search"].(map[string]any)
	if searchBundle["memory_count"] != float64(1) || searchBundle["fallback_count"] != float64(0) {
		t.Fatalf("recall_result.search should expose injected vector memory count, got %#v", searchBundle)
	}
	vectorShadow := recall["vector_shadow"].(map[string]any)
	if vectorShadow["search_result"] != "ok" {
		t.Fatalf("vector_shadow.search_result = %v, want ok", vectorShadow["search_result"])
	}
}

func TestPrepareTurnInputTransparencyRenderModelExposesSafeBlocksAndCounters(t *testing.T) {
	fake := &turnRecordingStore{
		returnMemories: []store.Memory{
			{
				ID:            1,
				ChatSessionID: "sess-247a-render",
				TurnIndex:     7,
				SummaryJSON: `{
					"turn_summary":"Gloria privately inherited the sealed crest.",
					"protected_secrets":[{
						"secret_kind":"power_inheritance",
						"secret_summary":"Gloria privately inherited the sealed crest.",
						"disclosure_policy":"owner_private_until_revealed",
						"knowledge_scope":{"known_by":["Gloria"],"unknown_to":["Siwoo"]}
					}],
					"character_identity_accuracy":[{
						"surface_identity_name":"Lia",
						"true_identity_name":"Gloria",
						"canonical_entity_name":"Gloria",
						"identity_kind":"cover_identity",
						"same_entity":true,
						"reveal_policy":"owner_private_until_revealed",
						"knowledge_scope":{"known_by":["Gloria"],"unknown_to":["Siwoo"]}
					}]
				}`,
				Importance: 0.95,
			},
			{ID: 2, ChatSessionID: "sess-247a-render", TurnIndex: 8, SummaryJSON: `{"turn_summary":"Recent unrelated market note."}`, Importance: 0.9},
		},
	}
	cfg := config.Default()
	cfg.StoreMode = config.StoreModeDualShadow
	cfg.ChromaEndpoint = "http://127.0.0.1:8000"
	cfg.Readiness.ChromaConfigured = true
	srv := NewServer(cfg)
	srv.Store = fake
	srv.Vector = &fakeVectorStore{
		healthSnapshot: vector.HealthSnapshot{Status: "ok", TotalCount: 2, ModelReady: true},
		searchResults: []vector.VectorDocument{
			{ID: "memory:sess-247a-render:1", Tier: "memory", ChatSessionID: "sess-247a-render", SourceTable: "memories", SourceRowID: "1", DocumentText: "sealed crest protected identity"},
		},
	}
	srv.VectorOpenError = nil

	mux := http.NewServeMux()
	srv.RegisterRoutes(mux)

	body := `{
		"chat_session_id":"sess-247a-render",
		"turn_index":9,
		"raw_user_input":"Continue the private scene carefully.",
		"client_meta":{"chroma_query_vector":[0.4,0.2]},
		"settings":{"apply_mode":"shadow","max_injection_chars":3000,"injection_enabled":true,"input_context_enabled":false,"top_k":1}
	}`
	req := httptest.NewRequest(http.MethodPost, "/prepare-turn", strings.NewReader(body))
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
	model := mapFromAny(resp["input_transparency_model"])
	if model["contract_version"] != "input_transparency_render.v1" || model["read_only"] != true || model["llm_call_attempted"] != false {
		t.Fatalf("input_transparency_model contract mismatch: %#v", model)
	}
	if model["secret_display_policy"] != "counts_only_no_secret_text" {
		t.Fatalf("secret_display_policy = %v", model["secret_display_policy"])
	}
	counts := mapFromAny(model["counts"])
	for key, want := range map[string]int{
		"vector_found":                   1,
		"vector_hydrated":                1,
		"vector_injected":                1,
		"memory_injected":                1,
		"protected_secret_count":         1,
		"identity_accuracy_count":        1,
		"protected_memory_guarded_count": 1,
	} {
		if got := intFromAny(counts[key], 0); got != want {
			t.Fatalf("input_transparency_model.counts[%s] = %d, want %d; counts=%#v", key, got, want, counts)
		}
	}
	related := map[string]any(nil)
	for _, raw := range sliceFromAny(model["blocks"]) {
		block := mapFromAny(raw)
		if block["key"] == "related_memories" {
			related = block
			break
		}
	}
	if related == nil || related["status"] != "included" {
		t.Fatalf("related_memories block missing or not included: %#v", model["blocks"])
	}
	relatedText := extractionStringFromAny(related["text"])
	if !strings.Contains(relatedText, "Protected identity continuity") || !strings.Contains(relatedText, "kind=cover_identity") {
		t.Fatalf("related memory block should contain protected guard text, got %q", relatedText)
	}
	modelJSON := mustCompactJSON(model)
	for _, leaked := range []string{"Gloria privately inherited the sealed crest", "secret_summary", "true_identity_name", "surface_identity_name"} {
		if strings.Contains(modelJSON, leaked) {
			t.Fatalf("input_transparency_model leaked protected detail %q: %s", leaked, modelJSON)
		}
	}
	preview := mapFromAny(resp["effective_input_preview"])
	if preview["contract_version"] != "effective_input_preview.v1" || preview["payload_apply_mode"] != "shadow" || preview["raw_user_rewritten"] != false {
		t.Fatalf("effective_input_preview contract mismatch: %#v", preview)
	}
	if intFromAny(preview["auxiliary_context_chars"], 0) <= 0 {
		t.Fatalf("effective_input_preview auxiliary_context_chars not populated: %#v", preview)
	}
}

func TestPrepareTurnVectorHydrationUsesVectorIDFallbackAndFiltersNonMemory(t *testing.T) {
	memories := []store.Memory{
		{
			ID:          4,
			TurnIndex:   4,
			SummaryJSON: `{"turn_summary":"The shrine key was wrapped in red cloth."}`,
			Importance:  6,
		},
	}
	vectorShadow := map[string]any{
		"search_result": "ok",
		"search_results": []map[string]any{
			{
				"id":    "episode:sess-vector:99",
				"tier":  "episode",
				"score": 0.99,
			},
			{
				"id":   "memory:sess-vector:4",
				"tier": "memory",
			},
			{
				"id":   "memory:sess-vector:4",
				"tier": "memory",
			},
		},
	}

	selection := selectPrepareTurnMemoryLanesWithVector(memories, "red cloth key", 3, vectorShadow)
	if len(selection.VectorRelevant) != 1 || selection.VectorRelevant[0].ID != 4 {
		t.Fatalf("expected one hydrated memory from vector id fallback, got %#v", selection.VectorRelevant)
	}
	trace := mapFromAny(selection.Trace["vector_recall"])
	if got := intFromAny(trace["non_memory_count"], 0); got != 1 {
		t.Fatalf("non_memory_count = %d, want 1; trace=%#v", got, trace)
	}
	if got := intFromAny(trace["duplicate_count"], 0); got != 1 {
		t.Fatalf("duplicate_count = %d, want 1; trace=%#v", got, trace)
	}
	if got := intFromAny(trace["hydrated_count"], 0); got != 1 {
		t.Fatalf("hydrated_count = %d, want 1; trace=%#v", got, trace)
	}
}

func TestPrepareTurnStorylineSelectionPreventsStaleAmplification(t *testing.T) {
	fake := &turnRecordingStore{
		returnStorylines: []store.Storyline{
			{ID: 1, ChatSessionID: "sess-e1f", Name: "Fresh confrontation", Status: "active", CurrentContext: "Fresh confrontation escalates near the gate", Confidence: 0.82, EvidenceCount: 3, LastEvidenceTurn: 11, LastTurn: 11},
			{ID: 2, ChatSessionID: "sess-e1f", Name: "Old corridor rumor", Status: "active", CurrentContext: "Old corridor rumor repeats without evidence", Confidence: 0.9, EvidenceCount: 1, LastEvidenceTurn: 1, LastTurn: 1},
			{ID: 3, ChatSessionID: "sess-e1f", Name: "Resolved apology", Status: "resolved", CurrentContext: "Resolved apology should stay compressed", Confidence: 0.7, EvidenceCount: 2, LastEvidenceTurn: 8, LastTurn: 8},
			{ID: 4, ChatSessionID: "sess-e1f", Name: "Suppressed detour", Status: "active", CurrentContext: "Suppressed detour must not enter prompt", Confidence: 1, EvidenceCount: 5, LastEvidenceTurn: 12, LastTurn: 12, Suppressed: true},
		},
	}
	cfg := config.Default()
	cfg.StoreMode = config.StoreModeDualShadow
	srv := NewServer(cfg)
	srv.Store = fake
	mux := http.NewServeMux()
	srv.RegisterRoutes(mux)

	body := `{"chat_session_id":"sess-e1f","turn_index":12,"raw_user_input":"continue","settings":{"max_injection_chars":800,"injection_enabled":true,"input_context_enabled":false,"top_k":5}}`
	req := httptest.NewRequest(http.MethodPost, "/prepare-turn", strings.NewReader(body))
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
	if selection["selected_count"] != float64(1) {
		t.Fatalf("selected_count = %v, want 1: %#v", selection["selected_count"], selection)
	}
	if selection["stale_dropped_count"] != float64(1) {
		t.Fatalf("stale_dropped_count = %v, want 1: %#v", selection["stale_dropped_count"], selection)
	}
	if selection["suppressed_count"] != float64(1) {
		t.Fatalf("suppressed_count = %v, want 1: %#v", selection["suppressed_count"], selection)
	}

	contextText, _ := pack["storylines_context"].(string)
	if !strings.Contains(contextText, "Fresh confrontation") {
		t.Fatalf("storylines_context missing fresh storyline: %q", contextText)
	}
	for _, forbidden := range []string{"Old corridor rumor repeats", "Suppressed detour must not enter prompt"} {
		if strings.Contains(contextText, forbidden) {
			t.Fatalf("storylines_context contains forbidden stale/suppressed text %q: %q", forbidden, contextText)
		}
	}

	injectionPack := resp["injection_pack"].(map[string]any)
	storylineText, _ := injectionPack["storyline_text"].(string)
	if !strings.Contains(storylineText, "Fresh confrontation") {
		t.Fatalf("storyline_text missing fresh storyline: %q", storylineText)
	}
	if strings.Contains(storylineText, "Old corridor rumor") || strings.Contains(storylineText, "Suppressed detour") || strings.Contains(storylineText, "Resolved apology should stay compressed") {
		t.Fatalf("storyline_text contains stale/resolved/suppressed storyline: %q", storylineText)
	}
}

func TestPrepareTurnBundleIncludesFallbackChatLogsWhenMemoriesAreThin(t *testing.T) {
	fake := &turnRecordingStore{
		returnMemories: []store.Memory{
			{ID: 1, ChatSessionID: "sess-fallback", TurnIndex: 1, SummaryJSON: `{"turn_summary":"Only one memory is available"}`, Importance: 0.5},
		},
		returnKGTriples: []store.KGTriple{
			{ID: 10, ChatSessionID: "sess-fallback", Subject: "Door", Predicate: "hides", Object: "key"},
		},
		returnChatLogs: []store.ChatLog{
			{ID: 100, ChatSessionID: "sess-fallback", TurnIndex: 1, Role: "user", Content: "I check the hallway."},
			{ID: 101, ChatSessionID: "sess-fallback", TurnIndex: 1, Role: "assistant", Content: "The hallway has a brass key under the rug."},
		},
	}

	cfg := config.Default()
	cfg.StoreMode = config.StoreModeDualShadow
	srv := NewServer(cfg)
	srv.Store = fake

	mux := http.NewServeMux()
	srv.RegisterRoutes(mux)

	body := `{"chat_session_id":"sess-fallback","turn_index":2,"raw_user_input":"Use the key","settings":{"max_injection_chars":900,"max_input_context_chars":400,"injection_enabled":true,"input_context_enabled":true,"top_k":2}}`
	req := httptest.NewRequest(http.MethodPost, "/prepare-turn", bytes.NewReader([]byte(body)))
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

	injectionPack, ok := resp["injection_pack"].(map[string]any)
	if !ok {
		t.Fatalf("injection_pack is not an object")
	}
	fallbackText, _ := injectionPack["fallback_text"].(string)
	if !strings.Contains(fallbackText, "brass key under the rug") {
		t.Fatalf("fallback_text missing recent chat fallback: %q", fallbackText)
	}
	budget, ok := injectionPack["budget_decisions"].(map[string]any)
	if !ok {
		t.Fatalf("budget_decisions is not an object")
	}
	if budget["fallback_chat_log_included"] != true {
		t.Errorf("fallback_chat_log_included = %v, want true", budget["fallback_chat_log_included"])
	}
	if budget["fallback_reason"] != "memory_below_threshold" {
		t.Errorf("fallback_reason = %v, want memory_below_threshold", budget["fallback_reason"])
	}
	packCounts, ok := injectionPack["counts"].(map[string]any)
	if !ok {
		t.Fatalf("injection_pack.counts is not an object")
	}
	if packCounts["memory_count"] != float64(1) || packCounts["fallback_count"] != float64(2) {
		t.Errorf("injection_pack counts memory/fallback = %v/%v, want 1/2", packCounts["memory_count"], packCounts["fallback_count"])
	}

	recall, ok := resp["recall_result"].(map[string]any)
	if !ok {
		t.Fatalf("recall_result is not an object")
	}
	items, ok := recall["items"].([]any)
	if !ok {
		t.Fatalf("recall_result.items is not an array")
	}
	foundFallbackItem := false
	for _, item := range items {
		m, _ := item.(map[string]any)
		if m["source"] == "chat_log" && strings.Contains(fmt.Sprint(m["content"]), "brass key") {
			foundFallbackItem = true
			break
		}
	}
	if !foundFallbackItem {
		t.Fatalf("recall_result.items missing source=chat_log fallback item: %#v", items)
	}
	recallCounts, ok := recall["counts"].(map[string]any)
	if !ok {
		t.Fatalf("recall_result.counts is not an object")
	}
	if recallCounts["memory_count"] != float64(1) || recallCounts["fallback_count"] != float64(2) {
		t.Errorf("recall_result counts memory/fallback = %v/%v, want 1/2", recallCounts["memory_count"], recallCounts["fallback_count"])
	}
}

func TestPrepareTurnTopKPrioritizesRelevantMemoryOverRecentTail(t *testing.T) {
	fake := &turnRecordingStore{
		returnMemories: []store.Memory{
			{ID: 1, ChatSessionID: "sess-recall-lanes", TurnIndex: 1, SummaryJSON: `{"turn_summary":"brass key old one"}`},
			{ID: 2, ChatSessionID: "sess-recall-lanes", TurnIndex: 2, SummaryJSON: `{"turn_summary":"brass key old two"}`},
			{ID: 3, ChatSessionID: "sess-recall-lanes", TurnIndex: 3, SummaryJSON: `{"turn_summary":"brass key old three"}`},
			{ID: 4, ChatSessionID: "sess-recall-lanes", TurnIndex: 4, SummaryJSON: `{"turn_summary":"recent unrelated four"}`},
			{ID: 5, ChatSessionID: "sess-recall-lanes", TurnIndex: 5, SummaryJSON: `{"turn_summary":"recent unrelated five"}`},
			{ID: 6, ChatSessionID: "sess-recall-lanes", TurnIndex: 6, SummaryJSON: `{"turn_summary":"recent unrelated six"}`},
		},
		returnChatLogs: []store.ChatLog{
			{ID: 101, ChatSessionID: "sess-recall-lanes", TurnIndex: 1, Role: "user", Content: "turn one user"},
			{ID: 102, ChatSessionID: "sess-recall-lanes", TurnIndex: 1, Role: "assistant", Content: "turn one assistant"},
			{ID: 201, ChatSessionID: "sess-recall-lanes", TurnIndex: 2, Role: "user", Content: "turn two user"},
			{ID: 202, ChatSessionID: "sess-recall-lanes", TurnIndex: 2, Role: "assistant", Content: "turn two assistant"},
			{ID: 301, ChatSessionID: "sess-recall-lanes", TurnIndex: 3, Role: "user", Content: "turn three user"},
			{ID: 302, ChatSessionID: "sess-recall-lanes", TurnIndex: 3, Role: "assistant", Content: "turn three assistant"},
			{ID: 401, ChatSessionID: "sess-recall-lanes", TurnIndex: 4, Role: "user", Content: "turn four user"},
			{ID: 402, ChatSessionID: "sess-recall-lanes", TurnIndex: 4, Role: "assistant", Content: "turn four assistant"},
			{ID: 501, ChatSessionID: "sess-recall-lanes", TurnIndex: 5, Role: "user", Content: "turn five user"},
			{ID: 502, ChatSessionID: "sess-recall-lanes", TurnIndex: 5, Role: "assistant", Content: "turn five assistant"},
			{ID: 601, ChatSessionID: "sess-recall-lanes", TurnIndex: 6, Role: "user", Content: "turn six user"},
			{ID: 602, ChatSessionID: "sess-recall-lanes", TurnIndex: 6, Role: "assistant", Content: "turn six assistant"},
		},
	}

	cfg := config.Default()
	cfg.StoreMode = config.StoreModeDualShadow
	srv := NewServer(cfg)
	srv.Store = fake

	mux := http.NewServeMux()
	srv.RegisterRoutes(mux)

	body := `{"chat_session_id":"sess-recall-lanes","turn_index":7,"raw_user_input":"brass key","settings":{"max_injection_chars":2000,"injection_enabled":true,"input_context_enabled":false,"top_k":3}}`
	req := httptest.NewRequest(http.MethodPost, "/prepare-turn", bytes.NewReader([]byte(body)))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200: %s", rec.Code, rec.Body.String())
	}
	if fake.lastEpisodeLimit != 3 {
		t.Fatalf("episode summary read limit = %d, want topK 3", fake.lastEpisodeLimit)
	}
	if fake.lastPersonaLimit != 3 {
		t.Fatalf("persona recollection read limit = %d, want topK 3", fake.lastPersonaLimit)
	}
	if fake.lastEntityMemoryLimit != 3 {
		t.Fatalf("character-private recollection read limit = %d, want topK 3", fake.lastEntityMemoryLimit)
	}
	var resp map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	injectionPack := resp["injection_pack"].(map[string]any)
	memoryText, _ := injectionPack["memory_text"].(string)
	for _, want := range []string{"brass key old three", "brass key old two", "brass key old one"} {
		if !strings.Contains(memoryText, want) {
			t.Fatalf("memory_text missing %q: %s", want, memoryText)
		}
	}
	for _, unwanted := range []string{"recent unrelated six", "recent unrelated five", "recent unrelated four"} {
		if strings.Contains(memoryText, unwanted) {
			t.Fatalf("unrelated recent memory was selected ahead of relevant memory %q: %s", unwanted, memoryText)
		}
	}

	recentRawTurnText, _ := injectionPack["recent_raw_turn_text"].(string)
	for _, want := range []string{"turn four user", "turn five user", "turn six user"} {
		if !strings.Contains(recentRawTurnText, want) {
			t.Fatalf("recent_raw_turn_text missing %q: %s", want, recentRawTurnText)
		}
	}
	if strings.Contains(recentRawTurnText, "turn three user") {
		t.Fatalf("recent_raw_turn_text exceeded topK recent turns: %s", recentRawTurnText)
	}

	counts := injectionPack["counts"].(map[string]any)
	if counts["top_k_memory_target"] != float64(3) || counts["recent_memory_bound"] != float64(0) || counts["relevant_memory_bound"] != float64(3) {
		t.Fatalf("topK/recent counts mismatch: %+v", counts)
	}

	recall := resp["recall_result"].(map[string]any)
	searchBundle := recall["search"].(map[string]any)
	if searchBundle["memory_count"] != float64(3) || searchBundle["fallback_count"] != float64(0) {
		t.Fatalf("recall_result.search counts mismatch: %+v", searchBundle)
	}
	recallLanes := recall["recall_lanes"].(map[string]any)
	recent := recallLanes["recent"].(map[string]any)
	if recent["count"] != float64(0) {
		t.Fatalf("recent lane count = %v, want 0 when relevant lane fills topK: %+v", recent["count"], recent)
	}
	relevant := recallLanes["relevant"].(map[string]any)
	items := relevant["items"].([]any)
	if len(items) != 3 {
		t.Fatalf("relevant lane item count = %d, want 3: %+v", len(items), items)
	}
	wantTurns := []float64{3, 2, 1}
	for i, item := range items {
		row := item.(map[string]any)
		if row["turn_index"] != wantTurns[i] {
			t.Fatalf("relevant lane item %d turn_index = %v, want %v: %+v", i, row["turn_index"], wantTurns[i], items)
		}
	}
	trace := recall["trace"].(map[string]any)
	laneTrace := trace["r2_recall_lanes"].(map[string]any)
	if laneTrace["top_k_memory_target"] != float64(3) || laneTrace["recent_memory_count"] != float64(0) || laneTrace["relevant_memory_count"] != float64(3) {
		t.Fatalf("trace topK/recent mismatch: %+v", laneTrace)
	}
}

func TestRecentPrepareTurnRawTurnFollowsTopKWithoutEightTurnCap(t *testing.T) {
	logs := []store.ChatLog{}
	for turn := 1; turn <= 10; turn++ {
		logs = append(logs,
			store.ChatLog{TurnIndex: turn, Role: "user", Content: fmt.Sprintf("turn %02d user", turn)},
			store.ChatLog{TurnIndex: turn, Role: "assistant", Content: fmt.Sprintf("turn %02d assistant", turn)},
		)
	}

	text := recentPrepareTurnRawTurn(logs, 10)
	for _, want := range []string{"turn 01 user", "turn 08 user", "turn 10 assistant"} {
		if !strings.Contains(text, want) {
			t.Fatalf("recent raw turn text missing %q: %s", want, text)
		}
	}
}

func TestPrepareTurnRecallLanesUseRawFallbackWhenVectorIndexNotReady(t *testing.T) {
	fake := &turnRecordingStore{
		returnMemories: []store.Memory{
			{ID: 1, ChatSessionID: "sess-vector-degrade", TurnIndex: 1, SummaryJSON: `{"turn_summary":"old memory one"}`},
			{ID: 2, ChatSessionID: "sess-vector-degrade", TurnIndex: 2, SummaryJSON: `{"turn_summary":"old memory two"}`},
		},
		returnChatLogs: []store.ChatLog{
			{ID: 101, ChatSessionID: "sess-vector-degrade", TurnIndex: 1, Role: "user", Content: "first raw turn"},
			{ID: 102, ChatSessionID: "sess-vector-degrade", TurnIndex: 1, Role: "assistant", Content: "first assistant raw turn"},
			{ID: 201, ChatSessionID: "sess-vector-degrade", TurnIndex: 2, Role: "user", Content: "second raw turn"},
			{ID: 202, ChatSessionID: "sess-vector-degrade", TurnIndex: 2, Role: "assistant", Content: "second assistant raw turn"},
		},
	}

	cfg := config.Default()
	cfg.StoreMode = config.StoreModeDualShadow
	cfg.ChromaEndpoint = "http://127.0.0.1:8000"
	cfg.Readiness.ChromaConfigured = true
	srv := NewServer(cfg)
	srv.Store = fake
	srv.Vector = vector.NewFakeVectorStore()
	srv.VectorOpenError = nil

	mux := http.NewServeMux()
	srv.RegisterRoutes(mux)

	body := `{"chat_session_id":"sess-vector-degrade","turn_index":3,"raw_user_input":"Continue","settings":{"injection_enabled":true,"input_context_enabled":false,"top_k":2}}`
	req := httptest.NewRequest(http.MethodPost, "/prepare-turn", bytes.NewReader([]byte(body)))
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
	recall := resp["recall_result"].(map[string]any)
	readiness := recall["vector_readiness"].(map[string]any)
	if readiness["status"] != "embedding_model_not_ready" || readiness["fallback_recommended"] != true || readiness["reindex_recommended"] != true {
		t.Fatalf("vector readiness mismatch: %+v", readiness)
	}
	recallLanes := recall["recall_lanes"].(map[string]any)
	rawFallback := recallLanes["raw_fallback"].(map[string]any)
	if rawFallback["active"] != true || rawFallback["count"] != float64(4) {
		t.Fatalf("raw_fallback lane mismatch: %+v", rawFallback)
	}
}

func TestPrepareTurnChromaRecallReadUsesClientMetaVector(t *testing.T) {
	fake := &turnRecordingStore{}
	cfg := config.Default()
	cfg.StoreMode = config.StoreModeDualShadow
	srv := NewServer(cfg)
	srv.Store = fake

	mux := http.NewServeMux()
	srv.RegisterRoutes(mux)

	body := `{
		"chat_session_id":"sess-prep",
		"turn_index":1,
		"raw_user_input":"Find the relevant memory",
		"client_meta":{
			"chroma_query_vector":[0.1,0.2],
			"chroma_filter":"chat_session_id == \"sess-prep\""
		},
		"settings":{"top_k":2}
	}`
	req := httptest.NewRequest(http.MethodPost, "/prepare-turn", bytes.NewReader([]byte(body)))
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
	recall, ok := resp["recall_result"].(map[string]any)
	if !ok {
		t.Fatalf("recall_result missing or not object")
	}
	if recall["would_call_vector"] != true {
		t.Errorf("recall_result.would_call_vector = %v, want true", recall["would_call_vector"])
	}
	vectorShadow, ok := recall["vector_shadow"].(map[string]any)
	if !ok {
		t.Fatalf("vector_shadow missing or not object")
	}
	if vectorShadow["recall_read_drill_enabled"] != true {
		t.Errorf("recall_read_drill_enabled = %v, want true", vectorShadow["recall_read_drill_enabled"])
	}
	if vectorShadow["engine"] != "chromadb" {
		t.Errorf("engine = %v, want chromadb", vectorShadow["engine"])
	}
	if vectorShadow["search_attempted"] != true {
		t.Errorf("search_attempted = %v, want true", vectorShadow["search_attempted"])
	}
	if vectorShadow["search_result"] != "not_found" {
		t.Errorf("search_result = %v, want not_found", vectorShadow["search_result"])
	}
	if vectorShadow["query_vector_dim"] != float64(2) {
		t.Errorf("query_vector_dim = %v, want 2", vectorShadow["query_vector_dim"])
	}
	if vectorShadow["live_retrieval_enabled"] != false {
		t.Errorf("live_retrieval_enabled = %v, want false", vectorShadow["live_retrieval_enabled"])
	}
	for _, key := range []string{"milvus_required", "milvus_live_enabled", "optional_engine"} {
		if _, ok := vectorShadow[key]; ok {
			t.Errorf("vector_shadow.%s should not be exposed in ChromaDB-only runtime: %+v", key, vectorShadow)
		}
	}
}

func TestPrepareTurnChromaEndpointMarksR2Source(t *testing.T) {
	fake := &turnRecordingStore{}
	cfg := config.Default()
	cfg.StoreMode = config.StoreModeDualShadow
	cfg.ChromaEndpoint = "http://127.0.0.1:8000"
	cfg.Readiness.ChromaConfigured = true
	srv := NewServer(cfg)
	srv.Store = fake
	srv.Vector = vector.NewFakeVectorStore()
	srv.VectorOpenError = nil

	mux := http.NewServeMux()
	srv.RegisterRoutes(mux)

	body := `{
		"chat_session_id":"sess-prep",
		"turn_index":1,
		"raw_user_input":"Find the relevant memory",
		"client_meta":{
			"chroma_query_vector":[0.1,0.2],
			"chroma_filter":"chat_session_id == \"sess-prep\""
		},
		"settings":{"top_k":2}
	}`
	req := httptest.NewRequest(http.MethodPost, "/prepare-turn", bytes.NewReader([]byte(body)))
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
	recall, ok := resp["recall_result"].(map[string]any)
	if !ok {
		t.Fatalf("recall_result missing or not object")
	}
	if recall["source"] != "go_r2_chromadb_product_read" {
		t.Errorf("recall_result.source = %v, want go_r2_chromadb_product_read", recall["source"])
	}
	vectorShadow, ok := recall["vector_shadow"].(map[string]any)
	if !ok {
		t.Fatalf("vector_shadow missing or not object")
	}
	if vectorShadow["product_read_enabled"] != true {
		t.Errorf("product_read_enabled = %v, want true", vectorShadow["product_read_enabled"])
	}
	if vectorShadow["live_retrieval_enabled"] != true {
		t.Errorf("live_retrieval_enabled = %v, want true", vectorShadow["live_retrieval_enabled"])
	}
	if vectorShadow["chromadb_live_enabled"] != true {
		t.Errorf("chromadb_live_enabled = %v, want true", vectorShadow["chromadb_live_enabled"])
	}
	for _, key := range []string{"milvus_required", "milvus_live_enabled", "optional_engine"} {
		if _, ok := vectorShadow[key]; ok {
			t.Errorf("vector_shadow.%s should not be exposed in ChromaDB-only runtime: %+v", key, vectorShadow)
		}
	}
}

func TestPrepareTurnChromaRecallAutoEmbedsRawUserInput(t *testing.T) {
	oldClient := proxyHTTPClient
	proxyHTTPClient = &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		if got := r.URL.String(); got != "https://api.example.test/v1/embeddings" {
			t.Fatalf("upstream URL = %q", got)
		}
		if got := r.Header.Get("Authorization"); got != "Bearer embed-key" {
			t.Fatalf("Authorization = %q", got)
		}
		raw, _ := io.ReadAll(r.Body)
		if !strings.Contains(string(raw), "Find the relevant memory") {
			t.Fatalf("embedding request did not include raw user input: %s", raw)
		}
		return &http.Response{
			StatusCode: http.StatusOK,
			Status:     "200 OK",
			Header:     make(http.Header),
			Body:       io.NopCloser(strings.NewReader(`{"model":"embed-model","data":[{"embedding":[0.1,0.2,0.3]}]}`)),
		}, nil
	})}
	defer func() { proxyHTTPClient = oldClient }()

	fake := &turnRecordingStore{}
	cfg := config.Default()
	cfg.StoreMode = config.StoreModeDualShadow
	cfg.ChromaEndpoint = "http://127.0.0.1:8000"
	cfg.Readiness.ChromaConfigured = true
	srv := NewServer(cfg)
	srv.Store = fake
	srv.Vector = &turnRecordingVectorStore{}
	srv.VectorOpenError = nil

	mux := http.NewServeMux()
	srv.RegisterRoutes(mux)

	body := `{
		"chat_session_id":"sess-prep",
		"turn_index":1,
		"raw_user_input":"Find the relevant memory",
		"client_meta":{
			"embedding":{
				"api_key":"embed-key",
				"endpoint":"https://api.example.test/v1",
				"model":"embed-model",
				"provider":"openai"
			}
		},
		"settings":{"top_k":2}
	}`
	req := httptest.NewRequest(http.MethodPost, "/prepare-turn", bytes.NewReader([]byte(body)))
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
	recall, ok := resp["recall_result"].(map[string]any)
	if !ok {
		t.Fatalf("recall_result missing or not object")
	}
	vectorShadow, ok := recall["vector_shadow"].(map[string]any)
	if !ok {
		t.Fatalf("vector_shadow missing or not object")
	}
	if vectorShadow["query_embedding_status"] != "ok" {
		t.Errorf("query_embedding_status = %v, want ok", vectorShadow["query_embedding_status"])
	}
	if vectorShadow["query_vector_key"] != "server_query_embedding" {
		t.Errorf("query_vector_key = %v, want server_query_embedding", vectorShadow["query_vector_key"])
	}
	if vectorShadow["query_vector_dim"] != float64(3) {
		t.Errorf("query_vector_dim = %v, want 3", vectorShadow["query_vector_dim"])
	}
	if vectorShadow["search_attempted"] != true {
		t.Errorf("search_attempted = %v, want true", vectorShadow["search_attempted"])
	}
	if vectorShadow["source"] != "go_r2_chromadb_product_read" {
		t.Errorf("source = %v, want go_r2_chromadb_product_read", vectorShadow["source"])
	}
}

func TestPrepareTurnCharCapsAndTruncation(t *testing.T) {
	// Build data that will overflow the character caps.
	fake := &turnRecordingStore{
		returnMemories: []store.Memory{
			{ID: 1, ChatSessionID: "sess-cap", TurnIndex: 1, SummaryJSON: `{"turn_summary":"` + strings.Repeat("x", 300) + `"}`, Importance: 0.5},
			{ID: 2, ChatSessionID: "sess-cap", TurnIndex: 2, SummaryJSON: `{"turn_summary":"` + strings.Repeat("y", 300) + `"}`, Importance: 0.5},
		},
		returnKGTriples: []store.KGTriple{
			{ID: 1, ChatSessionID: "sess-cap", Subject: "A", Predicate: "B", Object: "C"},
		},
		returnEvidence:        []store.DirectEvidence{},
		returnChatLogs:        []store.ChatLog{},
		returnResumePack:      nil,
		returnStorylines:      nil,
		returnWorldRules:      nil,
		returnCharStates:      nil,
		returnPendingThreads:  nil,
		returnActiveStates:    nil,
		returnCanonicalLayers: nil,
		returnEpisodeSums:     nil,
	}

	cfg := config.Default()
	cfg.StoreMode = config.StoreModeDualShadow
	srv := NewServer(cfg)
	srv.Store = fake

	mux := http.NewServeMux()
	srv.RegisterRoutes(mux)

	// Very small caps to force truncation
	body := `{"chat_session_id":"sess-cap","turn_index":3,"raw_user_input":"test","settings":{"max_injection_chars":50,"max_input_context_chars":50,"injection_enabled":true,"input_context_enabled":true,"top_k":10}}`
	req := httptest.NewRequest(http.MethodPost, "/prepare-turn", bytes.NewReader([]byte(body)))
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

	gp, ok := resp["generation_packet"].(map[string]any)
	if !ok {
		t.Fatalf("generation_packet is not an object")
	}

	injection, _ := gp["injection_text"].(string)
	if len(injection) > 50 {
		t.Errorf("injection_text length %d exceeds cap 50", len(injection))
	}

	trace, ok := gp["trace_summary"].(map[string]any)
	if !ok {
		t.Fatalf("trace_summary is not an object")
	}

	// With overflow data, injection_truncated should be true
	if trace["injection_truncated"] != true {
		t.Errorf("injection_truncated = %v, want true (injection was truncated)", trace["injection_truncated"])
	}
}

// prepareTurnNotEnabledStore returns ErrNotEnabled for all narrative/read methods.
type prepareTurnNotEnabledStore struct {
	memoryFakeStore
}

func (n *prepareTurnNotEnabledStore) ListMemories(ctx context.Context, sid string, fromTurn, toTurn int) ([]store.Memory, error) {
	return nil, store.ErrNotEnabled
}
func (n *prepareTurnNotEnabledStore) ListKGTriples(ctx context.Context, sid string) ([]store.KGTriple, error) {
	return nil, store.ErrNotEnabled
}
func (n *prepareTurnNotEnabledStore) ListEvidence(ctx context.Context, sid string) ([]store.DirectEvidence, error) {
	return nil, store.ErrNotEnabled
}
func (n *prepareTurnNotEnabledStore) ListChatLogs(ctx context.Context, sid string, fromTurn, toTurn int) ([]store.ChatLog, error) {
	return nil, store.ErrNotEnabled
}
func (n *prepareTurnNotEnabledStore) GetResumePack(ctx context.Context, sid, trigger string) (*store.ResumePack, error) {
	return nil, store.ErrNotEnabled
}
func (n *prepareTurnNotEnabledStore) ListStorylines(ctx context.Context, sid string) ([]store.Storyline, error) {
	return nil, store.ErrNotEnabled
}
func (n *prepareTurnNotEnabledStore) ListWorldRules(ctx context.Context, sid string) ([]store.WorldRule, error) {
	return nil, store.ErrNotEnabled
}
func (n *prepareTurnNotEnabledStore) ListCharacterStates(ctx context.Context, sid string) ([]store.CharacterState, error) {
	return nil, store.ErrNotEnabled
}
func (n *prepareTurnNotEnabledStore) ListPendingThreads(ctx context.Context, sid, status string) ([]store.PendingThread, error) {
	return nil, store.ErrNotEnabled
}
func (n *prepareTurnNotEnabledStore) ListActiveStates(ctx context.Context, sid, stateType string) ([]store.ActiveState, error) {
	return nil, store.ErrNotEnabled
}
func (n *prepareTurnNotEnabledStore) ListCanonicalStateLayers(ctx context.Context, sid, layerType string) ([]store.CanonicalStateLayer, error) {
	return nil, store.ErrNotEnabled
}
func (n *prepareTurnNotEnabledStore) ListEpisodeSummaries(ctx context.Context, sid string, limit, fromTurn, toTurn int) ([]store.EpisodeSummary, error) {
	return nil, store.ErrNotEnabled
}

func TestPrepareTurnErrNotEnabledSafeFallback(t *testing.T) {
	cfg := config.Default()
	srv := NewServer(cfg)
	srv.Store = &prepareTurnNotEnabledStore{}

	mux := http.NewServeMux()
	srv.RegisterRoutes(mux)

	body := `{"chat_session_id":"sess-ne","turn_index":1,"raw_user_input":"hello"}`
	req := httptest.NewRequest(http.MethodPost, "/prepare-turn", bytes.NewReader([]byte(body)))
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

	gp, ok := resp["generation_packet"].(map[string]any)
	if !ok {
		t.Fatalf("generation_packet missing")
	}
	if gp["degraded"] != true {
		t.Errorf("degraded = %v, want true", gp["degraded"])
	}
	if gp["packet_mode"] != "off" {
		t.Errorf("packet_mode = %v, want off", gp["packet_mode"])
	}

	rr, ok := resp["recall_result"].(map[string]any)
	if !ok {
		t.Fatalf("recall_result missing or not object")
	}
	if rr["status"] != "degraded" {
		t.Errorf("recall_result.status = %v, want degraded", rr["status"])
	}
	if rr["source"] != "go_r1_read_shadow" {
		t.Errorf("recall_result.source = %v, want go_r1_read_shadow", rr["source"])
	}
	if rr["would_write"] != false {
		t.Errorf("would_write = %v, want false", rr["would_write"])
	}
	vs, ok := rr["vector_shadow"].(map[string]any)
	if !ok {
		t.Fatalf("recall_result.vector_shadow missing")
	}
	if vs["status"] != "shadow" {
		t.Errorf("vector_shadow.status = %v, want shadow", vs["status"])
	}
	if vs["health_checked"] != true {
		t.Errorf("vector_shadow.health_checked = %v, want true", vs["health_checked"])
	}
	if vs["search_attempted"] != false {
		t.Errorf("vector_shadow.search_attempted = %v, want false", vs["search_attempted"])
	}

	// session_state degraded
	ss, ok := resp["session_state"].(map[string]any)
	if !ok {
		t.Fatalf("session_state is not an object")
	}
	if ss["snapshot_status"] != "degraded" {
		t.Errorf("session_state.snapshot_status = %v, want degraded", ss["snapshot_status"])
	}
	ssMeta, _ := ss["section_meta"].(map[string]any)
	if ssMeta["storyline_count"] != float64(0) {
		t.Errorf("session_state.section_meta.storyline_count = %v, want 0", ssMeta["storyline_count"])
	}

	// narrative_control skeleton
	nc, ok := resp["narrative_control"].(map[string]any)
	if !ok {
		t.Fatalf("narrative_control is not an object")
	}
	if nc["state_status"] != "skeleton" {
		t.Errorf("narrative_control.state_status = %v, want skeleton", nc["state_status"])
	}
	if nc["storyline_count"] != float64(0) {
		t.Errorf("narrative_control.storyline_count = %v, want 0", nc["storyline_count"])
	}
	if nc["would_call_llm"] != false {
		t.Errorf("narrative_control.would_call_llm = %v, want false", nc["would_call_llm"])
	}

	// continuity_pack degraded
	cp, ok := resp["continuity_pack"].(map[string]any)
	if !ok {
		t.Fatalf("continuity_pack is not an object")
	}
	if cp["status"] != "degraded" {
		t.Errorf("continuity_pack.status = %v, want degraded", cp["status"])
	}
	if cp["resume_pack_present"] != false {
		t.Errorf("continuity_pack.resume_pack_present = %v, want false", cp["resume_pack_present"])
	}
	cpItems, _ := cp["items"].([]any)
	if len(cpItems) != 0 {
		t.Errorf("continuity_pack.items len = %d, want 0", len(cpItems))
	}
	if cp["would_call_llm"] != false {
		t.Errorf("continuity_pack.would_call_llm = %v, want false", cp["would_call_llm"])
	}
}

func TestPrepareTurnRespectsDisabledFlags(t *testing.T) {
	fake := &turnRecordingStore{
		returnMemories: []store.Memory{
			{ID: 1, ChatSessionID: "sess-flag", TurnIndex: 1, SummaryJSON: `{"turn_summary":"summary"}`, Importance: 0.5},
		},
		returnKGTriples: []store.KGTriple{
			{ID: 10, ChatSessionID: "sess-flag", Subject: "A", Predicate: "B", Object: "C"},
		},
	}

	cfg := config.Default()
	cfg.StoreMode = config.StoreModeDualShadow
	srv := NewServer(cfg)
	srv.Store = fake

	mux := http.NewServeMux()
	srv.RegisterRoutes(mux)

	body := `{"chat_session_id":"sess-flag","turn_index":2,"raw_user_input":"test","settings":{"injection_enabled":false,"input_context_enabled":false}}`
	req := httptest.NewRequest(http.MethodPost, "/prepare-turn", bytes.NewReader([]byte(body)))
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

	if resp["injection_text"] != nil {
		t.Errorf("injection_text = %v, want nil when disabled", resp["injection_text"])
	}
	if resp["input_context_text"] != nil {
		t.Errorf("input_context_text = %v, want nil when disabled", resp["input_context_text"])
	}

	rr, ok := resp["recall_result"].(map[string]any)
	if !ok {
		t.Fatalf("recall_result missing or not object")
	}
	if rr["status"] != "ready" {
		t.Errorf("recall_result.status = %v, want ready", rr["status"])
	}
	if rr["query_preview"] != "test" {
		t.Errorf("query_preview = %v, want test", rr["query_preview"])
	}
	if _, ok := rr["counts"]; !ok {
		t.Errorf("recall_result.counts missing")
	}
	if _, ok := rr["vector_shadow"]; !ok {
		t.Errorf("recall_result.vector_shadow missing")
	}
}

// TestPrepareTurnDegradedFailOpenPreservesTruthFloor proves that a degraded store
// (timeout/failure simulation) does not block the main turn path and does not
// overwrite canonical state. It verifies the response still returns 200 OK,
// injection_text is present, progression_ledger has would_write=false, and
// trace_preview contains fallback indicators. (SEQ-12-P33 RMG-01)
func TestPrepareTurnDegradedFailOpenPreservesTruthFloor(t *testing.T) {
	cfg := config.Default()
	srv := NewServer(cfg)
	srv.Store = &prepareTurnNotEnabledStore{}

	mux := http.NewServeMux()
	srv.RegisterRoutes(mux)

	body := `{"chat_session_id":"sess-p33","turn_index":1,"raw_user_input":"hello","settings":{"max_injection_chars":500,"injection_enabled":true,"input_context_enabled":true}}`
	req := httptest.NewRequest(http.MethodPost, "/prepare-turn", bytes.NewReader([]byte(body)))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200 (main turn must not be blocked)", rec.Code)
	}

	var resp map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}

	// 1) Main turn path not blocked
	if resp["status"] != "ok" {
		t.Fatalf("status = %v, want ok", resp["status"])
	}

	// 2) Degraded mode acknowledged but injection still returned (truth floor preserved)
	gp, ok := resp["generation_packet"].(map[string]any)
	if !ok {
		t.Fatalf("generation_packet missing")
	}
	if gp["degraded"] != true {
		t.Fatalf("degraded = %v, want true", gp["degraded"])
	}
	if gp["packet_mode"] != "off" {
		t.Fatalf("packet_mode = %v, want off", gp["packet_mode"])
	}

	// 3) Injection text field exists (may be null) to preserve turn contract shape
	if _, ok := resp["injection_text"]; !ok {
		t.Fatalf("injection_text key must be present to preserve turn contract")
	}

	// 4) Progression ledger exists and would_write=false (no canonical overwrite)
	pl, ok := resp["progression_ledger"].(map[string]any)
	if !ok {
		t.Fatalf("progression_ledger missing")
	}
	if pl["would_write"] != false {
		t.Fatalf("progression_ledger.would_write = %v, want false (truth floor must not be overwritten)", pl["would_write"])
	}
	if pl["status"] != "degraded" {
		t.Fatalf("progression_ledger.status = %v, want degraded", pl["status"])
	}

	// 5) Trace preview contains fallback reason and no LLM write attempts
	tracePreview, ok := resp["trace_preview"].(map[string]any)
	if !ok {
		t.Fatalf("trace_preview missing")
	}
	if tracePreview["would_write"] != false {
		t.Fatalf("trace_preview.would_write = %v, want false", tracePreview["would_write"])
	}
	if tracePreview["would_call_llm"] != false {
		t.Fatalf("trace_preview.would_call_llm = %v, want false", tracePreview["would_call_llm"])
	}

	// 6) Recall result is degraded but not erroring
	rr, ok := resp["recall_result"].(map[string]any)
	if !ok {
		t.Fatalf("recall_result missing")
	}
	if rr["status"] != "degraded" {
		t.Fatalf("recall_result.status = %v, want degraded", rr["status"])
	}
	if rr["would_write"] != false {
		t.Fatalf("recall_result.would_write = %v, want false", rr["would_write"])
	}
}
func TestPrepareTurnPromptAssemblyNotConfigured(t *testing.T) {
	cfg := config.Default()
	srv := NewServer(cfg)
	srv.Store = &prepareTurnNotEnabledStore{}

	mux := http.NewServeMux()
	srv.RegisterRoutes(mux)

	body := `{"chat_session_id":"sess-pa","turn_index":1,"raw_user_input":"hello"}`
	req := httptest.NewRequest(http.MethodPost, "/prepare-turn", bytes.NewReader([]byte(body)))
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

	gp, ok := resp["generation_packet"].(map[string]any)
	if !ok {
		t.Fatalf("generation_packet missing")
	}

	pa, ok := gp["prompt_assembly"].(map[string]any)
	if !ok {
		t.Fatalf("prompt_assembly missing")
	}

	if pa["prompt_source"] != "not_configured" {
		t.Errorf("prompt_source = %v, want not_configured", pa["prompt_source"])
	}
	if pa["files_found"] != float64(0) {
		t.Errorf("files_found = %v, want 0", pa["files_found"])
	}
	if pa["would_call_llm"] != false {
		t.Errorf("would_call_llm = %v, want false", pa["would_call_llm"])
	}

	rr, ok := resp["recall_result"].(map[string]any)
	if !ok {
		t.Fatalf("recall_result missing or not object")
	}
	if rr["status"] != "degraded" {
		t.Errorf("recall_result.status = %v, want degraded", rr["status"])
	}
	if rr["source"] != "go_r1_read_shadow" {
		t.Errorf("recall_result.source = %v, want go_r1_read_shadow", rr["source"])
	}
	if rr["would_write"] != false {
		t.Errorf("would_write = %v, want false", rr["would_write"])
	}
	vs, ok := rr["vector_shadow"].(map[string]any)
	if !ok {
		t.Fatalf("recall_result.vector_shadow missing")
	}
	if vs["status"] != "shadow" {
		t.Errorf("vector_shadow.status = %v, want shadow", vs["status"])
	}
	if vs["health_checked"] != true {
		t.Errorf("vector_shadow.health_checked = %v, want true", vs["health_checked"])
	}
	if vs["search_attempted"] != false {
		t.Errorf("vector_shadow.search_attempted = %v, want false", vs["search_attempted"])
	}

	// session_state degraded (not configured store)
	ss, ok := resp["session_state"].(map[string]any)
	if !ok {
		t.Fatalf("session_state is not an object")
	}
	if ss["snapshot_status"] != "degraded" {
		t.Errorf("session_state.snapshot_status = %v, want degraded", ss["snapshot_status"])
	}

	// narrative_control skeleton
	nc, ok := resp["narrative_control"].(map[string]any)
	if !ok {
		t.Fatalf("narrative_control is not an object")
	}
	if nc["state_status"] != "skeleton" {
		t.Errorf("narrative_control.state_status = %v, want skeleton", nc["state_status"])
	}

	// continuity_pack degraded
	cp, ok := resp["continuity_pack"].(map[string]any)
	if !ok {
		t.Fatalf("continuity_pack is not an object")
	}
	if cp["status"] != "degraded" {
		t.Errorf("continuity_pack.status = %v, want degraded", cp["status"])
	}
	if cp["would_call_llm"] != false {
		t.Errorf("continuity_pack.would_call_llm = %v, want false", cp["would_call_llm"])
	}

	// progression_ledger degraded
	pl, ok := resp["progression_ledger"].(map[string]any)
	if !ok {
		t.Fatalf("progression_ledger is not an object")
	}
	if pl["status"] != "degraded" {
		t.Errorf("progression_ledger.status = %v, want degraded", pl["status"])
	}
	if pl["would_write"] != false {
		t.Errorf("progression_ledger.would_write = %v, want false", pl["would_write"])
	}

	// autonomy_plan degraded
	ap, ok := resp["autonomy_plan"].(map[string]any)
	if !ok {
		t.Fatalf("autonomy_plan is not an object")
	}
	if ap["status"] != "degraded" {
		t.Errorf("autonomy_plan.status = %v, want degraded", ap["status"])
	}
	if ap["suggested_action"] != "continue" {
		t.Errorf("autonomy_plan.suggested_action = %v, want continue", ap["suggested_action"])
	}
	if ap["would_call_llm"] != false {
		t.Errorf("autonomy_plan.would_call_llm = %v, want false", ap["would_call_llm"])
	}
	if ap["would_write"] != false {
		t.Errorf("autonomy_plan.would_write = %v, want false", ap["would_write"])
	}

	// micro_beat_proposal degraded
	mb, ok := resp["micro_beat_proposal"].(map[string]any)
	if !ok {
		t.Fatalf("micro_beat_proposal is not an object")
	}
	if mb["status"] != "degraded" {
		t.Errorf("micro_beat_proposal.status = %v, want degraded", mb["status"])
	}
	mbBeats, _ := mb["beats"].([]any)
	if len(mbBeats) != 0 {
		t.Errorf("micro_beat_proposal.beats len = %d, want 0", len(mbBeats))
	}
	if mb["would_call_llm"] != false {
		t.Errorf("micro_beat_proposal.would_call_llm = %v, want false", mb["would_call_llm"])
	}

	// scene_step_proposal degraded
	sp, ok := resp["scene_step_proposal"].(map[string]any)
	if !ok {
		t.Fatalf("scene_step_proposal is not an object")
	}
	if sp["status"] != "degraded" {
		t.Errorf("scene_step_proposal.status = %v, want degraded", sp["status"])
	}
	spSteps, _ := sp["steps"].([]any)
	if len(spSteps) != 0 {
		t.Errorf("scene_step_proposal.steps len = %d, want 0", len(spSteps))
	}
	if sp["would_call_llm"] != false {
		t.Errorf("scene_step_proposal.would_call_llm = %v, want false", sp["would_call_llm"])
	}

	// combined_proposal degraded
	cp2, ok := resp["combined_proposal"].(map[string]any)
	if !ok {
		t.Fatalf("combined_proposal is not an object")
	}
	if cp2["status"] != "degraded" {
		t.Errorf("combined_proposal.status = %v, want degraded", cp2["status"])
	}
	if cp2["micro_beat_count"] != float64(0) {
		t.Errorf("combined_proposal.micro_beat_count = %v, want 0", cp2["micro_beat_count"])
	}
	if cp2["scene_step_count"] != float64(0) {
		t.Errorf("combined_proposal.scene_step_count = %v, want 0", cp2["scene_step_count"])
	}
	if cp2["source"] != "go_r1_read_shadow" {
		t.Errorf("combined_proposal.source = %v, want go_r1_read_shadow", cp2["source"])
	}
	if cp2["would_call_llm"] != false {
		t.Errorf("combined_proposal.would_call_llm = %v, want false", cp2["would_call_llm"])
	}
	if cp2["would_write"] != false {
		t.Errorf("combined_proposal.would_write = %v, want false", cp2["would_write"])
	}

	// writeback_preview degraded
	wp, ok := resp["writeback_preview"].(map[string]any)
	if !ok {
		t.Fatalf("writeback_preview is not an object")
	}
	if wp["status"] != "degraded" {
		t.Errorf("writeback_preview.status = %v, want degraded", wp["status"])
	}
	if wp["would_write"] != false {
		t.Errorf("writeback_preview.would_write = %v, want false", wp["would_write"])
	}
}

func TestPrepareTurnPromptAssemblyWithDir(t *testing.T) {
	tmpDir := t.TempDir()

	if err := os.WriteFile(filepath.Join(tmpDir, "supervisor_system.txt"), []byte("supervisor system content"), 0644); err != nil {
		t.Fatalf("write file: %v", err)
	}
	if err := os.WriteFile(filepath.Join(tmpDir, "critic_prompt.txt"), []byte("critic prompt content here"), 0644); err != nil {
		t.Fatalf("write file: %v", err)
	}

	cfg := config.Default()
	cfg.PromptDir = tmpDir
	srv := NewServer(cfg)
	srv.Store = &prepareTurnNotEnabledStore{}

	mux := http.NewServeMux()
	srv.RegisterRoutes(mux)

	body := `{"chat_session_id":"sess-pa2","turn_index":1,"raw_user_input":"hello"}`
	req := httptest.NewRequest(http.MethodPost, "/prepare-turn", bytes.NewReader([]byte(body)))
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

	gp, ok := resp["generation_packet"].(map[string]any)
	if !ok {
		t.Fatalf("generation_packet missing")
	}

	pa, ok := gp["prompt_assembly"].(map[string]any)
	if !ok {
		t.Fatalf("prompt_assembly missing")
	}

	if pa["prompt_source"] != "configured" {
		t.Errorf("prompt_source = %v, want configured", pa["prompt_source"])
	}
	if pa["files_found"] != float64(2) {
		t.Errorf("files_found = %v, want 2", pa["files_found"])
	}
	if pa["total_chars"] != float64(51) {
		t.Errorf("total_chars = %v, want 51", pa["total_chars"])
	}
	if pa["would_call_llm"] != false {
		t.Errorf("would_call_llm = %v, want false", pa["would_call_llm"])
	}

	files, ok := pa["files"].([]any)
	if !ok {
		t.Fatalf("files is not an array")
	}
	if len(files) != 4 {
		t.Fatalf("files len = %d, want 4", len(files))
	}

	rr, ok := resp["recall_result"].(map[string]any)
	if !ok {
		t.Fatalf("recall_result missing or not object")
	}
	if rr["status"] != "degraded" {
		t.Errorf("recall_result.status = %v, want degraded", rr["status"])
	}
	if rr["would_write"] != false {
		t.Errorf("would_write = %v, want false", rr["would_write"])
	}
	vs, ok := rr["vector_shadow"].(map[string]any)
	if !ok {
		t.Fatalf("recall_result.vector_shadow missing")
	}
	if vs["status"] != "shadow" {
		t.Errorf("vector_shadow.status = %v, want shadow", vs["status"])
	}
	if vs["health_checked"] != true {
		t.Errorf("vector_shadow.health_checked = %v, want true", vs["health_checked"])
	}
	if vs["search_attempted"] != false {
		t.Errorf("vector_shadow.search_attempted = %v, want false", vs["search_attempted"])
	}
}

// ---------------------------------------------------------------------------
// /turns/repair-replay tests
// ---------------------------------------------------------------------------

func TestRepairReplayShadowPlan(t *testing.T) {
	cfg := config.Default()
	srv := NewServer(cfg)

	mux := http.NewServeMux()
	srv.RegisterRoutes(mux)

	body := `{"chat_session_id":"sess-repair","dry_run":true,"entries":[{"assistant_content":"entry one"},{"assistant_content":"entry two"},{"assistant_content":"entry three"},{"assistant_content":"entry four"}]}`
	req := httptest.NewRequest(http.MethodPost, "/turns/repair-replay", bytes.NewReader([]byte(body)))
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
		t.Errorf("status = %v, want ok", resp["status"])
	}

	rp, ok := resp["repair_replay_plan"].(map[string]any)
	if !ok {
		t.Fatalf("repair_replay_plan is not an object")
	}
	if rp["status"] != "shadow_plan" {
		t.Errorf("repair_replay_plan.status = %v, want shadow_plan", rp["status"])
	}
	if rp["entries_count"] != float64(4) {
		t.Errorf("repair_replay_plan.entries_count = %v, want 4", rp["entries_count"])
	}
	if rp["dry_run"] != true {
		t.Errorf("repair_replay_plan.dry_run = %v, want true", rp["dry_run"])
	}
	if rp["would_replay"] != false {
		t.Errorf("repair_replay_plan.would_replay = %v, want false", rp["would_replay"])
	}
	if rp["would_write"] != false {
		t.Errorf("repair_replay_plan.would_write = %v, want false", rp["would_write"])
	}
	if rp["mutation_enabled"] != false {
		t.Errorf("repair_replay_plan.mutation_enabled = %v, want false", rp["mutation_enabled"])
	}

	preview, _ := rp["entries_preview"].([]any)
	if len(preview) != 3 {
		t.Errorf("entries_preview len = %d, want 3", len(preview))
	}

	notes, _ := rp["notes"].(string)
	if !strings.Contains(notes, "R1 read-shadow") {
		t.Errorf("repair_replay_plan.notes missing R1 marker: %q", notes)
	}
}

func TestRepairReplayEmptyEntries(t *testing.T) {
	cfg := config.Default()
	srv := NewServer(cfg)

	mux := http.NewServeMux()
	srv.RegisterRoutes(mux)

	body := `{"chat_session_id":"sess-repair-empty"}`
	req := httptest.NewRequest(http.MethodPost, "/turns/repair-replay", bytes.NewReader([]byte(body)))
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

	rp, ok := resp["repair_replay_plan"].(map[string]any)
	if !ok {
		t.Fatalf("repair_replay_plan is not an object")
	}
	if rp["entries_count"] != float64(0) {
		t.Errorf("repair_replay_plan.entries_count = %v, want 0", rp["entries_count"])
	}
	if rp["dry_run"] != false {
		t.Errorf("repair_replay_plan.dry_run = %v, want false", rp["dry_run"])
	}
}

func TestRepairReplayWriteStoreDryRunAndReplay(t *testing.T) {
	fake := &turnRecordingStore{
		returnChatLogs: []store.ChatLog{
			{ChatSessionID: "sess-repair-write", TurnIndex: 1, Role: "user", Content: "already saved"},
		},
	}
	cfg := config.Default()
	cfg.StoreMode = config.StoreModeMariaDBAuthority
	srv := NewServer(cfg)
	srv.Store = fake
	srv.StoreOpenError = nil

	mux := http.NewServeMux()
	srv.RegisterRoutes(mux)

	body := `{"chat_session_id":"sess-repair-write","dry_run":true,"entries":[{"turn_index":1,"user_content":"already saved","assistant_content":"missing assistant"},{"turn_index":2,"user_content":"new user","assistant_content":"new assistant"}]}`
	req := httptest.NewRequest(http.MethodPost, "/turns/repair-replay", bytes.NewReader([]byte(body)))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("dry-run status = %d, want 200: %s", rec.Code, rec.Body.String())
	}
	var dryResp map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &dryResp); err != nil {
		t.Fatalf("decode dry-run: %v", err)
	}
	if dryResp["total_missing_role_count"] != float64(3) || dryResp["total_repaired_role_count"] != float64(0) {
		t.Fatalf("dry-run counts mismatch: %+v", dryResp)
	}
	if len(fake.savedChatLogs) != 0 {
		t.Fatalf("dry-run should not save chat logs, got %d", len(fake.savedChatLogs))
	}

	body = strings.Replace(body, `"dry_run":true`, `"dry_run":false`, 1)
	req = httptest.NewRequest(http.MethodPost, "/turns/repair-replay", bytes.NewReader([]byte(body)))
	req.Header.Set("Content-Type", "application/json")
	rec = httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("replay status = %d, want 200: %s", rec.Code, rec.Body.String())
	}
	var replayResp map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &replayResp); err != nil {
		t.Fatalf("decode replay: %v", err)
	}
	if replayResp["total_repaired_role_count"] != float64(3) {
		t.Fatalf("total_repaired_role_count = %v, want 3", replayResp["total_repaired_role_count"])
	}
	if len(fake.savedChatLogs) != 3 {
		t.Fatalf("savedChatLogs = %d, want 3", len(fake.savedChatLogs))
	}
	if fake.savedChatLogs[0].Role != "assistant" || fake.savedChatLogs[0].TurnIndex != 1 {
		t.Fatalf("first repaired role = %#v, want missing assistant turn 1", fake.savedChatLogs[0])
	}
	foundAudit := false
	for _, audit := range fake.savedAuditLogs {
		if audit.EventType == "repair_replay" {
			foundAudit = true
			break
		}
	}
	if !foundAudit {
		t.Fatalf("expected repair_replay audit, got %#v", fake.savedAuditLogs)
	}
}

func TestAdminRescanRegeneratesMissingArtifactsFromRawTurn(t *testing.T) {
	fake := &turnRecordingStore{
		returnChatLogs: []store.ChatLog{
			{ChatSessionID: "sess-rescan", TurnIndex: 1, Role: "user", Content: "Mina found the blue key.", CreatedAt: time.Now()},
			{ChatSessionID: "sess-rescan", TurnIndex: 1, Role: "assistant", Content: "Mina promised to keep the blue key safe.", CreatedAt: time.Now()},
		},
	}
	cfg := config.Default()
	cfg.StoreMode = config.StoreModeMariaDBAuthority
	srv := NewServer(cfg)
	srv.Store = fake
	srv.StoreOpenError = nil

	extractionBytes, _ := json.Marshal(map[string]any{
		"turn_summary":      "Mina found and kept the blue key safe.",
		"importance_score":  8,
		"evidence_excerpts": []any{"Mina promised to keep the blue key safe."},
		"kg_triples":        []any{map[string]any{"subject": "Mina", "predicate": "keeps", "object": "blue key"}},
		"entities":          map[string]any{"characters": []any{map[string]any{"name": "Mina"}}},
	})
	chatResp, _ := json.Marshal(map[string]any{
		"model":   "rescan-critic",
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
	updateBody := `{"criticApiKey":"sk-rescan","criticEndpoint":"https://api.example.com/v1","criticModel":"rescan-critic","criticProvider":"openai","criticTimeout":45}`
	updateReq := httptest.NewRequest(http.MethodPost, "/config/update", bytes.NewReader([]byte(updateBody)))
	updateReq.Header.Set("Content-Type", "application/json")
	updateRec := httptest.NewRecorder()
	mux.ServeHTTP(updateRec, updateReq)
	if updateRec.Code != http.StatusOK {
		t.Fatalf("config/update status = %d, want 200: %s", updateRec.Code, updateRec.Body.String())
	}

	req := httptest.NewRequest(http.MethodPost, "/admin/rescan", bytes.NewReader([]byte(`{"chat_session_id":"sess-rescan","max_items":10}`)))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("rescan status = %d, want 200: %s", rec.Code, rec.Body.String())
	}
	var resp map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode rescan: %v", err)
	}
	if resp["succeeded"] != float64(1) || resp["failed"] != float64(0) {
		t.Fatalf("rescan counts mismatch: %+v", resp)
	}
	if len(fake.savedMemories) != 1 || len(fake.savedEvidence) != 1 || len(fake.savedKGTriples) != 1 || len(fake.savedEntities) != 1 {
		t.Fatalf("expected memory/evidence/KG/entity from rescan, memories=%d evidence=%d kg=%d entities=%d", len(fake.savedMemories), len(fake.savedEvidence), len(fake.savedKGTriples), len(fake.savedEntities))
	}
	if !hasAuditEvent(fake.savedAuditLogs, "admin_rescan") {
		t.Fatalf("expected admin_rescan audit, got %#v", fake.savedAuditLogs)
	}
}

func TestAdminRescanNoCandidatesHonest(t *testing.T) {
	fake := &turnRecordingStore{
		returnChatLogs: []store.ChatLog{
			{ChatSessionID: "sess-rescan-empty", TurnIndex: 1, Role: "user", Content: "already indexed", CreatedAt: time.Now()},
		},
		returnMemories: []store.Memory{
			{ChatSessionID: "sess-rescan-empty", TurnIndex: 1, SummaryJSON: `{"turn_summary":"already indexed"}`},
		},
	}
	cfg := config.Default()
	cfg.StoreMode = config.StoreModeMariaDBAuthority
	srv := NewServer(cfg)
	srv.Store = fake
	srv.StoreOpenError = nil

	mux := http.NewServeMux()
	srv.RegisterRoutes(mux)
	req := httptest.NewRequest(http.MethodPost, "/admin/rescan", bytes.NewReader([]byte(`{"chat_session_id":"sess-rescan-empty","max_items":10}`)))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("rescan status = %d, want 200: %s", rec.Code, rec.Body.String())
	}
	var resp map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode rescan: %v", err)
	}
	if resp["candidate_count"] != float64(0) || resp["succeeded"] != float64(0) || resp["failed"] != float64(0) {
		t.Fatalf("expected honest no-candidates response, got %+v", resp)
	}
	if !strings.Contains(fmt.Sprint(resp["note"]), "no raw chat_log turns missing memory") {
		t.Fatalf("unexpected note: %v", resp["note"])
	}
	if !hasAuditEvent(fake.savedAuditLogs, "admin_rescan") {
		t.Fatalf("expected admin_rescan audit for no-candidates path, got %#v", fake.savedAuditLogs)
	}
}

func TestAdminReindexWritesAuditWithoutClaimingExecution(t *testing.T) {
	fake := &turnRecordingStore{}
	cfg := config.Default()
	cfg.StoreMode = config.StoreModeMariaDBAuthority
	srv := NewServer(cfg)
	srv.Store = fake
	srv.StoreOpenError = nil

	mux := http.NewServeMux()
	srv.RegisterRoutes(mux)
	req := httptest.NewRequest(http.MethodPost, "/admin/reindex", bytes.NewReader([]byte(`{"chat_session_id":"sess-reindex","dry_run":true}`)))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("reindex status = %d, want 200: %s", rec.Code, rec.Body.String())
	}
	var resp map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode reindex: %v", err)
	}
	if resp["audit_written"] != true {
		t.Fatalf("audit_written = %v, want true: %#v", resp["audit_written"], resp)
	}
	if resp["reindex_executed"] != false {
		t.Fatalf("reindex_executed = %v, want false", resp["reindex_executed"])
	}
	if !hasAuditEvent(fake.savedAuditLogs, "admin_reindex") {
		t.Fatalf("expected admin_reindex audit, got %#v", fake.savedAuditLogs)
	}
}

func hasAuditEvent(items []*store.AuditLog, eventType string) bool {
	for _, item := range items {
		if item != nil && item.EventType == eventType {
			return true
		}
	}
	return false
}

func TestSeq123P104SaveUpdateDeleteSyncReplayGateMarkers(t *testing.T) {
	srv := NewServer(config.Default())
	mux := http.NewServeMux()
	srv.RegisterRoutes(mux)

	body := `{"chat_session_id":"sess-p104","dry_run":true,"entries":[{"turn_index":4,"user_content":"u","assistant_content":"a"}]}`
	req := httptest.NewRequest(http.MethodPost, "/turns/repair-replay", bytes.NewReader([]byte(body)))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("repair-replay status = %d, want 200: %s", rec.Code, rec.Body.String())
	}
	var replayResp map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &replayResp); err != nil {
		t.Fatalf("decode repair-replay: %v", err)
	}
	replayPlan, ok := replayResp["repair_replay_plan"].(map[string]any)
	if !ok {
		t.Fatalf("repair_replay_plan missing: %#v", replayResp)
	}
	if replayPlan["sync_replay_gate"] != true || replayPlan["save_update_delete_gate"] != true {
		t.Fatalf("repair replay gate markers missing: %#v", replayPlan)
	}
	if replayPlan["mutation_enabled"] != false || replayPlan["would_replay"] != false || replayPlan["would_write"] != false {
		t.Fatalf("shadow repair replay should not mutate: %#v", replayPlan)
	}
	if replayPlan["write_scope"] != "chat_log_effective_input_memory_evidence_kg" {
		t.Fatalf("write_scope = %v", replayPlan["write_scope"])
	}
	if replayPlan["delete_scope"] != "rollback_delete_gate_only" || replayPlan["canonical_input_source"] != "sqlite_store" {
		t.Fatalf("delete/canonical gate mismatch: %#v", replayPlan)
	}

	req = httptest.NewRequest(http.MethodDelete, "/rollback/4?chat_session_id=sess-p104&req_source=validation_gate", nil)
	rec = httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("rollback status = %d, want 200: %s", rec.Code, rec.Body.String())
	}
	var rollbackResp map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &rollbackResp); err != nil {
		t.Fatalf("decode rollback: %v", err)
	}
	rollbackPlan, ok := rollbackResp["rollback_plan"].(map[string]any)
	if !ok {
		t.Fatalf("rollback_plan missing: %#v", rollbackResp)
	}
	if rollbackPlan["save_update_delete_gate"] != true || rollbackPlan["would_delete"] != false || rollbackPlan["mutation_enabled"] != false {
		t.Fatalf("rollback shadow gate mismatch: %#v", rollbackPlan)
	}
}

func TestSeq123P105StaleVectorRollbackRebuildReplayGateMarkers(t *testing.T) {
	srv := NewServer(config.Default())
	mux := http.NewServeMux()
	srv.RegisterRoutes(mux)

	req := httptest.NewRequest(http.MethodPost, "/chroma-shadow/release-hygiene", strings.NewReader(`{"chat_session_id":"sess-p105"}`))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("release-hygiene status = %d, want 200: %s", rec.Code, rec.Body.String())
	}
	var hygieneResp map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &hygieneResp); err != nil {
		t.Fatalf("decode release-hygiene: %v", err)
	}
	evidence, ok := hygieneResp["evidence"].(map[string]any)
	if !ok {
		t.Fatalf("release hygiene evidence missing: %#v", hygieneResp)
	}
	if evidence["stale_vector_policy"] != "tombstone_before_delete" || evidence["delete_policy"] != "canonical_row_first" || evidence["rollback_policy"] != "vector_doc_rollback_with_id" {
		t.Fatalf("stale vector policy mismatch: %#v", evidence)
	}

	req = httptest.NewRequest(http.MethodDelete, "/rollback/8?chat_session_id=sess-p105&req_source=validation_gate", nil)
	rec = httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("rollback status = %d, want 200: %s", rec.Code, rec.Body.String())
	}
	var rollbackResp map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &rollbackResp); err != nil {
		t.Fatalf("decode rollback: %v", err)
	}
	rollbackPlan := rollbackResp["rollback_plan"].(map[string]any)
	if rollbackPlan["stale_vector_replay_gate"] != true || rollbackPlan["rollback_vector_delete_gate"] != true {
		t.Fatalf("rollback stale vector gate mismatch: %#v", rollbackPlan)
	}
	if rollbackPlan["vector_doc_delete_policy"] != "canonical_row_first_then_vector" || rollbackPlan["rebuild_owner"] != "chroma_shadow_orchestrator" {
		t.Fatalf("rollback vector/rebuild markers mismatch: %#v", rollbackPlan)
	}

	req = httptest.NewRequest(http.MethodPost, "/chroma-shadow/rebuild-drill", strings.NewReader(`{"chat_session_id":"sess-p105"}`))
	req.Header.Set("Content-Type", "application/json")
	rec = httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("rebuild-drill status = %d, want 503", rec.Code)
	}
	var rebuildResp map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &rebuildResp); err != nil {
		t.Fatalf("decode rebuild-drill: %v", err)
	}
	trace := rebuildResp["trace_summary"].(map[string]any)
	if trace["rebuild_owner"] != "chroma_shadow_orchestrator" {
		t.Fatalf("rebuild_owner = %v", trace["rebuild_owner"])
	}
}

func TestSeq123P106FailOpenSQLiteBaselineReplayGateMarkers(t *testing.T) {
	srv := NewServer(config.Default())
	mux := http.NewServeMux()
	srv.RegisterRoutes(mux)

	req := httptest.NewRequest(http.MethodPost, "/chroma-shadow/fallback-runbook", strings.NewReader(`{"chat_session_id":"sess-p106"}`))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("fallback-runbook status = %d, want 200: %s", rec.Code, rec.Body.String())
	}
	var resp map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode fallback-runbook: %v", err)
	}
	evidence := resp["evidence"].(map[string]any)
	if evidence["fallback_policy"] != "store_first_then_vector" || evidence["fail_open_baseline"] != true {
		t.Fatalf("fallback fail-open markers mismatch: %#v", evidence)
	}
	if evidence["retrieval_baseline"] != "sqlite_canonical" || evidence["canonical_baseline_source"] != "sqlite_store" || evidence["sqlite_canonical_baseline"] != true {
		t.Fatalf("sqlite canonical baseline markers mismatch: %#v", evidence)
	}
}

func TestSeq123P107Future125OwnerDecisionChecklistMarkers(t *testing.T) {
	srv := NewServer(config.Default())
	mux := http.NewServeMux()
	srv.RegisterRoutes(mux)

	req := httptest.NewRequest(http.MethodPost, "/chroma-shadow/adoption-gate", strings.NewReader(`{"chat_session_id":"sess-p107"}`))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("adoption-gate status = %d, want 200: %s", rec.Code, rec.Body.String())
	}
	var resp map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode adoption-gate: %v", err)
	}
	if resp["live_cutover_allowed"] != false || resp["adoption_gate_state"] != "closed" {
		t.Fatalf("adoption gate must stay closed: %#v", resp)
	}
	if resp["owner_decision_state"] != "pending_pre_12_5" || resp["scope_truth_authority"] != "store_canonical_truth" {
		t.Fatalf("owner decision top-level markers mismatch: %#v", resp)
	}
	decision, ok := resp["future_125_owner_decision"].(map[string]any)
	if !ok {
		t.Fatalf("future_125_owner_decision missing: %#v", resp)
	}
	if decision["long_memory_input_quality"] != "requires_replay_green" || decision["scope_truth_authority"] != "store_canonical_truth" {
		t.Fatalf("future 12.5 decision mismatch: %#v", decision)
	}
}

func TestSeq123P111SQLiteCanonicalInputDisciplineChecklistPass(t *testing.T) {
	srv := NewServer(config.Default())
	mux := http.NewServeMux()
	srv.RegisterRoutes(mux)

	req := httptest.NewRequest(http.MethodPost, "/turns/repair-replay", bytes.NewReader([]byte(`{"chat_session_id":"sess-p111","entries":[{"turn_index":1,"user_content":"u"}]}`)))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("repair-replay status = %d, want 200: %s", rec.Code, rec.Body.String())
	}
	var replayResp map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &replayResp); err != nil {
		t.Fatalf("decode repair-replay: %v", err)
	}
	replayPlan := replayResp["repair_replay_plan"].(map[string]any)
	if replayPlan["canonical_input_source"] != "sqlite_store" || replayPlan["sync_replay_gate"] != true {
		t.Fatalf("canonical input discipline markers mismatch: %#v", replayPlan)
	}
}

func TestSeq123P112ChromaSyncStaleGuardComplete(t *testing.T) {
	srv := NewServer(config.Default())
	mux := http.NewServeMux()
	srv.RegisterRoutes(mux)

	req := httptest.NewRequest(http.MethodPost, "/chroma-shadow/release-hygiene", strings.NewReader(`{"chat_session_id":"sess-p112"}`))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("release-hygiene status = %d, want 200: %s", rec.Code, rec.Body.String())
	}
	var resp map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode release-hygiene: %v", err)
	}
	evidence := resp["evidence"].(map[string]any)
	if evidence["stale_vector_policy"] != "tombstone_before_delete" || evidence["merge_policy"] != "merge_stale_vectors_to_tombstone" {
		t.Fatalf("stale guard markers mismatch: %#v", evidence)
	}
}

func TestSeq123P113FailOpenDriftRebuildVocabularyCleanupComplete(t *testing.T) {
	srv := NewServer(config.Default())
	srv.Vector = nil
	mux := http.NewServeMux()
	srv.RegisterRoutes(mux)

	req := httptest.NewRequest(http.MethodPost, "/chroma-shadow/fallback-runbook", strings.NewReader(`{"chat_session_id":"sess-p113"}`))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("fallback-runbook status = %d, want 200: %s", rec.Code, rec.Body.String())
	}
	var fallbackResp map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &fallbackResp); err != nil {
		t.Fatalf("decode fallback-runbook: %v", err)
	}
	fallbackEvidence := fallbackResp["evidence"].(map[string]any)
	if fallbackEvidence["degraded_mode"] != "canonical_baseline" || fallbackEvidence["retrieval_baseline"] != "sqlite_canonical" {
		t.Fatalf("fallback vocabulary mismatch: %#v", fallbackEvidence)
	}

	req = httptest.NewRequest(http.MethodPost, "/chroma-shadow/visibility-guard", strings.NewReader(`{"chat_session_id":"sess-p113"}`))
	req.Header.Set("Content-Type", "application/json")
	rec = httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("visibility-guard status = %d, want 200: %s", rec.Code, rec.Body.String())
	}
	var visibilityResp map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &visibilityResp); err != nil {
		t.Fatalf("decode visibility-guard: %v", err)
	}
	visibilityEvidence := visibilityResp["evidence"].(map[string]any)
	if visibilityEvidence["drift_policy"] != "shadow_degraded" || visibilityEvidence["drift_action"] != "keep_canonical_baseline" {
		t.Fatalf("drift vocabulary mismatch: %#v", visibilityEvidence)
	}
}

func TestSeq123P114Future125ScopeTruthAuthorityLongMemoryInputQualityExtend(t *testing.T) {
	srv := NewServer(config.Default())
	mux := http.NewServeMux()
	srv.RegisterRoutes(mux)

	req := httptest.NewRequest(http.MethodPost, "/chroma-shadow/adoption-gate", strings.NewReader(`{"chat_session_id":"sess-p114"}`))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("adoption-gate status = %d, want 200: %s", rec.Code, rec.Body.String())
	}
	var resp map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode adoption-gate: %v", err)
	}
	decision := resp["future_125_owner_decision"].(map[string]any)
	if decision["scope_truth_authority"] != "store_canonical_truth" || decision["long_memory_input_quality"] != "requires_replay_green" {
		t.Fatalf("scope/input-quality extension mismatch: %#v", decision)
	}
	gates, ok := decision["required_green_gates"].([]any)
	if !ok || len(gates) < 3 {
		t.Fatalf("required_green_gates missing: %#v", decision)
	}
}

// ---------------------------------------------------------------------------
// /rollback tests
// ---------------------------------------------------------------------------

func TestRollbackShadowPlan(t *testing.T) {
	cfg := config.Default()
	srv := NewServer(cfg)

	mux := http.NewServeMux()
	srv.RegisterRoutes(mux)

	req := httptest.NewRequest(http.MethodDelete, "/rollback/5?chat_session_id=sess-rollback&req_source=adapter", nil)
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
	if resp["turn_index"] != float64(5) {
		t.Errorf("turn_index = %v, want 5", resp["turn_index"])
	}

	rb, ok := resp["rollback_plan"].(map[string]any)
	if !ok {
		t.Fatalf("rollback_plan is not an object")
	}
	if rb["status"] != "shadow_plan" {
		t.Errorf("rollback_plan.status = %v, want shadow_plan", rb["status"])
	}
	if rb["turn_index"] != float64(5) {
		t.Errorf("rollback_plan.turn_index = %v, want 5", rb["turn_index"])
	}
	if rb["chat_session_id"] != "sess-rollback" {
		t.Errorf("rollback_plan.chat_session_id = %v, want sess-rollback", rb["chat_session_id"])
	}
	if rb["req_source"] != "adapter" {
		t.Errorf("rollback_plan.req_source = %v, want adapter", rb["req_source"])
	}
	if rb["would_delete"] != false {
		t.Errorf("rollback_plan.would_delete = %v, want false", rb["would_delete"])
	}
	if rb["would_write"] != false {
		t.Errorf("rollback_plan.would_write = %v, want false", rb["would_write"])
	}
	if rb["mutation_enabled"] != false {
		t.Errorf("rollback_plan.mutation_enabled = %v, want false", rb["mutation_enabled"])
	}
	if rb["reason"] != "R1 shadow mode: rollback not executed" {
		t.Errorf("rollback_plan.reason = %v, want R1 shadow mode: rollback not executed", rb["reason"])
	}
}

func TestRollbackInvalidTurnIndex(t *testing.T) {
	cfg := config.Default()
	srv := NewServer(cfg)

	mux := http.NewServeMux()
	srv.RegisterRoutes(mux)

	req := httptest.NewRequest(http.MethodDelete, "/rollback/abc?chat_session_id=sess-bad", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", rec.Code)
	}
}

func TestRollbackNegativeTurnIndex(t *testing.T) {
	cfg := config.Default()
	srv := NewServer(cfg)

	mux := http.NewServeMux()
	srv.RegisterRoutes(mux)

	req := httptest.NewRequest(http.MethodDelete, "/rollback/-1?chat_session_id=sess-neg", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", rec.Code)
	}
}

// rollbackRecordingStore wraps a Store and records RollbackStore calls.
type rollbackRecordingStore struct {
	store.Store
	deletes   []string
	deleteErr error
	audits    []*store.AuditLog
}

func (r *rollbackRecordingStore) DeleteChatLogs(ctx context.Context, sid string, fromTurn int) error {
	if r.deleteErr != nil {
		return r.deleteErr
	}
	r.deletes = append(r.deletes, fmt.Sprintf("chat_logs:%s:%d", sid, fromTurn))
	return nil
}
func (r *rollbackRecordingStore) DeleteEffectiveInputs(ctx context.Context, sid string, fromTurn int) error {
	if r.deleteErr != nil {
		return r.deleteErr
	}
	r.deletes = append(r.deletes, fmt.Sprintf("effective_inputs:%s:%d", sid, fromTurn))
	return nil
}
func (r *rollbackRecordingStore) DeleteMemories(ctx context.Context, sid string, fromTurn int) error {
	if r.deleteErr != nil {
		return r.deleteErr
	}
	r.deletes = append(r.deletes, fmt.Sprintf("memories:%s:%d", sid, fromTurn))
	return nil
}
func (r *rollbackRecordingStore) DeleteEvidence(ctx context.Context, sid string, fromTurn int) error {
	if r.deleteErr != nil {
		return r.deleteErr
	}
	r.deletes = append(r.deletes, fmt.Sprintf("direct_evidence:%s:%d", sid, fromTurn))
	return nil
}
func (r *rollbackRecordingStore) DeleteKGTriples(ctx context.Context, sid string, fromTurn int) error {
	if r.deleteErr != nil {
		return r.deleteErr
	}
	r.deletes = append(r.deletes, fmt.Sprintf("kg_triples:%s:%d", sid, fromTurn))
	return nil
}
func (r *rollbackRecordingStore) DeleteCriticFeedback(ctx context.Context, sid string, fromTurn int) error {
	if r.deleteErr != nil {
		return r.deleteErr
	}
	r.deletes = append(r.deletes, fmt.Sprintf("critic_feedback:%s:%d", sid, fromTurn))
	return nil
}
func (r *rollbackRecordingStore) DeleteCharacterEvents(ctx context.Context, sid string, fromTurn int) error {
	if r.deleteErr != nil {
		return r.deleteErr
	}
	r.deletes = append(r.deletes, fmt.Sprintf("character_events:%s:%d", sid, fromTurn))
	return nil
}
func (r *rollbackRecordingStore) DeleteEntities(ctx context.Context, sid string, fromTurn int) error {
	if r.deleteErr != nil {
		return r.deleteErr
	}
	r.deletes = append(r.deletes, fmt.Sprintf("entities:%s:%d", sid, fromTurn))
	return nil
}
func (r *rollbackRecordingStore) DeleteTrustStates(ctx context.Context, sid string, fromTurn int) error {
	if r.deleteErr != nil {
		return r.deleteErr
	}
	r.deletes = append(r.deletes, fmt.Sprintf("trust_states:%s:%d", sid, fromTurn))
	return nil
}
func (r *rollbackRecordingStore) DeleteStorylines(ctx context.Context, sid string, fromTurn int) error {
	if r.deleteErr != nil {
		return r.deleteErr
	}
	r.deletes = append(r.deletes, fmt.Sprintf("storylines:%s:%d", sid, fromTurn))
	return nil
}
func (r *rollbackRecordingStore) DeleteWorldRules(ctx context.Context, sid string, fromTurn int) error {
	if r.deleteErr != nil {
		return r.deleteErr
	}
	r.deletes = append(r.deletes, fmt.Sprintf("world_rules:%s:%d", sid, fromTurn))
	return nil
}
func (r *rollbackRecordingStore) DeleteCharacterStates(ctx context.Context, sid string, fromTurn int) error {
	if r.deleteErr != nil {
		return r.deleteErr
	}
	r.deletes = append(r.deletes, fmt.Sprintf("character_states:%s:%d", sid, fromTurn))
	return nil
}
func (r *rollbackRecordingStore) DeletePendingThreads(ctx context.Context, sid string, fromTurn int) error {
	if r.deleteErr != nil {
		return r.deleteErr
	}
	r.deletes = append(r.deletes, fmt.Sprintf("pending_threads:%s:%d", sid, fromTurn))
	return nil
}
func (r *rollbackRecordingStore) DeleteActiveStates(ctx context.Context, sid string, fromTurn int) error {
	if r.deleteErr != nil {
		return r.deleteErr
	}
	r.deletes = append(r.deletes, fmt.Sprintf("active_states:%s:%d", sid, fromTurn))
	return nil
}
func (r *rollbackRecordingStore) DeleteCanonicalStateLayers(ctx context.Context, sid string, fromTurn int) error {
	if r.deleteErr != nil {
		return r.deleteErr
	}
	r.deletes = append(r.deletes, fmt.Sprintf("canonical_state_layers:%s:%d", sid, fromTurn))
	return nil
}
func (r *rollbackRecordingStore) DeleteEpisodeSummaries(ctx context.Context, sid string, fromTurn int) error {
	if r.deleteErr != nil {
		return r.deleteErr
	}
	r.deletes = append(r.deletes, fmt.Sprintf("episode_summaries:%s:%d", sid, fromTurn))
	return nil
}
func (r *rollbackRecordingStore) DeleteGuidancePlanState(ctx context.Context, sid string, fromTurn int) error {
	if r.deleteErr != nil {
		return r.deleteErr
	}
	r.deletes = append(r.deletes, fmt.Sprintf("guidance_plan_states:%s:%d", sid, fromTurn))
	return nil
}
func (r *rollbackRecordingStore) DeleteChapterSummaries(ctx context.Context, sid string, fromTurn int) error {
	if r.deleteErr != nil {
		return r.deleteErr
	}
	r.deletes = append(r.deletes, fmt.Sprintf("chapter_summaries:%s:%d", sid, fromTurn))
	return nil
}
func (r *rollbackRecordingStore) DeleteArcSummaries(ctx context.Context, sid string, fromTurn int) error {
	if r.deleteErr != nil {
		return r.deleteErr
	}
	r.deletes = append(r.deletes, fmt.Sprintf("arc_summaries:%s:%d", sid, fromTurn))
	return nil
}
func (r *rollbackRecordingStore) DeleteSagaDigests(ctx context.Context, sid string, fromTurn int) error {
	if r.deleteErr != nil {
		return r.deleteErr
	}
	r.deletes = append(r.deletes, fmt.Sprintf("saga_digests:%s:%d", sid, fromTurn))
	return nil
}
func (r *rollbackRecordingStore) DeleteSessionActiveScopes(ctx context.Context, sid string, fromTurn int) error {
	if r.deleteErr != nil {
		return r.deleteErr
	}
	r.deletes = append(r.deletes, fmt.Sprintf("session_active_scopes:%s:%d", sid, fromTurn))
	return nil
}
func (r *rollbackRecordingStore) DeleteProtagonistEntityMemories(ctx context.Context, sid string, fromTurn int) error {
	if r.deleteErr != nil {
		return r.deleteErr
	}
	r.deletes = append(r.deletes, fmt.Sprintf("subjective_entity_memories:%s:%d", sid, fromTurn))
	return nil
}
func (r *rollbackRecordingStore) DeleteConsequenceRecords(ctx context.Context, sid string, fromTurn int) error {
	if r.deleteErr != nil {
		return r.deleteErr
	}
	r.deletes = append(r.deletes, fmt.Sprintf("consequence_records:%s:%d", sid, fromTurn))
	return nil
}
func (r *rollbackRecordingStore) DeletePsychologyBranches(ctx context.Context, sid string, fromTurn int) error {
	if r.deleteErr != nil {
		return r.deleteErr
	}
	r.deletes = append(r.deletes, fmt.Sprintf("psychology_branches:%s:%d", sid, fromTurn))
	return nil
}
func (r *rollbackRecordingStore) DeleteThemeOffscreenCarries(ctx context.Context, sid string, fromTurn int) error {
	if r.deleteErr != nil {
		return r.deleteErr
	}
	r.deletes = append(r.deletes, fmt.Sprintf("theme_offscreen_carries:%s:%d", sid, fromTurn))
	return nil
}
func (r *rollbackRecordingStore) DeleteCaptureVerificationRecords(ctx context.Context, sid string, fromTurn int) error {
	if r.deleteErr != nil {
		return r.deleteErr
	}
	r.deletes = append(r.deletes, fmt.Sprintf("capture_verification_records:%s:%d", sid, fromTurn))
	return nil
}
func (r *rollbackRecordingStore) DeleteStatusCurrentValues(ctx context.Context, sid string, fromTurn int) error {
	if r.deleteErr != nil {
		return r.deleteErr
	}
	r.deletes = append(r.deletes, fmt.Sprintf("status_current_values:%s:%d", sid, fromTurn))
	return nil
}
func (r *rollbackRecordingStore) DeleteStatusChangeEvents(ctx context.Context, sid string, fromTurn int) error {
	if r.deleteErr != nil {
		return r.deleteErr
	}
	r.deletes = append(r.deletes, fmt.Sprintf("status_change_events:%s:%d", sid, fromTurn))
	return nil
}
func (r *rollbackRecordingStore) DeleteStatusEffects(ctx context.Context, sid string, fromTurn int) error {
	if r.deleteErr != nil {
		return r.deleteErr
	}
	r.deletes = append(r.deletes, fmt.Sprintf("status_effects:%s:%d", sid, fromTurn))
	return nil
}
func (r *rollbackRecordingStore) DeleteSession(ctx context.Context, sid string) error {
	if r.deleteErr != nil {
		return r.deleteErr
	}
	r.deletes = append(r.deletes, fmt.Sprintf("session:%s", sid))
	return nil
}
func (r *rollbackRecordingStore) SaveAuditLog(ctx context.Context, a *store.AuditLog) error {
	r.audits = append(r.audits, a)
	return nil
}

func TestRollbackLiveWriteExecutesDeletions(t *testing.T) {
	cfg := config.Default()
	cfg.StoreMode = config.StoreModeMariaDBAuthority

	vec := &turnRecordingVectorStore{}
	rec := &rollbackRecordingStore{Store: &turnRecordingStore{returnMemories: []store.Memory{
		{ID: 41, ChatSessionID: "sess-live", TurnIndex: 5},
		{ID: 42, ChatSessionID: "sess-live", TurnIndex: 6},
	}}}
	srv := &Server{
		Cfg:            cfg,
		Store:          rec,
		StoreOpenError: nil,
		Vector:         vec,
	}

	mux := http.NewServeMux()
	srv.RegisterRoutes(mux)

	req := httptest.NewRequest(http.MethodDelete, "/rollback/5?chat_session_id=sess-live&req_source=auto_rollback", nil)
	recorder := httptest.NewRecorder()
	mux.ServeHTTP(recorder, req)

	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200: %s", recorder.Code, recorder.Body.String())
	}

	var resp map[string]any
	if err := json.Unmarshal(recorder.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	if resp["status"] != "ok" {
		t.Errorf("status = %v, want ok", resp["status"])
	}
	if resp["source"] != "mariadb_authority" {
		t.Errorf("source = %v, want mariadb_authority", resp["source"])
	}

	rb, ok := resp["rollback_plan"].(map[string]any)
	if !ok {
		t.Fatalf("rollback_plan is not an object")
	}
	if rb["status"] != "executed" {
		t.Errorf("rollback_plan.status = %v, want executed", rb["status"])
	}
	if rb["would_delete"] != true {
		t.Errorf("rollback_plan.would_delete = %v, want true", rb["would_delete"])
	}
	if rb["mutation_enabled"] != true {
		t.Errorf("rollback_plan.mutation_enabled = %v, want true", rb["mutation_enabled"])
	}
	if rb["turn_delete_policy"] != "tail_from_earliest_deleted_turn" {
		t.Errorf("rollback_plan.turn_delete_policy = %v", rb["turn_delete_policy"])
	}
	if rb["hierarchy_invalidation"] != "delete_overlapping_episode_chapter_arc_saga_ranges" {
		t.Errorf("rollback_plan.hierarchy_invalidation = %v", rb["hierarchy_invalidation"])
	}
	if rb["step23_invalidation"] != "delete_turn_scoped_support_records_from_from_turn" {
		t.Errorf("rollback_plan.step23_invalidation = %v", rb["step23_invalidation"])
	}

	wantDeletes := []string{
		"chat_logs:sess-live:5",
		"effective_inputs:sess-live:5",
		"memories:sess-live:5",
		"direct_evidence:sess-live:5",
		"kg_triples:sess-live:5",
		"critic_feedback:sess-live:5",
		"character_events:sess-live:5",
		"entities:sess-live:5",
		"trust_states:sess-live:5",
		"storylines:sess-live:5",
		"world_rules:sess-live:5",
		"character_states:sess-live:5",
		"pending_threads:sess-live:5",
		"active_states:sess-live:5",
		"canonical_state_layers:sess-live:5",
		"episode_summaries:sess-live:5",
		"guidance_plan_states:sess-live:5",
		"chapter_summaries:sess-live:5",
		"arc_summaries:sess-live:5",
		"saga_digests:sess-live:5",
		"session_active_scopes:sess-live:5",
		"subjective_entity_memories:sess-live:5",
		"consequence_records:sess-live:5",
		"psychology_branches:sess-live:5",
		"theme_offscreen_carries:sess-live:5",
		"capture_verification_records:sess-live:5",
		"status_current_values:sess-live:5",
		"status_change_events:sess-live:5",
		"status_effects:sess-live:5",
	}
	if len(rec.deletes) != len(wantDeletes) {
		t.Errorf("delete call count = %d, want %d", len(rec.deletes), len(wantDeletes))
	}
	for i, want := range wantDeletes {
		if i >= len(rec.deletes) {
			break
		}
		if rec.deletes[i] != want {
			t.Errorf("delete[%d] = %s, want %s", i, rec.deletes[i], want)
		}
	}
	wantVectorIDs := []string{"memory:sess-live:41", "memory:41", "memory:sess-live:42", "memory:42"}
	if len(vec.deletedDocumentIDs) != len(wantVectorIDs) {
		t.Fatalf("deleted vector ids = %#v, want %#v", vec.deletedDocumentIDs, wantVectorIDs)
	}
	for i, want := range wantVectorIDs {
		if vec.deletedDocumentIDs[i] != want {
			t.Fatalf("deleted vector ids = %#v, want %#v", vec.deletedDocumentIDs, wantVectorIDs)
		}
	}
	if len(rec.audits) != 1 {
		t.Fatalf("audit count = %d, want 1", len(rec.audits))
	}
	if rec.audits[0].Source != "auto_rollback" {
		t.Fatalf("audit source = %q, want auto_rollback", rec.audits[0].Source)
	}
}

func TestRollbackLiveWritePartialError(t *testing.T) {
	cfg := config.Default()
	cfg.StoreMode = config.StoreModeMariaDBShadow
	cfg.MilvusSDKEnabled = true

	rec := &rollbackRecordingStore{Store: store.NewNoopStore()}
	rec.deleteErr = errors.New("boom")
	srv := &Server{
		Cfg:            cfg,
		Store:          rec,
		StoreOpenError: nil,
		Vector:         vector.NewFakeVectorStore(),
	}

	mux := http.NewServeMux()
	srv.RegisterRoutes(mux)

	req := httptest.NewRequest(http.MethodDelete, "/rollback/3?chat_session_id=sess-partial", nil)
	recorder := httptest.NewRecorder()
	mux.ServeHTTP(recorder, req)

	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200: %s", recorder.Code, recorder.Body.String())
	}

	var resp map[string]any
	if err := json.Unmarshal(recorder.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	if resp["status"] != "partial_error" {
		t.Errorf("status = %v, want partial_error", resp["status"])
	}
	errs, ok := resp["errors"].([]any)
	if !ok || len(errs) == 0 {
		t.Errorf("expected errors, got %v", resp["errors"])
	}
}

func TestCompleteTurnCriticDeltaPromotesCanonicalStateLayersWithProvenance(t *testing.T) {
	fake := &turnRecordingStore{}
	srv := NewServer(config.Default())
	srv.Store = fake
	srv.StoreOpenError = nil

	extraction := normalizeCriticExtraction(map[string]any{
		"turn_summary":     "Mina kept the scene tense while a promise stayed open.",
		"importance_score": 7,
		"relationship_memory": map[string]any{
			"bond_and_distance": "Mina trusts Rowan after the rescue.",
			"confidence":        0.86,
			"verification":      "verified",
		},
		"state_deltas": map[string]any{
			"scene_state":  map[string]any{"mood": "tense", "location": "archive hall"},
			"confidence":   0.82,
			"verification": "verified",
		},
		"pending_threads": []any{map[string]any{
			"thread_type": "promise",
			"title":       "Rowan must answer Mina later",
			"confidence":  0.9,
		}},
	})

	result := srv.saveCriticExtractionArtifacts(context.Background(), "sess-hs", 12, extraction, "Mina trusts Rowan.", completeTurnEmbeddingConfig{}, time.Unix(1200, 0))
	if result.ActiveStates != 3 {
		t.Fatalf("active states saved = %d, want 3", result.ActiveStates)
	}
	if result.CanonicalStateLayers != 3 || len(fake.savedCanonicalLayers) != 3 {
		t.Fatalf("canonical layers saved = %d/%d, want 3", result.CanonicalStateLayers, len(fake.savedCanonicalLayers))
	}
	seen := map[string]store.CanonicalStateLayer{}
	for _, item := range fake.savedCanonicalLayers {
		seen[item.LayerType] = *item
		if item.SourceTurn != 12 || item.LastVerifiedTurn != 12 || item.Confidence < 0.7 {
			t.Fatalf("canonical provenance not preserved: %#v", item)
		}
	}
	for _, layerType := range []string{"relationship_state", "scene_state", "unresolved_threads"} {
		if _, ok := seen[layerType]; !ok {
			t.Fatalf("missing canonical layer %q in %#v", layerType, seen)
		}
	}
}

func TestLC1BCanonicalStateWriteCostMeasurementClassifiesModes(t *testing.T) {
	newServer := func(fake *turnRecordingStore) *Server {
		srv := NewServer(config.Default())
		srv.Store = fake
		srv.StoreOpenError = nil
		return srv
	}
	extractionForState := func(state map[string]any) map[string]any {
		return normalizeCriticExtraction(map[string]any{
			"turn_summary":     "The current scene state was verified.",
			"importance_score": 7,
			"state_deltas":     state,
		})
	}
	state := map[string]any{
		"scene_state":  map[string]any{"mood": "tense", "location": "archive hall"},
		"confidence":   0.91,
		"verification": "verified",
	}

	bootstrapFake := &turnRecordingStore{}
	bootstrapResult := newServer(bootstrapFake).saveCriticExtractionArtifacts(context.Background(), "sess-lc1b-bootstrap", 20, extractionForState(state), "verified scene", completeTurnEmbeddingConfig{}, time.Unix(2000, 0))
	if bootstrapResult.CanonicalStateWriteCost == nil {
		t.Fatal("bootstrap cost measurement missing")
	}
	if bootstrapResult.CanonicalStateWriteCost.PolicyVersion != "lc1b.v1" {
		t.Fatalf("bootstrap policy = %q, want lc1b.v1", bootstrapResult.CanonicalStateWriteCost.PolicyVersion)
	}
	if bootstrapResult.CanonicalStateWriteCost.StateWriteCount != 1 || bootstrapResult.CanonicalStateWriteCost.FullRewriteCount != 1 {
		t.Fatalf("bootstrap counts = %#v, want one full rewrite write", bootstrapResult.CanonicalStateWriteCost)
	}
	if got := bootstrapResult.CanonicalStateWriteCost.Items[0]["write_mode"]; got != "full_rewrite_bootstrap" {
		t.Fatalf("bootstrap write_mode = %v", got)
	}

	sanitizedState := sanitizeStateDeltasForParticipant(state)
	deltaFake := &turnRecordingStore{returnCanonicalLayers: []store.CanonicalStateLayer{{
		ChatSessionID: "sess-lc1b-delta",
		LayerType:     "scene_state",
		Content:       mustCompactJSON(sanitizedState),
		TurnIndex:     19,
	}}}
	deltaResult := newServer(deltaFake).saveCriticExtractionArtifacts(context.Background(), "sess-lc1b-delta", 21, extractionForState(state), "verified scene", completeTurnEmbeddingConfig{}, time.Unix(2100, 0))
	if deltaResult.CanonicalStateWriteCost == nil {
		t.Fatal("delta cost measurement missing")
	}
	if deltaResult.CanonicalStateWriteCost.StateWriteCount != 1 || deltaResult.CanonicalStateWriteCost.DeltaUpdateCount != 1 {
		t.Fatalf("delta counts = %#v, want one delta update write", deltaResult.CanonicalStateWriteCost)
	}
	if got := deltaResult.CanonicalStateWriteCost.Items[0]["write_mode"]; got != "delta_update" {
		t.Fatalf("delta write_mode = %v", got)
	}

	rewriteFake := &turnRecordingStore{returnCanonicalLayers: []store.CanonicalStateLayer{{
		ChatSessionID: "sess-lc1b-rewrite",
		LayerType:     "scene_state",
		Content:       `{"scene_state":{"mood":"peaceful","location":"sunlit garden"},"confidence":0.91,"verification":"verified"}`,
		TurnIndex:     19,
	}}}
	rewriteResult := newServer(rewriteFake).saveCriticExtractionArtifacts(context.Background(), "sess-lc1b-rewrite", 22, extractionForState(state), "verified scene", completeTurnEmbeddingConfig{}, time.Unix(2200, 0))
	if rewriteResult.CanonicalStateWriteCost == nil {
		t.Fatal("full rewrite cost measurement missing")
	}
	if rewriteResult.CanonicalStateWriteCost.StateWriteCount != 1 || rewriteResult.CanonicalStateWriteCost.FullRewriteCount != 1 {
		t.Fatalf("rewrite counts = %#v, want one full rewrite write", rewriteResult.CanonicalStateWriteCost)
	}
	if got := rewriteResult.CanonicalStateWriteCost.Items[0]["write_mode"]; got != "full_rewrite" {
		t.Fatalf("rewrite write_mode = %v", got)
	}
}

func TestCompleteTurnCanonicalStatePromotionRequiresVerifiedConfidence(t *testing.T) {
	fake := &turnRecordingStore{}
	srv := NewServer(config.Default())
	srv.Store = fake
	srv.StoreOpenError = nil

	extraction := normalizeCriticExtraction(map[string]any{
		"turn_summary":     "Low confidence state should stay active only.",
		"importance_score": 5,
		"state_deltas": map[string]any{
			"scene_state":  map[string]any{"mood": "uncertain"},
			"confidence":   0.95,
			"verification": "pending",
		},
		"pending_threads": []any{map[string]any{
			"thread_type": "open_question",
			"title":       "Maybe this is a thread",
			"confidence":  0.4,
		}},
	})

	result := srv.saveCriticExtractionArtifacts(context.Background(), "sess-hs-low", 13, extraction, "Maybe.", completeTurnEmbeddingConfig{}, time.Unix(1300, 0))
	if result.ActiveStates != 2 {
		t.Fatalf("active states saved = %d, want 2", result.ActiveStates)
	}
	if result.CanonicalStateLayers != 0 || len(fake.savedCanonicalLayers) != 0 {
		t.Fatalf("canonical layers should not promote pending/low-confidence state, got result=%d layers=%#v", result.CanonicalStateLayers, fake.savedCanonicalLayers)
	}
}

func TestCompleteTurnDualShadowWithCriticSavesAllArtifacts(t *testing.T) {
	fake := &turnRecordingStore{
		returnCharStates: []store.CharacterState{{CharacterName: "Alice"}},
	}
	vec := &turnRecordingVectorStore{}
	cfg := config.Default()
	cfg.StoreMode = config.StoreModeMariaDBShadow
	cfg.ChromaEndpoint = "http://127.0.0.1:8000"
	srv := NewServer(cfg)
	// Wrap the recording store as the shadow side of dual-write
	srv.Store = store.NewDualWriteStore(store.NewNoopStore(), fake)
	srv.StoreOpenError = nil
	srv.Vector = vec
	srv.VectorOpenError = nil

	extraction := map[string]any{
		"turn_summary":           "Alice decided to trust Bob after the rescue.",
		"importance_score":       8,
		"relationship_memory":    map[string]any{"bond_and_distance": "Alice trusts Bob more after he helped her.", "trust": 0.8},
		"entities":               map[string]any{"characters": []any{map[string]any{"name": "Alicee", "role": "protagonist", "status_emotion": "relieved"}}},
		"kg_triples":             []any{map[string]any{"subject": "Alicee", "predicate": "trusts", "object": "Bob", "valid_from": 2}},
		"archive_hint":           map[string]any{"wing": "wing_general", "room": "hall_relationships"},
		"evidence_excerpts":      []any{"I trust Bob."},
		"emotional_intensity":    0.7,
		"narrative_significance": 0.9,
		"state_deltas":           map[string]any{"scene_state": map[string]any{"mood": "warm"}},
		"character_deltas": []any{map[string]any{
			"name":   "Alicee",
			"status": map[string]any{"emotion": "relieved"},
			"events": []any{map[string]any{"type": "relationship_shift", "detail": "Alice's trust in Bob increased."}},
		}},
		"pending_threads": []any{map[string]any{"thread_type": "promise", "title": "Alice thanks Bob later", "confidence": 0.85}},
		"world_rules":     []any{map[string]any{"scope": "session", "category": "relationship", "key": "trust_changes_need_evidence", "value": "Trust shifts should be grounded in visible actions."}},
	}
	extractionBytes, _ := json.Marshal(extraction)
	chatResp, _ := json.Marshal(map[string]any{
		"model":   "critic-model",
		"choices": []any{map[string]any{"message": map[string]any{"content": string(extractionBytes)}}},
	})
	oldClient := proxyHTTPClient
	proxyHTTPClient = &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		if strings.HasSuffix(r.URL.Path, "/embeddings") {
			return &http.Response{
				StatusCode: http.StatusOK,
				Header:     make(http.Header),
				Body:       io.NopCloser(strings.NewReader(`{"model":"embed-model","data":[{"embedding":[0.1,0.2,0.3]}]}`)),
			}, nil
		}
		return &http.Response{
			StatusCode: http.StatusOK,
			Header:     make(http.Header),
			Body:       io.NopCloser(bytes.NewReader(chatResp)),
		}, nil
	})}
	defer func() { proxyHTTPClient = oldClient }()

	mux := http.NewServeMux()
	srv.RegisterRoutes(mux)

	body := map[string]any{
		"chat_session_id":   "sess-dual-shadow",
		"turn_index":        2,
		"user_input":        "I trust Bob.",
		"assistant_content": "Alice relaxed after Bob helped her.",
		"context_messages":  []any{},
		"improvement_trace": map[string]any{"score": 9},
		"client_meta":       map[string]any{"critic": map[string]any{"api_key": "sk-test", "endpoint": "https://api.example.com/v1", "model": "critic-model", "provider": "openai", "max_tokens": 1200}, "embedding": map[string]any{"api_key": "sk-test", "endpoint": "https://api.example.com/v1", "model": "embed-model", "provider": "openai"}},
		"request_type":      "model",
	}
	raw, _ := json.Marshal(body)
	req := httptest.NewRequest(http.MethodPost, "/complete-turn", bytes.NewReader(raw))
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
		t.Fatalf("critic_triggered = %v, want true", resp["critic_triggered"])
	}
	if len(fake.savedMemories) != 1 {
		t.Fatalf("expected one memory, got %d", len(fake.savedMemories))
	}
	if len(fake.savedEvidence) != 1 {
		t.Fatalf("expected one evidence, got %d", len(fake.savedEvidence))
	}
	if len(fake.savedKGTriples) != 1 {
		t.Fatalf("expected one KG triple, got %d", len(fake.savedKGTriples))
	}
	if len(fake.savedEntities) != 1 {
		t.Fatalf("expected one entity, got %d", len(fake.savedEntities))
	}
	if len(fake.savedTrusts) != 1 {
		t.Fatalf("expected one trust state, got %d", len(fake.savedTrusts))
	}
	if len(fake.savedCharacterEvents) != 1 {
		t.Fatalf("expected one character event, got %d", len(fake.savedCharacterEvents))
	}
	if len(fake.savedCharacterStates) != 1 {
		t.Fatalf("expected one character state, got %d", len(fake.savedCharacterStates))
	}
	if len(fake.savedPendingThreads) != 1 {
		t.Fatalf("expected one pending thread, got %d", len(fake.savedPendingThreads))
	}
	if len(fake.savedActiveStates) == 0 {
		t.Fatalf("expected at least one active state, got %d", len(fake.savedActiveStates))
	}
	if len(fake.savedWorldRules) != 1 {
		t.Fatalf("expected one world rule, got %d", len(fake.savedWorldRules))
	}
	if len(fake.savedStorylines) != 1 {
		t.Fatalf("expected one storyline, got %d", len(fake.savedStorylines))
	}
	if len(vec.docs) != 3 {
		t.Fatalf("expected memory/evidence/world-rule vector upserts, got %d", len(vec.docs))
	}
}

func TestBuildRecallResultIntentExecutionShadow(t *testing.T) {
	docs := []map[string]any{
		{"tier": "memory", "document_id": "d1", "text": "hello"},
		{"tier": "episode", "document_id": "d2", "text": "world"},
		{"tier": "chapter", "document_id": "d3", "text": "foo"},
	}
	shadow := buildIntentExecutionShadow(docs, map[string]any{}, "long", q3PacketBudgetPolicy())
	if shadow["version"] != "p29a.v1" {
		t.Fatalf("version mismatch: %v", shadow["version"])
	}
	if shadow["routing_mode"] != "per_intent_shadow" {
		t.Fatalf("routing_mode mismatch: %v", shadow["routing_mode"])
	}
	if shadow["status"] != "ready" {
		t.Fatalf("status mismatch: %v", shadow["status"])
	}
	intents, ok := shadow["intents"].([]map[string]any)
	if !ok || len(intents) != 4 {
		t.Fatalf("intents mismatch: %v", shadow["intents"])
	}
	be, ok := shadow["budget_enforcement"].(map[string]any)
	if !ok || be["mode"] != "enforced_shadow" {
		t.Fatalf("budget_enforcement mismatch")
	}
	gt, ok := shadow["guarded_takeover"].(map[string]any)
	if !ok || gt["decision"] != "shadow_compare" {
		t.Fatalf("guarded_takeover mismatch")
	}
	et, ok := shadow["enforced_takeover"].(map[string]any)
	if !ok || et["decision"] != "enforced_shadow" {
		t.Fatalf("enforced_takeover mismatch")
	}
	tpv, ok := shadow["tier_priority_verification"].(map[string]any)
	if !ok {
		t.Fatalf("tier_priority_verification missing")
	}
	if tpv["version"] != "t1d.v1" {
		t.Fatalf("tier_priority_verification version = %v, want t1d.v1", tpv["version"])
	}
	if tpv["mode"] != "verification_only" {
		t.Fatalf("tier_priority_verification mode = %v, want verification_only", tpv["mode"])
	}
	if tpv["status"] != "ready" {
		t.Fatalf("tier_priority_verification status = %v, want ready", tpv["status"])
	}
	if tpv["priority_verdict"] != "tier_priority_verification_shadow" {
		t.Fatalf("tier_priority_verification priority_verdict = %v, want tier_priority_verification_shadow", tpv["priority_verdict"])
	}
	if tpv["requires_manual_review"] != false {
		t.Fatalf("tier_priority_verification requires_manual_review = %v, want false", tpv["requires_manual_review"])
	}
	if tpv["saga_collision_policy"] != "none" {
		t.Fatalf("tier_priority_verification saga_collision_policy = %v, want none", tpv["saga_collision_policy"])
	}
	te, ok := tpv["tier_events"].([]map[string]any)
	if !ok || len(te) != 4 {
		t.Fatalf("tier_events mismatch: %v", tpv["tier_events"])
	}
}

func TestBuildRecallResultIntentExecutionShadowFailOpen(t *testing.T) {
	shadow := buildIntentExecutionShadow([]map[string]any{}, map[string]any{}, "long", q3PacketBudgetPolicy())
	if shadow["status"] != "off" {
		t.Fatalf("status = %v, want off", shadow["status"])
	}
	gt := shadow["guarded_takeover"].(map[string]any)
	if gt["decision"] != "fail_open" {
		t.Fatalf("guarded_takeover decision = %v, want fail_open", gt["decision"])
	}
	et := shadow["enforced_takeover"].(map[string]any)
	if et["decision"] != "fail_open" {
		t.Fatalf("enforced_takeover decision = %v, want fail_open", et["decision"])
	}
	tpv := shadow["tier_priority_verification"].(map[string]any)
	if tpv["status"] != "off" {
		t.Fatalf("tier_priority_verification status = %v, want off", tpv["status"])
	}
	if tpv["saga_collision_policy"] != "none" {
		t.Fatalf("tier_priority_verification saga_collision_policy = %v, want none", tpv["saga_collision_policy"])
	}
}

func TestBuildRecallResultIntentContractTierPrioritySurface(t *testing.T) {
	contract := buildIntentContractQ3()
	if contract["version"] != "q3a.v1" {
		t.Fatalf("version = %v, want q3a.v1", contract["version"])
	}
	rstp, ok := contract["routing_shadow_tier_priority"].(map[string]any)
	if !ok {
		t.Fatal("routing_shadow_tier_priority missing")
	}
	if rstp["version"] != "t1d.v1" {
		t.Fatalf("version = %v, want t1d.v1", rstp["version"])
	}
	if rstp["mode"] != "verification_only" {
		t.Fatalf("mode = %v, want verification_only", rstp["mode"])
	}
	if rstp["status"] != "shadow_only" {
		t.Fatalf("status = %v, want shadow_only", rstp["status"])
	}
	if rstp["priority_verdict"] != "tier_priority_verification_shadow" {
		t.Fatalf("priority_verdict = %v, want tier_priority_verification_shadow", rstp["priority_verdict"])
	}
	if rstp["requires_manual_review"] != false {
		t.Fatalf("requires_manual_review = %v, want false", rstp["requires_manual_review"])
	}
	tc, ok := rstp["tier_counts"].(map[string]int)
	if !ok {
		t.Fatal("tier_counts missing")
	}
	if tc["memory"] != 3 {
		t.Fatalf("memory tier count = %v, want 3", tc["memory"])
	}
	if tc["episode"] != 2 {
		t.Fatalf("episode tier count = %v, want 2", tc["episode"])
	}
	if tc["chapter"] != 2 {
		t.Fatalf("chapter tier count = %v, want 2", tc["chapter"])
	}
	if tc["arc"] != 3 {
		t.Fatalf("arc tier count = %v, want 3", tc["arc"])
	}
	if tc["saga"] != 2 {
		t.Fatalf("saga tier count = %v, want 2", tc["saga"])
	}
}

func TestBuildRecallResultIntentExecutionShadowUltraProfile(t *testing.T) {
	docs := []map[string]any{
		{"tier": "memory", "document_id": "d1", "text": "hello"},
		{"tier": "saga", "document_id": "d2", "text": "world"},
	}
	shadow := buildIntentExecutionShadow(docs, map[string]any{}, "ultra", q3PacketBudgetPolicy())
	tpv, ok := shadow["tier_priority_verification"].(map[string]any)
	if !ok {
		t.Fatal("tier_priority_verification missing")
	}
	if tpv["saga_collision_policy"] != "saga_floor_reserve_v0d" {
		t.Fatalf("saga_collision_policy = %v, want saga_floor_reserve_v0d", tpv["saga_collision_policy"])
	}
	te, ok := tpv["tier_events"].([]map[string]any)
	if !ok || len(te) != 4 {
		t.Fatalf("tier_events len = %v, want 4", len(te))
	}
}

func TestBuildRecallResultIntentExecutionShadowExtremeProfile(t *testing.T) {
	docs := []map[string]any{
		{"tier": "memory", "document_id": "d1", "text": "hello"},
	}
	shadow := buildIntentExecutionShadow(docs, map[string]any{}, "extreme", q3PacketBudgetPolicy())
	tpv, ok := shadow["tier_priority_verification"].(map[string]any)
	if !ok {
		t.Fatal("tier_priority_verification missing")
	}
	if tpv["saga_collision_policy"] != "saga_floor_reserve_v0d" {
		t.Fatalf("saga_collision_policy = %v, want saga_floor_reserve_v0d", tpv["saga_collision_policy"])
	}
}

func TestBuildRecallResultHierarchyConsistencyTrace(t *testing.T) {
	episodeSums := []store.EpisodeSummary{
		{ID: 10, ChatSessionID: "s1", FromTurn: 1, ToTurn: 3, SummaryText: "ep1"},
	}
	resumePack := &store.ResumePack{
		Chapter: &store.ChapterSummary{
			ID: 20, ChatSessionID: "s1", FromTurn: 1, ToTurn: 5, ChapterIndex: 1, ChapterTitle: "Ch1", SummaryText: "ch1",
		},
		Saga: &store.SagaDigest{
			ID: 40, ChatSessionID: "s1", FromTurn: 1, ToTurn: 20, EraLabel: "E1", SagaSummary: "s1",
		},
	}
	trace := buildHierarchyConsistencyTrace(nil, resumePack, episodeSums)
	if trace["version"] != "p59a.v1" {
		t.Fatalf("version mismatch: %v", trace["version"])
	}
	if trace["episode_present"] != true {
		t.Fatal("episode_present should be true")
	}
	if trace["chapter_present"] != true {
		t.Fatal("chapter_present should be true")
	}
	if trace["saga_present"] != true {
		t.Fatal("saga_present should be true")
	}
	if trace["chapter_episode_aligned"] != true {
		t.Fatalf("chapter_episode_aligned = %v, want true", trace["chapter_episode_aligned"])
	}
	if trace["consistency_score"] != 0.75 {
		t.Fatalf("consistency_score = %v, want 0.75", trace["consistency_score"])
	}
}

func TestBuildRecallResultSummaryFailureStability(t *testing.T) {
	chatLogs := []store.ChatLog{
		{ID: 1, ChatSessionID: "s1", TurnIndex: 1, Role: "user", Content: "hello world"},
	}
	stab := buildSummaryFailureStability(false, chatLogs)
	if stab["version"] != "p46a.v1" {
		t.Fatalf("version mismatch: %v", stab["version"])
	}
	if stab["last_good_fallback"] != "hello world" {
		t.Fatalf("last_good_fallback = %v, want hello world", stab["last_good_fallback"])
	}
	if stab["retry_ready"] != true {
		t.Fatal("retry_ready should be true")
	}
	if stab["continuity_guard"] != "trace_only" {
		t.Fatalf("continuity_guard = %v, want trace_only", stab["continuity_guard"])
	}
	stabDeg := buildSummaryFailureStability(true, nil)
	if stabDeg["retry_ready"] != false {
		t.Fatal("retry_ready should be false when degraded")
	}
}

func TestBuildRecallResultANNTakeoverGuard(t *testing.T) {
	ann := map[string]any{
		"benchmark": map[string]any{
			"overlap_ratio": 0.4,
		},
	}
	guard := buildANNTakeoverGuard(ann, map[string]any{"profile": "wide"})
	if guard["version"] != "p33a.v1" {
		t.Fatalf("version mismatch: %v", guard["version"])
	}
	if guard["profile"] != "wide" {
		t.Fatalf("profile = %v, want wide", guard["profile"])
	}
	if guard["overlap_threshold"] != 0.25 {
		t.Fatalf("overlap_threshold = %v, want 0.25", guard["overlap_threshold"])
	}
	if guard["guard_decision"] != "shadow_compare" {
		t.Fatalf("guard_decision = %v, want shadow_compare", guard["guard_decision"])
	}
	ev := guard["evidence"].(map[string]any)
	if ev["threshold_met"] != true {
		t.Fatal("threshold_met should be true")
	}
}

func TestPrepareTurnIntentExecutionShadow(t *testing.T) {
	fake := &turnRecordingStore{}
	cfg := config.Default()
	cfg.StoreMode = config.StoreModeDualShadow
	srv := NewServer(cfg)
	srv.Store = fake
	mux := http.NewServeMux()
	srv.RegisterRoutes(mux)

	body := `{"chat_session_id":"test-intent-shadow","raw_user_input":"query","settings":{"injection_enabled":true,"input_context_enabled":true}}`
	req := httptest.NewRequest(http.MethodPost, "/prepare-turn", bytes.NewReader([]byte(body)))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d", rec.Code)
	}
	var resp map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatal(err)
	}
	recall, ok := resp["recall_result"].(map[string]any)
	if !ok {
		t.Fatal("recall_result missing")
	}
	shadow, ok := recall["intent_execution_shadow"].(map[string]any)
	if !ok {
		t.Fatal("intent_execution_shadow missing")
	}
	if shadow["routing_mode"] != "per_intent_shadow" {
		t.Fatalf("routing_mode = %v", shadow["routing_mode"])
	}
}

func TestPrepareTurnBudgetEnforcement(t *testing.T) {
	fake := &turnRecordingStore{}
	cfg := config.Default()
	cfg.StoreMode = config.StoreModeDualShadow
	srv := NewServer(cfg)
	srv.Store = fake
	mux := http.NewServeMux()
	srv.RegisterRoutes(mux)

	body := `{"chat_session_id":"test-budget","raw_user_input":"query","settings":{"injection_enabled":true}}`
	req := httptest.NewRequest(http.MethodPost, "/prepare-turn", bytes.NewReader([]byte(body)))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d", rec.Code)
	}
	var resp map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatal(err)
	}
	recall := resp["recall_result"].(map[string]any)
	shadow := recall["intent_execution_shadow"].(map[string]any)
	be := shadow["budget_enforcement"].(map[string]any)
	if be == nil {
		t.Fatal("budget_enforcement missing")
	}
	if be["mode"] != "enforced_shadow" {
		t.Fatalf("mode = %v", be["mode"])
	}
}

func TestPrepareTurnTakeover(t *testing.T) {
	fake := &turnRecordingStore{}
	cfg := config.Default()
	cfg.StoreMode = config.StoreModeDualShadow
	srv := NewServer(cfg)
	srv.Store = fake
	mux := http.NewServeMux()
	srv.RegisterRoutes(mux)

	body := `{"chat_session_id":"test-takeover","raw_user_input":"query"}`
	req := httptest.NewRequest(http.MethodPost, "/prepare-turn", bytes.NewReader([]byte(body)))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d", rec.Code)
	}
	var resp map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatal(err)
	}
	recall := resp["recall_result"].(map[string]any)
	shadow := recall["intent_execution_shadow"].(map[string]any)
	gt := shadow["guarded_takeover"].(map[string]any)
	if gt == nil {
		t.Fatal("guarded_takeover missing")
	}
	et := shadow["enforced_takeover"].(map[string]any)
	if et == nil {
		t.Fatal("enforced_takeover missing")
	}
}

func TestPrepareTurnHierarchy(t *testing.T) {
	fake := &turnRecordingStore{}
	cfg := config.Default()
	cfg.StoreMode = config.StoreModeDualShadow
	srv := NewServer(cfg)
	srv.Store = fake
	mux := http.NewServeMux()
	srv.RegisterRoutes(mux)

	body := `{"chat_session_id":"test-hierarchy","raw_user_input":"query"}`
	req := httptest.NewRequest(http.MethodPost, "/prepare-turn", bytes.NewReader([]byte(body)))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d", rec.Code)
	}
	var resp map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatal(err)
	}
	recall := resp["recall_result"].(map[string]any)
	trace := recall["hierarchy_consistency_trace"].(map[string]any)
	if trace == nil {
		t.Fatal("hierarchy_consistency_trace missing")
	}
	if trace["version"] != "p59a.v1" {
		t.Fatalf("version = %v", trace["version"])
	}
}

func TestPrepareTurnSummary(t *testing.T) {
	fake := &turnRecordingStore{}
	cfg := config.Default()
	cfg.StoreMode = config.StoreModeDualShadow
	srv := NewServer(cfg)
	srv.Store = fake
	mux := http.NewServeMux()
	srv.RegisterRoutes(mux)

	body := `{"chat_session_id":"test-summary","raw_user_input":"query"}`
	req := httptest.NewRequest(http.MethodPost, "/prepare-turn", bytes.NewReader([]byte(body)))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d", rec.Code)
	}
	var resp map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatal(err)
	}
	recall := resp["recall_result"].(map[string]any)
	stab := recall["summary_failure_stability"].(map[string]any)
	if stab == nil {
		t.Fatal("summary_failure_stability missing")
	}
	if stab["version"] != "p46a.v1" {
		t.Fatalf("version = %v", stab["version"])
	}
	if stab["continuity_guard"] != "trace_only" {
		t.Fatalf("continuity_guard = %v", stab["continuity_guard"])
	}
}

func TestPrepareTurnANN(t *testing.T) {
	fake := &turnRecordingStore{}
	cfg := config.Default()
	cfg.StoreMode = config.StoreModeDualShadow
	srv := NewServer(cfg)
	srv.Store = fake
	mux := http.NewServeMux()
	srv.RegisterRoutes(mux)

	body := `{"chat_session_id":"test-ann","raw_user_input":"query"}`
	req := httptest.NewRequest(http.MethodPost, "/prepare-turn", bytes.NewReader([]byte(body)))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d", rec.Code)
	}
	var resp map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatal(err)
	}
	recall := resp["recall_result"].(map[string]any)
	guard := recall["ann_default_takeover_guard"].(map[string]any)
	if guard == nil {
		t.Fatal("ann_default_takeover_guard missing")
	}
	if guard["version"] != "p33a.v1" {
		t.Fatalf("version = %v", guard["version"])
	}
}

func TestPrepareTurnGenerationPacketShadowCompareRecordIncludesChapterFields(t *testing.T) {
	now := time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC)
	fake := &turnRecordingStore{
		returnMemories: []store.Memory{
			{ID: 1, ChatSessionID: "sess-ch", TurnIndex: 2, SummaryJSON: `{"turn_summary":"chapter one"}`, Importance: 0.9},
		},
		returnResumePack: &store.ResumePack{
			PackStatus:    "ready",
			AssembledText: "Session resume with chapter material.",
			Chapter: &store.ChapterSummary{
				ID:            1,
				ChatSessionID: "sess-ch",
				FromTurn:      1,
				ToTurn:        5,
				ChapterIndex:  1,
				ChapterTitle:  "The Beginning",
				SummaryText:   "A chapter summary.",
				CreatedAt:     &now,
			},
		},
	}

	cfg := config.Default()
	cfg.StoreMode = config.StoreModeDualShadow
	srv := NewServer(cfg)
	srv.Store = fake

	mux := http.NewServeMux()
	srv.RegisterRoutes(mux)

	body := `{"chat_session_id":"sess-ch","turn_index":6,"raw_user_input":"What happens next?","settings":{"injection_enabled":true,"input_context_enabled":true,"max_injection_chars":500,"max_input_context_chars":400,"top_k":2}}`
	req := httptest.NewRequest(http.MethodPost, "/prepare-turn", bytes.NewReader([]byte(body)))
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

	gp, ok := resp["generation_packet"].(map[string]any)
	if !ok {
		t.Fatalf("generation_packet is not an object")
	}

	scr, ok := gp["shadow_compare_record"].(map[string]any)
	if !ok {
		t.Fatalf("shadow_compare_record is not an object")
	}
	if scr["version"] != "p249a.v1" {
		t.Fatalf("version = %v, want p249a.v1", scr["version"])
	}
	if scr["new_has_chapter"] != true {
		t.Fatalf("new_has_chapter = %v, want true", scr["new_has_chapter"])
	}
	if scr["new_chapter_chars"].(float64) <= 0 {
		t.Fatalf("new_chapter_chars = %v, want > 0", scr["new_chapter_chars"])
	}
	if scr["new_has_chapter_input"] != true {
		t.Fatalf("new_has_chapter_input = %v, want true", scr["new_has_chapter_input"])
	}
	if scr["old_has_chapter"] != false {
		t.Fatalf("old_has_chapter = %v, want false", scr["old_has_chapter"])
	}
	if scr["old_chapter_chars"].(float64) != 0 {
		t.Fatalf("old_chapter_chars = %v, want 0", scr["old_chapter_chars"])
	}
	if scr["old_has_chapter_input"] != false {
		t.Fatalf("old_has_chapter_input = %v, want false", scr["old_has_chapter_input"])
	}
	if scr["divergence_chapter"] != true {
		t.Fatalf("divergence_chapter = %v, want true", scr["divergence_chapter"])
	}
	if scr["divergence_chapter_input"] != true {
		t.Fatalf("divergence_chapter_input = %v, want true", scr["divergence_chapter_input"])
	}
}

func TestPrepareTurnCanonicalStateHardFloorFiltersStaleLayers(t *testing.T) {
	fake := &turnRecordingStore{
		returnCanonicalLayers: []store.CanonicalStateLayer{
			{ID: 1, ChatSessionID: "sess-hs-prep", LayerType: "relationship_state", Content: "Mina currently trusts Rowan.", SourceStateType: "relationship_state", TurnIndex: 20, SourceTurn: 20, LastVerifiedTurn: 20, Confidence: 0.9},
			{ID: 2, ChatSessionID: "sess-hs-prep", LayerType: "relationship_state", Content: "Stale rumor says Mina distrusts Rowan.", SourceStateType: "stale_relationship_state", TurnIndex: 10, SourceTurn: 10, LastVerifiedTurn: 10, Confidence: 0.95},
			{ID: 3, ChatSessionID: "sess-hs-prep", LayerType: "settings_state", Content: "Low confidence setting should not promote.", SourceStateType: "settings_state", TurnIndex: 21, SourceTurn: 21, LastVerifiedTurn: 21, Confidence: 0.3},
		},
		returnChatLogs: []store.ChatLog{{ID: 1, ChatSessionID: "sess-hs-prep", TurnIndex: 21, Role: "assistant", Content: "They pause at the archive gate."}},
	}
	cfg := config.Default()
	cfg.StoreMode = config.StoreModeDualShadow
	srv := NewServer(cfg)
	srv.Store = fake

	mux := http.NewServeMux()
	srv.RegisterRoutes(mux)

	body := `{"chat_session_id":"sess-hs-prep","turn_index":22,"raw_user_input":"Continue","settings":{"max_injection_chars":900,"max_input_context_chars":300,"injection_enabled":true,"input_context_enabled":true}}`
	req := httptest.NewRequest(http.MethodPost, "/prepare-turn", bytes.NewReader([]byte(body)))
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
	injectionPack, ok := resp["injection_pack"].(map[string]any)
	if !ok {
		t.Fatalf("injection_pack is not an object")
	}
	canonText, _ := injectionPack["canon_text"].(string)
	if !strings.Contains(canonText, "currently trusts Rowan") {
		t.Fatalf("canon_text missing verified canonical state: %q", canonText)
	}
	if strings.Contains(canonText, "Stale rumor") || strings.Contains(canonText, "Low confidence") {
		t.Fatalf("canon_text included stale/low-confidence layer: %q", canonText)
	}
	budget, ok := injectionPack["budget_decisions"].(map[string]any)
	if !ok {
		t.Fatalf("budget_decisions is not an object")
	}
	if budget["canonical_state_hard_floor_enabled"] != true {
		t.Fatalf("canonical hard floor flag = %v, want true", budget["canonical_state_hard_floor_enabled"])
	}
	counts, ok := resp["counts"].(map[string]any)
	if ok && counts["canonical_state_layers_filtered_count"] != float64(2) {
		t.Fatalf("canonical_state_layers_filtered_count = %v, want 2", counts["canonical_state_layers_filtered_count"])
	}
}

func TestPrepareTurnTM1aCanonicalConsistencyInputsSurface(t *testing.T) {
	fake := &turnRecordingStore{
		returnMemories: []store.Memory{{
			ID:            1,
			ChatSessionID: "sess-tm1a",
			TurnIndex:     96,
			SummaryJSON:   `{"turn_summary":"Mina waits in the reading alcove after Rowan's promise."}`,
			Importance:    0.88,
			PlaceWing:     "North Wing",
			PlaceRoom:     "Scene Room",
		}},
		returnCharStates: []store.CharacterState{{
			ChatSessionID:     "sess-tm1a",
			CharacterName:     "Mina",
			StatusJSON:        `{"mood":"watchful"}`,
			RelationshipsJSON: `{"Rowan":{"trust":74,"last_change":"Rowan promised to return with evidence"}}`,
			TurnIndex:         97,
		}},
		returnPendingThreads: []store.PendingThread{{
			ChatSessionID: "sess-tm1a",
			ThreadKey:     "rowan-answer",
			Description:   "Rowan still owes Mina an answer about the hidden archive key.",
			Status:        "open",
			SourceTurn:    97,
		}},
		returnCanonicalLayers: []store.CanonicalStateLayer{{
			ID:               1,
			ChatSessionID:    "sess-tm1a",
			LayerType:        "scene_state",
			Content:          `{"location":"North Wing / Scene Room","pressure":"low"}`,
			SourceStateType:  "scene_state",
			TurnIndex:        97,
			SourceTurn:       97,
			LastVerifiedTurn: 97,
			Confidence:       0.92,
		}},
	}
	cfg := config.Default()
	cfg.StoreMode = config.StoreModeDualShadow
	srv := NewServer(cfg)
	srv.Store = fake

	mux := http.NewServeMux()
	srv.RegisterRoutes(mux)

	body := `{"chat_session_id":"sess-tm1a","turn_index":98,"raw_user_input":"Continue from the archive room.","settings":{"injection_enabled":true,"input_context_enabled":true,"max_injection_chars":1200,"max_input_context_chars":500,"top_k":5}}`
	req := httptest.NewRequest(http.MethodPost, "/prepare-turn", bytes.NewReader([]byte(body)))
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
	ip, ok := resp["injection_pack"].(map[string]any)
	if !ok {
		t.Fatalf("injection_pack is not an object")
	}
	memoryText, _ := ip["memory_text"].(string)
	if !strings.Contains(memoryText, "Mina waits") || !strings.Contains(memoryText, "archive_wing=North Wing") || !strings.Contains(memoryText, "archive_room=Scene Room") {
		t.Fatalf("memory_text did not preserve scene archive placement: %q", memoryText)
	}
	characterText, _ := ip["character_text"].(string)
	if !strings.Contains(characterText, "Mina") || !strings.Contains(characterText, "Rowan") || !strings.Contains(characterText, "trust") || !strings.Contains(characterText, "74") {
		t.Fatalf("character_text did not preserve relationships_json: %q", characterText)
	}
	pendingText, _ := ip["pending_thread_text"].(string)
	if !strings.Contains(pendingText, "status=open") || !strings.Contains(pendingText, "owes Mina an answer") {
		t.Fatalf("pending_thread_text did not preserve open unresolved thread: %q", pendingText)
	}
	canonText, _ := ip["canon_text"].(string)
	if !strings.Contains(canonText, "scene_state") || !strings.Contains(canonText, "North Wing") || !strings.Contains(canonText, "Scene Room") {
		t.Fatalf("canon_text did not preserve verified scene_state layer: %q", canonText)
	}
	counts, ok := ip["counts"].(map[string]any)
	if !ok {
		t.Fatalf("counts is not an object")
	}
	if counts["character_state_count"] != float64(1) || counts["pending_thread_count"] != float64(1) || counts["canonical_state_scene_layers_count"] != float64(1) {
		t.Fatalf("counts missing TM-1a surfaces: %#v", counts)
	}
	if len(fake.savedChatLogs) != 0 || len(fake.savedMemories) != 0 || len(fake.savedCanonicalLayers) != 0 {
		t.Fatalf("prepare-turn TM-1a verification should be read-only, writes logs=%d memories=%d canonical=%d", len(fake.savedChatLogs), len(fake.savedMemories), len(fake.savedCanonicalLayers))
	}
}

func TestPrepareTurnStoreBackedAssemblySagaEvidence(t *testing.T) {
	now := time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC)
	fake := &turnRecordingStore{
		returnMemories: []store.Memory{
			{ID: 1, ChatSessionID: "sess-saga", TurnIndex: 2, SummaryJSON: `{"turn_summary":"A warm evening"}`, Importance: 0.9},
		},
		returnResumePack: &store.ResumePack{
			PackStatus:    "ready",
			AssembledText: "Session resume text.",
			Saga: &store.SagaDigest{
				ID:            1,
				ChatSessionID: "sess-saga",
				FromTurn:      1,
				ToTurn:        20,
				EraLabel:      "Era One",
				SagaSummary:   "An epic saga of mystery and discovery.",
				CreatedAt:     &now,
			},
		},
	}

	cfg := config.Default()
	cfg.StoreMode = config.StoreModeDualShadow
	srv := NewServer(cfg)
	srv.Store = fake

	mux := http.NewServeMux()
	srv.RegisterRoutes(mux)

	body := `{"chat_session_id":"sess-saga","turn_index":3,"raw_user_input":"What happens next?","settings":{"injection_enabled":true,"input_context_enabled":true,"max_injection_chars":500,"max_input_context_chars":400,"top_k":2}}`
	req := httptest.NewRequest(http.MethodPost, "/prepare-turn", bytes.NewReader([]byte(body)))
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

	injectionPack, ok := resp["injection_pack"].(map[string]any)
	if !ok {
		t.Fatalf("injection_pack is not an object")
	}

	sagaText, ok := injectionPack["saga_text"].(string)
	if !ok || sagaText == "" {
		t.Fatalf("injection_pack.saga_text missing or empty: %v", injectionPack["saga_text"])
	}
	if !strings.Contains(sagaText, "saga") && !strings.Contains(sagaText, "epic") {
		t.Fatalf("saga_text does not contain saga material: %q", sagaText)
	}

	if injectionPack["saga_delivered"] != true {
		t.Fatalf("injection_pack.saga_delivered = %v, want true", injectionPack["saga_delivered"])
	}

	gp, ok := resp["generation_packet"].(map[string]any)
	if !ok {
		t.Fatalf("generation_packet is not an object")
	}

	traceSummary, ok := gp["trace_summary"].(map[string]any)
	if !ok {
		t.Fatalf("trace_summary is not an object")
	}

	if traceSummary["saga_delivered"] != true {
		t.Fatalf("trace_summary.saga_delivered = %v, want true", traceSummary["saga_delivered"])
	}

	sagaTextChars, ok := traceSummary["saga_text_chars"].(float64)
	if !ok {
		t.Fatalf("trace_summary.saga_text_chars type = %T, want float64", traceSummary["saga_text_chars"])
	}
	if sagaTextChars <= 0 {
		t.Fatalf("trace_summary.saga_text_chars = %v, want > 0", sagaTextChars)
	}
}

func TestPrepareTurnEnforcedBudgetModeReady(t *testing.T) {
	fake := &turnRecordingStore{
		returnMemories: []store.Memory{
			{ID: 1, ChatSessionID: "sess-budget", TurnIndex: 2, SummaryJSON: `{"turn_summary":"test"}`, Importance: 0.9},
		},
	}

	cfg := config.Default()
	cfg.StoreMode = config.StoreModeDualShadow
	srv := NewServer(cfg)
	srv.Store = fake

	mux := http.NewServeMux()
	srv.RegisterRoutes(mux)

	body := `{"chat_session_id":"sess-budget","turn_index":3,"raw_user_input":"hello","settings":{"injection_enabled":true,"input_context_enabled":true,"max_injection_chars":500,"max_input_context_chars":400,"top_k":2}}`
	req := httptest.NewRequest(http.MethodPost, "/prepare-turn", bytes.NewReader([]byte(body)))
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

	rr, ok := resp["recall_result"].(map[string]any)
	if !ok {
		t.Fatalf("recall_result is not an object")
	}

	pbp, ok := rr["packet_budget_policy"].(map[string]any)
	if !ok {
		t.Fatalf("packet_budget_policy is not an object")
	}

	if pbp["budget_mode"] != "enforced" {
		t.Fatalf("budget_mode = %v, want enforced", pbp["budget_mode"])
	}
}

func TestPrepareTurnSingleQuerySharedPreserved(t *testing.T) {
	fake := &turnRecordingStore{
		returnMemories: []store.Memory{
			{ID: 1, ChatSessionID: "sess-route", TurnIndex: 2, SummaryJSON: `{"turn_summary":"test"}`, Importance: 0.9},
		},
	}

	cfg := config.Default()
	cfg.StoreMode = config.StoreModeDualShadow
	srv := NewServer(cfg)
	srv.Store = fake

	mux := http.NewServeMux()
	srv.RegisterRoutes(mux)

	body := `{"chat_session_id":"sess-route","turn_index":3,"raw_user_input":"hello","settings":{"injection_enabled":true,"input_context_enabled":true,"max_injection_chars":500,"max_input_context_chars":400,"top_k":2}}`
	req := httptest.NewRequest(http.MethodPost, "/prepare-turn", bytes.NewReader([]byte(body)))
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

	rr, ok := resp["recall_result"].(map[string]any)
	if !ok {
		t.Fatalf("recall_result is not an object")
	}

	tr, ok := rr["trace"].(map[string]any)
	if !ok {
		t.Fatalf("trace is not an object")
	}

	if tr["intent_route"] != "single_query_shared" {
		t.Fatalf("intent_route = %v, want single_query_shared", tr["intent_route"])
	}
}

func TestPrepareTurnSyntheticLongSession(t *testing.T) {
	fake := &turnRecordingStore{
		returnMemories: []store.Memory{
			{ID: 1, ChatSessionID: "sess-long", TurnIndex: 99, SummaryJSON: `{"turn_summary":"near the end"}`, Importance: 0.9},
		},
		returnResumePack: &store.ResumePack{
			PackStatus:    "ready",
			AssembledText: "Long session resume.",
		},
	}

	cfg := config.Default()
	cfg.StoreMode = config.StoreModeDualShadow
	srv := NewServer(cfg)
	srv.Store = fake

	mux := http.NewServeMux()
	srv.RegisterRoutes(mux)

	body := `{"chat_session_id":"sess-long","turn_index":100,"raw_user_input":"Continue","settings":{"injection_enabled":true,"input_context_enabled":true,"max_injection_chars":500,"max_input_context_chars":400,"top_k":2}}`
	req := httptest.NewRequest(http.MethodPost, "/prepare-turn", bytes.NewReader([]byte(body)))
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

	gp, ok := resp["generation_packet"].(map[string]any)
	if !ok {
		t.Fatalf("generation_packet is not an object")
	}

	trace, ok := gp["trace_summary"].(map[string]any)
	if !ok {
		t.Fatalf("trace_summary is not an object")
	}

	if trace["reads_ok"].(float64) <= 0 {
		t.Fatalf("reads_ok = %v, want > 0", trace["reads_ok"])
	}

	if gp["degraded"] != false {
		t.Fatalf("degraded = %v, want false", gp["degraded"])
	}
}

func TestPrepareTurnOutboundRewriteGuard(t *testing.T) {
	fake := &turnRecordingStore{
		returnMemories: []store.Memory{
			{ID: 1, ChatSessionID: "sess-rewrite", TurnIndex: 2, SummaryJSON: `{"turn_summary":"test"}`, Importance: 0.9},
		},
	}

	cfg := config.Default()
	cfg.StoreMode = config.StoreModeDualShadow
	srv := NewServer(cfg)
	srv.Store = fake

	mux := http.NewServeMux()
	srv.RegisterRoutes(mux)

	body := `{"chat_session_id":"sess-rewrite","turn_index":3,"raw_user_input":"hello","settings":{"injection_enabled":true,"input_context_enabled":true,"max_injection_chars":500,"max_input_context_chars":400,"top_k":2}}`
	req := httptest.NewRequest(http.MethodPost, "/prepare-turn", bytes.NewReader([]byte(body)))
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

	gp, ok := resp["generation_packet"].(map[string]any)
	if !ok {
		t.Fatalf("generation_packet is not an object")
	}

	trace, ok := gp["trace_summary"].(map[string]any)
	if !ok {
		t.Fatalf("trace_summary is not an object")
	}

	org, ok := trace["outbound_rewrite_guard"].(map[string]any)
	if !ok {
		t.Fatalf("outbound_rewrite_guard is not an object")
	}

	if org["version"] != "p34a.v1" {
		t.Fatalf("version = %v, want p34a.v1", org["version"])
	}
	if org["rewrite_allowed"] != false {
		t.Fatalf("rewrite_allowed = %v, want false", org["rewrite_allowed"])
	}
}

func TestPrepareTurnSagaConsumedEvidence(t *testing.T) {
	now := time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC)
	fake := &turnRecordingStore{
		returnMemories: []store.Memory{
			{ID: 1, ChatSessionID: "sess-consume", TurnIndex: 2, SummaryJSON: `{"turn_summary":"A warm evening"}`, Importance: 0.9},
		},
		returnResumePack: &store.ResumePack{
			PackStatus:    "ready",
			AssembledText: "Session resume text.",
			Saga: &store.SagaDigest{
				ID:            1,
				ChatSessionID: "sess-consume",
				FromTurn:      1,
				ToTurn:        20,
				EraLabel:      "Era One",
				SagaSummary:   "An epic saga of mystery and discovery.",
				CreatedAt:     &now,
			},
		},
	}

	cfg := config.Default()
	cfg.StoreMode = config.StoreModeDualShadow
	srv := NewServer(cfg)
	srv.Store = fake

	mux := http.NewServeMux()
	srv.RegisterRoutes(mux)

	body := `{"chat_session_id":"sess-consume","turn_index":3,"raw_user_input":"What happens next?","settings":{"injection_enabled":true,"input_context_enabled":true,"max_injection_chars":500,"max_input_context_chars":400,"top_k":2}}`
	req := httptest.NewRequest(http.MethodPost, "/prepare-turn", bytes.NewReader([]byte(body)))
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

	gp, ok := resp["generation_packet"].(map[string]any)
	if !ok {
		t.Fatalf("generation_packet is not an object")
	}

	trace, ok := gp["trace_summary"].(map[string]any)
	if !ok {
		t.Fatalf("trace_summary is not an object")
	}

	if trace["saga_delivered"] != true {
		t.Fatalf("saga_delivered = %v, want true", trace["saga_delivered"])
	}

	if trace["saga_consumed"] != true {
		t.Fatalf("saga_consumed = %v, want true (saga text should be consumed into assembly text)", trace["saga_consumed"])
	}
}

func TestBuildRecallResultPerIntentActualExecution(t *testing.T) {
	fake := &turnRecordingStore{
		returnMemories: []store.Memory{
			{ID: 1, ChatSessionID: "sess-exec", TurnIndex: 2, SummaryJSON: `{"turn_summary":"test"}`, Importance: 0.9},
		},
	}

	cfg := config.Default()
	cfg.StoreMode = config.StoreModeDualShadow
	srv := NewServer(cfg)
	srv.Store = fake

	mux := http.NewServeMux()
	srv.RegisterRoutes(mux)

	body := `{"chat_session_id":"sess-exec","turn_index":3,"raw_user_input":"hello","settings":{"injection_enabled":true,"input_context_enabled":true,"max_injection_chars":500,"max_input_context_chars":400,"top_k":2}}`
	req := httptest.NewRequest(http.MethodPost, "/prepare-turn", bytes.NewReader([]byte(body)))
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

	rr, ok := resp["recall_result"].(map[string]any)
	if !ok {
		t.Fatalf("recall_result is not an object")
	}

	ies, ok := rr["intent_execution_shadow"].(map[string]any)
	if !ok {
		t.Fatalf("intent_execution_shadow is not an object")
	}

	ae, ok := ies["actual_execution"].(map[string]any)
	if !ok {
		t.Fatalf("actual_execution is not an object")
	}

	if ae["version"] != "p44a.v1" {
		t.Fatalf("version = %v, want p44a.v1", ae["version"])
	}
	if ae["retrieval_ran"] != true {
		t.Fatalf("retrieval_ran = %v, want true", ae["retrieval_ran"])
	}
	if ae["intents_ran"].(float64) <= 0 {
		t.Fatalf("intents_ran = %v, want > 0", ae["intents_ran"])
	}
}

func TestPrepareTurnEnforcedBudgetReasonTrace(t *testing.T) {
	fake := &turnRecordingStore{
		returnMemories: []store.Memory{
			{ID: 1, ChatSessionID: "sess-budget2", TurnIndex: 2, SummaryJSON: `{"turn_summary":"test"}`, Importance: 0.9},
		},
	}

	cfg := config.Default()
	cfg.StoreMode = config.StoreModeDualShadow
	srv := NewServer(cfg)
	srv.Store = fake

	mux := http.NewServeMux()
	srv.RegisterRoutes(mux)

	body := `{"chat_session_id":"sess-budget2","turn_index":3,"raw_user_input":"hello","settings":{"injection_enabled":true,"input_context_enabled":true,"max_injection_chars":500,"max_input_context_chars":400,"top_k":2}}`
	req := httptest.NewRequest(http.MethodPost, "/prepare-turn", bytes.NewReader([]byte(body)))
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

	rr, ok := resp["recall_result"].(map[string]any)
	if !ok {
		t.Fatalf("recall_result is not an object")
	}

	ies, ok := rr["intent_execution_shadow"].(map[string]any)
	if !ok {
		t.Fatalf("intent_execution_shadow is not an object")
	}

	be, ok := ies["budget_enforcement"].(map[string]any)
	if !ok {
		t.Fatalf("budget_enforcement is not an object")
	}

	if be["mode"] != "enforced_shadow" {
		t.Fatalf("mode = %v, want enforced_shadow", be["mode"])
	}

	if _, ok := be["reason_counts"]; !ok {
		t.Fatalf("reason_counts missing")
	}
}

func TestBuildRecallResultLastGoodFallbackRetryEvidence(t *testing.T) {
	fake := &turnRecordingStore{
		returnMemories: []store.Memory{
			{ID: 1, ChatSessionID: "sess-fb", TurnIndex: 2, SummaryJSON: `{"turn_summary":"test"}`, Importance: 0.9},
		},
		returnChatLogs: []store.ChatLog{
			{ID: 1, ChatSessionID: "sess-fb", TurnIndex: 1, Role: "user", Content: "Hello there"},
			{ID: 2, ChatSessionID: "sess-fb", TurnIndex: 2, Role: "assistant", Content: ""},
			{ID: 3, ChatSessionID: "sess-fb", TurnIndex: 3, Role: "user", Content: "Try again"},
		},
	}

	cfg := config.Default()
	cfg.StoreMode = config.StoreModeDualShadow
	srv := NewServer(cfg)
	srv.Store = fake

	mux := http.NewServeMux()
	srv.RegisterRoutes(mux)

	body := `{"chat_session_id":"sess-fb","turn_index":4,"raw_user_input":"hello","settings":{"injection_enabled":true,"input_context_enabled":true,"max_injection_chars":500,"max_input_context_chars":400,"top_k":2}}`
	req := httptest.NewRequest(http.MethodPost, "/prepare-turn", bytes.NewReader([]byte(body)))
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

	rr, ok := resp["recall_result"].(map[string]any)
	if !ok {
		t.Fatalf("recall_result is not an object")
	}

	sfs, ok := rr["summary_failure_stability"].(map[string]any)
	if !ok {
		t.Fatalf("summary_failure_stability is not an object")
	}

	if sfs["version"] != "p46a.v1" {
		t.Fatalf("version = %v, want p46a.v1", sfs["version"])
	}
	if sfs["retry_ready"] != true {
		t.Fatalf("retry_ready = %v, want true", sfs["retry_ready"])
	}
	if sfs["retry_count"].(float64) != 1 {
		t.Fatalf("retry_count = %v, want 1", sfs["retry_count"])
	}
	if sfs["last_retry_turn"].(float64) != 2 {
		t.Fatalf("last_retry_turn = %v, want 2", sfs["last_retry_turn"])
	}
	ce, ok := sfs["compression_evidence"].(map[string]any)
	if !ok {
		t.Fatalf("compression_evidence is not an object")
	}
	if ce["chat_log_count"].(float64) != 3 {
		t.Fatalf("chat_log_count = %v, want 3", ce["chat_log_count"])
	}
}

func TestPrepareTurnUltraProfileCompression(t *testing.T) {
	fake := &turnRecordingStore{
		returnMemories: []store.Memory{},
		returnChatLogs: []store.ChatLog{
			{ID: 1, ChatSessionID: "sess-ultra", TurnIndex: 1, Role: "user", Content: "This is a very long chat log message that should be compressed"},
			{ID: 2, ChatSessionID: "sess-ultra", TurnIndex: 2, Role: "assistant", Content: "Response"},
		},
	}

	cfg := config.Default()
	cfg.StoreMode = config.StoreModeDualShadow
	srv := NewServer(cfg)
	srv.Store = fake

	mux := http.NewServeMux()
	srv.RegisterRoutes(mux)

	body := `{"chat_session_id":"sess-ultra","turn_index":3,"raw_user_input":"hello","settings":{"injection_enabled":true,"input_context_enabled":true,"max_injection_chars":500,"max_input_context_chars":400,"top_k":2},"client_meta":{"context_window_profile":"ultra"}}`
	req := httptest.NewRequest(http.MethodPost, "/prepare-turn", bytes.NewReader([]byte(body)))
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

	gp, ok := resp["generation_packet"].(map[string]any)
	if !ok {
		t.Fatalf("generation_packet is not an object")
	}

	ip, ok := gp["injection_text"].(string)
	if !ok {
		t.Fatalf("injection_text is not a string")
	}

	if len(ip) > 500 {
		t.Fatalf("injection_text length %d > 500", len(ip))
	}
}

func TestBuildRecallResultLongTierANNGuard(t *testing.T) {
	fake := &turnRecordingStore{
		returnMemories: []store.Memory{
			{ID: 1, ChatSessionID: "sess-ann", TurnIndex: 2, SummaryJSON: `{"turn_summary":"test"}`, Importance: 0.9},
		},
	}

	cfg := config.Default()
	cfg.StoreMode = config.StoreModeDualShadow
	srv := NewServer(cfg)
	srv.Store = fake

	mux := http.NewServeMux()
	srv.RegisterRoutes(mux)

	body := `{"chat_session_id":"sess-ann","turn_index":3,"raw_user_input":"hello","settings":{"injection_enabled":true,"input_context_enabled":true,"max_injection_chars":500,"max_input_context_chars":400,"top_k":2}}`
	req := httptest.NewRequest(http.MethodPost, "/prepare-turn", bytes.NewReader([]byte(body)))
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

	rr, ok := resp["recall_result"].(map[string]any)
	if !ok {
		t.Fatalf("recall_result is not an object")
	}

	ann, ok := rr["ann_default_takeover_guard"].(map[string]any)
	if !ok {
		t.Fatalf("ann_default_takeover_guard is not an object")
	}

	if ann["version"] != "p33a.v1" {
		t.Fatalf("version = %v, want p33a.v1", ann["version"])
	}

	// Verify profile-aware thresholds exist by checking the evidence structure
	ev, ok := ann["evidence"].(map[string]any)
	if !ok {
		t.Fatalf("evidence is not an object")
	}
	if _, ok := ev["threshold_met"]; !ok {
		t.Fatalf("threshold_met missing")
	}
}

func TestBuildRecallResultStaleContextGuard(t *testing.T) {
	fake := &turnRecordingStore{
		returnMemories: []store.Memory{
			{ID: 1, ChatSessionID: "sess-stale", TurnIndex: 2, SummaryJSON: `{"turn_summary":"test"}`, Importance: 0.9},
		},
		returnStorylines: []store.Storyline{
			{ID: 1, ChatSessionID: "sess-stale", Name: "Main", Status: "active", Suppressed: true},
		},
		returnWorldRules: []store.WorldRule{
			{ID: 1, ChatSessionID: "sess-stale", Key: "rule1", Scope: "session", Suppressed: false},
		},
		returnPendingThreads: []store.PendingThread{
			{ID: 1, ChatSessionID: "sess-stale", Description: "thread1", Suppressed: true},
		},
	}

	cfg := config.Default()
	cfg.StoreMode = config.StoreModeDualShadow
	srv := NewServer(cfg)
	srv.Store = fake

	mux := http.NewServeMux()
	srv.RegisterRoutes(mux)

	body := `{"chat_session_id":"sess-stale","turn_index":3,"raw_user_input":"hello","settings":{"injection_enabled":true,"input_context_enabled":true,"max_injection_chars":500,"max_input_context_chars":400,"top_k":2}}`
	req := httptest.NewRequest(http.MethodPost, "/prepare-turn", bytes.NewReader([]byte(body)))
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

	rr, ok := resp["recall_result"].(map[string]any)
	if !ok {
		t.Fatalf("recall_result is not an object")
	}

	scg, ok := rr["stale_context_guard"].(map[string]any)
	if !ok {
		t.Fatalf("stale_context_guard is not an object")
	}

	if scg["version"] != "p50a.v1" {
		t.Fatalf("version = %v, want p50a.v1", scg["version"])
	}
	if scg["forget_guard_active"] != true {
		t.Fatalf("forget_guard_active = %v, want true", scg["forget_guard_active"])
	}
	if scg["suppressed_storylines"].(float64) != 1 {
		t.Fatalf("suppressed_storylines = %v, want 1", scg["suppressed_storylines"])
	}
	if scg["suppressed_pending_threads"].(float64) != 1 {
		t.Fatalf("suppressed_pending_threads = %v, want 1", scg["suppressed_pending_threads"])
	}
	if scg["suppressed_world_rules"].(float64) != 0 {
		t.Fatalf("suppressed_world_rules = %v, want 0", scg["suppressed_world_rules"])
	}
	if scg["total_suppressed"].(float64) != 2 {
		t.Fatalf("total_suppressed = %v, want 2", scg["total_suppressed"])
	}
}

func TestPrepareTurnSagaTextInAuxiliaryPrompt(t *testing.T) {
	now := time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC)
	fake := &turnRecordingStore{
		returnMemories: []store.Memory{
			{ID: 1, ChatSessionID: "sess-saga-aux", TurnIndex: 2, SummaryJSON: `{"turn_summary":"A warm evening"}`, Importance: 0.9},
		},
		returnResumePack: &store.ResumePack{
			PackStatus:    "ready",
			AssembledText: "Session resume text.",
			Saga: &store.SagaDigest{
				ID:            1,
				ChatSessionID: "sess-saga-aux",
				FromTurn:      1,
				ToTurn:        20,
				EraLabel:      "Era One",
				SagaSummary:   "An epic saga of mystery and discovery.",
				CreatedAt:     &now,
			},
		},
	}

	cfg := config.Default()
	cfg.StoreMode = config.StoreModeDualShadow
	srv := NewServer(cfg)
	srv.Store = fake

	mux := http.NewServeMux()
	srv.RegisterRoutes(mux)

	body := `{"chat_session_id":"sess-saga-aux","turn_index":3,"raw_user_input":"What happens next?","settings":{"injection_enabled":true,"input_context_enabled":true,"max_injection_chars":500,"max_input_context_chars":400,"top_k":2}}`
	req := httptest.NewRequest(http.MethodPost, "/prepare-turn", bytes.NewReader([]byte(body)))
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

	gp, ok := resp["generation_packet"].(map[string]any)
	if !ok {
		t.Fatalf("generation_packet is not an object")
	}

	trace, ok := gp["trace_summary"].(map[string]any)
	if !ok {
		t.Fatalf("trace_summary is not an object")
	}

	if trace["saga_delivered"] != true {
		t.Fatalf("saga_delivered = %v, want true", trace["saga_delivered"])
	}
	if trace["saga_text_chars"].(float64) <= 0 {
		t.Fatalf("saga_text_chars = %v, want > 0", trace["saga_text_chars"])
	}

	ip, ok := resp["injection_pack"].(map[string]any)
	if !ok {
		t.Fatalf("injection_pack is not an object")
	}

	if ip["saga_delivered"] != true {
		t.Fatalf("injection_pack.saga_delivered = %v, want true", ip["saga_delivered"])
	}

	st, ok := ip["saga_text"].(string)
	if !ok || st == "" {
		t.Fatalf("injection_pack.saga_text missing or empty")
	}
	if !strings.Contains(st, "saga") {
		t.Fatalf("saga_text does not contain saga material: %q", st)
	}
}

func TestPrepareTurnChapterHierarchyEscalationConsumed(t *testing.T) {
	now := time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC)
	fake := &turnRecordingStore{
		returnMemories: []store.Memory{
			{ID: 1, ChatSessionID: "sess-chapter-aux", TurnIndex: 58, SummaryJSON: `{"turn_summary":"The bridge plan is underway"}`, Importance: 0.9},
		},
		returnResumePack: &store.ResumePack{
			PackStatus:    "ready",
			AssembledText: "Session resume text.",
			Chapter: &store.ChapterSummary{
				ID:            1,
				ChatSessionID: "sess-chapter-aux",
				FromTurn:      41,
				ToTurn:        60,
				ChapterIndex:  3,
				ChapterTitle:  "Bridge Operation",
				SummaryText:   "Luka and the group settle the demolition plan while unresolved trust tension remains.",
				OpenLoopsJSON: `[{"text":"Whether Hank accepts the final risk tradeoff"}]`,
				CreatedAt:     &now,
			},
		},
	}

	cfg := config.Default()
	cfg.StoreMode = config.StoreModeDualShadow
	srv := NewServer(cfg)
	srv.Store = fake

	mux := http.NewServeMux()
	srv.RegisterRoutes(mux)

	body := `{"chat_session_id":"sess-chapter-aux","turn_index":61,"raw_user_input":"계속 이어가자. 이전 작전 맥락을 잊지 말아줘.","settings":{"injection_enabled":true,"input_context_enabled":true,"max_injection_chars":900,"max_input_context_chars":400,"top_k":2}}`
	req := httptest.NewRequest(http.MethodPost, "/prepare-turn", bytes.NewReader([]byte(body)))
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

	gp, ok := resp["generation_packet"].(map[string]any)
	if !ok {
		t.Fatalf("generation_packet is not an object")
	}
	trace, ok := gp["trace_summary"].(map[string]any)
	if !ok {
		t.Fatalf("trace_summary is not an object")
	}
	if trace["chapter_delivered"] != true {
		t.Fatalf("chapter_delivered = %v, want true", trace["chapter_delivered"])
	}
	if trace["chapter_consumed"] != true {
		t.Fatalf("chapter_consumed = %v, want true", trace["chapter_consumed"])
	}

	ip, ok := resp["injection_pack"].(map[string]any)
	if !ok {
		t.Fatalf("injection_pack is not an object")
	}
	if ip["chapter_delivered"] != true {
		t.Fatalf("injection_pack.chapter_delivered = %v, want true", ip["chapter_delivered"])
	}
	chapterText, ok := ip["chapter_text"].(string)
	if !ok || !strings.Contains(chapterText, "Bridge Operation") {
		t.Fatalf("chapter_text missing expected chapter material: %v", ip["chapter_text"])
	}
	counts, ok := ip["counts"].(map[string]any)
	if !ok {
		t.Fatalf("counts is not an object")
	}
	escalation, ok := counts["hierarchy_escalation"].(map[string]any)
	if !ok {
		t.Fatalf("hierarchy_escalation is not an object: %#v", counts["hierarchy_escalation"])
	}
	if escalation["chapter_selected"] != true {
		t.Fatalf("hierarchy_escalation.chapter_selected = %v, want true", escalation["chapter_selected"])
	}
}

func TestIntentRoutingRuntimeConfig(t *testing.T) {
	cfg := config.Default()
	cfg.StoreMode = config.StoreModeDualShadow
	srv := NewServer(cfg)

	mux := http.NewServeMux()
	srv.RegisterRoutes(mux)

	body := `{"chat_session_id":"sess-irc","turn_index":1}`
	req := httptest.NewRequest(http.MethodPost, "/intent-routing/runtime-config", bytes.NewReader([]byte(body)))
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
	if resp["routing_version"] != "p58a.v1" {
		t.Fatalf("routing_version = %v, want p58a.v1", resp["routing_version"])
	}
	if resp["routing_mode"] != "per_intent_shadow" {
		t.Fatalf("routing_mode = %v, want per_intent_shadow", resp["routing_mode"])
	}
	if resp["default_route"] != "single_query_shared" {
		t.Fatalf("default_route = %v, want single_query_shared", resp["default_route"])
	}

	intents, ok := resp["intents"].([]any)
	if !ok {
		t.Fatalf("intents is not an array")
	}
	if len(intents) != 4 {
		t.Fatalf("intents len = %d, want 4", len(intents))
	}
}

func TestBuildRecallResultHierarchyCollisionRules(t *testing.T) {
	episodeSums := []store.EpisodeSummary{
		{ID: 10, ChatSessionID: "s1", FromTurn: 1, ToTurn: 3, SummaryText: "ep1"},
	}
	resumePack := &store.ResumePack{
		Chapter: &store.ChapterSummary{
			ID: 20, ChatSessionID: "s1", FromTurn: 1, ToTurn: 5, ChapterIndex: 1, ChapterTitle: "Ch1", SummaryText: "ch1",
		},
		Arc: &store.ArcSummary{
			ID: 30, ChatSessionID: "s1", FromTurn: 1, ToTurn: 10, ArcName: "Arc1", CoreConflict: "conflict",
		},
		Saga: &store.SagaDigest{
			ID: 40, ChatSessionID: "s1", FromTurn: 1, ToTurn: 20, EraLabel: "E1", SagaSummary: "s1",
		},
	}
	trace := buildHierarchyConsistencyTrace(nil, resumePack, episodeSums)
	if trace["version"] != "p59a.v1" {
		t.Fatalf("version mismatch: %v", trace["version"])
	}
	if trace["saga_covers_arc"] != true {
		t.Fatalf("saga_covers_arc = %v, want true", trace["saga_covers_arc"])
	}
	if trace["arc_covers_chapter"] != true {
		t.Fatalf("arc_covers_chapter = %v, want true", trace["arc_covers_chapter"])
	}
	collisionRules, ok := trace["collision_rules"].([]string)
	if !ok {
		t.Fatalf("collision_rules is not a string slice")
	}
	if len(collisionRules) != 4 {
		t.Fatalf("collision_rules len = %d, want 4", len(collisionRules))
	}
}

func TestPrepareTurnArcDeliveryPath(t *testing.T) {
	now := time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC)
	fake := &turnRecordingStore{
		returnMemories: []store.Memory{
			{ID: 1, ChatSessionID: "sess-arc", TurnIndex: 2, SummaryJSON: `{"turn_summary":"A warm evening"}`, Importance: 0.9},
		},
		returnResumePack: &store.ResumePack{
			PackStatus:    "ready",
			AssembledText: "Session resume text.",
			Arc: &store.ArcSummary{
				ID:            1,
				ChatSessionID: "sess-arc",
				FromTurn:      1,
				ToTurn:        20,
				ArcName:       "The Great Arc",
				CoreConflict:  "Man vs Nature",
				CreatedAt:     &now,
			},
		},
	}

	cfg := config.Default()
	cfg.StoreMode = config.StoreModeDualShadow
	srv := NewServer(cfg)
	srv.Store = fake

	mux := http.NewServeMux()
	srv.RegisterRoutes(mux)

	body := `{"chat_session_id":"sess-arc","turn_index":3,"raw_user_input":"What happens next?","settings":{"injection_enabled":true,"input_context_enabled":true,"max_injection_chars":500,"max_input_context_chars":400,"top_k":2}}`
	req := httptest.NewRequest(http.MethodPost, "/prepare-turn", bytes.NewReader([]byte(body)))
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

	gp, ok := resp["generation_packet"].(map[string]any)
	if !ok {
		t.Fatalf("generation_packet is not an object")
	}

	trace, ok := gp["trace_summary"].(map[string]any)
	if !ok {
		t.Fatalf("trace_summary is not an object")
	}

	if trace["arc_delivered"] != true {
		t.Fatalf("arc_delivered = %v, want true", trace["arc_delivered"])
	}
	if trace["arc_text_chars"].(float64) <= 0 {
		t.Fatalf("arc_text_chars = %v, want > 0", trace["arc_text_chars"])
	}

	ip, ok := resp["injection_pack"].(map[string]any)
	if !ok {
		t.Fatalf("injection_pack is not an object")
	}

	if ip["arc_delivered"] != true {
		t.Fatalf("injection_pack.arc_delivered = %v, want true", ip["arc_delivered"])
	}

	at, ok := ip["arc_text"].(string)
	if !ok || at == "" {
		t.Fatalf("injection_pack.arc_text missing or empty")
	}
}

func TestPrepareTurnRuntimeTokenProfile(t *testing.T) {
	fake := &turnRecordingStore{
		returnMemories: []store.Memory{
			{ID: 1, ChatSessionID: "sess-rt", TurnIndex: 2, SummaryJSON: `{"turn_summary":"test"}`, Importance: 0.9},
		},
	}

	cfg := config.Default()
	cfg.StoreMode = config.StoreModeDualShadow
	srv := NewServer(cfg)
	srv.Store = fake

	mux := http.NewServeMux()
	srv.RegisterRoutes(mux)

	body := `{"chat_session_id":"sess-rt","turn_index":3,"raw_user_input":"hello","settings":{"injection_enabled":true,"input_context_enabled":true,"max_injection_chars":500,"max_input_context_chars":400,"top_k":2},"client_meta":{"context_window_profile":"ultra"}}`
	req := httptest.NewRequest(http.MethodPost, "/prepare-turn", bytes.NewReader([]byte(body)))
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

	gp, ok := resp["generation_packet"].(map[string]any)
	if !ok {
		t.Fatalf("generation_packet is not an object")
	}

	trace, ok := gp["trace_summary"].(map[string]any)
	if !ok {
		t.Fatalf("trace_summary is not an object")
	}

	rtp, ok := trace["runtime_token_profile"].(map[string]any)
	if !ok {
		t.Fatalf("runtime_token_profile is not an object")
	}

	if rtp["version"] != "p61a.v1" {
		t.Fatalf("version = %v, want p61a.v1", rtp["version"])
	}
	if rtp["context_window_profile"] != "ultra" {
		t.Fatalf("context_window_profile = %v, want ultra", rtp["context_window_profile"])
	}
	if rtp["auto_optimized"] != true {
		t.Fatalf("auto_optimized = %v, want true", rtp["auto_optimized"])
	}
}

func TestBuildRecallResultTemporalProximityBoost(t *testing.T) {
	fake := &turnRecordingStore{
		returnMemories: []store.Memory{
			{ID: 1, ChatSessionID: "sess-tp", TurnIndex: 2, SummaryJSON: `{"turn_summary":"test"}`, Importance: 0.9},
		},
		returnChatLogs: []store.ChatLog{
			{ID: 1, ChatSessionID: "sess-tp", TurnIndex: 1, Role: "user", Content: "hello"},
			{ID: 2, ChatSessionID: "sess-tp", TurnIndex: 2, Role: "assistant", Content: "hi"},
		},
	}

	cfg := config.Default()
	cfg.StoreMode = config.StoreModeDualShadow
	srv := NewServer(cfg)
	srv.Store = fake

	mux := http.NewServeMux()
	srv.RegisterRoutes(mux)

	body := `{"chat_session_id":"sess-tp","turn_index":3,"raw_user_input":"hello","settings":{"injection_enabled":true,"input_context_enabled":true,"max_injection_chars":500,"max_input_context_chars":400,"top_k":2},"client_meta":{"context_window_profile":"ultra"}}`
	req := httptest.NewRequest(http.MethodPost, "/prepare-turn", bytes.NewReader([]byte(body)))
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

	rr, ok := resp["recall_result"].(map[string]any)
	if !ok {
		t.Fatalf("recall_result is not an object")
	}

	tpb, ok := rr["temporal_proximity_boost"].(map[string]any)
	if !ok {
		t.Fatalf("temporal_proximity_boost is not an object")
	}

	if tpb["version"] != "p71a.v1" {
		t.Fatalf("version = %v, want p71a.v1", tpb["version"])
	}
	if tpb["boost_active"] != true {
		t.Fatalf("boost_active = %v, want true", tpb["boost_active"])
	}
	if tpb["profile"] != "ultra" {
		t.Fatalf("profile = %v, want ultra", tpb["profile"])
	}
}

func TestBuildRecallResultBudgetTransitionEvidence(t *testing.T) {
	fake := &turnRecordingStore{
		returnMemories: []store.Memory{
			{ID: 1, ChatSessionID: "sess-bt", TurnIndex: 2, SummaryJSON: `{"turn_summary":"test"}`, Importance: 0.9},
		},
	}

	cfg := config.Default()
	cfg.StoreMode = config.StoreModeDualShadow
	srv := NewServer(cfg)
	srv.Store = fake

	mux := http.NewServeMux()
	srv.RegisterRoutes(mux)

	body := `{"chat_session_id":"sess-bt","turn_index":3,"raw_user_input":"hello","settings":{"injection_enabled":true,"input_context_enabled":true,"max_injection_chars":500,"max_input_context_chars":400,"top_k":2}}`
	req := httptest.NewRequest(http.MethodPost, "/prepare-turn", bytes.NewReader([]byte(body)))
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

	rr, ok := resp["recall_result"].(map[string]any)
	if !ok {
		t.Fatalf("recall_result is not an object")
	}

	pbp, ok := rr["packet_budget_policy"].(map[string]any)
	if !ok {
		t.Fatalf("packet_budget_policy is not an object")
	}

	bt, ok := pbp["budget_transition"].(map[string]any)
	if !ok {
		t.Fatalf("budget_transition is not an object")
	}

	if bt["version"] != "p75a.v1" {
		t.Fatalf("version = %v, want p75a.v1", bt["version"])
	}
	if bt["from_mode"] != "policy_only" {
		t.Fatalf("from_mode = %v, want policy_only", bt["from_mode"])
	}
	if bt["to_mode"] != "enforced_shadow" {
		t.Fatalf("to_mode = %v, want enforced_shadow", bt["to_mode"])
	}
	if bt["transition_ready"] != true {
		t.Fatalf("transition_ready = %v, want true", bt["transition_ready"])
	}
}

func TestBuildRecallResultBudgetCaps(t *testing.T) {
	fake := &turnRecordingStore{
		returnMemories: []store.Memory{
			{ID: 1, ChatSessionID: "sess-bc", TurnIndex: 2, SummaryJSON: `{"turn_summary":"test"}`, Importance: 0.9},
		},
	}

	cfg := config.Default()
	cfg.StoreMode = config.StoreModeDualShadow
	srv := NewServer(cfg)
	srv.Store = fake

	mux := http.NewServeMux()
	srv.RegisterRoutes(mux)

	body := `{"chat_session_id":"sess-bc","turn_index":3,"raw_user_input":"hello","settings":{"injection_enabled":true,"input_context_enabled":true,"max_injection_chars":500,"max_input_context_chars":400,"top_k":2}}`
	req := httptest.NewRequest(http.MethodPost, "/prepare-turn", bytes.NewReader([]byte(body)))
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

	rr, ok := resp["recall_result"].(map[string]any)
	if !ok {
		t.Fatalf("recall_result is not an object")
	}

	pbp, ok := rr["packet_budget_policy"].(map[string]any)
	if !ok {
		t.Fatalf("packet_budget_policy is not an object")
	}

	bc, ok := pbp["budget_caps"].(map[string]any)
	if !ok {
		t.Fatalf("budget_caps is not an object")
	}

	if bc["version"] != "p76a.v1" {
		t.Fatalf("version = %v, want p76a.v1", bc["version"])
	}
	if bc["layer_cap"].(float64) != 12 {
		t.Fatalf("layer_cap = %v, want 12", bc["layer_cap"])
	}
	if bc["char_cap"].(float64) != 3000 {
		t.Fatalf("char_cap = %v, want 3000", bc["char_cap"])
	}
	if bc["canon_hard_floor"].(float64) != 120 {
		t.Fatalf("canon_hard_floor = %v, want 120", bc["canon_hard_floor"])
	}
}

func TestPrepareTurnBudgetDecisionsT1a(t *testing.T) {
	fake := &turnRecordingStore{
		returnMemories: []store.Memory{
			{ID: 1, ChatSessionID: "sess-bd", TurnIndex: 2, SummaryJSON: `{"turn_summary":"test"}`, Importance: 0.9},
		},
	}

	cfg := config.Default()
	cfg.StoreMode = config.StoreModeDualShadow
	srv := NewServer(cfg)
	srv.Store = fake

	mux := http.NewServeMux()
	srv.RegisterRoutes(mux)

	body := `{"chat_session_id":"sess-bd","turn_index":3,"raw_user_input":"hello","settings":{"injection_enabled":true,"input_context_enabled":true,"max_injection_chars":500,"max_input_context_chars":400,"top_k":2}}`
	req := httptest.NewRequest(http.MethodPost, "/prepare-turn", bytes.NewReader([]byte(body)))
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

	ip, ok := resp["injection_pack"].(map[string]any)
	if !ok {
		t.Fatalf("injection_pack is not an object")
	}

	bd, ok := ip["budget_decisions"].(map[string]any)
	if !ok {
		t.Fatalf("budget_decisions is not an object")
	}

	if bd["t1a_enforced_ready"] != true {
		t.Fatalf("t1a_enforced_ready = %v, want true", bd["t1a_enforced_ready"])
	}
	if bd["t1a_transition"] != "policy_only_to_enforced_shadow" {
		t.Fatalf("t1a_transition = %v, want policy_only_to_enforced_shadow", bd["t1a_transition"])
	}
}

func TestBuildRecallResultRelationshipFirstBudget(t *testing.T) {
	fake := &turnRecordingStore{
		returnMemories: []store.Memory{
			{ID: 1, ChatSessionID: "sess-rf", TurnIndex: 2, SummaryJSON: `{"turn_summary":"test"}`, Importance: 0.9},
		},
	}

	cfg := config.Default()
	cfg.StoreMode = config.StoreModeDualShadow
	srv := NewServer(cfg)
	srv.Store = fake

	mux := http.NewServeMux()
	srv.RegisterRoutes(mux)

	body := `{"chat_session_id":"sess-rf","turn_index":3,"raw_user_input":"hello","settings":{"injection_enabled":true,"input_context_enabled":true,"max_injection_chars":500,"max_input_context_chars":400,"top_k":2}}`
	req := httptest.NewRequest(http.MethodPost, "/prepare-turn", bytes.NewReader([]byte(body)))
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

	ip, ok := resp["injection_pack"].(map[string]any)
	if !ok {
		t.Fatalf("injection_pack is not an object")
	}

	counts, ok := ip["counts"].(map[string]any)
	if !ok {
		t.Fatalf("counts is not an object")
	}

	rfb, ok := counts["relationship_first_budget"].(map[string]any)
	if !ok {
		t.Fatalf("relationship_first_budget is not an object")
	}

	if rfb["version"] != "p80a.v1" {
		t.Fatalf("version = %v, want p80a.v1", rfb["version"])
	}
	if rfb["status"] != "shadow_only" {
		t.Fatalf("status = %v, want shadow_only", rfb["status"])
	}
	if rfb["structure"] != "relationship_first" {
		t.Fatalf("structure = %v, want relationship_first", rfb["structure"])
	}
	if rfb["long_tier_cap"].(float64) != 2400 {
		t.Fatalf("long_tier_cap = %v, want 2400", rfb["long_tier_cap"])
	}
	if rfb["ultra_tier_cap"].(float64) != 1800 {
		t.Fatalf("ultra_tier_cap = %v, want 1800", rfb["ultra_tier_cap"])
	}
	if rfb["extreme_tier_cap"].(float64) != 1200 {
		t.Fatalf("extreme_tier_cap = %v, want 1200", rfb["extreme_tier_cap"])
	}
}

func TestPrepareTurnThreeHundredTurnRelationshipRecallKeepsCurrentState(t *testing.T) {
	chatLogs := make([]store.ChatLog, 0, 300)
	for i := 1; i <= 300; i++ {
		role := "user"
		if i%2 == 0 {
			role = "assistant"
		}
		chatLogs = append(chatLogs, store.ChatLog{
			ID:            int64(i),
			ChatSessionID: "sess-rel300",
			TurnIndex:     i,
			Role:          role,
			Content:       fmt.Sprintf("turn %03d old archive corridor scene", i),
		})
	}
	fake := &turnRecordingStore{
		returnChatLogs: chatLogs,
		returnMemories: []store.Memory{
			{ID: 1, ChatSessionID: "sess-rel300", TurnIndex: 42, SummaryJSON: `{"turn_summary":"old unrelated archive corridor memory"}`, Importance: 0.35},
		},
		returnCharStates: []store.CharacterState{{
			ChatSessionID:     "sess-rel300",
			CharacterName:     "Chloe",
			StatusJSON:        `{"mood":"guarded but attentive"}`,
			RelationshipsJSON: `{"Hero":{"affection":82,"tension":18,"last_change":"Chloe chose to trust Hero in the current scene"}}`,
			TurnIndex:         300,
		}},
	}

	cfg := config.Default()
	cfg.StoreMode = config.StoreModeDualShadow
	srv := NewServer(cfg)
	srv.Store = fake

	mux := http.NewServeMux()
	srv.RegisterRoutes(mux)

	body := `{"chat_session_id":"sess-rel300","turn_index":301,"raw_user_input":"Continue the current relationship scene with Chloe.","settings":{"injection_enabled":true,"input_context_enabled":true,"max_injection_chars":900,"max_input_context_chars":500,"top_k":5}}`
	req := httptest.NewRequest(http.MethodPost, "/prepare-turn", bytes.NewReader([]byte(body)))
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
	ip, ok := resp["injection_pack"].(map[string]any)
	if !ok {
		t.Fatalf("injection_pack is not an object")
	}
	characterText, _ := ip["character_text"].(string)
	if !strings.Contains(characterText, "Chloe") || !strings.Contains(characterText, "relationships") || !strings.Contains(characterText, "affection") || !strings.Contains(characterText, "82") {
		t.Fatalf("character_text did not preserve current relationship state after 300 turns: %q", characterText)
	}
	trace, ok := resp["trace_preview"].(map[string]any)
	if !ok {
		t.Fatalf("trace_preview is not an object")
	}
	evidenceCounts, ok := trace["evidence_counts"].(map[string]any)
	if !ok {
		t.Fatalf("trace_preview.evidence_counts is not an object")
	}
	if evidenceCounts["chat_logs"].(float64) != 300 {
		t.Fatalf("chat_logs = %v, want 300", evidenceCounts["chat_logs"])
	}
	counts, ok := ip["counts"].(map[string]any)
	if !ok {
		t.Fatalf("counts is not an object")
	}
	rfb, ok := counts["relationship_first_budget"].(map[string]any)
	if !ok || rfb["structure"] != "relationship_first" {
		t.Fatalf("relationship_first_budget missing or wrong: %#v", counts["relationship_first_budget"])
	}
}

func TestBuildRecallResultStalenessThreshold(t *testing.T) {
	fake := &turnRecordingStore{
		returnMemories: []store.Memory{
			{ID: 1, ChatSessionID: "sess-st", TurnIndex: 2, SummaryJSON: `{"turn_summary":"test"}`, Importance: 0.9},
		},
		returnChatLogs: []store.ChatLog{
			{ID: 1, ChatSessionID: "sess-st", TurnIndex: 1, Role: "user", Content: "a"},
			{ID: 2, ChatSessionID: "sess-st", TurnIndex: 2, Role: "assistant", Content: "b"},
			{ID: 3, ChatSessionID: "sess-st", TurnIndex: 3, Role: "user", Content: "c"},
			{ID: 4, ChatSessionID: "sess-st", TurnIndex: 4, Role: "assistant", Content: "d"},
			{ID: 5, ChatSessionID: "sess-st", TurnIndex: 5, Role: "user", Content: "e"},
			{ID: 6, ChatSessionID: "sess-st", TurnIndex: 6, Role: "assistant", Content: "f"},
		},
	}

	cfg := config.Default()
	cfg.StoreMode = config.StoreModeDualShadow
	srv := NewServer(cfg)
	srv.Store = fake

	mux := http.NewServeMux()
	srv.RegisterRoutes(mux)

	body := `{"chat_session_id":"sess-st","turn_index":7,"raw_user_input":"hello","settings":{"injection_enabled":true,"input_context_enabled":true,"max_injection_chars":500,"max_input_context_chars":400,"top_k":2}}`
	req := httptest.NewRequest(http.MethodPost, "/prepare-turn", bytes.NewReader([]byte(body)))
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

	rr, ok := resp["recall_result"].(map[string]any)
	if !ok {
		t.Fatalf("recall_result is not an object")
	}

	sfs, ok := rr["summary_failure_stability"].(map[string]any)
	if !ok {
		t.Fatalf("summary_failure_stability is not an object")
	}

	st, ok := sfs["staleness_threshold"].(map[string]any)
	if !ok {
		t.Fatalf("staleness_threshold is not an object")
	}

	if st["version"] != "p85a.v1" {
		t.Fatalf("version = %v, want p85a.v1", st["version"])
	}
	if st["threshold_turns"].(float64) != 5 {
		t.Fatalf("threshold_turns = %v, want 5", st["threshold_turns"])
	}
	if st["detected"] != true {
		t.Fatalf("detected = %v, want true", st["detected"])
	}
}

func TestBuildRecallResultRetryEnqueue(t *testing.T) {
	fake := &turnRecordingStore{
		returnMemories: []store.Memory{},
		returnChatLogs: []store.ChatLog{
			{ID: 1, ChatSessionID: "sess-re", TurnIndex: 1, Role: "assistant", Content: ""},
		},
	}

	cfg := config.Default()
	cfg.StoreMode = config.StoreModeDualShadow
	srv := NewServer(cfg)
	srv.Store = fake

	mux := http.NewServeMux()
	srv.RegisterRoutes(mux)

	body := `{"chat_session_id":"sess-re","turn_index":2,"raw_user_input":"hello","settings":{"injection_enabled":true,"input_context_enabled":true,"max_injection_chars":500,"max_input_context_chars":400,"top_k":2}}`
	req := httptest.NewRequest(http.MethodPost, "/prepare-turn", bytes.NewReader([]byte(body)))
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

	rr, ok := resp["recall_result"].(map[string]any)
	if !ok {
		t.Fatalf("recall_result is not an object")
	}

	sfs, ok := rr["summary_failure_stability"].(map[string]any)
	if !ok {
		t.Fatalf("summary_failure_stability is not an object")
	}

	re, ok := sfs["retry_enqueue"].(map[string]any)
	if !ok {
		t.Fatalf("retry_enqueue is not an object")
	}

	if re["version"] != "p86a.v1" {
		t.Fatalf("version = %v, want p86a.v1", re["version"])
	}
	if re["enqueue_ready"] != true {
		t.Fatalf("enqueue_ready = %v, want true", re["enqueue_ready"])
	}
	if re["force_regenerate"] != false {
		t.Fatalf("force_regenerate = %v, want false", re["force_regenerate"])
	}
}

func TestBuildRecallResultFailureWarning(t *testing.T) {
	fake := &turnRecordingStore{
		returnMemories: []store.Memory{},
	}

	cfg := config.Default()
	cfg.StoreMode = config.StoreModeDualShadow
	srv := NewServer(cfg)
	srv.Store = fake

	mux := http.NewServeMux()
	srv.RegisterRoutes(mux)

	body := `{"chat_session_id":"sess-fw","turn_index":2,"raw_user_input":"hello","settings":{"injection_enabled":true,"input_context_enabled":true,"max_injection_chars":500,"max_input_context_chars":400,"top_k":2}}`
	req := httptest.NewRequest(http.MethodPost, "/prepare-turn", bytes.NewReader([]byte(body)))
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

	rr, ok := resp["recall_result"].(map[string]any)
	if !ok {
		t.Fatalf("recall_result is not an object")
	}

	sfs, ok := rr["summary_failure_stability"].(map[string]any)
	if !ok {
		t.Fatalf("summary_failure_stability is not an object")
	}

	fw, ok := sfs["failure_warning"].(map[string]any)
	if !ok {
		t.Fatalf("failure_warning is not an object")
	}

	if fw["version"] != "p87a.v1" {
		t.Fatalf("version = %v, want p87a.v1", fw["version"])
	}
	if fw["warning_active"] != true {
		t.Fatalf("warning_active = %v, want true", fw["warning_active"])
	}
	if fw["warning_level"] != "warn" {
		t.Fatalf("warning_level = %v, want warn", fw["warning_level"])
	}
}

func TestBuildRecallResultReplayGate(t *testing.T) {
	fake := &turnRecordingStore{
		returnMemories: []store.Memory{
			{ID: 1, ChatSessionID: "sess-rg", TurnIndex: 2, SummaryJSON: `{"turn_summary":"test"}`, Importance: 0.9},
		},
		returnChatLogs: []store.ChatLog{
			{ID: 1, ChatSessionID: "sess-rg", TurnIndex: 1, Role: "user", Content: "hello"},
		},
	}

	cfg := config.Default()
	cfg.StoreMode = config.StoreModeDualShadow
	srv := NewServer(cfg)
	srv.Store = fake

	mux := http.NewServeMux()
	srv.RegisterRoutes(mux)

	body := `{"chat_session_id":"sess-rg","turn_index":3,"raw_user_input":"hello","settings":{"injection_enabled":true,"input_context_enabled":true,"max_injection_chars":500,"max_input_context_chars":400,"top_k":2}}`
	req := httptest.NewRequest(http.MethodPost, "/prepare-turn", bytes.NewReader([]byte(body)))
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

	rr, ok := resp["recall_result"].(map[string]any)
	if !ok {
		t.Fatalf("recall_result is not an object")
	}

	sfs, ok := rr["summary_failure_stability"].(map[string]any)
	if !ok {
		t.Fatalf("summary_failure_stability is not an object")
	}

	rg, ok := sfs["replay_gate"].(map[string]any)
	if !ok {
		t.Fatalf("replay_gate is not an object")
	}

	if rg["version"] != "p88a.v1" {
		t.Fatalf("version = %v, want p88a.v1", rg["version"])
	}
	if rg["gate_active"] != true {
		t.Fatalf("gate_active = %v, want true", rg["gate_active"])
	}
	if rg["session_captured"] != true {
		t.Fatalf("session_captured = %v, want true", rg["session_captured"])
	}
}

func TestPrepareTurnInjectionPackBudgetDecisionsOffMode(t *testing.T) {
	fake := &turnRecordingStore{}

	cfg := config.Default()
	cfg.StoreMode = config.StoreModeDualShadow
	srv := NewServer(cfg)
	srv.Store = fake

	mux := http.NewServeMux()
	srv.RegisterRoutes(mux)

	body := `{"chat_session_id":"sess-bd-off","turn_index":1,"raw_user_input":"hello","settings":{"injection_enabled":false,"input_context_enabled":false,"max_injection_chars":500,"max_input_context_chars":400,"top_k":2}}`
	req := httptest.NewRequest(http.MethodPost, "/prepare-turn", bytes.NewReader([]byte(body)))
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

	ip, ok := resp["injection_pack"].(map[string]any)
	if !ok {
		t.Fatalf("injection_pack is not an object")
	}

	bd, ok := ip["budget_decisions"].(map[string]any)
	if !ok {
		t.Fatalf("budget_decisions is not an object")
	}

	if bd["version"] != "t1c.v1" {
		t.Fatalf("version = %v, want t1c.v1", bd["version"])
	}
	if bd["mode"] != "read_only_surface" {
		t.Fatalf("mode = %v, want read_only_surface", bd["mode"])
	}
	if bd["status"] != "off" {
		t.Fatalf("status = %v, want off", bd["status"])
	}
	if bd["decision_count"] != float64(0) {
		t.Fatalf("decision_count = %v, want 0", bd["decision_count"])
	}
	if bd["source_mapping"] != "recall_result.intent_execution_shadow.budget_enforcement" {
		t.Fatalf("source_mapping = %v, want recall_result.intent_execution_shadow.budget_enforcement", bd["source_mapping"])
	}
	if bd["source_event"] != "budget_enforcement" {
		t.Fatalf("source_event = %v, want budget_enforcement", bd["source_event"])
	}
	decisions, ok := bd["decisions"].([]any)
	if !ok {
		t.Fatalf("decisions is not an array")
	}
	if len(decisions) != 0 {
		t.Fatalf("decisions len = %d, want 0", len(decisions))
	}
}

func TestPrepareTurnInjectionPackBudgetDecisionsReadyMode(t *testing.T) {
	fake := &turnRecordingStore{
		returnMemories: []store.Memory{
			{ID: 1, ChatSessionID: "sess-bd-ready", TurnIndex: 2, SummaryJSON: `{"turn_summary":"A warm evening in the garden"}`, Importance: 0.9, PlaceWing: "East", PlaceRoom: "Garden"},
			{ID: 2, ChatSessionID: "sess-bd-ready", TurnIndex: 3, SummaryJSON: `{"turn_summary":"The door creaks open"}`, Importance: 0.8, PlaceWing: "North", PlaceRoom: "Hall"},
		},
		returnEvidence: []store.DirectEvidence{
			{ID: 1, ChatSessionID: "sess-bd-ready", TurnAnchor: 2, EvidenceText: "The red wax seal was broken.", EvidenceKind: "item"},
		},
		returnChatLogs: []store.ChatLog{
			{ID: 1, ChatSessionID: "sess-bd-ready", TurnIndex: 1, Role: "user", Content: "Hello there"},
		},
	}

	cfg := config.Default()
	cfg.StoreMode = config.StoreModeDualShadow
	srv := NewServer(cfg)
	srv.Store = fake

	mux := http.NewServeMux()
	srv.RegisterRoutes(mux)

	body := `{"chat_session_id":"sess-bd-ready","turn_index":4,"raw_user_input":"continue","settings":{"injection_enabled":true,"input_context_enabled":true,"max_injection_chars":500,"max_input_context_chars":400,"top_k":2}}`
	req := httptest.NewRequest(http.MethodPost, "/prepare-turn", bytes.NewReader([]byte(body)))
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

	ip, ok := resp["injection_pack"].(map[string]any)
	if !ok {
		t.Fatalf("injection_pack is not an object")
	}

	bd, ok := ip["budget_decisions"].(map[string]any)
	if !ok {
		t.Fatalf("budget_decisions is not an object")
	}

	if bd["version"] != "t1c.v1" {
		t.Fatalf("version = %v, want t1c.v1", bd["version"])
	}
	if bd["mode"] != "read_only_surface" {
		t.Fatalf("mode = %v, want read_only_surface", bd["mode"])
	}
	if bd["status"] != "ready" {
		t.Fatalf("status = %v, want ready", bd["status"])
	}
	decisionCount, ok := bd["decision_count"].(float64)
	if !ok || decisionCount <= 0 {
		t.Fatalf("decision_count = %v, want > 0", bd["decision_count"])
	}
	decisions, ok := bd["decisions"].([]any)
	if !ok {
		t.Fatalf("decisions is not an array")
	}
	if len(decisions) == 0 {
		t.Fatalf("decisions len = 0, want > 0")
	}

	if bd["source_mapping"] != "recall_result.intent_execution_shadow.budget_enforcement" {
		t.Fatalf("source_mapping = %v, want recall_result.intent_execution_shadow.budget_enforcement", bd["source_mapping"])
	}
	if bd["source_event"] != "budget_enforcement" {
		t.Fatalf("source_event = %v, want budget_enforcement", bd["source_event"])
	}
	sourceCounters, ok := bd["source_counters"].([]any)
	if !ok || len(sourceCounters) == 0 {
		t.Fatalf("source_counters = %v, want non-empty array", bd["source_counters"])
	}

	requiredFields := []string{"intent", "tier", "document_id", "decision", "reason", "cap_scope", "char_cost", "running_total_chars", "cap_chars"}
	for i, d := range decisions {
		dec, ok := d.(map[string]any)
		if !ok {
			t.Fatalf("decision[%d] is not an object", i)
		}
		for _, field := range requiredFields {
			if _, ok := dec[field]; !ok {
				t.Fatalf("decision[%d] missing field %s", i, field)
			}
		}
	}
}

func TestBuildRecallResultIntentExecutionShadowBudgetEnforcementT1b(t *testing.T) {
	docs := []map[string]any{
		{"tier": "memory", "document_id": "d1", "text": "hello world"},
		{"tier": "episode", "document_id": "d2", "text": "episode summary here"},
		{"tier": "saga", "document_id": "d3", "text": "saga text"},
		{"tier": "arc", "document_id": "d4", "text": "arc content"},
	}
	shadow := buildIntentExecutionShadow(docs, map[string]any{}, "long", q3PacketBudgetPolicy())
	be, ok := shadow["budget_enforcement"].(map[string]any)
	if !ok {
		t.Fatal("budget_enforcement missing")
	}
	if be["version"] != "t1b.v1" {
		t.Fatalf("version = %v, want t1b.v1", be["version"])
	}
	if be["mode"] != "enforced_shadow" {
		t.Fatalf("mode = %v, want enforced_shadow", be["mode"])
	}
	if be["canon_hard_floor"] != 120 {
		t.Fatalf("canon_hard_floor = %v, want 120", be["canon_hard_floor"])
	}
	if be["canon_floor_reserved_chars"] != 120 {
		t.Fatalf("canon_floor_reserved_chars = %v, want 120", be["canon_floor_reserved_chars"])
	}
	if _, ok := be["canon_selected_chars"]; !ok {
		t.Fatal("canon_selected_chars missing")
	}
	if _, ok := be["retrieval_layer_caps"]; !ok {
		t.Fatal("retrieval_layer_caps missing")
	}
	rlc, ok := be["retrieval_layer_caps"].([]map[string]any)
	if !ok || len(rlc) != 4 {
		t.Fatalf("retrieval_layer_caps mismatch: %v", be["retrieval_layer_caps"])
	}
	for _, cap := range rlc {
		if cap["reason"] != "priority_deferred" {
			t.Fatalf("reason = %v, want priority_deferred", cap["reason"])
		}
		if cap["cap_scope"] != "layer_cap" {
			t.Fatalf("cap_scope = %v, want layer_cap", cap["cap_scope"])
		}
	}
	if _, ok := be["reason_counts"]; !ok {
		t.Fatal("reason_counts missing")
	}
	rc, ok := be["reason_counts"].(map[string]int)
	if !ok {
		t.Fatal("reason_counts type mismatch")
	}
	if _, ok := rc["floor_reserved"]; !ok {
		t.Fatal("floor_reserved missing in reason_counts")
	}
	et, ok := shadow["enforced_takeover"].(map[string]any)
	if !ok {
		t.Fatal("enforced_takeover missing")
	}
	if et["version"] != "t1e.v1" {
		t.Fatalf("enforced_takeover version = %v, want t1e.v1", et["version"])
	}
	if et["mode"] != "enforced_default_takeover_only" {
		t.Fatalf("enforced_takeover mode = %v, want enforced_default_takeover_only", et["mode"])
	}
	if _, ok := et["selected_candidates"]; !ok {
		t.Fatal("selected_candidates missing in enforced_takeover")
	}
}

func TestBuildRecallResultBudgetDecisionsCallbackSagaLayerCap(t *testing.T) {
	docs := []map[string]any{
		{"tier": "memory", "document_id": "d1", "text": "a"},
		{"tier": "arc", "document_id": "d2", "text": "b"},
		{"tier": "saga", "document_id": "d3", "text": "c"},
	}
	shadow := buildIntentExecutionShadow(docs, map[string]any{}, "long", q3PacketBudgetPolicy())
	be, ok := shadow["budget_enforcement"].(map[string]any)
	if !ok {
		t.Fatal("budget_enforcement missing")
	}
	rlc, ok := be["retrieval_layer_caps"].([]map[string]any)
	if !ok || len(rlc) != 4 {
		t.Fatalf("retrieval_layer_caps len = %v, want 4", len(rlc))
	}
	callbackFound := false
	for _, c := range rlc {
		if c["intent"] == "callback" {
			callbackFound = true
			if c["cap_scope"] != "layer_cap" {
				t.Fatalf("callback cap_scope = %v, want layer_cap", c["cap_scope"])
			}
			if c["reason"] != "priority_deferred" {
				t.Fatalf("callback reason = %v, want priority_deferred", c["reason"])
			}
		}
	}
	if !callbackFound {
		t.Fatal("callback layer cap not found")
	}
}

func TestBuildRecallResultBudgetDecisionsCanonHardFloor(t *testing.T) {
	docs := []map[string]any{
		{"tier": "memory", "document_id": "d1", "text": "canon memory text"},
		{"tier": "episode", "document_id": "d2", "text": "episode text"},
		{"tier": "arc", "document_id": "d3", "text": "arc text"},
	}
	shadow := buildIntentExecutionShadow(docs, map[string]any{}, "long", q3PacketBudgetPolicy())
	be, ok := shadow["budget_enforcement"].(map[string]any)
	if !ok {
		t.Fatal("budget_enforcement missing")
	}
	if be["canon_hard_floor"] != 120 {
		t.Fatalf("canon_hard_floor = %v, want 120", be["canon_hard_floor"])
	}
	if be["canon_floor_reserved_chars"] != 120 {
		t.Fatalf("canon_floor_reserved_chars = %v, want 120", be["canon_floor_reserved_chars"])
	}
	if _, ok := be["canon_selected_chars"]; !ok {
		t.Fatal("canon_selected_chars missing")
	}
}

func TestBuildRecallResultRoutingShadowEnforcedTakeoverOff(t *testing.T) {
	fake := &turnRecordingStore{}
	cfg := config.Default()
	cfg.StoreMode = config.StoreModeDualShadow
	srv := NewServer(cfg)
	srv.Store = fake
	mux := http.NewServeMux()
	srv.RegisterRoutes(mux)

	body := `{"chat_session_id":"test-takeover-off","raw_user_input":"query"}`
	req := httptest.NewRequest(http.MethodPost, "/prepare-turn", bytes.NewReader([]byte(body)))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d", rec.Code)
	}
	var resp map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatal(err)
	}
	recall := resp["recall_result"].(map[string]any)
	contract := recall["intent_contract"].(map[string]any)
	rsto, ok := contract["routing_shadow_enforced_takeover"].(map[string]any)
	if !ok {
		t.Fatal("routing_shadow_enforced_takeover missing")
	}
	if rsto["version"] != "t1e.v1" {
		t.Fatalf("version = %v, want t1e.v1", rsto["version"])
	}
	if rsto["mode"] != "enforced_default_takeover_only" {
		t.Fatalf("mode = %v, want enforced_default_takeover_only", rsto["mode"])
	}
	if rsto["status"] != "off" {
		t.Fatalf("status = %v, want off", rsto["status"])
	}
	if rsto["ready"] != false {
		t.Fatalf("ready = %v, want false", rsto["ready"])
	}
	if rsto["reason"] != "no_candidates" {
		t.Fatalf("reason = %v, want no_candidates", rsto["reason"])
	}
	if rsto["promote_candidate"] != nil {
		t.Fatalf("promote_candidate = %v, want nil", rsto["promote_candidate"])
	}
	pbp, ok := recall["packet_budget_policy"].(map[string]any)
	if !ok {
		t.Fatal("packet_budget_policy missing")
	}
	if pbp["budget_mode"] != "policy_only" {
		t.Fatalf("budget_mode = %v, want policy_only", pbp["budget_mode"])
	}
}

func TestBuildRecallResultRoutingShadowEnforcedTakeoverPending(t *testing.T) {
	fake := &turnRecordingStore{
		returnChatLogs: []store.ChatLog{
			{ID: 1, ChatSessionID: "sess-pend", TurnIndex: 1, Role: "user", Content: "hello"},
		},
		returnKGTriples: []store.KGTriple{
			{ID: 1, ChatSessionID: "sess-pend", Subject: "A", Predicate: "B", Object: "C"},
		},
	}
	cfg := config.Default()
	cfg.StoreMode = config.StoreModeDualShadow
	srv := NewServer(cfg)
	srv.Store = fake
	mux := http.NewServeMux()
	srv.RegisterRoutes(mux)

	body := `{"chat_session_id":"sess-pend","turn_index":2,"raw_user_input":"hello","settings":{"injection_enabled":true,"input_context_enabled":true,"max_injection_chars":500,"max_input_context_chars":400,"top_k":2}}`
	req := httptest.NewRequest(http.MethodPost, "/prepare-turn", bytes.NewReader([]byte(body)))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200: %s", rec.Code, rec.Body.String())
	}
	var resp map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatal(err)
	}
	recall := resp["recall_result"].(map[string]any)
	contract := recall["intent_contract"].(map[string]any)
	rsto, ok := contract["routing_shadow_enforced_takeover"].(map[string]any)
	if !ok {
		t.Fatal("routing_shadow_enforced_takeover missing")
	}
	if rsto["status"] != "pending" {
		t.Fatalf("status = %v, want pending", rsto["status"])
	}
	if rsto["ready"] != false {
		t.Fatalf("ready = %v, want false", rsto["ready"])
	}
	if rsto["reason"] != "guard_not_ready" {
		t.Fatalf("reason = %v, want guard_not_ready", rsto["reason"])
	}
	pbp, ok := recall["packet_budget_policy"].(map[string]any)
	if !ok {
		t.Fatal("packet_budget_policy missing")
	}
	if pbp["budget_mode"] != "policy_only" {
		t.Fatalf("budget_mode = %v, want policy_only", pbp["budget_mode"])
	}
}

func TestBuildRecallResultRoutingShadowEnforcedTakeoverReady(t *testing.T) {
	fake := &turnRecordingStore{
		returnMemories: []store.Memory{
			{ID: 1, ChatSessionID: "sess-ready", TurnIndex: 2, SummaryJSON: `{"turn_summary":"A warm evening"}`, Importance: 0.9, PlaceWing: "East", PlaceRoom: "Garden"},
		},
	}
	cfg := config.Default()
	cfg.StoreMode = config.StoreModeDualShadow
	srv := NewServer(cfg)
	srv.Store = fake
	mux := http.NewServeMux()
	srv.RegisterRoutes(mux)

	body := `{"chat_session_id":"sess-ready","turn_index":3,"raw_user_input":"continue","settings":{"injection_enabled":true,"input_context_enabled":true,"max_injection_chars":500,"max_input_context_chars":400,"top_k":2}}`
	req := httptest.NewRequest(http.MethodPost, "/prepare-turn", bytes.NewReader([]byte(body)))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200: %s", rec.Code, rec.Body.String())
	}
	var resp map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatal(err)
	}
	recall := resp["recall_result"].(map[string]any)
	contract := recall["intent_contract"].(map[string]any)
	rsto, ok := contract["routing_shadow_enforced_takeover"].(map[string]any)
	if !ok {
		t.Fatal("routing_shadow_enforced_takeover missing")
	}
	if rsto["status"] != "ready" {
		t.Fatalf("status = %v, want ready", rsto["status"])
	}
	if rsto["ready"] != true {
		t.Fatalf("ready = %v, want true", rsto["ready"])
	}
	if rsto["reason"] != "routing_shadow_takeover_ready" {
		t.Fatalf("reason = %v, want routing_shadow_takeover_ready", rsto["reason"])
	}
	if rsto["promote_candidate"] == nil {
		t.Fatal("promote_candidate should not be nil")
	}
	pbp, ok := recall["packet_budget_policy"].(map[string]any)
	if !ok {
		t.Fatal("packet_budget_policy missing")
	}
	if pbp["budget_mode"] != "enforced" {
		t.Fatalf("budget_mode = %v, want enforced", pbp["budget_mode"])
	}
	if bt, ok := pbp["budget_transition"].(map[string]any); ok {
		if bt["to_mode"] != "enforced_shadow" {
			t.Fatalf("budget_transition.to_mode = %v, want enforced_shadow", bt["to_mode"])
		}
		if bt["transition_ready"] != true {
			t.Fatalf("budget_transition.transition_ready = %v, want true", bt["transition_ready"])
		}
	} else {
		t.Fatal("budget_transition missing")
	}
}

// --- S-1d execution trace tests (SEQ-10-P370~P384) ---

func TestS1dTraceContractOffMode(t *testing.T) {
	shadow := buildIntentExecutionShadow([]map[string]any{}, map[string]any{}, "long", q3PacketBudgetPolicy())
	trace, ok := shadow["trace"].(map[string]any)
	if !ok {
		t.Fatal("trace missing")
	}
	if trace["version"] != "s1d.v1" {
		t.Fatalf("version = %v, want s1d.v1", trace["version"])
	}
	if trace["mode"] != "shadow_trace_only" {
		t.Fatalf("mode = %v, want shadow_trace_only", trace["mode"])
	}
	summary := trace["summary"].(map[string]any)
	if summary == nil {
		t.Fatal("summary missing")
	}
	if summary["executed_intent_count"].(int) != 0 {
		t.Fatalf("executed_intent_count = %v, want 0", summary["executed_intent_count"])
	}
	if summary["input_candidate_count"].(int) != 0 {
		t.Fatalf("input_candidate_count = %v, want 0", summary["input_candidate_count"])
	}
	if summary["budget_drop_count"].(int) != 0 {
		t.Fatalf("budget_drop_count = %v, want 0", summary["budget_drop_count"])
	}
}

func TestS1dTraceContractReadyMode(t *testing.T) {
	docs := []map[string]any{
		{"tier": "memory", "document_id": "d1", "text": "hello"},
		{"tier": "episode", "document_id": "d2", "text": "world"},
		{"tier": "chapter", "document_id": "d3", "text": "foo"},
	}
	shadow := buildIntentExecutionShadow(docs, map[string]any{}, "long", q3PacketBudgetPolicy())
	trace, ok := shadow["trace"].(map[string]any)
	if !ok {
		t.Fatal("trace missing")
	}
	if trace["version"] != "s1d.v1" {
		t.Fatalf("version = %v, want s1d.v1", trace["version"])
	}
	summary := trace["summary"].(map[string]any)
	if summary["executed_intent_count"].(int) != 4 {
		t.Fatalf("executed_intent_count = %v, want 4", summary["executed_intent_count"])
	}
	if summary["input_candidate_count"].(int) != 3 {
		t.Fatalf("input_candidate_count = %v, want 3", summary["input_candidate_count"])
	}
	selectionEvents, ok := trace["selection_events"].([]map[string]any)
	if !ok || len(selectionEvents) == 0 {
		t.Fatal("selection_events missing or empty")
	}
	budgetEvents, ok := trace["budget_events"].([]map[string]any)
	if !ok || len(budgetEvents) == 0 {
		t.Fatal("budget_events missing or empty")
	}
	qb := trace["query_builder"].(map[string]any)
	if qb["routing_mode"] != "single_query_shared" {
		t.Fatalf("routing_mode = %v, want single_query_shared", qb["routing_mode"])
	}
}

func TestS1dTraceSelectionCountConsistency(t *testing.T) {
	docs := []map[string]any{
		{"tier": "memory", "document_id": "d1", "text": "a"},
		{"tier": "memory", "document_id": "d2", "text": "b"},
		{"tier": "memory", "document_id": "d3", "text": "c"},
		{"tier": "memory", "document_id": "d4", "text": "d"},
	}
	shadow := buildIntentExecutionShadow(docs, map[string]any{}, "long", q3PacketBudgetPolicy())
	trace := shadow["trace"].(map[string]any)
	summary := trace["summary"].(map[string]any)
	selectedCount := summary["selected_count"].(int)
	selectionEvents := trace["selection_events"].([]map[string]any)
	selectedInEvents := 0
	for _, ev := range selectionEvents {
		if ev["selected"].(bool) {
			selectedInEvents++
		}
	}
	if selectedInEvents != selectedCount {
		t.Fatalf("selected_in_events = %d, want %d", selectedInEvents, selectedCount)
	}
}

func TestS1dTraceSuppressionConsistency(t *testing.T) {
	docs := []map[string]any{
		{"tier": "memory", "document_id": "d1", "text": "a"},
		{"tier": "memory", "document_id": "d2", "text": "b"},
		{"tier": "memory", "document_id": "d3", "text": "c"},
		{"tier": "memory", "document_id": "d4", "text": "d"},
	}
	shadow := buildIntentExecutionShadow(docs, map[string]any{}, "long", q3PacketBudgetPolicy())
	trace := shadow["trace"].(map[string]any)
	summary := trace["summary"].(map[string]any)
	suppressedCount := summary["suppressed_count"].(int)
	selectionEvents := trace["selection_events"].([]map[string]any)
	suppressedInEvents := 0
	for _, ev := range selectionEvents {
		if !ev["selected"].(bool) {
			suppressedInEvents++
		}
	}
	if suppressedInEvents != suppressedCount {
		t.Fatalf("suppressed_in_events = %d, want %d", suppressedInEvents, suppressedCount)
	}
}

func TestS1dTraceNoBehaviorRegression(t *testing.T) {
	docs := []map[string]any{
		{"tier": "memory", "document_id": "d1", "text": "hello"},
		{"tier": "episode", "document_id": "d2", "text": "world"},
	}
	shadow := buildIntentExecutionShadow(docs, map[string]any{}, "long", q3PacketBudgetPolicy())
	if shadow["version"] != "p29a.v1" {
		t.Fatalf("version = %v, want p29a.v1", shadow["version"])
	}
	be := shadow["budget_enforcement"].(map[string]any)
	if be["version"] != "t1b.v1" {
		t.Fatalf("budget_enforcement version = %v, want t1b.v1", be["version"])
	}
	gt := shadow["guarded_takeover"].(map[string]any)
	if gt["decision"] != "shadow_compare" {
		t.Fatalf("guarded_takeover decision = %v, want shadow_compare", gt["decision"])
	}
	et := shadow["enforced_takeover"].(map[string]any)
	if et["decision"] != "enforced_shadow" {
		t.Fatalf("enforced_takeover decision = %v, want enforced_shadow", et["decision"])
	}
}

// --- T-1a enforced shadow budget surface tests (SEQ-10-P388~P400) ---

func TestT1aShadowBudgetContractOffMode(t *testing.T) {
	shadow := buildIntentExecutionShadow([]map[string]any{}, map[string]any{}, "long", q3PacketBudgetPolicy())
	be := shadow["budget_enforcement"].(map[string]any)
	if be["event_count"].(int) != 0 {
		t.Fatalf("event_count = %v, want 0", be["event_count"])
	}
	reasons := be["budget_reasons"].(map[string]int)
	if reasons["no_cap"] != 1 {
		t.Fatalf("no_cap = %v, want 1", reasons["no_cap"])
	}
	if reasons["within_cap"] != 0 {
		t.Fatalf("within_cap = %v, want 0", reasons["within_cap"])
	}
}

func TestT1aShadowBudgetEnforcedReadyMode(t *testing.T) {
	docs := []map[string]any{
		{"tier": "memory", "document_id": "d1", "text": "hello"},
		{"tier": "episode", "document_id": "d2", "text": "world"},
	}
	shadow := buildIntentExecutionShadow(docs, map[string]any{}, "long", q3PacketBudgetPolicy())
	be := shadow["budget_enforcement"].(map[string]any)
	if be["event_count"].(int) != 2 {
		t.Fatalf("event_count = %v, want 2", be["event_count"])
	}
	reasons := be["budget_reasons"].(map[string]int)
	if reasons["within_cap"] != 2 {
		t.Fatalf("within_cap = %v, want 2", reasons["within_cap"])
	}
}

func TestT1aShadowBudgetDropsOverCapCandidates(t *testing.T) {
	docs := []map[string]any{
		{"tier": "memory", "document_id": "d1", "text": strings.Repeat("a", 2000)},
		{"tier": "memory", "document_id": "d2", "text": strings.Repeat("b", 2000)},
		{"tier": "memory", "document_id": "d3", "text": strings.Repeat("c", 2000)},
		{"tier": "memory", "document_id": "d4", "text": strings.Repeat("d", 2000)},
	}
	shadow := buildIntentExecutionShadow(docs, map[string]any{}, "long", q3PacketBudgetPolicy())
	trace := shadow["trace"].(map[string]any)
	budgetEvents := trace["budget_events"].([]map[string]any)
	if len(budgetEvents) != 3 {
		t.Fatalf("budget_events len = %v, want 3", len(budgetEvents))
	}
	for _, ev := range budgetEvents {
		if ev["decision"] != "keep" {
			t.Fatalf("decision = %v, want keep", ev["decision"])
		}
		if ev["reason"] != "within_cap" {
			t.Fatalf("reason = %v, want within_cap", ev["reason"])
		}
	}
}

// --- S-1g temporal shadow scoring tests (SEQ-10-P404~P416) ---

func TestS1gTemporalScoringOffMode(t *testing.T) {
	shadow := buildIntentExecutionShadow([]map[string]any{}, map[string]any{}, "long", q3PacketBudgetPolicy())
	ts, ok := shadow["temporal_scoring"].(map[string]any)
	if !ok {
		t.Fatal("temporal_scoring missing")
	}
	if ts["version"] != "s1g.v1" {
		t.Fatalf("version = %v, want s1g.v1", ts["version"])
	}
	if ts["mode"] != "shadow_temporal_scoring_only" {
		t.Fatalf("mode = %v, want shadow_temporal_scoring_only", ts["mode"])
	}
	if ts["status"] != "off" {
		t.Fatalf("status = %v, want off", ts["status"])
	}
	if ts["reason"] != "profile_not_target" {
		t.Fatalf("reason = %v, want profile_not_target", ts["reason"])
	}
}

func TestS1gTemporalScoringUltraApplies(t *testing.T) {
	docs := []map[string]any{
		{"tier": "memory", "document_id": "d1", "text": "hello"},
	}
	shadow := buildIntentExecutionShadow(docs, map[string]any{}, "ultra", q3PacketBudgetPolicy())
	ts, ok := shadow["temporal_scoring"].(map[string]any)
	if !ok {
		t.Fatal("temporal_scoring missing")
	}
	if ts["status"] != "ready" {
		t.Fatalf("status = %v, want ready", ts["status"])
	}
	if ts["reason"] != nil {
		t.Fatalf("reason = %v, want nil", ts["reason"])
	}
	ann, ok := ts["ann_recency_score"].(map[string]any)
	if !ok {
		t.Fatal("ann_recency_score missing")
	}
	if ann["score_source"] != "temporal_proximity" {
		t.Fatalf("score_source = %v, want temporal_proximity", ann["score_source"])
	}
}

func TestS1gTemporalScoringMidPassThrough(t *testing.T) {
	docs := []map[string]any{
		{"tier": "memory", "document_id": "d1", "text": "hello"},
	}
	shadow := buildIntentExecutionShadow(docs, map[string]any{}, "long", q3PacketBudgetPolicy())
	ts, ok := shadow["temporal_scoring"].(map[string]any)
	if !ok {
		t.Fatal("temporal_scoring missing")
	}
	if ts["status"] != "off" {
		t.Fatalf("status = %v, want off", ts["status"])
	}
	if ts["reason"] != "profile_not_target" {
		t.Fatalf("reason = %v, want profile_not_target", ts["reason"])
	}
}

func TestS1gTemporalScoringRegressionEquivalents(t *testing.T) {
	fake := &turnRecordingStore{
		returnMemories: []store.Memory{
			{ID: 1, ChatSessionID: "sess-temp", TurnIndex: 2, SummaryJSON: `{"turn_summary":"test"}`, Importance: 0.9},
		},
	}
	cfg := config.Default()
	cfg.StoreMode = config.StoreModeDualShadow
	srv := NewServer(cfg)
	srv.Store = fake
	mux := http.NewServeMux()
	srv.RegisterRoutes(mux)

	body := `{"chat_session_id":"sess-temp","turn_index":3,"raw_user_input":"hello","settings":{"injection_enabled":true,"input_context_enabled":true,"max_injection_chars":500,"max_input_context_chars":400,"top_k":2},"client_meta":{"context_window_profile":"extreme"}}`
	req := httptest.NewRequest(http.MethodPost, "/prepare-turn", bytes.NewReader([]byte(body)))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200: %s", rec.Code, rec.Body.String())
	}
	var resp map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatal(err)
	}
	rr := resp["recall_result"].(map[string]any)
	contract := rr["intent_contract"].(map[string]any)
	rst, ok := contract["routing_shadow_temporal"].(map[string]any)
	if !ok {
		t.Fatal("routing_shadow_temporal missing")
	}
	if rst["version"] != "s1g.v1" {
		t.Fatalf("version = %v, want s1g.v1", rst["version"])
	}
	if rst["applied_intent_count"].(float64) != 4 {
		t.Fatalf("applied_intent_count = %v, want 4", rst["applied_intent_count"])
	}
	ies := rr["intent_execution_shadow"].(map[string]any)
	ts2, ok := ies["temporal_scoring"].(map[string]any)
	if !ok {
		t.Fatal("intent_execution_shadow.temporal_scoring missing")
	}
	if ts2["status"] != "ready" {
		t.Fatalf("temporal_scoring status = %v, want ready", ts2["status"])
	}
}

// --- U-1e captured-session replay gate tests (SEQ-10-P420~P434) ---

func TestU1eReplayGateContractOffMode(t *testing.T) {
	shadow := buildIntentExecutionShadow([]map[string]any{}, map[string]any{}, "long", q3PacketBudgetPolicy())
	rg, ok := shadow["replay_gate"].(map[string]any)
	if !ok {
		t.Fatal("replay_gate missing")
	}
	if rg["version"] != "u1e.v1" {
		t.Fatalf("version = %v, want u1e.v1", rg["version"])
	}
	if rg["status"] != "off" {
		t.Fatalf("status = %v, want off", rg["status"])
	}
	if rg["reason"] != "runtime_mode_not_per_intent_shadow" {
		t.Fatalf("reason = %v, want runtime_mode_not_per_intent_shadow", rg["reason"])
	}
}

func TestU1eReplayGatePendingWithoutEvidence(t *testing.T) {
	fake := &turnRecordingStore{}
	cfg := config.Default()
	cfg.StoreMode = config.StoreModeDualShadow
	srv := NewServer(cfg)
	srv.Store = fake
	mux := http.NewServeMux()
	srv.RegisterRoutes(mux)

	body := `{"chat_session_id":"sess-rg-pend","turn_index":1,"raw_user_input":"hello","settings":{"injection_enabled":true,"input_context_enabled":true,"max_injection_chars":500,"max_input_context_chars":400,"top_k":2}}`
	req := httptest.NewRequest(http.MethodPost, "/prepare-turn", bytes.NewReader([]byte(body)))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200: %s", rec.Code, rec.Body.String())
	}
	var resp map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatal(err)
	}
	rr := resp["recall_result"].(map[string]any)
	contract := rr["intent_contract"].(map[string]any)
	rg, ok := contract["routing_shadow_replay_gate"].(map[string]any)
	if !ok {
		t.Fatal("routing_shadow_replay_gate missing")
	}
	if rg["status"] != "pending" {
		t.Fatalf("status = %v, want pending", rg["status"])
	}
	if rg["decision"] != "hold" {
		t.Fatalf("decision = %v, want hold", rg["decision"])
	}
	if rg["reason"] != "without_evidence" {
		t.Fatalf("reason = %v, want without_evidence", rg["reason"])
	}
}

func TestU1eReplayGateReadyWithPassedEvidence(t *testing.T) {
	fake := &turnRecordingStore{
		returnMemories: []store.Memory{
			{ID: 1, ChatSessionID: "sess-rg-ready", TurnIndex: 2, SummaryJSON: `{"turn_summary":"test"}`, Importance: 0.9},
		},
		returnChatLogs: []store.ChatLog{
			{ID: 1, ChatSessionID: "sess-rg-ready", TurnIndex: 1, Role: "user", Content: "hello"},
		},
	}
	cfg := config.Default()
	cfg.StoreMode = config.StoreModeDualShadow
	srv := NewServer(cfg)
	srv.Store = fake
	mux := http.NewServeMux()
	srv.RegisterRoutes(mux)

	body := `{"chat_session_id":"sess-rg-ready","turn_index":3,"raw_user_input":"hello","settings":{"injection_enabled":true,"input_context_enabled":true,"max_injection_chars":500,"max_input_context_chars":400,"top_k":2}}`
	req := httptest.NewRequest(http.MethodPost, "/prepare-turn", bytes.NewReader([]byte(body)))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200: %s", rec.Code, rec.Body.String())
	}
	var resp map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatal(err)
	}
	rr := resp["recall_result"].(map[string]any)
	contract := rr["intent_contract"].(map[string]any)
	rg, ok := contract["routing_shadow_replay_gate"].(map[string]any)
	if !ok {
		t.Fatal("routing_shadow_replay_gate missing")
	}
	if rg["status"] != "ready" {
		t.Fatalf("status = %v, want ready", rg["status"])
	}
	if rg["decision"] != "promote_candidate" {
		t.Fatalf("decision = %v, want promote_candidate", rg["decision"])
	}
	if rg["reason"] != "passed_evidence" {
		t.Fatalf("reason = %v, want passed_evidence", rg["reason"])
	}
}

func TestU1eReplayGateBlocksShortMidRegression(t *testing.T) {
	fake := &turnRecordingStore{
		returnMemories: []store.Memory{
			{ID: 1, ChatSessionID: "sess-rg-block", TurnIndex: 2, SummaryJSON: `{"turn_summary":"test"}`, Importance: 0.9},
		},
		returnChatLogs: []store.ChatLog{
			{ID: 1, ChatSessionID: "sess-rg-block", TurnIndex: 1, Role: "user", Content: "hello"},
		},
	}
	cfg := config.Default()
	cfg.StoreMode = config.StoreModeDualShadow
	srv := NewServer(cfg)
	srv.Store = fake
	mux := http.NewServeMux()
	srv.RegisterRoutes(mux)

	body := `{"chat_session_id":"sess-rg-block","turn_index":3,"raw_user_input":"hello","settings":{"injection_enabled":true,"input_context_enabled":true,"max_injection_chars":500,"max_input_context_chars":400,"top_k":2},"client_meta":{"context_window_profile":"compact"}}`
	req := httptest.NewRequest(http.MethodPost, "/prepare-turn", bytes.NewReader([]byte(body)))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200: %s", rec.Code, rec.Body.String())
	}
	var resp map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatal(err)
	}
	rr := resp["recall_result"].(map[string]any)
	contract := rr["intent_contract"].(map[string]any)
	rg, ok := contract["routing_shadow_replay_gate"].(map[string]any)
	if !ok {
		t.Fatal("routing_shadow_replay_gate missing")
	}
	if rg["status"] != "ready" {
		t.Fatalf("status = %v, want ready", rg["status"])
	}
	ies := rr["intent_execution_shadow"].(map[string]any)
	ts, ok := ies["temporal_scoring"].(map[string]any)
	if !ok {
		t.Fatal("temporal_scoring missing")
	}
	if ts["status"] != "off" {
		t.Fatalf("temporal_scoring status = %v, want off", ts["status"])
	}
}

// --- S-1e guarded default takeover tests (SEQ-10-P438~P452) ---

func TestS1eGuardedTakeoverContractOffMode(t *testing.T) {
	shadow := buildIntentExecutionShadow([]map[string]any{}, map[string]any{}, "long", q3PacketBudgetPolicy())
	rg, _ := shadow["replay_gate"].(map[string]any)
	if rg["status"] != "off" {
		t.Fatalf("replay_gate status = %v, want off", rg["status"])
	}
}

func TestS1eGuardedTakeoverPendingWhenReplayNotReady(t *testing.T) {
	fake := &turnRecordingStore{
		returnMemories: []store.Memory{
			{ID: 1, ChatSessionID: "sess-to-pend", TurnIndex: 2, SummaryJSON: `{"turn_summary":"test"}`, Importance: 0.9},
		},
	}
	cfg := config.Default()
	cfg.StoreMode = config.StoreModeDualShadow
	srv := NewServer(cfg)
	srv.Store = fake
	mux := http.NewServeMux()
	srv.RegisterRoutes(mux)

	body := `{"chat_session_id":"sess-to-pend","turn_index":3,"raw_user_input":"hello","settings":{"injection_enabled":true,"input_context_enabled":true,"max_injection_chars":500,"max_input_context_chars":400,"top_k":2}}`
	req := httptest.NewRequest(http.MethodPost, "/prepare-turn", bytes.NewReader([]byte(body)))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200: %s", rec.Code, rec.Body.String())
	}
	var resp map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatal(err)
	}
	rr := resp["recall_result"].(map[string]any)
	contract := rr["intent_contract"].(map[string]any)
	rst, ok := contract["routing_shadow_takeover"].(map[string]any)
	if !ok {
		t.Fatal("routing_shadow_takeover missing")
	}
	if rst["version"] != "s1e.v1" {
		t.Fatalf("version = %v, want s1e.v1", rst["version"])
	}
	if rst["status"] != "pending" {
		t.Fatalf("status = %v, want pending", rst["status"])
	}
	if rst["decision"] != "hold" {
		t.Fatalf("decision = %v, want hold", rst["decision"])
	}
	if rst["reason"] != "replay_gate_not_ready" {
		t.Fatalf("reason = %v, want replay_gate_not_ready", rst["reason"])
	}
}

func TestS1eGuardedTakeoverReadyWithReplayGatePass(t *testing.T) {
	fake := &turnRecordingStore{
		returnMemories: []store.Memory{
			{ID: 1, ChatSessionID: "sess-to-ready", TurnIndex: 2, SummaryJSON: `{"turn_summary":"test"}`, Importance: 0.9},
		},
		returnChatLogs: []store.ChatLog{
			{ID: 1, ChatSessionID: "sess-to-ready", TurnIndex: 1, Role: "user", Content: "hello"},
		},
	}
	cfg := config.Default()
	cfg.StoreMode = config.StoreModeDualShadow
	srv := NewServer(cfg)
	srv.Store = fake
	mux := http.NewServeMux()
	srv.RegisterRoutes(mux)

	body := `{"chat_session_id":"sess-to-ready","turn_index":3,"raw_user_input":"hello","settings":{"injection_enabled":true,"input_context_enabled":true,"max_injection_chars":500,"max_input_context_chars":400,"top_k":2}}`
	req := httptest.NewRequest(http.MethodPost, "/prepare-turn", bytes.NewReader([]byte(body)))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200: %s", rec.Code, rec.Body.String())
	}
	var resp map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatal(err)
	}
	rr := resp["recall_result"].(map[string]any)
	contract := rr["intent_contract"].(map[string]any)
	rst, ok := contract["routing_shadow_takeover"].(map[string]any)
	if !ok {
		t.Fatal("routing_shadow_takeover missing")
	}
	if rst["status"] != "ready" {
		t.Fatalf("status = %v, want ready", rst["status"])
	}
	if rst["decision"] != "promote_candidate" {
		t.Fatalf("decision = %v, want promote_candidate", rst["decision"])
	}
	if rst["reason"] != "guarded_takeover_gate_passed" {
		t.Fatalf("reason = %v, want guarded_takeover_gate_passed", rst["reason"])
	}
}

func TestS1eGuardedTakeoverBlocksWithoutShadowCandidates(t *testing.T) {
	fake := &turnRecordingStore{
		returnChatLogs: []store.ChatLog{
			{ID: 1, ChatSessionID: "sess-to-block", TurnIndex: 1, Role: "user", Content: "hello"},
		},
	}
	cfg := config.Default()
	cfg.StoreMode = config.StoreModeDualShadow
	srv := NewServer(cfg)
	srv.Store = fake
	mux := http.NewServeMux()
	srv.RegisterRoutes(mux)

	body := `{"chat_session_id":"sess-to-block","turn_index":2,"raw_user_input":"hello","settings":{"injection_enabled":true,"input_context_enabled":true,"max_injection_chars":500,"max_input_context_chars":400,"top_k":2}}`
	req := httptest.NewRequest(http.MethodPost, "/prepare-turn", bytes.NewReader([]byte(body)))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200: %s", rec.Code, rec.Body.String())
	}
	var resp map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatal(err)
	}
	rr := resp["recall_result"].(map[string]any)
	contract := rr["intent_contract"].(map[string]any)
	rst, ok := contract["routing_shadow_takeover"].(map[string]any)
	if !ok {
		t.Fatal("routing_shadow_takeover missing")
	}
	if rst["status"] != "pending" {
		t.Fatalf("status = %v, want pending", rst["status"])
	}
	if rst["decision"] != "hold" {
		t.Fatalf("decision = %v, want hold", rst["decision"])
	}
	if rst["reason"] != "no_shadow_candidates" {
		t.Fatalf("reason = %v, want no_shadow_candidates", rst["reason"])
	}
}

// TestCompleteTurnConflictResolutionAndRetentionInTrace verifies EA-1h/EA-1i/EA-1l
// policy helpers without relying on a full live provider replay.
func TestCompleteTurnConflictResolutionAndRetentionInTrace(t *testing.T) {
	lineage := parseJSONMap(`{"conflict_class":"hard_contradiction","confidence":0.72,"field_class":"identity","high_impact":true,"importance_tier":"critical"}`)
	decision := directEvidenceConflictResolution(
		store.DirectEvidence{ID: 1, EvidenceKind: "turn_excerpt", EvidenceText: "Alice no longer trusts Bob.", CaptureVerification: "verified"},
		lineage,
		"verified_direct",
		"verified",
		"finalize",
		"critical",
	)
	if decision["policy_version"] != "ea1h.v1" || decision["confidence_policy_version"] != "ea1i.v1" {
		t.Fatalf("conflict policy versions mismatch: %#v", decision)
	}
	if decision["classification"] != "hard_contradiction" || decision["route"] != "manual_review" || decision["requires_manual_review"] != true {
		t.Fatalf("conflict decision = %#v, want hard_contradiction/manual_review", decision)
	}
	contract := directEvidenceStateContract()
	if contract["retention_policy_version"] != "ea1l.v1" {
		t.Fatalf("retention policy version = %v, want ea1l.v1", contract["retention_policy_version"])
	}
	windows, ok := contract["retention_windows_turns"].(map[string]any)
	if !ok || windows["direct_evidence"] == nil || windows["previous_archive"] == nil || windows["tombstone"] == nil {
		t.Fatalf("retention windows missing: %#v", contract)
	}
}

// TestCompleteTurnRelationshipV2SurvivesCanonicalPromotion (P452 HS-1g, P502 HS-1j).
// V2 additive fields must survive save and canonical promotion with provenance.
func TestCompleteTurnRelationshipV2SurvivesCanonicalPromotion(t *testing.T) {
	fake := &turnRecordingStore{}
	srv := NewServer(config.Default())
	srv.Store = fake
	srv.StoreOpenError = nil

	extraction := normalizeCriticExtraction(map[string]any{
		"turn_summary":     "Elena reveals her hidden fear to Kael under moonlight.",
		"importance_score": 8,
		"relationship_memory": map[string]any{
			"bond_and_distance": "Elena trusts Kael deeply after the ritual.",
			"trust":             0.92,
			"confidence":        0.88,
			"identity":          map[string]any{"self_concept": "protector"},
			"core_state":        map[string]any{"affection": 0.9, "tension": 0.2},
			"dynamics":          map[string]any{"power_balance": "equal"},
			"context":           map[string]any{"setting": "moonlit garden"},
			"history":           map[string]any{"first_meeting": "archive hall"},
			"verification":      map[string]any{"source": "critic_v2"},
			"desire":            map[string]any{"stated": "protect Kael"},
			"fear":              map[string]any{"revealed": "losing Kael"},
			"wound":             map[string]any{"old": "betrayed by mentor"},
			"mask":              map[string]any{"public": "aloof scholar"},
			"bond":              map[string]any{"type": "trust"},
			"fixation":          map[string]any{"topic": "ancient seals"},
		},
	})

	result := srv.saveCriticExtractionArtifacts(context.Background(), "sess-v2", 15, extraction, "Elena trusts Kael.", completeTurnEmbeddingConfig{}, time.Unix(1500, 0))
	if result.CanonicalStateLayers != 1 || len(fake.savedCanonicalLayers) != 1 {
		t.Fatalf("expected 1 canonical layer, got result=%d store=%d", result.CanonicalStateLayers, len(fake.savedCanonicalLayers))
	}
	cl := fake.savedCanonicalLayers[0]
	if cl.LayerType != "relationship_state" {
		t.Fatalf("layer type = %q, want relationship_state", cl.LayerType)
	}
	if cl.SourceTurn != 15 || cl.LastVerifiedTurn != 15 || cl.Confidence < 0.7 {
		t.Fatalf("provenance not preserved: %#v", cl)
	}

	var content map[string]any
	if err := json.Unmarshal([]byte(cl.Content), &content); err != nil {
		t.Fatalf("content decode: %v", err)
	}
	for _, key := range []string{"identity", "core_state", "dynamics", "context", "history", "verification", "desire", "fear", "wound", "mask", "bond", "fixation"} {
		if _, ok := content[key]; !ok {
			t.Fatalf("missing v2 field %q in content", key)
		}
	}
	if content["bond_and_distance"] != "Elena trusts Kael deeply after the ritual." {
		t.Fatalf("v1 bond_and_distance lost: %v", content["bond_and_distance"])
	}
}

// TestCompleteTurnRelationshipV1BackfillDefaults (P518 HS-1k).
// Missing v2 sections should receive safe minimal defaults without destructive rewrite.
func TestCompleteTurnRelationshipV1BackfillDefaults(t *testing.T) {
	fake := &turnRecordingStore{}
	srv := NewServer(config.Default())
	srv.Store = fake
	srv.StoreOpenError = nil

	extraction := normalizeCriticExtraction(map[string]any{
		"turn_summary":     "Old v1 payload without v2 sections.",
		"importance_score": 5,
		"relationship_memory": map[string]any{
			"bond_and_distance": "Mina tolerates Rowan.",
			"trust":             0.6,
			"confidence":        0.85,
			"verification":      "verified",
		},
	})

	result := srv.saveCriticExtractionArtifacts(context.Background(), "sess-v1", 10, extraction, "Mina tolerates Rowan.", completeTurnEmbeddingConfig{}, time.Unix(1000, 0))
	if result.CanonicalStateLayers != 1 || len(fake.savedCanonicalLayers) != 1 {
		t.Fatalf("expected 1 canonical layer, got result=%d store=%d", result.CanonicalStateLayers, len(fake.savedCanonicalLayers))
	}
	cl := fake.savedCanonicalLayers[0]
	var content map[string]any
	if err := json.Unmarshal([]byte(cl.Content), &content); err != nil {
		t.Fatalf("content decode: %v", err)
	}
	if content["bond_and_distance"] != "Mina tolerates Rowan." {
		t.Fatalf("v1 bond_and_distance lost: %v", content["bond_and_distance"])
	}
	if content["trust"] != 0.6 {
		t.Fatalf("v1 trust lost: %v", content["trust"])
	}
	for _, key := range []string{"identity", "core_state", "dynamics", "context", "history", "desire", "fear", "wound", "mask", "bond", "fixation"} {
		v, ok := content[key]
		if !ok {
			t.Fatalf("missing v2 default field %q", key)
		}
		m, isMap := v.(map[string]any)
		if !isMap || len(m) != 0 {
			t.Fatalf("v2 default field %q should be empty map, got %v", key, v)
		}
	}
}

// TestCompleteTurnWorldStatePromotesWhenVerified (P469 HS-1h).
// World rules / faction status / region pressure should promote to canonical world_state.
func TestCompleteTurnWorldStatePromotesWhenVerified(t *testing.T) {
	fake := &turnRecordingStore{}
	srv := NewServer(config.Default())
	srv.Store = fake
	srv.StoreOpenError = nil

	extraction := normalizeCriticExtraction(map[string]any{
		"turn_summary":     "The Northern faction gains influence after the treaty.",
		"importance_score": 7,
		"world_rules": []any{
			map[string]any{"key": "faction_north", "value": "ascendant", "scope": "global"},
			map[string]any{"key": "region_pressure", "value": "high", "scope": "borderlands"},
		},
		"faction_status":    map[string]any{"north": "rising", "south": "stable"},
		"region_pressure":   map[string]any{"borderlands": 0.8},
		"offscreen_threads": []any{map[string]any{"title": "spy network", "status": "active"}},
	})

	result := srv.saveCriticExtractionArtifacts(context.Background(), "sess-world", 20, extraction, "Northern faction ascendant.", completeTurnEmbeddingConfig{}, time.Unix(2000, 0))
	if result.ActiveStates != 1 {
		t.Fatalf("active states saved = %d, want 1", result.ActiveStates)
	}
	if result.CanonicalStateLayers != 1 || len(fake.savedCanonicalLayers) != 1 {
		t.Fatalf("expected 1 canonical layer, got result=%d store=%d", result.CanonicalStateLayers, len(fake.savedCanonicalLayers))
	}
	cl := fake.savedCanonicalLayers[0]
	if cl.LayerType != "world_state" {
		t.Fatalf("layer type = %q, want world_state", cl.LayerType)
	}
	if cl.SourceTurn != 20 || cl.LastVerifiedTurn != 20 || cl.Confidence < 0.7 {
		t.Fatalf("provenance not preserved: %#v", cl)
	}
	var content map[string]any
	if err := json.Unmarshal([]byte(cl.Content), &content); err != nil {
		t.Fatalf("content decode: %v", err)
	}
	rules, ok := content["rules"].([]any)
	if !ok || len(rules) != 2 {
		t.Fatalf("expected 2 rules, got %v", content["rules"])
	}
	if content["version"] != "world_state.v1" {
		t.Fatalf("version mismatch: %v", content["version"])
	}
	if _, ok := content["faction_status"]; !ok {
		t.Fatalf("faction_status missing")
	}
	if _, ok := content["region_pressure"]; !ok {
		t.Fatalf("region_pressure missing")
	}
	if _, ok := content["offscreen_threads"]; !ok {
		t.Fatalf("offscreen_threads missing")
	}
}

func TestCompleteTurnWorldStateRulesAlsoSaveWorldRules(t *testing.T) {
	fake := &turnRecordingStore{}
	srv := NewServer(config.Default())
	srv.Store = fake
	srv.StoreOpenError = nil

	extraction := normalizeCriticExtraction(map[string]any{
		"turn_summary":     "The heated beam can crack under river ice shock.",
		"importance_score": 7,
		"world_state": map[string]any{
			"version":      "world_state.v1",
			"confidence":   0.9,
			"verification": "verified",
			"rules": []any{
				map[string]any{"category": "setting", "key": "ice_wedge_effect", "scope": "session", "scope_name": "Demolition Logic", "value": "Superheated steel can fracture when shocked with freezing river water."},
			},
		},
	})

	result := srv.saveCriticExtractionArtifacts(context.Background(), "sess-world-rules", 22, extraction, "The team confirms the ice wedge demolition logic.", completeTurnEmbeddingConfig{}, time.Unix(2200, 0))
	if result.WorldRules != 1 || len(fake.savedWorldRules) != 1 {
		t.Fatalf("world_state.rules should save 1 world rule, got result=%d store=%d", result.WorldRules, len(fake.savedWorldRules))
	}
	saved := fake.savedWorldRules[0]
	if saved.Key != "ice_wedge_effect" || saved.Category != "setting" || saved.ScopeName != "Demolition Logic" {
		t.Fatalf("world rule fields not preserved: %+v", saved)
	}
	if result.CanonicalStateLayers != 1 || len(fake.savedCanonicalLayers) != 1 {
		t.Fatalf("world_state should still promote to canonical layer, got result=%d store=%d", result.CanonicalStateLayers, len(fake.savedCanonicalLayers))
	}
}

// TestCompleteTurnWorldStateLowConfidenceBlocked (P469 HS-1h filtering).
// Low-confidence or unverified world state must not promote to canonical.
func TestCompleteTurnWorldStateLowConfidenceBlocked(t *testing.T) {
	fake := &turnRecordingStore{}
	srv := NewServer(config.Default())
	srv.Store = fake
	srv.StoreOpenError = nil

	extraction := normalizeCriticExtraction(map[string]any{
		"turn_summary":     "Rumors about world changes remain unverified.",
		"importance_score": 4,
		"world_rules": []any{
			map[string]any{"key": "rumor", "value": "maybe true", "scope": "global"},
		},
		"world_state": map[string]any{
			"confidence":   0.4,
			"verification": "pending",
		},
	})

	result := srv.saveCriticExtractionArtifacts(context.Background(), "sess-world-low", 21, extraction, "Rumors.", completeTurnEmbeddingConfig{}, time.Unix(2100, 0))
	if result.ActiveStates != 1 {
		t.Fatalf("active states saved = %d, want 1", result.ActiveStates)
	}
	if result.CanonicalStateLayers != 0 || len(fake.savedCanonicalLayers) != 0 {
		t.Fatalf("low-confidence/unverified world state should not promote, got result=%d layers=%#v", result.CanonicalStateLayers, fake.savedCanonicalLayers)
	}
}

// TestPrepareTurnRelationshipAndWorldStateTraceSurface (P486 HS-1i).
// Prepare-turn traces must expose counts/flags proving relationship/world current state
// was saved, read, and injected into the canon text.
func TestPrepareTurnRelationshipAndWorldStateTraceSurface(t *testing.T) {
	fake := &turnRecordingStore{
		returnCanonicalLayers: []store.CanonicalStateLayer{
			{ID: 1, ChatSessionID: "sess-trace", LayerType: "relationship_state", Content: `{"bond_and_distance":"Mina trusts Rowan.","trust":0.9}`, SourceStateType: "relationship_state", TurnIndex: 30, SourceTurn: 30, LastVerifiedTurn: 30, Confidence: 0.92},
			{ID: 2, ChatSessionID: "sess-trace", LayerType: "world_state", Content: `{"rules":[{"key":"faction_north","value":"rising"}],"version":"world_state.v1","faction_status":{"north":"rising"}}`, SourceStateType: "world_state", TurnIndex: 30, SourceTurn: 30, LastVerifiedTurn: 30, Confidence: 0.88},
			{ID: 3, ChatSessionID: "sess-trace", LayerType: "world_state", Content: `{"rules":[],"confidence":0.4,"verification":"pending"}`, SourceStateType: "world_state", TurnIndex: 25, SourceTurn: 25, LastVerifiedTurn: 25, Confidence: 0.4},
		},
		returnChatLogs: []store.ChatLog{{ID: 1, ChatSessionID: "sess-trace", TurnIndex: 31, Role: "assistant", Content: "They pause."}},
	}
	cfg := config.Default()
	cfg.StoreMode = config.StoreModeDualShadow
	srv := NewServer(cfg)
	srv.Store = fake

	mux := http.NewServeMux()
	srv.RegisterRoutes(mux)

	body := `{"chat_session_id":"sess-trace","turn_index":32,"raw_user_input":"Continue","settings":{"max_injection_chars":900,"max_input_context_chars":300,"injection_enabled":true,"input_context_enabled":true}}`
	req := httptest.NewRequest(http.MethodPost, "/prepare-turn", bytes.NewReader([]byte(body)))
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
	injectionPack, ok := resp["injection_pack"].(map[string]any)
	if !ok {
		t.Fatalf("injection_pack is not an object")
	}
	canonText, _ := injectionPack["canon_text"].(string)
	if !strings.Contains(canonText, "relationship_state") {
		t.Fatalf("canon_text missing relationship_state: %q", canonText)
	}
	if !strings.Contains(canonText, "world_state") {
		t.Fatalf("canon_text missing world_state: %q", canonText)
	}
	counts, ok := injectionPack["counts"].(map[string]any)
	if !ok {
		t.Fatalf("counts is not an object")
	}
	if counts["canonical_state_relationship_layers_count"] != float64(1) {
		t.Fatalf("canonical_state_relationship_layers_count = %v, want 1", counts["canonical_state_relationship_layers_count"])
	}
	if counts["canonical_state_world_layers_count"] != float64(1) {
		t.Fatalf("canonical_state_world_layers_count = %v, want 1", counts["canonical_state_world_layers_count"])
	}
	if counts["canonical_state_layers_filtered_count"] != float64(1) {
		t.Fatalf("canonical_state_layers_filtered_count = %v, want 1", counts["canonical_state_layers_filtered_count"])
	}
	bd, ok := injectionPack["budget_decisions"].(map[string]any)
	if !ok {
		t.Fatalf("budget_decisions is not an object")
	}
	if bd["canonical_state_hard_floor_enabled"] != true {
		t.Fatalf("canonical_state_hard_floor_enabled = %v, want true", bd["canonical_state_hard_floor_enabled"])
	}
}

func TestPrepareTurnTM1aCanonicalConsistencyRecallDocumentsSurface(t *testing.T) {
	fake := &turnRecordingStore{
		returnMemories: []store.Memory{
			{ID: 1, ChatSessionID: "sess-tm1a", TurnIndex: 2, SummaryJSON: `{"turn_summary":"A quiet scene in the archive room"}`, Importance: 0.9, PlaceWing: "North Wing", PlaceRoom: "Scene Room"},
		},
		returnCharStates: []store.CharacterState{
			{ID: 1, ChatSessionID: "sess-tm1a", CharacterName: "Mina", RelationshipsJSON: `{"Rowan":"trusts"}`},
		},
		returnPendingThreads: []store.PendingThread{
			{ID: 1, ChatSessionID: "sess-tm1a", ThreadKey: "thread_rooftop", Description: "Mina must answer Rowan about the plan", Status: "open", Title: "Rooftop plan", Owner: "Rowan", Target: "Mina"},
		},
		returnChatLogs: []store.ChatLog{
			{ID: 1, ChatSessionID: "sess-tm1a", TurnIndex: 3, Role: "user", Content: "What happens next?"},
		},
		returnResumePack: &store.ResumePack{
			PackStatus:    "ready",
			AssembledText: "Session resume: Mina and Rowan are in the archive.",
		},
		returnStorylines: []store.Storyline{
			{ID: 1, ChatSessionID: "sess-tm1a", Name: "Archive Plan", CurrentContext: "Mina and Rowan plan in the archive"},
		},
		returnActiveStates: []store.ActiveState{
			{ID: 1, ChatSessionID: "sess-tm1a", StateType: "location", Content: "Archive room"},
		},
	}

	cfg := config.Default()
	cfg.StoreMode = config.StoreModeDualShadow
	srv := NewServer(cfg)
	srv.Store = fake

	mux := http.NewServeMux()
	srv.RegisterRoutes(mux)

	body := `{"chat_session_id":"sess-tm1a","turn_index":4,"raw_user_input":"Continue","settings":{"max_injection_chars":900,"max_input_context_chars":400,"injection_enabled":true,"input_context_enabled":true,"top_k":2}}`
	req := httptest.NewRequest(http.MethodPost, "/prepare-turn", bytes.NewReader([]byte(body)))
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

	// 1. injection_text contains relationship text from RelationshipsJSON
	injectionText, _ := resp["injection_text"].(string)
	if !strings.Contains(injectionText, "trusts") {
		t.Fatalf("injection_text missing relationship text: %q", injectionText)
	}

	// 2. session_state.pending_threads contains open status
	sessionState, ok := resp["session_state"].(map[string]any)
	if !ok {
		t.Fatalf("session_state is not an object")
	}
	threads, ok := sessionState["pending_threads"].([]any)
	if !ok || len(threads) == 0 {
		t.Fatalf("session_state.pending_threads missing or empty: %#v", sessionState["pending_threads"])
	}
	firstThread, ok := threads[0].(map[string]any)
	if !ok {
		t.Fatalf("first pending thread is not an object")
	}
	if firstThread["status"] != "open" {
		t.Fatalf("pending thread status = %v, want open", firstThread["status"])
	}

	// 3. recall_result.documents contains memory with place_wing/place_room
	recallResult, ok := resp["recall_result"].(map[string]any)
	if !ok {
		t.Fatalf("recall_result is not an object")
	}
	documents, ok := recallResult["documents"].([]any)
	if !ok || len(documents) == 0 {
		t.Fatalf("recall_result.documents missing or empty: %#v", recallResult["documents"])
	}
	foundMemoryDoc := false
	for _, d := range documents {
		doc, ok := d.(map[string]any)
		if !ok {
			continue
		}
		if doc["source_type"] != "memory" {
			continue
		}
		meta, ok := doc["metadata"].(map[string]any)
		if !ok {
			t.Fatalf("memory document missing metadata: %#v", doc)
		}
		if meta["place_wing"] != "North Wing" {
			t.Fatalf("memory document place_wing = %v, want North Wing", meta["place_wing"])
		}
		if meta["place_room"] != "Scene Room" {
			t.Fatalf("memory document place_room = %v, want Scene Room", meta["place_room"])
		}
		foundMemoryDoc = true
		break
	}
	if !foundMemoryDoc {
		t.Fatalf("no memory document with place_wing/place_room found in recall_result.documents")
	}

	// 4. injection_pack.would_write = false (read-only verification)
	injectionPack, ok := resp["injection_pack"].(map[string]any)
	if !ok {
		t.Fatalf("injection_pack is not an object")
	}
	if injectionPack["would_write"] != false {
		t.Fatalf("injection_pack.would_write = %v, want false", injectionPack["would_write"])
	}

	// 5. recall_result.would_write = false
	if recallResult["would_write"] != false {
		t.Fatalf("recall_result.would_write = %v, want false", recallResult["would_write"])
	}
}

// --- SEQ-12.3-P83: long-memory promotion candidate markers ---

func TestSeq123P83LongMemoryPromotionCandidateMarkers(t *testing.T) {
	t.Run("critic_prompt_has_durable_extraction_markers", func(t *testing.T) {
		prompt := buildCompleteTurnCriticPrompt(
			"sess-p83", 3,
			"Mina found the brass key.",
			"Rowan nodded and followed.",
			nil, nil, nil,
		)
		required := []string{
			"Extract durable Archive Center memory data",
			"[User]",
			"[Assistant]",
			"<Latest_Turn>",
			"Omit unknown facts instead of inventing placeholders",
			"evidence_excerpts must be short exact excerpts",
			"persona_capsule_candidates",
			"support_only_persona_recollection",
			"requires later user/operator approval",
			"system/progression stories",
			"randomized or conditional acquisition",
			"challenge entry/clear/reward loops",
			"exchange/cost economy",
			"upgrade or unlock rules",
			"Mandatory world-rule audit",
			"world_rule_audit",
			"world_rules must not be empty",
			"1-7 turn session",
			"progression currency exchange",
			"challenge reward loops",
			"abstract invariant",
		}
		for _, needle := range required {
			if !strings.Contains(prompt, needle) {
				t.Fatalf("critic prompt missing P83 marker %q", needle)
			}
		}
	})

	t.Run("critic_prompt_includes_language_memory_contract", func(t *testing.T) {
		languageContext := map[string]any{
			"contract_version":        "language_memory.v1",
			"session_output_language": "en",
			"output_language_source":  "explicit_override",
			"raw_user_language":       "ko",
			"summary_language":        "en",
			"search_text_policy":      "summary_plus_raw_plus_aliases",
			"locked_for_turn":         true,
			"raw_evidence_rewritten":  true,
		}
		prompt := buildCompleteTurnCriticPromptWithLanguageContext(
			"sess-p83-lang", 4,
			"RAW-KO: Mina found the brass key.",
			"Mina found the brass key.",
			nil, nil, nil, languageContext,
		)
		for _, needle := range []string{
			"Language_Context_JSON",
			"Generated summaries and display/support fields should use summary_language/session_output_language",
			"Raw evidence excerpts must stay exact source text",
			"\"session_output_language\":\"en\"",
			"\"raw_evidence_rewritten\":false",
		} {
			if !strings.Contains(prompt, needle) {
				t.Fatalf("critic prompt missing language contract marker %q: %s", needle, prompt)
			}
		}
	})

	t.Run("non_empty_turn_summary_saves_canonical_memory", func(t *testing.T) {
		fake := &turnRecordingStore{}
		srv := NewServer(config.Default())
		srv.Store = fake
		extraction := map[string]any{
			"turn_summary":     "Mina found the brass key.",
			"importance_score": 7,
		}
		result := srv.saveCriticExtractionArtifacts(context.Background(), "sess-p83", 5, extraction, "Mina found the brass key.", completeTurnEmbeddingConfig{}, time.Unix(500, 0))
		if result.Memories != 1 {
			t.Fatalf("expected 1 memory save, got %d", result.Memories)
		}
		if len(fake.savedMemories) != 1 {
			t.Fatalf("expected 1 saved memory, got %d", len(fake.savedMemories))
		}
		mem := fake.savedMemories[0]
		if mem.ChatSessionID != "sess-p83" {
			t.Fatalf("memory ChatSessionID = %q, want sess-p83", mem.ChatSessionID)
		}
		if mem.TurnIndex != 5 {
			t.Fatalf("memory TurnIndex = %d, want 5", mem.TurnIndex)
		}
		if !strings.Contains(mem.SummaryJSON, "turn_summary") {
			t.Fatalf("memory SummaryJSON missing turn_summary: %s", mem.SummaryJSON)
		}
		if mem.Importance <= 0 {
			t.Fatalf("memory Importance = %v, want > 0", mem.Importance)
		}
		if result.EmbeddingStatus != "missing_config" {
			t.Fatalf("EmbeddingStatus = %q, want missing_config when no embed config", result.EmbeddingStatus)
		}
		if result.VectorStatus != "missing_embedding" {
			t.Fatalf("VectorStatus = %q, want missing_embedding when no embed config", result.VectorStatus)
		}
	})

	t.Run("memory_write_contract_persists_language_context_without_rewriting_raw_evidence", func(t *testing.T) {
		fake := &turnRecordingStore{}
		srv := NewServer(config.Default())
		srv.Store = fake
		extraction := map[string]any{
			"turn_summary":      "Mina found the brass key.",
			"importance_score":  7,
			"evidence_excerpts": []any{"RAW-KO: Mina found the brass key."},
			"language_context": map[string]any{
				"contract_version":        "language_memory.v1",
				"session_output_language": "en",
				"output_language_source":  "explicit_override",
				"raw_user_language":       "ko",
				"summary_language":        "en",
				"search_text_policy":      "summary_plus_raw_plus_aliases",
				"locked_for_turn":         true,
			},
		}
		content := "RAW-KO: Mina found the brass key.\nMina found the brass key."
		result := srv.saveCriticExtractionArtifacts(context.Background(), "sess-p83-lang", 6, extraction, content, completeTurnEmbeddingConfig{}, time.Unix(600, 0))
		if result.Memories != 1 || result.Evidence != 1 {
			t.Fatalf("expected memory and evidence saves, got result=%#v", result)
		}
		var summary map[string]any
		if err := json.Unmarshal([]byte(fake.savedMemories[0].SummaryJSON), &summary); err != nil {
			t.Fatalf("decode SummaryJSON: %v", err)
		}
		lang, _ := summary["language_context"].(map[string]any)
		if lang["session_output_language"] != "en" || lang["raw_user_language"] != "ko" {
			t.Fatalf("memory language_context = %#v", lang)
		}
		contract, _ := summary["memory_write_contract"].(map[string]any)
		if contract["raw_evidence_rewritten"] != false || contract["summary_language"] != "en" {
			t.Fatalf("memory write contract = %#v", contract)
		}
		ev := fake.savedEvidence[0]
		if ev.EvidenceText != "RAW-KO: Mina found the brass key." {
			t.Fatalf("raw evidence was rewritten: %q", ev.EvidenceText)
		}
		var lineage map[string]any
		if err := json.Unmarshal([]byte(ev.LineageJSON), &lineage); err != nil {
			t.Fatalf("decode LineageJSON: %v", err)
		}
		if lineage["lane"] != "raw_evidence" || lineage["raw_evidence_rewritten"] != false {
			t.Fatalf("evidence lineage missing raw evidence contract: %#v", lineage)
		}
	})

	t.Run("chromadb_memory_document_uses_cross_language_search_text", func(t *testing.T) {
		vec := &turnRecordingVectorStore{}
		cfg := config.Default()
		cfg.ChromaEndpoint = "http://chroma.test"
		srv := NewServer(cfg)
		srv.Vector = vec
		extraction := map[string]any{
			"turn_summary":      "Mina found the brass key.",
			"importance_score":  7,
			"evidence_excerpts": []any{"RAW-KO: Mina found the brass key."},
			"archive_hint": map[string]any{
				"wing": "North Wing",
				"room": "Scene Room",
			},
			"entities": []any{
				map[string]any{
					"name":    "Mina",
					"aliases": []any{"미나"},
				},
				map[string]any{"name": "Rowan"},
			},
			"kg_triples": []any{
				map[string]any{"subject": "Mina", "predicate": "found", "object": "brass key"},
			},
			"language_context": map[string]any{
				"contract_version":        "language_memory.v1",
				"session_output_language": "en",
				"output_language_source":  "explicit_override",
				"raw_user_language":       "ko",
				"summary_language":        "en",
				"search_text_policy":      "summary_plus_raw_plus_aliases",
				"locked_for_turn":         true,
			},
		}
		extraction = applyLanguageMemoryWriteContract(extraction, completeTurnLanguageContextFromExtraction(extraction))
		mem := &store.Memory{
			ID:            42,
			ChatSessionID: "sess-p83-lang-vector",
			TurnIndex:     6,
			SummaryJSON:   mustCompactJSON(extraction),
			Evidence:      mustCompactJSON(map[string]any{"evidence_excerpts": []string{"RAW-KO: Mina found the brass key."}}),
			PlaceWing:     "North Wing",
			PlaceRoom:     "Scene Room",
		}
		searchText := memorySearchTextFromMemory(*mem)
		result := artifactSaveResult{VectorStatus: "not_requested"}
		srv.upsertMemoryVector(context.Background(), "sess-p83-lang-vector", 6, mem, searchText.Text, []float32{0.1, 0.2}, &result)
		if result.VectorStatus != "ok" || result.VectorsUpserted != 1 {
			t.Fatalf("vector upsert result = %#v", result)
		}
		if len(vec.docs) != 1 {
			t.Fatalf("expected 1 vector doc, got %d", len(vec.docs))
		}
		doc := vec.docs[0]
		for _, needle := range []string{
			"[Canonical Summary]\nMina found the brass key.",
			"[Raw Evidence]\nRAW-KO: Mina found the brass key.",
			"[Aliases]",
			"미나",
			"North Wing",
			"brass key",
		} {
			if !strings.Contains(doc.DocumentText, needle) {
				t.Fatalf("vector document missing %q: %s", needle, doc.DocumentText)
			}
		}
		if doc.SearchTextPolicy != "summary_plus_raw_plus_aliases" || doc.RawLanguage != "ko" || doc.SummaryLanguage != "en" || doc.SessionOutputLanguage != "en" {
			t.Fatalf("vector language metadata = %#v", doc)
		}
		if doc.AliasCount == 0 {
			t.Fatalf("expected aliases to be indexed, doc=%#v", doc)
		}
		if doc.SchemaVersion != "memory.v2" {
			t.Fatalf("schema version = %q, want memory.v2", doc.SchemaVersion)
		}
	})

	t.Run("prepare_turn_language_aware_injection_uses_output_summary_and_preserves_raw_evidence", func(t *testing.T) {
		languageContext := map[string]any{
			"contract_version":        "language_memory.v1",
			"session_output_language": "en",
			"output_language_source":  "explicit_override",
			"raw_user_language":       "ko",
			"summary_language":        "en",
			"search_text_policy":      "summary_plus_raw_plus_aliases",
			"locked_for_turn":         true,
		}
		memories := []store.Memory{{
			ID:          77,
			TurnIndex:   4,
			SummaryJSON: `{"turn_summary":"Mina hid the brass key under the old shrine.","language_context":{"contract_version":"language_memory.v1","session_output_language":"en","raw_user_language":"ko","summary_language":"en","search_text_policy":"summary_plus_raw_plus_aliases"}}`,
			Evidence:    `{"evidence_excerpts":["미나는 오래된 사당 아래에 황동 열쇠를 숨겼다."]}`,
			Importance:  0.9,
		}}
		assembly := buildPrepareTurnInjectionAssembly(memories, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, 1, 2000, "Where is the brass key?", "default", nil, nil, languageContext)
		if !strings.Contains(assembly.MemoryText, "Mina hid the brass key under the old shrine.") {
			t.Fatalf("memory text missing output-language summary: %q", assembly.MemoryText)
		}
		if !strings.Contains(assembly.MemoryText, "raw_evidence: 미나는 오래된 사당 아래에 황동 열쇠를 숨겼다.") {
			t.Fatalf("memory text missing raw evidence: %q", assembly.MemoryText)
		}
		if !strings.Contains(assembly.MemoryText, "summary_language=en") || !strings.Contains(assembly.MemoryText, "raw_language=ko") {
			t.Fatalf("memory text missing language markers: %q", assembly.MemoryText)
		}
		if assembly.LanguageInjectionTrace["status"] != "ready" {
			t.Fatalf("language injection status = %#v", assembly.LanguageInjectionTrace)
		}
		memTrace, _ := assembly.LanguageInjectionTrace["memory_language_trace"].(map[string]any)
		if intFromAny(memTrace["memory_summary_language_match"], 0) != 1 || intFromAny(memTrace["raw_evidence_attached_count"], 0) != 1 {
			t.Fatalf("memory language trace = %#v", memTrace)
		}
	})

	t.Run("prepare_turn_planner_support_carries_session_output_language_contract", func(t *testing.T) {
		languageContext := map[string]any{
			"contract_version":        "language_memory.v1",
			"session_output_language": "ja",
			"output_language_source":  "plugin_setting",
			"raw_user_language":       "ko",
			"summary_language":        "ja",
			"locked_for_turn":         true,
		}
		pack := buildSupervisorInputPack(
			"sess-planner-lang",
			8,
			"다음 장면으로 이어가줘.",
			"standard",
			"weak",
			"balanced",
			"none",
			"",
			map[string]any{"prompt_source": "test"},
			map[string]any{"memory_count": 1},
			nil,
			storylineSupervisorSelection{},
			false,
			"",
			languageContext,
		)
		contract, _ := pack["planner_language_contract"].(map[string]any)
		if contract["planner_support_language"] != "ja" || contract["current_user_input_priority"] != "highest" {
			t.Fatalf("planner language contract = %#v", contract)
		}
		if contract["raw_user_input_rewritten"] != false || contract["raw_evidence_rewritten"] != false {
			t.Fatalf("planner contract rewrote raw lanes: %#v", contract)
		}
	})

	t.Run("protected_secret_contract_derives_owner_scoped_subjective_memory", func(t *testing.T) {
		extraction := normalizeCriticExtraction(map[string]any{
			"turn_summary":     "A private feeling is established.",
			"importance_score": 6,
			"protected_secrets": []any{
				map[string]any{
					"secret_kind":       "romantic_feeling",
					"owner":             "Mina",
					"subject":           []any{"Rowan"},
					"summary":           "Mina privately likes Rowan but has not revealed it.",
					"sensitivity":       "medium",
					"disclosure_policy": "owner_private_until_revealed",
					"knowledge_scope": map[string]any{
						"known_by":   []any{"Mina"},
						"unknown_to": []any{"Rowan"},
					},
				},
			},
		})
		secrets := sliceFromAny(extraction["protected_secrets"])
		if len(secrets) != 1 {
			t.Fatalf("protected secrets not normalized: %#v", extraction["protected_secrets"])
		}
		memories := sliceFromAny(extraction["subjective_entity_memories"])
		if len(memories) != 1 {
			t.Fatalf("protected secret should derive one subjective memory, got %#v", memories)
		}
		mem := mapFromAny(memories[0])
		if mem["secret_guard"] != true || mem["owner_visibility"] != "owner_private" || mem["target_reveal_policy"] != "owner_private_until_revealed" {
			t.Fatalf("derived protected subjective memory is not guarded: %#v", mem)
		}
		tags := stringsFromAny(mem["tags"])
		for _, want := range []string{"protected_secret", "secret_guard", "protected_secret_kind:romantic_feeling"} {
			if !containsStringFold(tags, want) {
				t.Fatalf("derived tags missing %q: %#v", want, tags)
			}
		}
	})

	t.Run("protected_secret_memory_injection_masks_secret_content", func(t *testing.T) {
		memories := []store.Memory{{
			ID:        88,
			TurnIndex: 5,
			SummaryJSON: mustCompactJSON(map[string]any{
				"turn_summary": "Mina privately likes Rowan but has not revealed it.",
				"protected_secrets": []any{
					map[string]any{
						"secret_kind":       "romantic_feeling",
						"owner":             "Mina",
						"summary":           "Mina privately likes Rowan but has not revealed it.",
						"disclosure_policy": "owner_private_until_revealed",
						"knowledge_scope": map[string]any{
							"known_by":   []string{"Mina"},
							"unknown_to": []string{"Rowan"},
						},
					},
				},
			}),
			Importance: 0.9,
		}}
		assembly := buildPrepareTurnInjectionAssembly(memories, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, 1, 2000, "Mina looks away.", "default", nil, nil, nil)
		if !strings.Contains(assembly.MemoryText, "Protected continuity guard") {
			t.Fatalf("protected memory did not produce guard text: %q", assembly.MemoryText)
		}
		if strings.Contains(assembly.MemoryText, "privately likes Rowan") {
			t.Fatalf("protected memory leaked secret content: %q", assembly.MemoryText)
		}
		if !strings.Contains(assembly.MemoryText, "kind=romantic_feeling") {
			t.Fatalf("protected memory guard missing kind: %q", assembly.MemoryText)
		}
	})

	t.Run("character_private_recollection_masks_secret_guard_content", func(t *testing.T) {
		text := characterPrivateRecollectionPromptLineText(store.ProtagonistEntityMemory{
			OwnerEntityKey:     "mina",
			OwnerEntityName:    "Mina",
			MemoryText:         "Mina privately likes Rowan but has not revealed it.",
			SecretGuard:        true,
			TargetRevealPolicy: "owner_private_until_revealed",
			TagsJSON:           `["protected_secret","secret_guard","protected_secret_kind:romantic_feeling"]`,
		}, 500)
		if !strings.Contains(text, "protected private knowledge is present") || !strings.Contains(text, "kind=romantic_feeling") {
			t.Fatalf("private recollection guard text missing: %q", text)
		}
		if strings.Contains(text, "privately likes Rowan") {
			t.Fatalf("private recollection leaked secret text: %q", text)
		}
	})

	t.Run("empty_turn_summary_skips_memory_save", func(t *testing.T) {
		fake := &turnRecordingStore{}
		srv := NewServer(config.Default())
		srv.Store = fake
		extraction := map[string]any{
			"turn_summary":     "",
			"importance_score": 7,
			"kg_triples":       []any{map[string]any{"subject": "Mina", "predicate": "found", "object": "key"}},
		}
		result := srv.saveCriticExtractionArtifacts(context.Background(), "sess-p83-empty", 2, extraction, "Mina found the brass key.", completeTurnEmbeddingConfig{}, time.Unix(100, 0))
		if result.Memories != 0 {
			t.Fatalf("expected 0 memory saves for empty turn_summary, got %d", result.Memories)
		}
		if len(fake.savedMemories) != 0 {
			t.Fatalf("expected 0 saved memories for empty turn_summary, got %d", len(fake.savedMemories))
		}
	})

	t.Run("ooc_control_turn_guard_skips_memory", func(t *testing.T) {
		fake := &turnRecordingStore{}
		cfg := config.Default()
		cfg.StoreMode = config.StoreModeMariaDBAuthority
		srv := NewServer(cfg)
		srv.Store = fake
		srv.StoreOpenError = nil

		mux := http.NewServeMux()
		srv.RegisterRoutes(mux)

		body := `{"chat_session_id":"sess-p83-ooc","turn_index":1,"user_input":"OOC: please change the plugin setting","assistant_content":"Sure, I will help.","context_messages":[]}`
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
		if resp["save_error"] != "skipped_by_ooc_guard" || resp["critic_triggered"] != false {
			t.Fatalf("unexpected OOC guard response: %+v", resp)
		}
		if len(fake.savedChatLogs) != 0 || len(fake.savedMemories) != 0 || len(fake.savedEvidence) != 0 || len(fake.savedKGTriples) != 0 {
			t.Fatalf("OOC control turn should skip all writes, logs=%d memories=%d evidence=%d kg=%d", len(fake.savedChatLogs), len(fake.savedMemories), len(fake.savedEvidence), len(fake.savedKGTriples))
		}
	})
}

func TestArchiveCenter24ReplayRegressionGate(t *testing.T) {
	t.Run("language_replay_cases_preserve_raw_evidence_and_output_summary_contract", func(t *testing.T) {
		cases := []struct {
			name    string
			rawLang string
			outLang string
		}{
			{name: "ko_to_ko", rawLang: "ko", outLang: "ko"},
			{name: "ko_to_en", rawLang: "ko", outLang: "en"},
			{name: "ko_to_ja", rawLang: "ko", outLang: "ja"},
			{name: "en_to_ja", rawLang: "en", outLang: "ja"},
			{name: "mid_session_ja_to_en", rawLang: "ja", outLang: "en"},
		}
		for _, tc := range cases {
			t.Run(tc.name, func(t *testing.T) {
				languageContext := normalizeCompleteTurnLanguageContext(map[string]any{
					"contract_version":        "language_memory.v1",
					"session_output_language": tc.outLang,
					"output_language_source":  "replay_fixture",
					"raw_user_language":       tc.rawLang,
					"summary_language":        tc.outLang,
					"search_text_policy":      "summary_plus_raw_plus_aliases",
					"locked_for_turn":         true,
				})
				planner := buildPrepareTurnPlannerLanguageContract(languageContext)
				if planner["planner_support_language"] != tc.outLang || planner["raw_evidence_rewritten"] != false {
					t.Fatalf("planner language contract mismatch for %s: %#v", tc.name, planner)
				}
				rawEvidence := "RAW-" + tc.rawLang + ": sealed-oath evidence remains exact."
				extraction := applyLanguageMemoryWriteContract(map[string]any{
					"turn_summary":      "SUMMARY-" + tc.outLang + ": The sealed oath remains active.",
					"importance_score":  7,
					"evidence_excerpts": []any{rawEvidence},
					"language_context":  languageContext,
				}, languageContext)
				searchText := completeTurnMemorySearchText("", extraction, rawEvidence+"\nSUMMARY-"+tc.outLang+": The sealed oath remains active.")
				if !strings.Contains(searchText.Text, "SUMMARY-"+tc.outLang) || !strings.Contains(searchText.Text, rawEvidence) {
					t.Fatalf("search text missing summary/raw evidence for %s: %s", tc.name, searchText.Text)
				}
				if got := extractionStringFromAny(searchText.LanguageContext["session_output_language"]); got != tc.outLang {
					t.Fatalf("search text language context output=%q, want %q", got, tc.outLang)
				}
			})
		}
	})

	t.Run("identity_accuracy_aliases_feed_vector_search_and_prompt_preserves_private_same_entity_continuity", func(t *testing.T) {
		content := "Lia spoke about when she was Gloria."
		languageContext := normalizeCompleteTurnLanguageContext(map[string]any{
			"contract_version":        "language_memory.v1",
			"session_output_language": "en",
			"raw_user_language":       "en",
			"summary_language":        "en",
			"search_text_policy":      "summary_plus_raw_plus_aliases",
			"locked_for_turn":         true,
		})
		extraction := normalizeCriticExtraction(map[string]any{
			"turn_summary":      "Identity continuity evidence exists.",
			"importance_score":  9,
			"evidence_excerpts": []any{content},
			"language_context":  languageContext,
			"character_identity_accuracy": []any{
				map[string]any{
					"surface_identity_name": "Lia",
					"true_identity_name":    "Gloria",
					"canonical_entity_name": "Gloria",
					"aliases":               []any{"리아", "글로리아"},
					"identity_kind":         "cover_identity",
					"same_entity":           true,
					"public_role":           "maid",
					"true_role":             "marchioness",
					"reveal_policy":         "owner_private_until_revealed",
					"knowledge_scope": map[string]any{
						"known_by":     []any{"Gloria"},
						"unknown_to":   []any{"Siwoo"},
						"suspected_by": []any{"Lucia"},
					},
				},
			},
		})
		extraction = applyLanguageMemoryWriteContract(extraction, languageContext)
		searchText := completeTurnMemorySearchText("", extraction, content)
		for _, needle := range []string{"Lia", "Gloria", "cover_identity", "maid", "marchioness", "Siwoo", "Lucia"} {
			if !strings.Contains(searchText.Text, needle) {
				t.Fatalf("identity replay search text missing alias/scope %q: %s", needle, searchText.Text)
			}
		}
		memories := []store.Memory{
			{
				ID:          2405,
				TurnIndex:   13,
				SummaryJSON: mustCompactJSON(map[string]any{"turn_summary": "An unrelated harvest meeting happened elsewhere."}),
				Importance:  1.0,
			},
			{
				ID:          2406,
				TurnIndex:   12,
				SummaryJSON: mustCompactJSON(extraction),
				Evidence:    mustCompactJSON(map[string]any{"evidence_excerpts": []string{content}}),
				Importance:  0.5,
			},
		}
		assembly := buildPrepareTurnInjectionAssembly(memories, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, 1, 3000, "Lia enters the room.", "default", nil, nil, languageContext)
		if !strings.Contains(assembly.MemoryText, "Protected identity continuity") || !strings.Contains(assembly.MemoryText, "kind=cover_identity") {
			t.Fatalf("identity replay did not inject protected guard: %q", assembly.MemoryText)
		}
		for _, needle := range []string{
			"Lia and Gloria refer to the same internal person",
			"not portray the surface identity and true identity as separate people",
			"not public character knowledge",
		} {
			if !strings.Contains(assembly.MemoryText, needle) {
				t.Fatalf("identity replay missing continuity guard %q: %q", needle, assembly.MemoryText)
			}
		}
		if !strings.Contains(assembly.MemoryText, "knowledge_scope=known:1 suspected:1") {
			t.Fatalf("identity replay missing scoped knowledge counts: %q", assembly.MemoryText)
		}
		for _, leaked := range []string{"marchioness", "spoke about when"} {
			if strings.Contains(assembly.MemoryText, leaked) {
				t.Fatalf("identity replay leaked protected identity content %q: %q", leaked, assembly.MemoryText)
			}
		}
	})

	t.Run("pov_scoped_identity_replay_treats_cover_identity_as_self_for_knower", func(t *testing.T) {
		extraction := normalizeCriticExtraction(map[string]any{
			"turn_summary":     "Gloria uses Lia as a protected cover identity.",
			"importance_score": 9,
			"character_identity_accuracy": []any{
				map[string]any{
					"surface_identity_name": "Lia",
					"true_identity_name":    "Gloria",
					"canonical_entity_name": "Gloria",
					"identity_kind":         "cover_identity",
					"same_entity":           true,
					"public_role":           "maid",
					"true_role":             "marchioness",
					"reveal_policy":         "owner_private_until_revealed",
					"knowledge_scope": map[string]any{
						"known_by":     []any{"Gloria"},
						"unknown_to":   []any{"Siwoo"},
						"suspected_by": []any{"Lucia"},
					},
				},
			},
		})
		fake := &turnRecordingStore{
			returnMemories: []store.Memory{{
				ID:            2408,
				ChatSessionID: "sess-pov-identity",
				TurnIndex:     18,
				SummaryJSON:   mustCompactJSON(extraction),
				Importance:    0.9,
			}},
		}
		cfg := config.Default()
		cfg.StoreMode = config.StoreModeDualShadow
		srv := NewServer(cfg)
		srv.Store = fake
		mux := http.NewServeMux()
		srv.RegisterRoutes(mux)

		body := `{
			"chat_session_id":"sess-pov-identity",
			"turn_index":19,
			"raw_user_input":"Gloria considers Lia's field report.",
			"client_meta":{"perspective_context":{"current_pov":"Gloria","source":"hidden_spoiler_pov"}},
			"settings":{"max_injection_chars":3000,"injection_enabled":true,"input_context_enabled":false,"top_k":1}
		}`
		req := httptest.NewRequest(http.MethodPost, "/prepare-turn", strings.NewReader(body))
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
		injectionPack := mapFromAny(resp["injection_pack"])
		memoryText := extractionStringFromAny(injectionPack["memory_text"])
		for _, needle := range []string{
			"POV-scoped identity continuity",
			"Lia is Gloria's own protected surface identity/persona",
			"treat Lia and Gloria as the same internal person",
			"not two separate characters",
			"Keep this as POV/private knowledge",
		} {
			if !strings.Contains(memoryText, needle) {
				t.Fatalf("POV identity replay missing %q: %q", needle, memoryText)
			}
		}
		perspective := mapFromAny(injectionPack["perspective_context"])
		if perspective["current_pov"] != "Gloria" || perspective["source"] != "hidden_spoiler_pov" {
			t.Fatalf("perspective context mismatch: %#v", perspective)
		}
	})

	t.Run("pov_scoped_identity_replay_infers_hidden_spoiler_pov_from_raw_input", func(t *testing.T) {
		extraction := normalizeCriticExtraction(map[string]any{
			"turn_summary":     "Gloria uses Lia as a protected cover identity.",
			"importance_score": 9,
			"character_identity_accuracy": []any{
				map[string]any{
					"surface_identity_name": "Lia",
					"true_identity_name":    "Gloria",
					"canonical_entity_name": "Gloria",
					"identity_kind":         "cover_identity",
					"same_entity":           true,
					"reveal_policy":         "owner_private_until_revealed",
					"knowledge_scope": map[string]any{
						"known_by":   []any{"Gloria"},
						"unknown_to": []any{"Siwoo"},
					},
				},
			},
		})
		fake := &turnRecordingStore{
			returnMemories: []store.Memory{{
				ID:            2410,
				ChatSessionID: "sess-pov-infer",
				TurnIndex:     20,
				SummaryJSON:   mustCompactJSON(extraction),
				Importance:    0.9,
			}},
		}
		cfg := config.Default()
		cfg.StoreMode = config.StoreModeDualShadow
		srv := NewServer(cfg)
		srv.Store = fake
		mux := http.NewServeMux()
		srv.RegisterRoutes(mux)

		body := `{
			"chat_session_id":"sess-pov-infer",
			"turn_index":21,
			"raw_user_input":"히든 스포일러: 글로리아 시점으로 파웰에게 리아의 현장 보고를 묻는다.",
			"settings":{"max_injection_chars":3000,"injection_enabled":true,"input_context_enabled":false,"top_k":1}
		}`
		req := httptest.NewRequest(http.MethodPost, "/prepare-turn", strings.NewReader(body))
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
		injectionPack := mapFromAny(resp["injection_pack"])
		memoryText := extractionStringFromAny(injectionPack["memory_text"])
		for _, needle := range []string{
			"POV-scoped identity continuity",
			"current_pov=글로리아",
			"treat Lia and Gloria as the same internal person",
		} {
			if !strings.Contains(memoryText, needle) {
				t.Fatalf("inferred POV identity replay missing %q: %q", needle, memoryText)
			}
		}
		perspective := mapFromAny(injectionPack["perspective_context"])
		if perspective["current_pov"] != "글로리아" || !strings.Contains(extractionStringFromAny(perspective["source"]), "inferred_raw_user_input") {
			t.Fatalf("inferred perspective context mismatch: %#v", perspective)
		}
	})

	t.Run("confirmed_identity_alias_canonicalizes_saved_artifacts_without_rewriting_raw_evidence", func(t *testing.T) {
		fake := &turnRecordingStore{}
		srv := NewServer(config.Default())
		srv.Store = fake

		evidence := "Lia wrote the field report while hiding that she was Gloria."
		content := evidence + " Powell noticed only the careful handwriting."
		extraction := normalizeCriticExtraction(map[string]any{
			"turn_summary":      "Lia coordinated the field report under her cover identity.",
			"importance_score":  9,
			"evidence_excerpts": []any{evidence},
			"entities": map[string]any{
				"characters": []any{
					map[string]any{"name": "Lia", "entity_type": "character", "description": "maid cover"},
				},
			},
			"kg_triples": []any{
				map[string]any{"subject": "Lia", "predicate": "submitted_report_to", "object": "Powell"},
			},
			"character_deltas": []any{
				map[string]any{"name": "Lia", "status": map[string]any{"cover_identity": "active"}},
			},
			"subjective_entity_memories": []any{
				map[string]any{
					"owner_entity_key":     "lia",
					"owner_entity_name":    "Lia",
					"owner_entity_role":    "npc",
					"owner_visibility":     "owner_private",
					"memory_text":          "Lia worries Powell will see through the maid cover.",
					"evidence_excerpt":     evidence,
					"secret_guard":         true,
					"target_reveal_policy": "owner_private_until_revealed",
				},
			},
			"character_identity_accuracy": []any{
				map[string]any{
					"surface_identity_name": "Lia",
					"true_identity_name":    "Gloria",
					"canonical_entity_name": "Gloria",
					"identity_kind":         "cover_identity",
					"same_entity":           true,
					"public_role":           "maid",
					"true_role":             "marchioness",
					"reveal_policy":         "owner_private_until_revealed",
					"knowledge_scope": map[string]any{
						"known_by":   []any{"Gloria"},
						"unknown_to": []any{"Siwoo", "Powell"},
					},
				},
			},
		})

		result := srv.saveCriticExtractionArtifacts(context.Background(), "sess-identity-merge", 24, extraction, content, completeTurnEmbeddingConfig{}, time.Unix(2400, 0))
		if result.Entities != 1 || result.KGTriples != 1 || result.CharacterStates != 1 || result.SubjectiveEntityMemories == 0 {
			t.Fatalf("expected identity merge to save canonical artifacts, result=%#v", result)
		}
		if len(fake.savedEvidence) != 1 || fake.savedEvidence[0].EvidenceText != evidence {
			t.Fatalf("raw direct evidence was rewritten or skipped: %#v", fake.savedEvidence)
		}
		if len(fake.savedEntities) != 1 || fake.savedEntities[0].Name != "Gloria" {
			t.Fatalf("entity was not canonicalized to Gloria: %#v", fake.savedEntities)
		}
		if !strings.Contains(fake.savedEntities[0].AliasesJSON, "Lia") {
			t.Fatalf("entity alias did not preserve surface identity: %s", fake.savedEntities[0].AliasesJSON)
		}
		if len(fake.savedKGTriples) != 1 || fake.savedKGTriples[0].Subject != "Gloria" || fake.savedKGTriples[0].Object != "Powell" {
			t.Fatalf("KG triple was not canonicalized safely: %#v", fake.savedKGTriples)
		}
		if len(fake.savedCharacterStates) != 1 || fake.savedCharacterStates[0].CharacterName != "Gloria" {
			t.Fatalf("character state was not canonicalized: %#v", fake.savedCharacterStates)
		}
		foundCanonicalOwner := false
		foundRawOwnerTag := false
		for _, memory := range fake.savedEntityMemories {
			if memory.OwnerEntityName == "Gloria" && memory.OwnerEntityKey == "gloria" {
				foundCanonicalOwner = true
			}
			if strings.Contains(memory.TagsJSON, "raw_owner_entity_name:Lia") && strings.Contains(memory.TagsJSON, "confirmed_identity_alias_canonicalized") {
				foundRawOwnerTag = true
			}
		}
		if !foundCanonicalOwner || !foundRawOwnerTag {
			t.Fatalf("subjective owner canonicalization missing, memories=%#v", fake.savedEntityMemories)
		}
		if len(fake.savedMemories) != 1 || !strings.Contains(fake.savedMemories[0].SummaryJSON, "confirmed_identity_alias_canonical_merge") {
			t.Fatalf("memory summary did not record canonical merge trace: %#v", fake.savedMemories)
		}
		if !strings.Contains(fake.savedMemories[0].SummaryJSON, `"surface_identity_name":"Lia"`) {
			t.Fatalf("surface identity was not retained for protected alias search: %s", fake.savedMemories[0].SummaryJSON)
		}
	})

	t.Run("protected_secret_replay_preserves_partial_reveal_scope_without_discovery", func(t *testing.T) {
		extraction := normalizeCriticExtraction(map[string]any{
			"turn_summary":     "A protected inheritance secret exists.",
			"importance_score": 8,
			"protected_secrets": []any{
				map[string]any{
					"secret_kind":       "power_inheritance",
					"owner":             "Ari",
					"subject":           []any{"sealed crest"},
					"summary":           "Ari inherited the sealed crest but has not revealed it.",
					"sensitivity":       "critical",
					"disclosure_policy": "explicit_reveal_event_required",
					"knowledge_scope": map[string]any{
						"known_by":     []any{"Ari", "Mentor"},
						"unknown_to":   []any{"Guard"},
						"suspected_by": []any{"Oracle"},
					},
				},
			},
		})
		searchText := completeTurnMemorySearchText("", extraction, "Ari inherited the sealed crest but has not revealed it.")
		for _, needle := range []string{"Ari", "sealed crest", "power_inheritance", "Mentor", "Guard", "Oracle"} {
			if !strings.Contains(searchText.Text, needle) {
				t.Fatalf("protected secret replay search text missing %q: %s", needle, searchText.Text)
			}
		}
		memories := []store.Memory{{
			ID:          2407,
			TurnIndex:   14,
			SummaryJSON: mustCompactJSON(extraction),
			Importance:  0.9,
		}}
		assembly := buildPrepareTurnInjectionAssembly(memories, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, 1, 3000, "Ari faces the guard.", "default", nil, nil, nil)
		if !strings.Contains(assembly.MemoryText, "Protected continuity guard") || !strings.Contains(assembly.MemoryText, "kind=power_inheritance") {
			t.Fatalf("protected secret replay did not inject guard: %q", assembly.MemoryText)
		}
		if !strings.Contains(assembly.MemoryText, "knowledge_scope=known:2 suspected:1") {
			t.Fatalf("protected secret replay missing partial reveal counts: %q", assembly.MemoryText)
		}
		for _, leaked := range []string{"sealed crest", "inherited", "Mentor", "Guard", "Oracle"} {
			if strings.Contains(assembly.MemoryText, leaked) {
				t.Fatalf("protected secret replay leaked scoped secret content %q: %q", leaked, assembly.MemoryText)
			}
		}
	})

	t.Run("vector_artifact_hits_hydrate_and_report_evidence_world_rule_counters", func(t *testing.T) {
		evidence := []store.DirectEvidence{
			{ID: 101, ChatSessionID: "sess-artifact-vector", EvidenceText: "Lia and Gloria are the same person under a cover identity.", TurnAnchor: 7, SourceTurnStart: 7, SourceTurnEnd: 7},
			{ID: 102, ChatSessionID: "sess-artifact-vector", EvidenceText: "This stale evidence is tombstoned.", TurnAnchor: 6, SourceTurnStart: 6, SourceTurnEnd: 6, Tombstoned: true},
		}
		worldRules := []store.WorldRule{
			{ID: 201, ChatSessionID: "sess-artifact-vector", Scope: "session", Category: "identity", Key: "cover_identity_same_entity", ValueJSON: `"Lia and Gloria must be resolved as one internal person."`, SourceTurn: 7},
			{ID: 202, ChatSessionID: "sess-artifact-vector", Scope: "session", Category: "identity", Key: "suppressed_rule", ValueJSON: `"hidden"`, SourceTurn: 7, Suppressed: true},
		}
		vectorShadow := map[string]any{
			"search_result": "ok",
			"search_results": []map[string]any{
				{"id": "evidence:sess-artifact-vector:101", "tier": "evidence", "source_table": "direct_evidence_records", "source_row_id": "101"},
				{"id": "world_rule:sess-artifact-vector:201", "tier": "world_rule", "source_table": "world_rules", "source_row_id": "201"},
				{"id": "evidence:sess-artifact-vector:102", "tier": "evidence", "source_table": "direct_evidence_records", "source_row_id": "102"},
				{"id": "world_rule:sess-artifact-vector:202", "tier": "world_rule", "source_table": "world_rules", "source_row_id": "202"},
			},
		}
		assembly := buildPrepareTurnInjectionAssembly(nil, nil, evidence, nil, nil, worldRules, nil, nil, nil, nil, nil, nil, nil, 4, 4000, "Gloria thinks about Lia.", "default", nil, vectorShadow, nil, map[string]any{"current_pov": "Gloria"})
		if !strings.Contains(assembly.DirectEvidenceText, "Lia and Gloria are the same person") {
			t.Fatalf("direct evidence vector hit was not injected: %q", assembly.DirectEvidenceText)
		}
		if !strings.Contains(assembly.WorldRulesText, "cover_identity_same_entity") {
			t.Fatalf("world rule vector hit was not prioritized/injected: %q", assembly.WorldRulesText)
		}
		counts := prepareTurnRenderCounts(assembly, "")
		if got := intFromAny(counts["vector_found"], 0); got != 4 {
			t.Fatalf("vector_found=%d, want 4 raw artifact hits", got)
		}
		if got := intFromAny(counts["vector_hydrated"], 0); got != 2 {
			t.Fatalf("vector_hydrated=%d, want 2", got)
		}
		if got := intFromAny(counts["vector_scope_filtered_count"], 0); got != 2 {
			t.Fatalf("vector_scope_filtered_count=%d, want 2", got)
		}
		if got := intFromAny(counts["vector_injected"], 0); got < 2 {
			t.Fatalf("vector_injected=%d, want at least 2: %#v", got, counts)
		}
	})

	t.Run("identity_guard_mentions_alias_merge_for_pov_cover_identity", func(t *testing.T) {
		identityAccuracy := []any{map[string]any{
			"surface_identity_name": "Lia",
			"true_identity_name":    "Gloria",
			"canonical_entity_name": "Gloria",
			"identity_kind":         "cover_identity",
			"same_entity":           true,
			"reveal_policy":         "owner_private_until_revealed",
			"knowledge_scope": map[string]any{
				"known_by":   []any{"Gloria"},
				"unknown_to": []any{"Siwoo"},
			},
		}}
		globalLine := prepareTurnProtectedIdentityContinuityGuardLine(identityAccuracy)
		if !strings.Contains(globalLine, "keep aliases merged in entity resolution") {
			t.Fatalf("global identity guard missing alias merge instruction: %q", globalLine)
		}
		povLine := prepareTurnPOVScopedIdentityGuardLine(identityAccuracy, map[string]any{"current_pov": "Gloria"})
		if !strings.Contains(povLine, "self/cover-role continuity") {
			t.Fatalf("POV identity guard missing cover-role continuity instruction: %q", povLine)
		}
	})

	t.Run("vector_replay_respects_topk_and_does_not_fill_recent_when_hits_exist", func(t *testing.T) {
		memories := []store.Memory{
			{ID: 1, TurnIndex: 3, SummaryJSON: `{"turn_summary":"Old semantic gate oath.","language_context":{"session_output_language":"en","raw_user_language":"ko","summary_language":"en","search_text_policy":"summary_plus_raw_plus_aliases"},"entities":[{"name":"Mina","aliases":["Gatekeeper"]}]}`, Importance: 0.4},
			{ID: 2, TurnIndex: 4, SummaryJSON: `{"turn_summary":"Old semantic shrine oath.","language_context":{"session_output_language":"en","raw_user_language":"ja","summary_language":"en","search_text_policy":"summary_plus_raw_plus_aliases"},"entities":[{"name":"Rowan","aliases":["Shrine witness"]}]}`, Importance: 0.6},
			{ID: 3, TurnIndex: 30, SummaryJSON: `{"turn_summary":"Recent unrelated dinner detail."}`, Importance: 1},
		}
		vectorShadow := map[string]any{
			"search_result": "ok",
			"search_results": []map[string]any{
				{"id": "episode:sess-24:77", "source_table": "episode_summaries", "source_row_id": "77"},
				{"id": "memory:sess-24:2", "source_table": "memories", "source_row_id": "2", "raw_language": "ja", "summary_language": "en", "session_output_language": "en", "alias_count": 2},
				{"id": "memory:sess-24:99", "source_table": "memories", "source_row_id": "99"},
				{"id": "memory:sess-24:1", "source_table": "memories", "source_row_id": "1", "raw_language": "ko", "summary_language": "en", "session_output_language": "en", "alias_count": 2},
			},
		}
		assembly := buildPrepareTurnInjectionAssembly(memories, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, 2, 3000, "Recall the old oath.", "default", nil, vectorShadow, nil)
		for _, want := range []string{"[vector_relevant, turn 4", "[vector_relevant, turn 3", "Old semantic shrine oath", "Old semantic gate oath"} {
			if !strings.Contains(assembly.MemoryText, want) {
				t.Fatalf("vector replay missing %q: %q", want, assembly.MemoryText)
			}
		}
		if strings.Contains(assembly.MemoryText, "Recent unrelated dinner detail") {
			t.Fatalf("vector replay filled topK with recent fallback despite vector hits: %q", assembly.MemoryText)
		}
		for key, want := range map[string]int{
			"vector_memory_hit_count":                       3,
			"vector_memory_hydrated_count":                  2,
			"vector_memory_selected_count":                  2,
			"vector_memory_injected_count":                  2,
			"vector_non_memory_hit_count":                   1,
			"vector_memory_missing_count":                   1,
			"vector_memory_hit_language_context_count":      2,
			"vector_memory_hydrated_language_context_count": 2,
			"selected_memory_total_count":                   2,
		} {
			if got := intFromAny(assembly.Counts[key], 0); got != want {
				t.Fatalf("vector replay count %s=%d, want %d; counts=%#v", key, got, want, assembly.Counts)
			}
		}
	})
}

func TestRunCompleteTurnCriticAuditsWorldRulesWhenInitialExtractionEmpty(t *testing.T) {
	firstExtraction, _ := json.Marshal(map[string]any{
		"turn_summary":      "The island run establishes a companion gacha and dungeon progression loop.",
		"importance_score":  8,
		"evidence_excerpts": []any{"companions are drawn by gacha and dungeon rewards buy skills"},
		"world_rule_audit":  map[string]any{"durable_rule_found": true, "reason": "The extraction noticed a progression system but omitted rules."},
		"world_rules":       []any{},
		"world_state":       map[string]any{"version": "world_state.v1", "confidence": 0, "verification": "", "rules": []any{}},
	})
	auditExtraction, _ := json.Marshal(map[string]any{
		"audit": map[string]any{"durable_rule_found": true, "reason": "The turn confirms a recurring progression economy."},
		"world_rules": []any{
			map[string]any{
				"scope":        "session",
				"scope_name":   "progression_system",
				"category":     "progression",
				"key":          "dungeon_rewards_buy_skills",
				"value":        "Dungeon rewards can be exchanged for skills and progression upgrades.",
				"confidence":   0.9,
				"verification": "verified",
			},
		},
		"world_state": map[string]any{
			"version":      "world_state.v1",
			"confidence":   0.9,
			"verification": "verified",
			"rules": []any{
				map[string]any{"scope": "session", "scope_name": "progression_system", "category": "progression", "key": "dungeon_rewards_buy_skills", "value": "Dungeon rewards can be exchanged for skills and progression upgrades."},
			},
		},
	})
	responses := []string{string(firstExtraction), string(auditExtraction)}
	calls := 0
	oldClient := proxyHTTPClient
	proxyHTTPClient = &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		if calls >= len(responses) {
			t.Fatalf("unexpected extra critic call %d", calls+1)
		}
		payload, _ := json.Marshal(map[string]any{
			"model":   "critic-test",
			"choices": []any{map[string]any{"message": map[string]any{"content": responses[calls]}}},
		})
		calls++
		return &http.Response{
			StatusCode: http.StatusOK,
			Header:     make(http.Header),
			Body:       io.NopCloser(strings.NewReader(string(payload))),
		}, nil
	})}
	defer func() { proxyHTTPClient = oldClient }()

	srv := NewServer(config.Default())
	cfg := completeTurnLLMConfig{
		APIKey:              "sk-test",
		Endpoint:            "https://api.example.com/v1",
		Model:               "critic-test",
		Provider:            "openai",
		TimeoutMs:           60000,
		Temperature:         0.2,
		MaxTokens:           1600,
		MaxCompletionTokens: 1600,
	}
	extraction, trace, err := srv.runCompleteTurnCritic(
		context.Background(),
		"sess-world-audit",
		3,
		"섬에서는 동료를 뽑기로 얻고 던전 보상 포인트로 스킬을 구매한다.",
		"관리자는 이 섬의 기본 진행 구조가 뽑기 동료와 던전 보상 경제라고 설명했다.",
		nil,
		nil,
		cfg,
	)
	if err != nil {
		t.Fatalf("runCompleteTurnCritic error: %v", err)
	}
	if calls != 2 {
		t.Fatalf("critic calls = %d, want 2", calls)
	}
	if got := len(worldRuleItemsForSave(extraction)); got == 0 {
		t.Fatalf("world-rule audit did not merge rules: %#v", extraction)
	}
	auditTrace := mapFromAny(trace["world_rule_audit"])
	if auditTrace["status"] != "ok" || intFromAny(auditTrace["merged_world_rule_count"], 0) == 0 {
		t.Fatalf("world_rule_audit trace mismatch: %+v", auditTrace)
	}
}

func TestRunCompleteTurnCriticForceWorldRuleAuditWhenInitialAuditMissing(t *testing.T) {
	firstExtraction, _ := json.Marshal(map[string]any{
		"turn_summary":      "The first session setup establishes a stable progression economy.",
		"importance_score":  8,
		"evidence_excerpts": []any{"dungeon points can be spent on skills"},
		"world_rules":       []any{},
		"world_state":       map[string]any{"version": "world_state.v1", "confidence": 0, "verification": "", "rules": []any{}},
	})
	auditExtraction, _ := json.Marshal(map[string]any{
		"audit": map[string]any{"durable_rule_found": true, "reason": "Forced cold-start audit found a progression economy."},
		"world_rules": []any{
			map[string]any{"scope": "session", "scope_name": "progression_system", "category": "economy", "key": "points_buy_skills", "value": "Dungeon points can be spent to purchase skills."},
		},
	})
	responses := []string{string(firstExtraction), string(auditExtraction)}
	calls := 0
	oldClient := proxyHTTPClient
	proxyHTTPClient = &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		if calls >= len(responses) {
			t.Fatalf("unexpected extra critic call %d", calls+1)
		}
		payload, _ := json.Marshal(map[string]any{
			"model":   "critic-test",
			"choices": []any{map[string]any{"message": map[string]any{"content": responses[calls]}}},
		})
		calls++
		return &http.Response{
			StatusCode: http.StatusOK,
			Header:     make(http.Header),
			Body:       io.NopCloser(strings.NewReader(string(payload))),
		}, nil
	})}
	defer func() { proxyHTTPClient = oldClient }()

	srv := NewServer(config.Default())
	cfg := completeTurnLLMConfig{
		APIKey:              "sk-test",
		Endpoint:            "https://api.example.com/v1",
		Model:               "critic-test",
		Provider:            "openai",
		TimeoutMs:           60000,
		Temperature:         0.2,
		MaxTokens:           1600,
		MaxCompletionTokens: 1600,
		ForceWorldRuleAudit: true,
	}
	extraction, trace, err := srv.runCompleteTurnCritic(
		context.Background(),
		"sess-world-audit-force",
		1,
		"던전 포인트로 스킬을 구매할 수 있다는 세계 구조를 설명한다.",
		"관리자는 던전 보상 포인트가 스킬 구매 재화라고 확인했다.",
		nil,
		nil,
		cfg,
	)
	if err != nil {
		t.Fatalf("runCompleteTurnCritic error: %v", err)
	}
	if calls != 2 {
		t.Fatalf("critic calls = %d, want 2", calls)
	}
	if got := len(worldRuleItemsForSave(extraction)); got != 1 {
		t.Fatalf("forced world-rule audit saved rules = %d, want 1: %#v", got, extraction)
	}
	auditTrace := mapFromAny(trace["world_rule_audit"])
	if auditTrace["status"] != "ok" {
		t.Fatalf("world_rule_audit trace mismatch: %+v", auditTrace)
	}
}

// --- SEQ-12.3-P84: memory summary normalization and minimum field persistence ---

func TestSeq123P84MemorySummaryNormalizationMinimumFields(t *testing.T) {
	t.Run("normalize_trims_summary_and_clamps_importance", func(t *testing.T) {
		raw := map[string]any{
			"turn_summary":     "  Mina found the key.  ",
			"importance_score": 15,
		}
		out := normalizeCriticExtraction(raw)
		summary, _ := out["turn_summary"].(string)
		if summary != "Mina found the key." {
			t.Fatalf("turn_summary not trimmed/normalized: %q", summary)
		}
		score, _ := out["importance_score"].(float64)
		if score != 10 {
			t.Fatalf("importance_score not clamped to 10: got %v", score)
		}
	})

	t.Run("normalize_clamps_importance_low", func(t *testing.T) {
		raw := map[string]any{
			"turn_summary":     "Brief aside.",
			"importance_score": -3,
		}
		out := normalizeCriticExtraction(raw)
		score, _ := out["importance_score"].(float64)
		if score != 1 {
			t.Fatalf("importance_score not clamped to 1: got %v", score)
		}
	})

	t.Run("memory_save_populates_minimum_fields", func(t *testing.T) {
		fake := &turnRecordingStore{}
		srv := NewServer(config.Default())
		srv.Store = fake
		extraction := map[string]any{
			"turn_summary":        "Mina found the brass key in the North Wing Scene Room.",
			"importance_score":    8,
			"evidence_excerpts":   []string{"found the brass key"},
			"relationship_memory": map[string]any{"trust_shift": 1},
			"archive_hint":        map[string]any{"wing": "North Wing", "room": "Scene Room"},
		}
		result := srv.saveCriticExtractionArtifacts(context.Background(), "sess-p84", 3, extraction, "Mina found the brass key in the North Wing Scene Room.", completeTurnEmbeddingConfig{}, time.Unix(300, 0))
		if result.Memories != 1 {
			t.Fatalf("expected 1 memory save, got %d", result.Memories)
		}
		if len(fake.savedMemories) != 1 {
			t.Fatalf("expected 1 saved memory, got %d", len(fake.savedMemories))
		}
		mem := fake.savedMemories[0]
		if mem.ChatSessionID != "sess-p84" {
			t.Fatalf("ChatSessionID = %q, want sess-p84", mem.ChatSessionID)
		}
		if mem.TurnIndex != 3 {
			t.Fatalf("TurnIndex = %d, want 3", mem.TurnIndex)
		}
		var summaryPayload map[string]any
		if err := json.Unmarshal([]byte(mem.SummaryJSON), &summaryPayload); err != nil {
			t.Fatalf("SummaryJSON must be compact JSON: %v", err)
		}
		if summaryPayload["turn_summary"] != "Mina found the brass key in the North Wing Scene Room." {
			t.Fatalf("SummaryJSON turn_summary = %v", summaryPayload["turn_summary"])
		}
		if summaryPayload["importance_score"] != float64(8) {
			t.Fatalf("SummaryJSON importance_score = %v, want 8", summaryPayload["importance_score"])
		}
		if mem.Importance <= 0 || mem.Importance > 1 {
			t.Fatalf("Importance = %v, want 0 < importance <= 1", mem.Importance)
		}
		if mem.Importance != 0.8 {
			t.Fatalf("Importance = %v, want 0.8", mem.Importance)
		}
		if mem.PlaceWing != "North Wing" {
			t.Fatalf("PlaceWing = %q, want North Wing", mem.PlaceWing)
		}
		if mem.PlaceRoom != "Scene Room" {
			t.Fatalf("PlaceRoom = %q, want Scene Room", mem.PlaceRoom)
		}
		var evidencePayload map[string]any
		if err := json.Unmarshal([]byte(mem.Evidence), &evidencePayload); err != nil {
			t.Fatalf("Evidence must be compact JSON: %v", err)
		}
		excerpts, ok := evidencePayload["evidence_excerpts"].([]any)
		if !ok || len(excerpts) != 1 || excerpts[0] != "found the brass key" {
			t.Fatalf("Evidence excerpts mismatch: %#v", evidencePayload["evidence_excerpts"])
		}
		relationship, ok := evidencePayload["relationship_memory"].(map[string]any)
		if !ok || relationship["trust_shift"] != float64(1) {
			t.Fatalf("Evidence relationship_memory mismatch: %#v", evidencePayload["relationship_memory"])
		}
		if result.EmbeddingStatus != "missing_config" {
			t.Fatalf("EmbeddingStatus = %q, want missing_config", result.EmbeddingStatus)
		}
	})

	t.Run("entity_placeholder_skipped", func(t *testing.T) {
		fake := &turnRecordingStore{}
		srv := NewServer(config.Default())
		srv.Store = fake
		extraction := map[string]any{
			"turn_summary":     "Mina met someone.",
			"importance_score": 5,
			"entities": map[string]any{
				"characters": []any{
					map[string]any{"name": "Mina", "entity_type": "character"},
					map[string]any{"name": "char_1", "entity_type": "character"},
				},
			},
		}
		_ = srv.saveCriticExtractionArtifacts(context.Background(), "sess-p84-ent", 4, extraction, "Mina met someone.", completeTurnEmbeddingConfig{}, time.Unix(400, 0))
		var names []string
		for _, e := range fake.savedEntities {
			names = append(names, e.Name)
		}
		if len(names) != 1 || names[0] != "Mina" {
			t.Fatalf("expected only concrete entity [Mina], got %v", names)
		}
	})

	t.Run("pending_thread_save_fields", func(t *testing.T) {
		fake := &turnRecordingStore{}
		srv := NewServer(config.Default())
		srv.Store = fake
		extraction := map[string]any{
			"turn_summary":     "A new mystery appears.",
			"importance_score": 6,
			"pending_threads": []any{
				map[string]any{
					"title":       "Find the hidden door",
					"details":     "Mina noticed a draft behind the bookshelf.",
					"thread_type": "open_question",
					"priority":    2,
					"confidence":  0.85,
					"owner":       "Mina",
				},
			},
		}
		result := srv.saveCriticExtractionArtifacts(context.Background(), "sess-p84-th", 5, extraction, "A new mystery appears.", completeTurnEmbeddingConfig{}, time.Unix(500, 0))
		if result.PendingThreads != 1 {
			t.Fatalf("expected 1 pending thread save, got %d", result.PendingThreads)
		}
		if len(fake.savedPendingThreads) != 1 {
			t.Fatalf("expected 1 saved pending thread, got %d", len(fake.savedPendingThreads))
		}
		pt := fake.savedPendingThreads[0]
		if pt.ChatSessionID != "sess-p84-th" {
			t.Fatalf("ChatSessionID = %q, want sess-p84-th", pt.ChatSessionID)
		}
		if pt.SourceTurn != 5 {
			t.Fatalf("SourceTurn = %d, want 5", pt.SourceTurn)
		}
		if pt.ThreadKey == "" {
			t.Fatalf("ThreadKey must not be empty")
		}
		if pt.Status != "open" {
			t.Fatalf("Status = %q, want open", pt.Status)
		}
		if pt.ThreadType != "open_question" {
			t.Fatalf("ThreadType = %q, want open_question", pt.ThreadType)
		}
		if pt.HookType != "open_question" {
			t.Fatalf("HookType = %q, want open_question", pt.HookType)
		}
		if pt.LastSeenTurn != 5 {
			t.Fatalf("LastSeenTurn = %d, want 5", pt.LastSeenTurn)
		}
		if pt.Confidence != 0.85 {
			t.Fatalf("Confidence = %v, want 0.85", pt.Confidence)
		}
		if pt.Title != "Find the hidden door" {
			t.Fatalf("Title = %q, want Find the hidden door", pt.Title)
		}
		if pt.Owner != "Mina" {
			t.Fatalf("Owner = %q, want Mina", pt.Owner)
		}
	})
}

// --- SEQ-12.3-P85: dedup / merge / correction path markers ---

func TestSeq123P85DedupMergeCorrectionPathMarkers(t *testing.T) {
	t.Run("same_incident_dedup_skips_insert_reinforces_importance", func(t *testing.T) {
		fake := &turnRecordingStore{
			returnMemories: []store.Memory{
				{ID: 42, ChatSessionID: "sess-p85", TurnIndex: 2, SummaryJSON: `{"turn_summary":"Mina promised Rowan she would return with the brass key."}`, Importance: 0.4},
			},
		}
		srv := NewServer(config.Default())
		srv.Store = fake
		extraction := map[string]any{
			"turn_summary":     "Mina promised Rowan she would return with the brass key.",
			"importance_score": 8,
		}
		result := srv.saveCriticExtractionArtifacts(context.Background(), "sess-p85", 6, extraction, "Mina promised Rowan she would return with the brass key.", completeTurnEmbeddingConfig{}, time.Unix(500, 0))
		if len(fake.savedMemories) != 0 || result.Memories != 0 {
			t.Fatalf("expected 0 SaveMemory calls for duplicate incident, saved=%d result.Memories=%d", len(fake.savedMemories), result.Memories)
		}
		if got := fake.updatedImportance[42]; got < 0.79 || got > 0.81 {
			t.Fatalf("expected existing memory importance reinforced to ~0.8, got %.2f", got)
		}
		if !containsString(result.Warnings, "memory_semantic_dedup_merged") {
			t.Fatalf("expected memory_semantic_dedup_merged warning, got %#v", result.Warnings)
		}
		foundAudit := false
		for _, item := range fake.savedAuditLogs {
			if item.EventType == "memory_semantic_dedup" && item.Source == "critic" {
				var details map[string]any
				if err := json.Unmarshal([]byte(item.DetailsJSON), &details); err == nil {
					if details["merged_memory_id"] == float64(42) {
						foundAudit = true
					}
				}
			}
		}
		if !foundAudit {
			t.Fatalf("expected memory_semantic_dedup audit log with merged_memory_id=42, got %#v", fake.savedAuditLogs)
		}
	})

	t.Run("truly_new_memory_inserts_once_without_dedup", func(t *testing.T) {
		fake := &turnRecordingStore{
			returnMemories: []store.Memory{
				{ID: 42, ChatSessionID: "sess-p85-new", TurnIndex: 2, SummaryJSON: `{"turn_summary":"Mina promised Rowan she would return with the brass key."}`, Importance: 0.4},
			},
		}
		srv := NewServer(config.Default())
		srv.Store = fake
		extraction := map[string]any{
			"turn_summary":     "The dragon attacked the northern village without warning.",
			"importance_score": 7,
		}
		result := srv.saveCriticExtractionArtifacts(context.Background(), "sess-p85-new", 6, extraction, "The dragon attacked the northern village without warning.", completeTurnEmbeddingConfig{}, time.Unix(500, 0))
		if len(fake.savedMemories) != 1 || result.Memories != 1 {
			t.Fatalf("expected 1 SaveMemory call for truly new memory, saved=%d result.Memories=%d", len(fake.savedMemories), result.Memories)
		}
		if containsString(result.Warnings, "memory_semantic_dedup_merged") {
			t.Fatalf("unexpected memory_semantic_dedup_merged warning for truly new memory: %#v", result.Warnings)
		}
		for _, item := range fake.savedAuditLogs {
			if item.EventType == "memory_semantic_dedup" {
				t.Fatalf("unexpected dedup audit log for truly new memory: %#v", item)
			}
		}
	})

	t.Run("superseding_summary_similar_to_existing_uses_merge_path", func(t *testing.T) {
		fake := &turnRecordingStore{
			returnMemories: []store.Memory{
				{ID: 43, ChatSessionID: "sess-p85-corr", TurnIndex: 2, SummaryJSON: `{"turn_summary":"Mina promised Rowan she would return with the brass key."}`, Importance: 0.4},
			},
		}
		srv := NewServer(config.Default())
		srv.Store = fake
		extraction := map[string]any{
			"turn_summary":     "Mina promised Rowan that she would return with the brass key.",
			"importance_score": 9,
		}
		result := srv.saveCriticExtractionArtifacts(context.Background(), "sess-p85-corr", 6, extraction, "Mina promised Rowan that she would return with the brass key.", completeTurnEmbeddingConfig{}, time.Unix(500, 0))
		if len(fake.savedMemories) != 0 || result.Memories != 0 {
			t.Fatalf("expected merge path (0 inserts) for superseding similar summary, saved=%d result.Memories=%d", len(fake.savedMemories), result.Memories)
		}
		if !containsString(result.Warnings, "memory_semantic_dedup_merged") {
			t.Fatalf("expected memory_semantic_dedup_merged warning for superseding similar summary, got %#v", result.Warnings)
		}
		foundAudit := false
		for _, item := range fake.savedAuditLogs {
			if item.EventType == "memory_semantic_dedup" && item.Source == "critic" {
				var details map[string]any
				if err := json.Unmarshal([]byte(item.DetailsJSON), &details); err == nil {
					if details["merged_memory_id"] == float64(43) {
						foundAudit = true
					}
				}
			}
		}
		if !foundAudit {
			t.Fatalf("expected memory_semantic_dedup audit log for superseding similar summary, got %#v", fake.savedAuditLogs)
		}
	})
}

// --- SEQ-12.3-P86: temporal / entity anchor hardening markers ---

func TestSeq123P86TemporalEntityAnchorHardeningMarkers(t *testing.T) {
	t.Run("temporal_anchors_preserved_on_store_artifacts", func(t *testing.T) {
		fake := &turnRecordingStore{}
		srv := NewServer(config.Default())
		srv.Store = fake
		extraction := map[string]any{
			"turn_summary":      "Mina found the brass key under the desk.",
			"importance_score":  7,
			"evidence_excerpts": []string{"found the brass key under the desk"},
			"kg_triples": []any{
				map[string]any{"subject": "Mina", "predicate": "found", "object": "brass key", "valid_from": 3},
			},
			"pending_threads": []any{
				map[string]any{"title": "Find the exit", "thread_type": "open_question", "priority": 1, "confidence": 0.9},
			},
			"entities": map[string]any{
				"characters": []any{map[string]any{"name": "Mina", "entity_type": "character"}},
			},
		}
		result := srv.saveCriticExtractionArtifacts(context.Background(), "sess-p86", 7, extraction, "Mina found the brass key under the desk.", completeTurnEmbeddingConfig{}, time.Unix(600, 0))

		if len(fake.savedMemories) != 1 {
			t.Fatalf("expected 1 saved memory, got %d", len(fake.savedMemories))
		}
		if fake.savedMemories[0].TurnIndex != 7 {
			t.Fatalf("memory TurnIndex = %d, want 7", fake.savedMemories[0].TurnIndex)
		}

		if len(fake.savedEvidence) != 1 {
			t.Fatalf("expected 1 saved evidence, got %d", len(fake.savedEvidence))
		}
		ev := fake.savedEvidence[0]
		if ev.SourceTurnStart != 7 || ev.SourceTurnEnd != 7 || ev.TurnAnchor != 7 {
			t.Fatalf("evidence temporal anchors mismatch: SourceTurnStart=%d SourceTurnEnd=%d TurnAnchor=%d, want 7", ev.SourceTurnStart, ev.SourceTurnEnd, ev.TurnAnchor)
		}

		if len(fake.savedKGTriples) != 1 {
			t.Fatalf("expected 1 saved KG triple, got %d", len(fake.savedKGTriples))
		}
		kg := fake.savedKGTriples[0]
		if kg.SourceTurn != 7 {
			t.Fatalf("kg triple SourceTurn = %d, want 7", kg.SourceTurn)
		}
		if kg.ValidFrom != 3 {
			t.Fatalf("kg triple ValidFrom = %d, want 3", kg.ValidFrom)
		}

		if len(fake.savedPendingThreads) != 1 {
			t.Fatalf("expected 1 saved pending thread, got %d", len(fake.savedPendingThreads))
		}
		pt := fake.savedPendingThreads[0]
		if pt.SourceTurn != 7 {
			t.Fatalf("pending thread SourceTurn = %d, want 7", pt.SourceTurn)
		}
		if pt.CreatedTurn != 7 {
			t.Fatalf("pending thread CreatedTurn = %d, want 7", pt.CreatedTurn)
		}
		if pt.LastSeenTurn != 7 {
			t.Fatalf("pending thread LastSeenTurn = %d, want 7", pt.LastSeenTurn)
		}

		// retrieval truth assembly guard: anchors on canonical Store artifacts, not vector-only
		if result.VectorStatus == "ok" || result.VectorsUpserted > 0 {
			t.Fatalf("expected no vector live write assumption when embedding config missing, VectorStatus=%q VectorsUpserted=%d", result.VectorStatus, result.VectorsUpserted)
		}
	})

	t.Run("entity_concrete_names_stored_with_seen_turns", func(t *testing.T) {
		fake := &turnRecordingStore{}
		srv := NewServer(config.Default())
		srv.Store = fake
		extraction := map[string]any{
			"turn_summary":     "Mina and Rowan entered the garden.",
			"importance_score": 6,
			"entities": map[string]any{
				"characters": []any{
					map[string]any{"name": "Mina", "entity_type": "character"},
					map[string]any{"name": "Rowan", "entity_type": "character"},
				},
				"locations": []any{
					map[string]any{"name": "garden", "entity_type": "location"},
				},
			},
		}
		_ = srv.saveCriticExtractionArtifacts(context.Background(), "sess-p86-ent", 4, extraction, "Mina and Rowan entered the garden.", completeTurnEmbeddingConfig{}, time.Unix(700, 0))

		if len(fake.savedEntities) != 3 {
			t.Fatalf("expected 3 saved entities, got %d", len(fake.savedEntities))
		}
		for _, e := range fake.savedEntities {
			if e.FirstSeenTurn != 4 || e.LastSeenTurn != 4 {
				t.Fatalf("entity %q missing turn anchors: FirstSeenTurn=%d LastSeenTurn=%d", e.Name, e.FirstSeenTurn, e.LastSeenTurn)
			}
		}
	})

	t.Run("placeholder_entities_and_kg_skipped", func(t *testing.T) {
		fake := &turnRecordingStore{}
		srv := NewServer(config.Default())
		srv.Store = fake
		extraction := map[string]any{
			"turn_summary":     "Someone found something.",
			"importance_score": 5,
			"entities": map[string]any{
				"characters": []any{
					map[string]any{"name": "char_1", "entity_type": "character"},
					map[string]any{"name": "assistant", "entity_type": "character"},
					map[string]any{"name": "Mina", "entity_type": "character"},
				},
			},
			"kg_triples": []any{
				map[string]any{"subject": "char_1", "predicate": "found", "object": "assistant"},
				map[string]any{"subject": "Mina", "predicate": "found", "object": "brass key"},
			},
		}
		result := srv.saveCriticExtractionArtifacts(context.Background(), "sess-p86-ph", 5, extraction, "Someone found something.", completeTurnEmbeddingConfig{}, time.Unix(800, 0))

		// only Mina should be stored as entity
		var names []string
		for _, e := range fake.savedEntities {
			names = append(names, e.Name)
		}
		if len(names) != 1 || names[0] != "Mina" {
			t.Fatalf("expected only concrete entity [Mina], got %v", names)
		}

		// only the Mina -> brass key triple should be stored
		if len(fake.savedKGTriples) != 1 {
			t.Fatalf("expected 1 saved KG triple after placeholder skip, got %d", len(fake.savedKGTriples))
		}
		if fake.savedKGTriples[0].Subject != "Mina" || fake.savedKGTriples[0].Object != "brass key" {
			t.Fatalf("expected KG triple subject=Mina object=brass key, got subject=%q object=%q", fake.savedKGTriples[0].Subject, fake.savedKGTriples[0].Object)
		}

		// vector should not be assumed as primary
		if result.VectorStatus == "ok" || result.VectorsUpserted > 0 {
			t.Fatalf("unexpected vector live write for placeholder skip test, VectorStatus=%q VectorsUpserted=%d", result.VectorStatus, result.VectorsUpserted)
		}
	})
}
