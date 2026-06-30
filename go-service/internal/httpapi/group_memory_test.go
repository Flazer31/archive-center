package httpapi

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/risulongmemory/archive-center-go/internal/config"
	"github.com/risulongmemory/archive-center-go/internal/store"
	"github.com/risulongmemory/archive-center-go/internal/vector"
)

// memoryFakeStore implements store.Store for memory/retrieval read-surface tests.
type memoryFakeStore struct {
	memories             []store.Memory
	evidenceItems        []store.DirectEvidence
	kgTriples            []store.KGTriple
	chatLogs             []store.ChatLog
	auditLogs            []store.AuditLog
	effectiveInput       *store.EffectiveInput
	episodes             []store.EpisodeSummary
	chapters             []store.ChapterSummary
	arcs                 []store.ArcSummary
	sagas                []store.SagaDigest
	resumePack           *store.ResumePack
	statsResult          store.StatsResult
	statsErr             error
	updatedMemory        []store.MemoryExplorerPatch
	updatedKG            []store.KGTripleExplorerPatch
	updatedEvidence      []store.DirectEvidenceExplorerPatch
	deletedMemoryID      int64
	deletedEvidenceID    int64
	deletedKGID          int64
	deletedCharacterName string
}

func (f *memoryFakeStore) SaveChatLog(ctx context.Context, log *store.ChatLog) error { return nil }
func (f *memoryFakeStore) ListChatLogs(ctx context.Context, sid string, from, to int) ([]store.ChatLog, error) {
	return f.chatLogs, nil
}
func (f *memoryFakeStore) SaveEffectiveInput(ctx context.Context, in *store.EffectiveInput) error {
	return nil
}
func (f *memoryFakeStore) GetEffectiveInput(ctx context.Context, sid string, turn int) (*store.EffectiveInput, error) {
	if f.effectiveInput != nil {
		return f.effectiveInput, nil
	}
	return nil, store.ErrNotFound
}
func (f *memoryFakeStore) SaveMemory(ctx context.Context, m *store.Memory) error { return nil }
func (f *memoryFakeStore) ListMemories(ctx context.Context, sid string, fromTurn, toTurn int) ([]store.Memory, error) {
	return f.memories, nil
}
func (f *memoryFakeStore) SaveEvidence(ctx context.Context, e *store.DirectEvidence) error {
	return nil
}
func (f *memoryFakeStore) ListEvidence(ctx context.Context, sid string) ([]store.DirectEvidence, error) {
	return f.evidenceItems, nil
}
func (f *memoryFakeStore) SaveKGTriple(ctx context.Context, t *store.KGTriple) error { return nil }
func (f *memoryFakeStore) ListKGTriples(ctx context.Context, sid string) ([]store.KGTriple, error) {
	return f.kgTriples, nil
}
func (f *memoryFakeStore) ListStorylines(ctx context.Context, chatSessionID string) ([]store.Storyline, error) {
	return nil, nil
}
func (f *memoryFakeStore) ListWorldRules(ctx context.Context, chatSessionID string) ([]store.WorldRule, error) {
	return nil, nil
}
func (f *memoryFakeStore) ListInheritedWorldRules(ctx context.Context, chatSessionID string, activeScope, scopeName string) ([]store.WorldRule, error) {
	return nil, nil
}
func (f *memoryFakeStore) ListCharacterStates(ctx context.Context, chatSessionID string) ([]store.CharacterState, error) {
	return nil, nil
}
func (f *memoryFakeStore) GetCharacterState(ctx context.Context, chatSessionID, characterName string) (*store.CharacterState, error) {
	return nil, store.ErrNotFound
}
func (f *memoryFakeStore) ListPendingThreads(ctx context.Context, chatSessionID, status string) ([]store.PendingThread, error) {
	return nil, nil
}
func (f *memoryFakeStore) ListActiveStates(ctx context.Context, chatSessionID, stateType string) ([]store.ActiveState, error) {
	return nil, nil
}
func (f *memoryFakeStore) ListCanonicalStateLayers(ctx context.Context, chatSessionID, layerType string) ([]store.CanonicalStateLayer, error) {
	return nil, nil
}
func (f *memoryFakeStore) ListEpisodeSummaries(ctx context.Context, chatSessionID string, limit, fromTurn, toTurn int) ([]store.EpisodeSummary, error) {
	return f.episodes, nil
}
func (f *memoryFakeStore) GetEpisodeSummary(ctx context.Context, episodeID int64) (*store.EpisodeSummary, error) {
	return nil, store.ErrNotFound
}
func (f *memoryFakeStore) SaveAuditLog(ctx context.Context, a *store.AuditLog) error {
	f.auditLogs = append([]store.AuditLog{*a}, f.auditLogs...)
	return nil
}
func (f *memoryFakeStore) ListAuditLogs(ctx context.Context, sid, eventType string, limit int) ([]store.AuditLog, error) {
	out := []store.AuditLog{}
	for _, item := range f.auditLogs {
		if sid != "" && item.ChatSessionID != sid {
			continue
		}
		if eventType != "" && item.EventType != eventType {
			continue
		}
		out = append(out, item)
		if limit > 0 && len(out) >= limit {
			break
		}
	}
	return out, nil
}
func (f *memoryFakeStore) SaveCriticFeedback(ctx context.Context, cf *store.CriticFeedback) error {
	return nil
}
func (f *memoryFakeStore) ListCriticFeedback(ctx context.Context, sid, targetType string, targetID int64) ([]store.CriticFeedback, error) {
	return nil, nil
}
func (f *memoryFakeStore) SaveCharacterEvent(ctx context.Context, e *store.CharacterEvent) error {
	return nil
}
func (f *memoryFakeStore) ListCharacterEvents(ctx context.Context, sid, name string) ([]store.CharacterEvent, error) {
	return nil, nil
}
func (f *memoryFakeStore) Stats(ctx context.Context) (store.StatsResult, error) {
	return f.statsResult, f.statsErr
}
func (f *memoryFakeStore) ListSessions(ctx context.Context) ([]store.SessionSummary, error) {
	return nil, nil
}
func (f *memoryFakeStore) GetResumePack(ctx context.Context, sid, trigger string) (*store.ResumePack, error) {
	return f.resumePack, nil
}
func (f *memoryFakeStore) SaveChapterSummary(ctx context.Context, item *store.ChapterSummary) error {
	if item != nil {
		f.chapters = append(f.chapters, *item)
	}
	return nil
}
func (f *memoryFakeStore) SearchChapterSummaries(ctx context.Context, chatSessionID, query string, fromTurn, toTurn, limit int) ([]store.ChapterSummary, error) {
	return f.chapters, nil
}
func (f *memoryFakeStore) SaveArcSummary(ctx context.Context, chatSessionID string, item *store.ArcSummary) error {
	if item != nil {
		f.arcs = append(f.arcs, *item)
	}
	return nil
}
func (f *memoryFakeStore) GetLatestArcSummary(ctx context.Context, chatSessionID string) (*store.ArcSummary, error) {
	if len(f.arcs) == 0 {
		return nil, store.ErrNotFound
	}
	cp := f.arcs[0]
	return &cp, nil
}
func (f *memoryFakeStore) ListArcSummaries(ctx context.Context, chatSessionID string, status string, limit int) ([]store.ArcSummary, error) {
	return f.arcs, nil
}
func (f *memoryFakeStore) SearchArcSummaries(ctx context.Context, chatSessionID, query string, fromTurn, toTurn, limit int) ([]store.ArcSummary, error) {
	return f.arcs, nil
}
func (f *memoryFakeStore) SaveSagaDigest(ctx context.Context, chatSessionID string, item *store.SagaDigest) error {
	if item != nil {
		f.sagas = append(f.sagas, *item)
	}
	return nil
}
func (f *memoryFakeStore) GetLatestSagaDigest(ctx context.Context, chatSessionID string) (*store.SagaDigest, error) {
	if len(f.sagas) == 0 {
		return nil, store.ErrNotFound
	}
	cp := f.sagas[0]
	return &cp, nil
}
func (f *memoryFakeStore) ListSagaDigests(ctx context.Context, chatSessionID string, limit int) ([]store.SagaDigest, error) {
	return f.sagas, nil
}
func (f *memoryFakeStore) SearchSagaDigests(ctx context.Context, chatSessionID, query string, fromTurn, toTurn, limit int) ([]store.SagaDigest, error) {
	return f.sagas, nil
}

func (f *memoryFakeStore) UpdateMemoryExplorerFields(ctx context.Context, sid string, memoryID int64, patch store.MemoryExplorerPatch) error {
	f.updatedMemory = append(f.updatedMemory, patch)
	for i := range f.memories {
		if f.memories[i].ID == memoryID && f.memories[i].ChatSessionID == sid {
			if patch.SummaryJSON != nil {
				f.memories[i].SummaryJSON = *patch.SummaryJSON
			}
			if patch.Importance != nil {
				f.memories[i].Importance = *patch.Importance
			}
			if patch.PlaceWing != nil {
				f.memories[i].PlaceWing = *patch.PlaceWing
			}
			if patch.PlaceRoom != nil {
				f.memories[i].PlaceRoom = *patch.PlaceRoom
			}
			return nil
		}
	}
	return store.ErrNotFound
}

func (f *memoryFakeStore) UpdateKGTripleExplorerFields(ctx context.Context, sid string, tripleID int64, patch store.KGTripleExplorerPatch) error {
	f.updatedKG = append(f.updatedKG, patch)
	for i := range f.kgTriples {
		if f.kgTriples[i].ID == tripleID && f.kgTriples[i].ChatSessionID == sid {
			if patch.Subject != nil {
				f.kgTriples[i].Subject = *patch.Subject
			}
			if patch.Predicate != nil {
				f.kgTriples[i].Predicate = *patch.Predicate
			}
			if patch.Object != nil {
				f.kgTriples[i].Object = *patch.Object
			}
			if patch.ValidFrom.Set {
				if patch.ValidFrom.Value == nil {
					f.kgTriples[i].ValidFrom = 0
				} else {
					f.kgTriples[i].ValidFrom = *patch.ValidFrom.Value
				}
			}
			if patch.ValidTo.Set {
				if patch.ValidTo.Value == nil {
					f.kgTriples[i].ValidTo = 0
				} else {
					f.kgTriples[i].ValidTo = *patch.ValidTo.Value
				}
			}
			return nil
		}
	}
	return store.ErrNotFound
}

func (f *memoryFakeStore) UpdateDirectEvidenceExplorerFields(ctx context.Context, sid string, recordID int64, patch store.DirectEvidenceExplorerPatch) error {
	f.updatedEvidence = append(f.updatedEvidence, patch)
	for i := range f.evidenceItems {
		if f.evidenceItems[i].ID == recordID && f.evidenceItems[i].ChatSessionID == sid {
			if patch.ArchiveState != nil {
				f.evidenceItems[i].ArchiveState = *patch.ArchiveState
			}
			if patch.CaptureVerification != nil {
				f.evidenceItems[i].CaptureVerification = *patch.CaptureVerification
			}
			if patch.CommittedGate != nil {
				f.evidenceItems[i].CommittedGate = *patch.CommittedGate
			}
			if patch.RepairNeeded != nil {
				f.evidenceItems[i].RepairNeeded = *patch.RepairNeeded
			}
			if patch.Tombstoned != nil {
				f.evidenceItems[i].Tombstoned = *patch.Tombstoned
			}
			if patch.SupersededByID.Set {
				if patch.SupersededByID.Value == nil {
					f.evidenceItems[i].SupersededByID = 0
				} else {
					f.evidenceItems[i].SupersededByID = int64(*patch.SupersededByID.Value)
				}
			}
			return nil
		}
	}
	return store.ErrNotFound
}

func (f *memoryFakeStore) DeleteMemoryByID(ctx context.Context, sid string, memoryID int64) error {
	for i := range f.memories {
		if f.memories[i].ID == memoryID && f.memories[i].ChatSessionID == sid {
			f.deletedMemoryID = memoryID
			f.memories = append(f.memories[:i], f.memories[i+1:]...)
			return nil
		}
	}
	return store.ErrNotFound
}

func (f *memoryFakeStore) DeleteDirectEvidenceByID(ctx context.Context, sid string, recordID int64) error {
	for i := range f.evidenceItems {
		if f.evidenceItems[i].ID == recordID && f.evidenceItems[i].ChatSessionID == sid {
			f.deletedEvidenceID = recordID
			f.evidenceItems = append(f.evidenceItems[:i], f.evidenceItems[i+1:]...)
			return nil
		}
	}
	return store.ErrNotFound
}

func (f *memoryFakeStore) DeleteKGTripleByID(ctx context.Context, sid string, tripleID int64) error {
	for i := range f.kgTriples {
		if f.kgTriples[i].ID == tripleID && f.kgTriples[i].ChatSessionID == sid {
			f.deletedKGID = tripleID
			f.kgTriples = append(f.kgTriples[:i], f.kgTriples[i+1:]...)
			return nil
		}
	}
	return store.ErrNotFound
}

func (f *memoryFakeStore) DeleteCharacterByName(ctx context.Context, sid string, characterName string) error {
	f.deletedCharacterName = characterName
	return nil
}

// Test 1: POST /search returns Store-backed data when chat_session_id provided
func TestSearchReturnsMemoriesFromStore(t *testing.T) {
	fake := &memoryFakeStore{
		memories: []store.Memory{
			{ID: 10, ChatSessionID: "sess-1", TurnIndex: 3, SummaryJSON: `{"summary":"test memory"}`, Importance: 0.8},
			{ID: 11, ChatSessionID: "sess-1", TurnIndex: 5, SummaryJSON: `{"summary":"hello gate memory"}`, Importance: 0.5},
		},
	}
	cfg := config.Default()
	srv := NewServer(cfg)
	srv.Store = fake

	mux := http.NewServeMux()
	srv.RegisterRoutes(mux)

	body := `{"user_input":"hello","chat_session_id":"sess-1","top_k":5}`
	req := httptest.NewRequest(http.MethodPost, "/search", strings.NewReader(body))
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
	items, ok := resp["items"].([]any)
	if !ok {
		t.Fatalf("items is not an array: %T", resp["items"])
	}
	if len(items) != 2 {
		t.Errorf("items count = %d, want 2", len(items))
	}
	first, ok := items[0].(map[string]any)
	if !ok {
		t.Fatalf("first item is not an object: %T", items[0])
	}
	if first["id"] != float64(11) {
		t.Errorf("first item id = %v, want the query-matching recent memory id 11", first["id"])
	}
	for _, key := range []string{"similarity_score", "importance_score", "recency_score", "final_score", "score_breakdown", "summary_preview"} {
		if _, ok := first[key]; !ok {
			t.Errorf("first item missing search scoring key %q", key)
		}
	}
	if got, ok := first["source"].(string); !ok || got != "memory" {
		t.Errorf("first item source = %v, want memory", first["source"])
	}
	if resp["memory_count"] != float64(2) {
		t.Errorf("memory_count = %v, want 2", resp["memory_count"])
	}
	if resp["total_count"] != float64(2) {
		t.Errorf("total_count = %v, want 2", resp["total_count"])
	}
	// Preserve 0.8 response keys
	if _, ok := resp["injection_text"]; !ok {
		t.Error("missing key injection_text")
	}
	if text, _ := resp["injection_text"].(string); !strings.Contains(text, "hello gate memory") {
		t.Errorf("injection_text = %q, want scored memory summary", text)
	}
	if _, ok := resp["fallback_count"]; !ok {
		t.Error("missing key fallback_count")
	}
	if _, ok := resp["has_fallback"]; !ok {
		t.Error("missing key has_fallback")
	}
}

func TestMemorySummaryPreviewUsesTurnSummaryBeforeStructuredPayload(t *testing.T) {
	raw := `{"turn_summary":"Luka confirms the island's point exchange rule after clearing a challenge.","archive_hint":{"wing":"wing_general","room":"hall_events"},"character_deltas":[{"name":"Luka"}],"kg_triples":[{"subject":"Luka","predicate":"clears","object":"challenge"}]}`
	got := memorySummaryPreview(raw)
	if !strings.Contains(got, "Luka confirms") {
		t.Fatalf("preview = %q, want turn_summary text", got)
	}
	for _, leaked := range []string{"archive_hint", "character_deltas", "kg_triples"} {
		if strings.Contains(got, leaked) {
			t.Fatalf("preview leaked structured extraction key %q: %q", leaked, got)
		}
	}
}

func TestMemorySummaryPreviewSkipsPollutedTurnSummaryObject(t *testing.T) {
	raw := `{"turn_summary":{"archive_hint":{"room":"hall_events"},"character_deltas":[{"name":"Luka"}]},"summary":"Fallback natural memory sentence."}`
	got := memorySummaryPreview(raw)
	if got != "Fallback natural memory sentence." {
		t.Fatalf("preview = %q, want clean fallback summary", got)
	}
	if strings.Contains(got, "archive_hint") || strings.Contains(got, "character_deltas") {
		t.Fatalf("preview leaked polluted turn_summary object: %q", got)
	}
}

func TestExplorerMemoriesSortsByTurnIndexAfterReplay(t *testing.T) {
	base := time.Date(2026, 6, 24, 12, 0, 0, 0, time.UTC)
	fake := &memoryFakeStore{
		memories: []store.Memory{
			{ID: 15, ChatSessionID: "sess-replay", TurnIndex: 2, SummaryJSON: `{"turn_summary":"turn two regenerated later"}`, CreatedAt: base.Add(10 * time.Minute)},
			{ID: 13, ChatSessionID: "sess-replay", TurnIndex: 7, SummaryJSON: `{"turn_summary":"turn seven"}`, CreatedAt: base},
			{ID: 10, ChatSessionID: "sess-replay", TurnIndex: 6, SummaryJSON: `{"turn_summary":"turn six"}`, CreatedAt: base.Add(1 * time.Minute)},
		},
	}
	cfg := config.Default()
	srv := NewServer(cfg)
	srv.Store = fake

	mux := http.NewServeMux()
	srv.RegisterRoutes(mux)

	req := httptest.NewRequest(http.MethodGet, "/explorer/memories?chat_session_id=sess-replay&limit=10", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200: %s", rec.Code, rec.Body.String())
	}
	var resp map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	items, ok := resp["items"].([]any)
	if !ok || len(items) != 3 {
		t.Fatalf("items = %#v, want 3 rows", resp["items"])
	}
	wantTurns := []float64{7, 6, 2}
	for i, want := range wantTurns {
		item, _ := items[i].(map[string]any)
		if item["source_turn"] != want {
			t.Fatalf("items[%d].source_turn = %v, want %v; items=%#v", i, item["source_turn"], want, items)
		}
	}
	first := items[0].(map[string]any)
	if first["summary_preview"] != "turn seven" {
		t.Fatalf("first summary_preview = %q, want turn seven", first["summary_preview"])
	}
}

func TestSeq18HybridSearchScoresExposeKeywordSoftBiasAndStopwordGuard(t *testing.T) {
	fake := &memoryFakeStore{
		memories: []store.Memory{
			{
				ID:            21,
				ChatSessionID: "sess-hy",
				TurnIndex:     8,
				SummaryJSON:   `{"summary":"Mira returns to the observatory rebellion vault","speaker":"Mira","location":"observatory","storyline":"rebellion"}`,
				PlaceWing:     "observatory",
				Importance:    0.4,
			},
			{
				ID:            22,
				ChatSessionID: "sess-hy",
				TurnIndex:     9,
				SummaryJSON:   `{"summary":"the and of to filler memory"}`,
				Importance:    0.9,
			},
		},
	}
	cfg := config.Default()
	srv := NewServer(cfg)
	srv.Store = fake

	mux := http.NewServeMux()
	srv.RegisterRoutes(mux)

	body := `{"user_input":"the Mira and observatory rebellion","chat_session_id":"sess-hy","top_k":2}`
	req := httptest.NewRequest(http.MethodPost, "/search", strings.NewReader(body))
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
	items, ok := resp["items"].([]any)
	if !ok || len(items) == 0 {
		t.Fatalf("items = %T/%v, want non-empty array", resp["items"], resp["items"])
	}
	first, ok := items[0].(map[string]any)
	if !ok {
		t.Fatalf("first item is not an object: %T", items[0])
	}
	if first["id"] != float64(21) {
		t.Fatalf("first item id = %v, want hybrid-matching memory 21", first["id"])
	}
	if first["hybrid_baseline_policy_version"] != "hy1a.v1" {
		t.Fatalf("hybrid policy = %v, want hy1a.v1", first["hybrid_baseline_policy_version"])
	}
	if first["soft_bias_policy_version"] != "hy1b.v1" {
		t.Fatalf("soft bias policy = %v, want hy1b.v1", first["soft_bias_policy_version"])
	}
	if got, ok := first["keyword_overlap_score"].(float64); !ok || got <= 0 {
		t.Fatalf("keyword_overlap_score = %v, want > 0", first["keyword_overlap_score"])
	}
	if got, ok := first["soft_bias_score"].(float64); !ok || got != 0.12 {
		t.Fatalf("soft_bias_score = %v, want capped 0.12", first["soft_bias_score"])
	}
	terms, ok := first["keyword_overlap_terms"].([]any)
	if !ok {
		t.Fatalf("keyword_overlap_terms = %T, want array", first["keyword_overlap_terms"])
	}
	joinedTerms := strings.Join(anySliceStrings(terms), ",")
	for _, filler := range []string{"the", "and"} {
		if strings.Contains(joinedTerms, filler) {
			t.Fatalf("keyword_overlap_terms = %v, should exclude stopword %q", terms, filler)
		}
	}
	for _, key := range []string{"semantic_rank_score", "hybrid_baseline_score", "speaker_bias_score", "location_bias_score", "storyline_bias_score", "speaker_bias_terms", "location_bias_terms", "storyline_bias_terms"} {
		if _, ok := first[key]; !ok {
			t.Fatalf("first item missing HY field %q", key)
		}
	}
	breakdown, ok := first["score_breakdown"].(map[string]any)
	if !ok {
		t.Fatalf("score_breakdown = %T, want object", first["score_breakdown"])
	}
	hybrid, ok := breakdown["hybrid"].(map[string]any)
	if !ok {
		t.Fatalf("score_breakdown.hybrid = %T, want object", breakdown["hybrid"])
	}
	if hybrid["hybrid_baseline_policy_version"] != "hy1a.v1" {
		t.Fatalf("score_breakdown.hybrid policy = %v", hybrid["hybrid_baseline_policy_version"])
	}
}

func TestSeq18HybridTailBudgetRescuesNearCutoffWithinTopK(t *testing.T) {
	items := scoredSearchMemoryItems([]store.Memory{
		{
			ID:            41,
			ChatSessionID: "sess-hy-tail",
			TurnIndex:     99,
			SummaryJSON:   `{"summary":"Mira observatory rebellion vault","speaker":"Mira","location":"observatory","storyline":"rebellion"}`,
			PlaceWing:     "observatory",
			Importance:    1,
		},
		{
			ID:            42,
			ChatSessionID: "sess-hy-tail",
			TurnIndex:     100,
			SummaryJSON:   `{"summary":"Mira observatory"}`,
			Importance:    1,
		},
		{
			ID:            43,
			ChatSessionID: "sess-hy-tail",
			TurnIndex:     1,
			SummaryJSON:   `{"summary":"Mira observatory","location":"observatory","storyline":"rebellion"}`,
			PlaceWing:     "observatory",
			Importance:    0,
		},
	}, "Mira observatory rebellion vault", 2, embeddingModelIdentity{Model: "text-embedding-3-small", Source: "test"})

	if len(items) != 2 {
		t.Fatalf("items len = %d, want same top_k budget 2", len(items))
	}
	first, ok := items[0].(map[string]any)
	if !ok {
		t.Fatalf("first item = %T, want object", items[0])
	}
	second, ok := items[1].(map[string]any)
	if !ok {
		t.Fatalf("second item = %T, want object", items[1])
	}
	if first["id"] != int64(41) {
		t.Fatalf("first id = %v, want stable top item 41", first["id"])
	}
	if second["id"] != int64(43) {
		t.Fatalf("second id = %v, want near-cutoff rescue item 43", second["id"])
	}
	if second["tail_budget_policy_version"] != "hy1d.v1" {
		t.Fatalf("tail policy = %v, want hy1d.v1", second["tail_budget_policy_version"])
	}
	if second["tail_budget_original_rank"] != 3 {
		t.Fatalf("tail original rank = %v, want 3", second["tail_budget_original_rank"])
	}
	if second["tail_budget_promoted"] != true {
		t.Fatalf("tail promoted = %v, want true", second["tail_budget_promoted"])
	}
	if second["tail_budget_reason"] != "keyword_soft_bias_stronger_than_cutline" {
		t.Fatalf("tail reason = %v", second["tail_budget_reason"])
	}
	if gap, ok := second["tail_budget_score_gap"].(float64); !ok || gap <= 0 {
		t.Fatalf("tail score gap = %v, want positive float64", second["tail_budget_score_gap"])
	}
}

func anySliceStrings(values []any) []string {
	out := make([]string, 0, len(values))
	for _, value := range values {
		if s, ok := value.(string); ok {
			out = append(out, s)
		}
	}
	return out
}

func TestSearchThousandTurnStoreBackedTopKRespectsRequest(t *testing.T) {
	// SEQ-09-P120 evidence: /search latency must not degrade with long-context profiles.
	// This test proves that even with 1000 memories, requested top_k is honored and
	// store-backed search stays under a bounded smoke threshold (2s).
	memories := make([]store.Memory, 0, 1000)
	for i := 1; i <= 1000; i++ {
		summary := fmt.Sprintf(`{"summary":"long session memory %d rooftop callback promise"}`, i)
		memories = append(memories, store.Memory{
			ID:            int64(i),
			ChatSessionID: "sess-1000",
			TurnIndex:     i,
			SummaryJSON:   summary,
			Importance:    0.5,
		})
	}
	fake := &memoryFakeStore{memories: memories}
	cfg := config.Default()
	srv := NewServer(cfg)
	srv.Store = fake

	mux := http.NewServeMux()
	srv.RegisterRoutes(mux)

	started := time.Now()
	body := `{"user_input":"rooftop callback promise","chat_session_id":"sess-1000","top_k":75}`
	req := httptest.NewRequest(http.MethodPost, "/search", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	elapsed := time.Since(started)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200: %s", rec.Code, rec.Body.String())
	}
	var resp map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	items, ok := resp["items"].([]any)
	if !ok {
		t.Fatalf("items is not an array: %T", resp["items"])
	}
	if len(items) != 75 {
		t.Fatalf("items len = %d, want requested top_k 75", len(items))
	}
	if resp["total_count"] != float64(1000) {
		t.Errorf("total_count = %v, want 1000", resp["total_count"])
	}
	if resp["memory_count"] != float64(75) {
		t.Errorf("memory_count = %v, want 75", resp["memory_count"])
	}
	if elapsed > 2*time.Second {
		t.Errorf("1000-turn store-backed search took %s; expected bounded smoke under 2s", elapsed)
	}
}

func TestStep29RegressionSearchTopKReturnsRequestedCountNotSingleTurn(t *testing.T) {
	memories := make([]store.Memory, 0, 12)
	for i := 1; i <= 12; i++ {
		memories = append(memories, store.Memory{
			ID:            int64(i),
			ChatSessionID: "sess-29-topk",
			TurnIndex:     i,
			SummaryJSON:   fmt.Sprintf(`{"summary":"topk regression anchor memory %02d"}`, i),
			Importance:    0.5,
		})
	}
	fake := &memoryFakeStore{memories: memories}
	cfg := config.Default()
	srv := NewServer(cfg)
	srv.Store = fake

	mux := http.NewServeMux()
	srv.RegisterRoutes(mux)

	body := `{"user_input":"topk regression anchor","chat_session_id":"sess-29-topk","top_k":10}`
	req := httptest.NewRequest(http.MethodPost, "/search", strings.NewReader(body))
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
	items, ok := resp["items"].([]any)
	if !ok {
		t.Fatalf("items is not an array: %T", resp["items"])
	}
	if len(items) != 10 {
		t.Fatalf("items len = %d, want requested top_k 10, not a single-turn result", len(items))
	}
	seenTurns := map[float64]bool{}
	for _, raw := range items {
		item, _ := raw.(map[string]any)
		seenTurns[item["turn_index"].(float64)] = true
	}
	if len(seenTurns) < 10 {
		t.Fatalf("items collapsed to fewer distinct turns than topK: items=%#v", items)
	}
	if resp["memory_count"] != float64(10) {
		t.Fatalf("memory_count = %v, want 10", resp["memory_count"])
	}
}

func TestSearchReturnsLongMemoryStoryCards(t *testing.T) {
	fake := &memoryFakeStore{
		memories: []store.Memory{
			{ID: 21, ChatSessionID: "sess-cards", TurnIndex: 7, SummaryJSON: `{"entity_type":"person","name":"Mira","summary":"Mira keeps the rooftop promise alive."}`, Importance: 0.9},
			{ID: 22, ChatSessionID: "sess-cards", TurnIndex: 8, SummaryJSON: `{"entity_type":"place","location":"School rooftop","summary":"The rooftop holds the confession memory."}`, PlaceWing: "school", PlaceRoom: "rooftop", Importance: 0.8},
			{ID: 23, ChatSessionID: "sess-cards", TurnIndex: 9, SummaryJSON: `{"entity_type":"item","item":"brass key","summary":"The brass key opens the archive stairwell."}`, Importance: 0.7},
		},
	}
	cfg := config.Default()
	srv := NewServer(cfg)
	srv.Store = fake

	mux := http.NewServeMux()
	srv.RegisterRoutes(mux)

	body := `{"user_input":"Mira rooftop brass key","chat_session_id":"sess-cards","top_k":10}`
	req := httptest.NewRequest(http.MethodPost, "/search", strings.NewReader(body))
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
	cards, ok := resp["story_cards"].([]any)
	if !ok {
		t.Fatalf("story_cards is not an array: %T", resp["story_cards"])
	}
	if len(cards) != 3 {
		t.Fatalf("story_cards len = %d, want 3: %#v", len(cards), cards)
	}
	seenTypes := map[string]bool{}
	for _, raw := range cards {
		card, ok := raw.(map[string]any)
		if !ok {
			t.Fatalf("story card is not an object: %T", raw)
		}
		cardType, _ := card["card_type"].(string)
		seenTypes[cardType] = true
		for _, key := range []string{"title", "summary", "source_turn", "display_mode"} {
			if _, ok := card[key]; !ok {
				t.Fatalf("story card missing %q: %#v", key, card)
			}
		}
		if card["display_mode"] != "story_card" {
			t.Fatalf("display_mode = %v, want story_card", card["display_mode"])
		}
	}
	for _, cardType := range []string{"person", "place", "item"} {
		if !seenTypes[cardType] {
			t.Fatalf("missing story card type %q in %#v", cardType, cards)
		}
	}
	if resp["story_cards_count"] != float64(3) {
		t.Fatalf("story_cards_count = %v, want 3", resp["story_cards_count"])
	}
}

func TestSearchFallsBackToChatLogsWhenMemoryResultsAreSparse(t *testing.T) {
	fake := &memoryFakeStore{
		memories: []store.Memory{
			{ID: 10, ChatSessionID: "sess-fb", TurnIndex: 1, SummaryJSON: `{"summary":"garden promise"}`, Importance: 0.7},
		},
		chatLogs: []store.ChatLog{
			{ID: 20, ChatSessionID: "sess-fb", TurnIndex: 2, Role: "assistant", Content: "The brass key was hidden under the library gate."},
			{ID: 21, ChatSessionID: "sess-fb", TurnIndex: 3, Role: "assistant", Content: "Unrelated weather note."},
		},
	}
	cfg := config.Default()
	srv := NewServer(cfg)
	srv.Store = fake

	mux := http.NewServeMux()
	srv.RegisterRoutes(mux)

	body := `{"user_input":"brass key","chat_session_id":"sess-fb","top_k":3}`
	req := httptest.NewRequest(http.MethodPost, "/search", strings.NewReader(body))
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
	if resp["fallback_count"] != float64(1) {
		t.Fatalf("fallback_count = %v, want 1", resp["fallback_count"])
	}
	if resp["has_fallback"] != true {
		t.Fatalf("has_fallback = %v, want true", resp["has_fallback"])
	}
	items := resp["items"].([]any)
	foundFallback := false
	for _, raw := range items {
		item := raw.(map[string]any)
		if item["source"] == "chat_log" {
			foundFallback = true
			if item["similarity_score"].(float64) < 0.65 {
				t.Errorf("fallback similarity_score = %v, want >= 0.65", item["similarity_score"])
			}
			if !strings.Contains(item["content_preview"].(string), "brass key") {
				t.Errorf("content_preview = %q, want brass key excerpt", item["content_preview"])
			}
		}
	}
	if !foundFallback {
		t.Fatal("expected chat_log fallback item")
	}
	if text, _ := resp["injection_text"].(string); !strings.Contains(text, "brass key") {
		t.Errorf("injection_text = %q, want fallback excerpt", text)
	}
}

func TestSearchVectorFirstHydratesMemoryWithoutLexicalTopKFill(t *testing.T) {
	embeddingServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{"model":"embed-model","data":[{"embedding":[0.1,0.2]}]}`)
	}))
	defer embeddingServer.Close()

	fake := &memoryFakeStore{
		memories: []store.Memory{
			{ID: 1, ChatSessionID: "sess-vector-search", TurnIndex: 1, SummaryJSON: `{"summary":"Old semantic shrine memory"}`, Importance: 0.2},
			{ID: 2, ChatSessionID: "sess-vector-search", TurnIndex: 9, SummaryJSON: `{"summary":"Recent lexical key memory"}`, Importance: 0.9},
		},
		chatLogs: []store.ChatLog{
			{ID: 31, ChatSessionID: "sess-vector-search", TurnIndex: 10, Role: "assistant", Content: "The key appears in a recent chat log but must not fill vector topK."},
		},
	}
	cfg := config.Default()
	cfg.ChromaEndpoint = "http://127.0.0.1:8000"
	cfg.Readiness.ChromaConfigured = true
	srv := NewServer(cfg)
	srv.Store = fake
	vectorFake := &fakeVectorStore{
		healthSnapshot: vector.HealthSnapshot{Status: "ok", TotalCount: 2, ModelReady: true},
		searchResults: []vector.VectorDocument{
			{ID: "memory:sess-vector-search:1", Tier: "memory", ChatSessionID: "sess-vector-search", SourceTable: "memories", SourceRowID: "1", DocumentText: "semantic old shrine"},
		},
	}
	srv.Vector = vectorFake
	srv.RuntimeConfig.Synced = true
	srv.RuntimeConfig.EmbeddingProvider = "openai"
	srv.RuntimeConfig.EmbeddingAPIKey = "embed-key"
	srv.RuntimeConfig.EmbeddingEndpoint = embeddingServer.URL
	srv.RuntimeConfig.EmbeddingModel = "embed-model"
	srv.RuntimeConfig.EmbeddingTimeoutSec = 30

	mux := http.NewServeMux()
	srv.RegisterRoutes(mux)

	body := `{"user_input":"key","chat_session_id":"sess-vector-search","top_k":2}`
	req := httptest.NewRequest(http.MethodPost, "/search", strings.NewReader(body))
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
	if resp["retrieval_mode"] != "vector_first" {
		t.Fatalf("retrieval_mode = %v, want vector_first; resp=%#v", resp["retrieval_mode"], resp)
	}
	if vectorFake.searchCalls != 1 {
		t.Fatalf("vector search calls = %d, want 1", vectorFake.searchCalls)
	}
	items, ok := resp["items"].([]any)
	if !ok || len(items) != 1 {
		t.Fatalf("items = %T/%v, want only hydrated Chroma result", resp["items"], resp["items"])
	}
	first, ok := items[0].(map[string]any)
	if !ok {
		t.Fatalf("first item = %T, want object", items[0])
	}
	if first["id"] != float64(1) || first["lane"] != "vector_relevant" || first["vector_hydrated"] != true {
		t.Fatalf("first item = %#v, want hydrated vector memory id 1", first)
	}
	if _, ok := first["vector_rank_score"]; !ok {
		t.Fatalf("first item missing vector_rank_score: %#v", first)
	}
	trace, ok := resp["vector_search"].(map[string]any)
	if !ok {
		t.Fatalf("vector_search = %T, want object", resp["vector_search"])
	}
	if trace["vector_memory_count"] != float64(1) {
		t.Fatalf("vector_memory_count = %v, want 1", trace["vector_memory_count"])
	}
	if trace["lexical_fill_enabled"] != false || trace["vector_recall_ready"] != true {
		t.Fatalf("vector-first trace should block lexical topK fill: %#v", trace)
	}
	if trace["chat_log_fallback_enabled"] != false {
		t.Fatalf("chat_log_fallback_enabled = %v, want false in vector-first /search trace", trace["chat_log_fallback_enabled"])
	}
	if resp["fallback_count"] != float64(0) || resp["has_fallback"] != false {
		t.Fatalf("chat log fallback should not fill vector-first results: fallback_count=%v has_fallback=%v", resp["fallback_count"], resp["has_fallback"])
	}
}

func TestSearchVectorDegradedDoesNotLexicalFillAfterVectorAttempt(t *testing.T) {
	embeddingServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{"model":"embed-model","data":[{"embedding":[0.1,0.2]}]}`)
	}))
	defer embeddingServer.Close()

	fake := &memoryFakeStore{
		memories: []store.Memory{
			{ID: 4, ChatSessionID: "sess-vector-degraded", TurnIndex: 4, SummaryJSON: `{"summary":"Lexical fallback brass lantern memory"}`, Importance: 0.8},
		},
	}
	cfg := config.Default()
	cfg.ChromaEndpoint = "http://127.0.0.1:8000"
	cfg.Readiness.ChromaConfigured = true
	srv := NewServer(cfg)
	srv.Store = fake
	vectorFake := &fakeVectorStore{
		healthSnapshot: vector.HealthSnapshot{Status: "ok", TotalCount: 1, ModelReady: true},
		searchErr:      vector.ErrNotFound,
	}
	srv.Vector = vectorFake
	srv.RuntimeConfig.Synced = true
	srv.RuntimeConfig.EmbeddingProvider = "openai"
	srv.RuntimeConfig.EmbeddingAPIKey = "embed-key"
	srv.RuntimeConfig.EmbeddingEndpoint = embeddingServer.URL
	srv.RuntimeConfig.EmbeddingModel = "embed-model"
	srv.RuntimeConfig.EmbeddingTimeoutSec = 30

	mux := http.NewServeMux()
	srv.RegisterRoutes(mux)

	body := `{"user_input":"brass lantern","chat_session_id":"sess-vector-degraded","top_k":3}`
	req := httptest.NewRequest(http.MethodPost, "/search", strings.NewReader(body))
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
	if resp["retrieval_mode"] != "vector_degraded_no_fill" {
		t.Fatalf("retrieval_mode = %v, want vector_degraded_no_fill; resp=%#v", resp["retrieval_mode"], resp)
	}
	if reason := strings.TrimSpace(fmt.Sprint(resp["retrieval_fallback_reason"])); reason == "" || reason == "<nil>" {
		t.Fatalf("retrieval_fallback_reason = %v, want non-empty degraded reason", resp["retrieval_fallback_reason"])
	}
	items, ok := resp["items"].([]any)
	if !ok || len(items) != 0 {
		t.Fatalf("items = %T/%v, want no lexical fill after vector attempt", resp["items"], resp["items"])
	}
	trace, ok := resp["vector_search"].(map[string]any)
	if !ok {
		t.Fatalf("vector_search = %T, want object", resp["vector_search"])
	}
	if trace["search_result"] != "not_found" {
		t.Fatalf("search_result = %v, want not_found", trace["search_result"])
	}
	if trace["lexical_fill_enabled"] != false || trace["vector_recall_attempted"] != true {
		t.Fatalf("vector degraded trace should block lexical fill after attempt: %#v", trace)
	}
}

func TestStep29RegressionSearchVectorFirstDefeatsRecentOnlyRanking(t *testing.T) {
	embeddingServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{"model":"embed-model","data":[{"embedding":[0.3,0.4]}]}`)
	}))
	defer embeddingServer.Close()

	memories := []store.Memory{
		{ID: 1, ChatSessionID: "sess-29-vector-recent", TurnIndex: 1, SummaryJSON: `{"summary":"Ancient shrine vow is the scene's actual semantic answer."}`, Importance: 0.1},
	}
	for i := 2; i <= 8; i++ {
		memories = append(memories, store.Memory{
			ID:            int64(i),
			ChatSessionID: "sess-29-vector-recent",
			TurnIndex:     40 + i,
			SummaryJSON:   fmt.Sprintf(`{"summary":"Recent unrelated market chatter %d"}`, i),
			Importance:    0.9,
		})
	}
	fake := &memoryFakeStore{memories: memories}
	cfg := config.Default()
	cfg.ChromaEndpoint = "http://127.0.0.1:8000"
	cfg.Readiness.ChromaConfigured = true
	srv := NewServer(cfg)
	srv.Store = fake
	vectorFake := &fakeVectorStore{
		healthSnapshot: vector.HealthSnapshot{Status: "ok", TotalCount: 8, ModelReady: true},
		searchResults: []vector.VectorDocument{
			{ID: "memory:sess-29-vector-recent:1", Tier: "memory", ChatSessionID: "sess-29-vector-recent", SourceTable: "memories", SourceRowID: "1", DocumentText: "ancient shrine vow"},
		},
	}
	srv.Vector = vectorFake
	srv.RuntimeConfig.Synced = true
	srv.RuntimeConfig.EmbeddingProvider = "openai"
	srv.RuntimeConfig.EmbeddingAPIKey = "embed-key"
	srv.RuntimeConfig.EmbeddingEndpoint = embeddingServer.URL
	srv.RuntimeConfig.EmbeddingModel = "embed-model"
	srv.RuntimeConfig.EmbeddingTimeoutSec = 30

	mux := http.NewServeMux()
	srv.RegisterRoutes(mux)

	body := `{"user_input":"ancient shrine vow","chat_session_id":"sess-29-vector-recent","top_k":3}`
	req := httptest.NewRequest(http.MethodPost, "/search", strings.NewReader(body))
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
	items, ok := resp["items"].([]any)
	if !ok || len(items) != 1 {
		t.Fatalf("items = %T/%v, want only Chroma-selected semantic result", resp["items"], resp["items"])
	}
	first := items[0].(map[string]any)
	if first["id"] != float64(1) || first["lane"] != "vector_relevant" || first["vector_hydrated"] != true {
		t.Fatalf("first item = %#v, want old semantic vector memory before recent-only candidates", first)
	}
	if resp["retrieval_mode"] != "vector_first" {
		t.Fatalf("retrieval_mode = %v, want vector_first", resp["retrieval_mode"])
	}
	trace := resp["vector_search"].(map[string]any)
	if trace["vector_memory_count"] != float64(1) || trace["lexical_memory_count"] != float64(0) || trace["lexical_fill_enabled"] != false {
		t.Fatalf("vector/lexical counts mismatch: %#v", trace)
	}
}

// Test 2: GET /explorer/memories returns Store-backed data
func TestExplorerMemoriesReturnsStoreData(t *testing.T) {
	fake := &memoryFakeStore{
		memories: []store.Memory{
			{ID: 20, ChatSessionID: "sess-2", TurnIndex: 1, SummaryJSON: `{"summary":"m1"}`, Importance: 0.9},
		},
	}
	cfg := config.Default()
	srv := NewServer(cfg)
	srv.Store = fake

	mux := http.NewServeMux()
	srv.RegisterRoutes(mux)

	req := httptest.NewRequest(http.MethodGet, "/explorer/memories?chat_session_id=sess-2", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200: %s", rec.Code, rec.Body.String())
	}
	var resp map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	items, ok := resp["items"].([]any)
	if !ok {
		t.Fatalf("items is not an array: %T", resp["items"])
	}
	if len(items) != 1 {
		t.Errorf("items count = %d, want 1", len(items))
	}
	if resp["total"] != float64(1) {
		t.Errorf("total = %v, want 1", resp["total"])
	}
}

// Test 3: GET /explorer/direct-evidence returns Store-backed data
func TestExplorerDirectEvidenceReturnsStoreData(t *testing.T) {
	fake := &memoryFakeStore{
		evidenceItems: []store.DirectEvidence{
			{ID: 30, ChatSessionID: "sess-3", EvidenceKind: "fact", EvidenceText: "evidence text that is longer than one hundred and twenty characters so that we can verify the preview truncation dots and more", ArchiveState: "pending_capture", CaptureVerification: "verified", CaptureStage: "critic_extract", CommittedGate: "", LineageJSON: `{"origin":"critic_extract","bucket":"direct_archive"}`, SourceMessageIDsJSON: `["msg-1","msg-5"]`, SourceHash: "sha256:direct-evidence-fixture", SourceTurnStart: 1, SourceTurnEnd: 5, TurnAnchor: 3, RepairNeeded: false, Tombstoned: false, SupersededByID: 0},
		},
		auditLogs: []store.AuditLog{
			{ID: 11, EventType: "critic_ingest_trace", ChatSessionID: "sess-3", DetailsJSON: `{"surface":"direct_evidence","trace":{"elapsed_ms":8.669,"inserted":2,"skipped":1,"write_chars":93}}`},
			{ID: 10, EventType: "critic_ingest_trace", ChatSessionID: "other", DetailsJSON: `{"surface":"direct_evidence","trace":{"elapsed_ms":999,"inserted":9,"skipped":9,"write_chars":999}}`},
		},
	}
	cfg := config.Default()
	srv := NewServer(cfg)
	srv.Store = fake

	mux := http.NewServeMux()
	srv.RegisterRoutes(mux)

	req := httptest.NewRequest(http.MethodGet, "/explorer/direct-evidence?chat_session_id=sess-3", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200: %s", rec.Code, rec.Body.String())
	}
	var resp map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	items, ok := resp["items"].([]any)
	if !ok {
		t.Fatalf("items is not an array: %T", resp["items"])
	}
	if len(items) != 1 {
		t.Errorf("items count = %d, want 1", len(items))
	}
	if resp["total"] != float64(1) {
		t.Errorf("total = %v, want 1", resp["total"])
	}
	it := items[0].(map[string]any)
	if it["evidence_preview"] != "evidence text that is longer than one hundred and twenty characters so that we can verify the preview truncation dots an..." {
		t.Errorf("evidence_preview mismatch: got %q", it["evidence_preview"])
	}
	if it["committed_gate"] != nil {
		t.Errorf("committed_gate = %v, want nil", it["committed_gate"])
	}
	if it["normalized_committed_gate"] != "finalize" {
		t.Errorf("normalized_committed_gate = %v, want finalize", it["normalized_committed_gate"])
	}
	if it["archive_bucket"] != "pending_capture" {
		t.Errorf("archive_bucket = %v, want pending_capture", it["archive_bucket"])
	}
	if it["retention_ttl_turns"] != float64(180) {
		t.Errorf("retention_ttl_turns = %v, want 180", it["retention_ttl_turns"])
	}
	if it["source_message_ids"] == nil {
		t.Error("source_message_ids is nil")
	}
	sourceIDs := it["source_message_ids"].([]any)
	if len(sourceIDs) != 2 || sourceIDs[0] != "msg-1" || sourceIDs[1] != "msg-5" {
		t.Fatalf("source_message_ids = %#v, want msg-1/msg-5 lineage", sourceIDs)
	}
	if it["source_turn_start"] != float64(1) || it["source_turn_end"] != float64(5) {
		t.Fatalf("source turn range = %v-%v, want 1-5", it["source_turn_start"], it["source_turn_end"])
	}
	if it["source_hash"] != "sha256:direct-evidence-fixture" {
		t.Fatalf("source_hash = %v, want fixture hash", it["source_hash"])
	}
	if it["lineage"] == nil {
		t.Error("lineage is nil")
	}
	lineage := it["lineage"].(map[string]any)
	if lineage["origin"] != "critic_extract" || lineage["bucket"] != "direct_archive" {
		t.Fatalf("lineage = %#v, want direct archive lineage", lineage)
	}
	if it["excluded_from_current_truth"] != false {
		t.Errorf("excluded_from_current_truth = %v, want false", it["excluded_from_current_truth"])
	}
	if it["turn_anchor"] != float64(3) {
		t.Errorf("turn_anchor = %v, want 3", it["turn_anchor"])
	}
	if resp["cost_measurement"] == nil {
		t.Error("cost_measurement is nil")
	}
	cm := resp["cost_measurement"].(map[string]any)
	write := cm["direct_evidence_write"].(map[string]any)
	if write["sample_count"] != float64(1) {
		t.Errorf("direct_evidence_write.sample_count = %v, want 1", write["sample_count"])
	}
	if write["avg_latency_ms"] != 8.669 {
		t.Errorf("direct_evidence_write.avg_latency_ms = %v, want 8.669", write["avg_latency_ms"])
	}
	if write["avg_write_chars"] != float64(93) {
		t.Errorf("direct_evidence_write.avg_write_chars = %v, want 93", write["avg_write_chars"])
	}
	rq := cm["repair_queue"].(map[string]any)
	if rq["queue_count"] != float64(0) {
		t.Errorf("repair_queue.queue_count = %v, want 0", rq["queue_count"])
	}
	contract := resp["state_contract"].(map[string]any)
	if contract["conflict_resolution_policy_version"] != "ea1h.v1" || contract["conflict_confidence_policy_version"] != "ea1i.v1" {
		t.Fatalf("conflict policy contract mismatch: %#v", contract)
	}
	cr := it["conflict_resolution"].(map[string]any)
	if cr["policy_version"] != "ea1h.v1" || cr["confidence_policy_version"] != "ea1i.v1" {
		t.Fatalf("conflict resolution policy mismatch: %#v", cr)
	}
	if cr["classification"] != "state_transition" || cr["route"] != "superseded" {
		t.Fatalf("conflict resolution = %#v, want state_transition/superseded", cr)
	}
}

func TestExplorerDirectEvidenceConflictStateMachineAndRetention(t *testing.T) {
	fake := &memoryFakeStore{
		evidenceItems: []store.DirectEvidence{
			{ID: 101, ChatSessionID: "sess-conflict", EvidenceKind: "fact", EvidenceText: "Alice now trusts Bob.", ArchiveState: "verified_direct", CaptureVerification: "verified", LineageJSON: `{"conflict_class":"state_transition","confidence":0.92,"field_class":"relationship","importance_tier":"high"}`, SourceTurnStart: 10, SourceTurnEnd: 10, TurnAnchor: 10, CreatedAt: time.Date(2026, 6, 1, 0, 0, 4, 0, time.UTC)},
			{ID: 102, ChatSessionID: "sess-conflict", EvidenceKind: "fact", EvidenceText: "Alice is secretly another person.", ArchiveState: "verified_direct", CaptureVerification: "verified", LineageJSON: `{"conflict_class":"hard_contradiction","confidence":0.72,"field_class":"identity","high_impact":true}`, SourceTurnStart: 11, SourceTurnEnd: 11, TurnAnchor: 11, CreatedAt: time.Date(2026, 6, 1, 0, 0, 3, 0, time.UTC)},
			{ID: 103, ChatSessionID: "sess-conflict", EvidenceKind: "fact", EvidenceText: "An older archive says Alice avoided Bob.", ArchiveState: "previous_archive", CaptureVerification: "verified", LineageJSON: `{"conflict_class":"parallel_context","confidence":0.75,"field_class":"relationship"}`, SourceTurnStart: 1, SourceTurnEnd: 1, TurnAnchor: 1, CreatedAt: time.Date(2026, 6, 1, 0, 0, 2, 0, time.UTC)},
			{ID: 104, ChatSessionID: "sess-conflict", EvidenceKind: "fact", EvidenceText: "A weak rumor says the trust scene was false.", ArchiveState: "pending_capture", CaptureVerification: "pending", LineageJSON: `{"conflict_class":"low_confidence_noise","confidence":0.22,"field_class":"rumor"}`, SourceTurnStart: 12, SourceTurnEnd: 12, TurnAnchor: 12, CreatedAt: time.Date(2026, 6, 1, 0, 0, 1, 0, time.UTC)},
			{ID: 105, ChatSessionID: "sess-conflict", EvidenceKind: "fact", EvidenceText: "A deleted turn once contradicted Alice's trust.", ArchiveState: "verified_direct", CaptureVerification: "verified", LineageJSON: `{"importance_tier":"critical"}`, SourceTurnStart: 13, SourceTurnEnd: 13, TurnAnchor: 13, Tombstoned: true, CreatedAt: time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC)},
		},
		chatLogs: []store.ChatLog{{ChatSessionID: "sess-conflict", TurnIndex: 20}},
	}
	cfg := config.Default()
	srv := NewServer(cfg)
	srv.Store = fake

	mux := http.NewServeMux()
	srv.RegisterRoutes(mux)

	req := httptest.NewRequest(http.MethodGet, "/explorer/direct-evidence?chat_session_id=sess-conflict&limit=10", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200: %s", rec.Code, rec.Body.String())
	}
	var resp map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	contract := resp["state_contract"].(map[string]any)
	if contract["conflict_resolution_policy_version"] != "ea1h.v1" || contract["conflict_confidence_policy_version"] != "ea1i.v1" {
		t.Fatalf("contract conflict policy mismatch: %#v", contract)
	}
	if _, ok := contract["conflict_confidence_thresholds"].(map[string]any); !ok {
		t.Fatalf("contract missing confidence thresholds: %#v", contract)
	}
	items := resp["items"].([]any)
	byID := map[float64]map[string]any{}
	for _, raw := range items {
		item := raw.(map[string]any)
		byID[item["id"].(float64)] = item
	}
	assertConflict := func(id float64, class, route string, manual bool) {
		item, ok := byID[id]
		if !ok {
			t.Fatalf("missing evidence id %.0f in %#v", id, byID)
		}
		cr := item["conflict_resolution"].(map[string]any)
		if cr["classification"] != class || cr["route"] != route || cr["requires_manual_review"] != manual {
			t.Fatalf("id %.0f conflict = %#v, want %s/%s/manual=%v", id, cr, class, route, manual)
		}
	}
	assertConflict(101, "state_transition", "superseded", false)
	assertConflict(102, "hard_contradiction", "manual_review", true)
	assertConflict(103, "parallel_context", "hold", false)
	assertConflict(104, "low_confidence_noise", "hold", false)
	assertConflict(105, "hard_contradiction", "tombstone", false)

	tombstone := byID[105]["conflict_resolution"].(map[string]any)
	if tombstone["retention_importance_tier"] != "critical" {
		t.Fatalf("tombstone retention tier = %v, want critical", tombstone["retention_importance_tier"])
	}
	if byID[105]["retention_ttl_turns"] != float64(240) || byID[105]["excluded_from_current_truth"] != true {
		t.Fatalf("tombstone retention/current truth fields mismatch: %#v", byID[105])
	}
}

// Test 4: GET /explorer/kg_triples returns Store-backed data
func TestExplorerKGTriplesReturnsStoreData(t *testing.T) {
	fake := &memoryFakeStore{
		kgTriples: []store.KGTriple{
			{ID: 40, ChatSessionID: "sess-4", Subject: "Alice", Predicate: "knows", Object: "Bob", ValidFrom: 1, ValidTo: 999, SourceTurn: 2},
		},
	}
	cfg := config.Default()
	srv := NewServer(cfg)
	srv.Store = fake

	mux := http.NewServeMux()
	srv.RegisterRoutes(mux)

	req := httptest.NewRequest(http.MethodGet, "/explorer/kg_triples?chat_session_id=sess-4", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200: %s", rec.Code, rec.Body.String())
	}
	var resp map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	items, ok := resp["items"].([]any)
	if !ok {
		t.Fatalf("items is not an array: %T", resp["items"])
	}
	if len(items) != 1 {
		t.Errorf("items count = %d, want 1", len(items))
	}
	if resp["total"] != float64(1) {
		t.Errorf("total = %v, want 1", resp["total"])
	}
	it := items[0].(map[string]any)
	if it["valid_from"] != float64(1) {
		t.Errorf("valid_from = %v, want 1", it["valid_from"])
	}
	if it["valid_to"] != float64(999) {
		t.Errorf("valid_to = %v, want 999", it["valid_to"])
	}
	if it["source_turn"] != float64(2) {
		t.Errorf("source_turn = %v, want 2", it["source_turn"])
	}
	if it["created_at"] != nil {
		t.Errorf("created_at = %v, want nil", it["created_at"])
	}
}

func TestExplorerKGTriplesNullableAndSort(t *testing.T) {
	now := time.Now().UTC()
	fake := &memoryFakeStore{
		kgTriples: []store.KGTriple{
			{ID: 1, ChatSessionID: "sess-4", Subject: "A", Predicate: "p1", Object: "o1", ValidFrom: 0, ValidTo: 0, SourceTurn: 0, CreatedAt: now.Add(-2 * time.Hour)},
			{ID: 2, ChatSessionID: "sess-4", Subject: "B", Predicate: "p2", Object: "o2", ValidFrom: 3, ValidTo: 0, SourceTurn: 1, CreatedAt: now.Add(-1 * time.Hour)},
		},
	}
	cfg := config.Default()
	srv := NewServer(cfg)
	srv.Store = fake

	mux := http.NewServeMux()
	srv.RegisterRoutes(mux)

	req := httptest.NewRequest(http.MethodGet, "/explorer/kg_triples?chat_session_id=sess-4", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200: %s", rec.Code, rec.Body.String())
	}
	var resp map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	items, ok := resp["items"].([]any)
	if !ok {
		t.Fatalf("items is not an array: %T", resp["items"])
	}
	if len(items) != 2 {
		t.Fatalf("items count = %d, want 2", len(items))
	}
	// Should be sorted by created_at DESC, id DESC -> ID 2 first, then ID 1
	first := items[0].(map[string]any)
	if first["id"] != float64(2) {
		t.Errorf("first id = %v, want 2", first["id"])
	}
	if first["valid_from"] != float64(3) {
		t.Errorf("first valid_from = %v, want 3", first["valid_from"])
	}
	if first["valid_to"] != nil {
		t.Errorf("first valid_to = %v, want nil", first["valid_to"])
	}
	if first["source_turn"] != float64(1) {
		t.Errorf("first source_turn = %v, want 1", first["source_turn"])
	}
	second := items[1].(map[string]any)
	if second["id"] != float64(1) {
		t.Errorf("second id = %v, want 1", second["id"])
	}
	if second["valid_from"] != nil {
		t.Errorf("second valid_from = %v, want nil", second["valid_from"])
	}
	if second["valid_to"] != nil {
		t.Errorf("second valid_to = %v, want nil", second["valid_to"])
	}
	if second["source_turn"] != nil {
		t.Errorf("second source_turn = %v, want nil", second["source_turn"])
	}
}

// Test 5: POST /kg/recall returns Store-backed data
func TestKGRecallReturnsTriplesFromStore(t *testing.T) {
	fake := &memoryFakeStore{
		kgTriples: []store.KGTriple{
			{ID: 50, ChatSessionID: "sess-5", Subject: "Cat", Predicate: "sits_on", Object: "Mat", ValidFrom: 0, ValidTo: 0, SourceTurn: 3},
		},
	}
	cfg := config.Default()
	srv := NewServer(cfg)
	srv.Store = fake

	mux := http.NewServeMux()
	srv.RegisterRoutes(mux)

	body := `{"chat_session_id":"sess-5","entities":["Cat","Mat"]}`
	req := httptest.NewRequest(http.MethodPost, "/kg/recall", strings.NewReader(body))
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
	items, ok := resp["items"].([]any)
	if !ok {
		t.Fatalf("items is not an array: %T", resp["items"])
	}
	if len(items) != 1 {
		t.Errorf("items count = %d, want 1", len(items))
	}
	if resp["count"] != float64(1) {
		t.Errorf("count = %v, want 1", resp["count"])
	}
	if resp["entities_received"] != float64(2) {
		t.Errorf("entities_received = %v, want 2", resp["entities_received"])
	}
}

func TestKGRecallMatchesMultilingualNormalizedEntityKeys(t *testing.T) {
	fake := &memoryFakeStore{
		kgTriples: []store.KGTriple{
			{ID: 60, ChatSessionID: "sess-f2", Subject: "민아", Predicate: "trusts", Object: "アキラ", SourceTurn: 4},
			{ID: 61, ChatSessionID: "sess-f2", Subject: "Unrelated", Predicate: "ignores", Object: "Nobody", SourceTurn: 4},
		},
	}
	cfg := config.Default()
	srv := NewServer(cfg)
	srv.Store = fake

	mux := http.NewServeMux()
	srv.RegisterRoutes(mux)

	body := `{"chat_session_id":"sess-f2","entities":["Mina","Akira"],"limit":10}`
	req := httptest.NewRequest(http.MethodPost, "/kg/recall", strings.NewReader(body))
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
	items, ok := resp["items"].([]any)
	if !ok {
		t.Fatalf("items is not an array: %T", resp["items"])
	}
	if len(items) != 1 {
		t.Fatalf("items count = %d, want 1: %#v", len(items), items)
	}
	item := items[0].(map[string]any)
	if item["subject"] != "민아" || item["object"] != "アキラ" {
		t.Fatalf("matched item = %#v, want multilingual KG triple", item)
	}
}

func TestKGRecallKeepsEnglishMatchesWithMultilingualEntityQuery(t *testing.T) {
	fake := &memoryFakeStore{
		kgTriples: []store.KGTriple{
			{ID: 71, ChatSessionID: "sess-f2-runtime", Subject: "Mina", Predicate: "trusts", Object: "Rowan", SourceTurn: 5},
			{ID: 70, ChatSessionID: "sess-f2-runtime", Subject: "\uC544\uD0A4\uB77C", Predicate: "guards", Object: "\u30A2\u30AD\u30E9", SourceTurn: 4},
			{ID: 69, ChatSessionID: "sess-f2-runtime", Subject: "Unrelated", Predicate: "ignores", Object: "Nobody", SourceTurn: 3},
		},
	}
	cfg := config.Default()
	srv := NewServer(cfg)
	srv.Store = fake

	mux := http.NewServeMux()
	srv.RegisterRoutes(mux)

	body := `{"chat_session_id":"sess-f2-runtime","entities":["Mina","Rowan","Akira","\uC544\uD0A4\uB77C","\u30A2\u30AD\u30E9"],"limit":10}`
	req := httptest.NewRequest(http.MethodPost, "/kg/recall", strings.NewReader(body))
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
	items, ok := resp["items"].([]any)
	if !ok {
		t.Fatalf("items is not an array: %T", resp["items"])
	}
	if len(items) != 2 {
		t.Fatalf("items count = %d, want 2: %#v", len(items), items)
	}
	seen := map[string]bool{}
	for _, raw := range items {
		item := raw.(map[string]any)
		seen[item["subject"].(string)+"->"+item["object"].(string)] = true
	}
	if !seen["Mina->Rowan"] {
		t.Fatalf("English KG recall match disappeared: %#v", items)
	}
	if !seen["\uC544\uD0A4\uB77C->\u30A2\u30AD\u30E9"] {
		t.Fatalf("multilingual KG recall match disappeared: %#v", items)
	}
}

func TestKGRecallFiltersExpiredTriplesWhenCurrentTurnProvided(t *testing.T) {
	fake := &memoryFakeStore{
		kgTriples: []store.KGTriple{
			{ID: 50, ChatSessionID: "sess-5", Subject: "Cat", Predicate: "sits_on", Object: "Mat", ValidFrom: 1, ValidTo: 3, SourceTurn: 1},
			{ID: 51, ChatSessionID: "sess-5", Subject: "Cat", Predicate: "guards", Object: "Mat", ValidFrom: 4, ValidTo: 0, SourceTurn: 4},
		},
	}
	cfg := config.Default()
	srv := NewServer(cfg)
	srv.Store = fake

	mux := http.NewServeMux()
	srv.RegisterRoutes(mux)

	body := `{"chat_session_id":"sess-5","entities":["Cat","Mat"],"current_turn":5}`
	req := httptest.NewRequest(http.MethodPost, "/kg/recall", strings.NewReader(body))
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
	items, ok := resp["items"].([]any)
	if !ok {
		t.Fatalf("items is not an array: %T", resp["items"])
	}
	if len(items) != 1 {
		t.Fatalf("items count = %d, want 1", len(items))
	}
	item := items[0].(map[string]any)
	if item["predicate"] != "guards" {
		t.Errorf("predicate = %v, want guards", item["predicate"])
	}
	if resp["current_turn"] != float64(5) {
		t.Errorf("current_turn = %v, want 5", resp["current_turn"])
	}
	if resp["expired_filtered"] != float64(1) {
		t.Errorf("expired_filtered = %v, want 1", resp["expired_filtered"])
	}
}

// Test 6: GET /retrieval-index/{sid} returns Store-backed document_count and status
func TestRetrievalIndexSnapshotReturnsStoreData(t *testing.T) {
	fake := &memoryFakeStore{
		memories: []store.Memory{
			{ID: 1, ChatSessionID: "sess-6", TurnIndex: 1},
			{ID: 2, ChatSessionID: "sess-6", TurnIndex: 2},
		},
		evidenceItems: []store.DirectEvidence{
			{ID: 3, ChatSessionID: "sess-6", EvidenceKind: "fact"},
		},
		kgTriples: []store.KGTriple{
			{ID: 4, ChatSessionID: "sess-6", Subject: "A"},
		},
	}
	cfg := config.Default()
	srv := NewServer(cfg)
	srv.Store = fake

	mux := http.NewServeMux()
	srv.RegisterRoutes(mux)

	req := httptest.NewRequest(http.MethodGet, "/retrieval-index/sess-6", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200: %s", rec.Code, rec.Body.String())
	}
	var resp map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if resp["document_count"] != float64(4) {
		t.Errorf("document_count = %v, want 4", resp["document_count"])
	}
	if resp["status"] != "ok" {
		t.Errorf("status = %v, want ok", resp["status"])
	}
	stc, ok := resp["source_type_counts"].(map[string]any)
	if !ok {
		t.Fatalf("source_type_counts is not an object: %T", resp["source_type_counts"])
	}
	if stc["memories"] != float64(2) {
		t.Errorf("source_type_counts.memories = %v, want 2", stc["memories"])
	}
	if stc["direct_evidence"] != float64(1) {
		t.Errorf("source_type_counts.direct_evidence = %v, want 1", stc["direct_evidence"])
	}
	if stc["kg_triples"] != float64(1) {
		t.Errorf("source_type_counts.kg_triples = %v, want 1", stc["kg_triples"])
	}
	tc, ok := resp["tier_counts"].(map[string]any)
	if !ok {
		t.Fatalf("tier_counts is not an object: %T", resp["tier_counts"])
	}
	if tc["memory"] != float64(2) {
		t.Errorf("tier_counts.memory = %v, want 2", tc["memory"])
	}
}

// Test 7: Disabled store returns safe fallback
func TestSearchWithNoopStoreReturnsEmptyNotError(t *testing.T) {
	cfg := config.Default()
	srv := NewServer(cfg)
	// Default is noop store which returns nil, nil

	mux := http.NewServeMux()
	srv.RegisterRoutes(mux)

	body := `{"user_input":"hello","chat_session_id":"sess-noop"}`
	req := httptest.NewRequest(http.MethodPost, "/search", strings.NewReader(body))
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
	if resp["memory_count"] != float64(0) {
		t.Errorf("memory_count = %v, want 0", resp["memory_count"])
	}
	items, ok := resp["items"].([]any)
	if !ok {
		t.Fatalf("items is not an array: %T", resp["items"])
	}
	if len(items) != 0 {
		t.Errorf("items count = %d, want 0", len(items))
	}
}

// Test 8: GET /chroma-shadow/preflight matches Python 0.8 shape
func TestChromaPreflightShape(t *testing.T) {
	cfg := config.Default()
	srv := NewServer(cfg)

	mux := http.NewServeMux()
	srv.RegisterRoutes(mux)

	req := httptest.NewRequest(http.MethodGet, "/chroma-shadow/preflight", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	var resp map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if resp["status"] != "ok" {
		t.Errorf("status = %v, want ok", resp["status"])
	}
	if resp["step"] != "17-C1" {
		t.Errorf("step = %v, want 17-C1", resp["step"])
	}
	if _, ok := resp["vector_health"]; ok {
		t.Errorf("vector_health should not be present in Python-aligned preflight")
	}
}

// Test 9: GET /retrieval-index/{sid}/source-row with type and id params
func TestRetrievalIndexSourceRowReturnsMemory(t *testing.T) {
	ts, _ := time.Parse(time.RFC3339, "2026-01-01T00:00:00Z")
	fake := &memoryFakeStore{
		memories: []store.Memory{
			{ID: 99, ChatSessionID: "sess-src", TurnIndex: 5, SummaryJSON: `{"s":"v"}`, Importance: 0.7, CreatedAt: ts},
		},
	}
	cfg := config.Default()
	srv := NewServer(cfg)
	srv.Store = fake

	mux := http.NewServeMux()
	srv.RegisterRoutes(mux)

	req := httptest.NewRequest(http.MethodGet, "/retrieval-index/sess-src/source-row?document_id=memory:99", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200: %s", rec.Code, rec.Body.String())
	}
	var resp map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	sr, ok := resp["source_row"].(map[string]any)
	if !ok {
		t.Fatalf("source_row is not an object: %T", resp["source_row"])
	}
	if sr["type"] != "memory" {
		t.Errorf("source_row.type = %v, want memory", sr["type"])
	}
	if sr["id"] != float64(99) {
		t.Errorf("source_row.id = %v, want 99", sr["id"])
	}
}

func TestRetrievalIndexSourceRowReturnsHierarchyRows(t *testing.T) {
	now := time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC)
	fake := &memoryFakeStore{
		episodes: []store.EpisodeSummary{
			{ID: 11, ChatSessionID: "sess-hier", FromTurn: 1, ToTurn: 20, SummaryText: "Episode gate", CreatedAt: now},
		},
		chapters: []store.ChapterSummary{
			{ID: 22, ChatSessionID: "sess-hier", FromTurn: 1, ToTurn: 60, ChapterIndex: 1, ChapterTitle: "Gate Chapter", SummaryText: "Chapter gate", CreatedAt: &now},
		},
		arcs: []store.ArcSummary{
			{ID: 33, ChatSessionID: "sess-hier", FromTurn: 1, ToTurn: 240, ArcIndex: 1, ArcName: "Gate Arc", ArcStatus: "active", ArcResumeText: "Arc gate", CreatedAt: &now},
		},
		sagas: []store.SagaDigest{
			{ID: 44, ChatSessionID: "sess-hier", FromTurn: 1, ToTurn: 960, EraLabel: "Gate Era", ResumePackText: "Saga gate", CreatedAt: &now},
		},
	}
	cfg := config.Default()
	srv := NewServer(cfg)
	srv.Store = fake

	mux := http.NewServeMux()
	srv.RegisterRoutes(mux)

	cases := []struct {
		docID       string
		sourceTable string
		tier        string
	}{
		{"episode:11", "episode_summaries", "episode"},
		{"chapter:22", "chapter_summaries", "chapter"},
		{"arc:33", "arc_summaries", "arc"},
		{"saga:44", "saga_digests", "saga"},
	}
	for _, tc := range cases {
		req := httptest.NewRequest(http.MethodGet, "/retrieval-index/sess-hier/source-row?document_id="+tc.docID, nil)
		rec := httptest.NewRecorder()
		mux.ServeHTTP(rec, req)
		if rec.Code != http.StatusOK {
			t.Fatalf("%s status = %d, want 200: %s", tc.docID, rec.Code, rec.Body.String())
		}
		var resp map[string]any
		if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
			t.Fatalf("decode %s: %v", tc.docID, err)
		}
		ref, ok := resp["source_ref"].(map[string]any)
		if !ok {
			t.Fatalf("%s source_ref is not an object: %T", tc.docID, resp["source_ref"])
		}
		if ref["source_table"] != tc.sourceTable {
			t.Errorf("%s source_table = %v, want %s", tc.docID, ref["source_table"], tc.sourceTable)
		}
		if ref["tier"] != tc.tier {
			t.Errorf("%s tier = %v, want %s", tc.docID, ref["tier"], tc.tier)
		}
	}
}

// Test 10: ErrNotEnabled store returns safe empty shapes, not 500
type notEnabledStore struct {
	memoryFakeStore
}

func (n *notEnabledStore) ListMemories(ctx context.Context, sid string, fromTurn, toTurn int) ([]store.Memory, error) {
	return nil, store.ErrNotEnabled
}
func (n *notEnabledStore) ListEvidence(ctx context.Context, sid string) ([]store.DirectEvidence, error) {
	return nil, store.ErrNotEnabled
}
func (n *notEnabledStore) ListKGTriples(ctx context.Context, sid string) ([]store.KGTriple, error) {
	return nil, store.ErrNotEnabled
}

func TestErrNotEnabledReturnsSafeFallbackNot500(t *testing.T) {
	cfg := config.Default()
	srv := NewServer(cfg)
	srv.Store = &notEnabledStore{}

	mux := http.NewServeMux()
	srv.RegisterRoutes(mux)

	// Test explorer/memories
	req := httptest.NewRequest(http.MethodGet, "/explorer/memories?chat_session_id=sess-ne", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("explorer/memories status = %d, want 200", rec.Code)
	}

	// Test explorer/direct-evidence
	req = httptest.NewRequest(http.MethodGet, "/explorer/direct-evidence?chat_session_id=sess-ne", nil)
	rec = httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("explorer/direct-evidence status = %d, want 200", rec.Code)
	}

	// Test explorer/kg_triples
	req = httptest.NewRequest(http.MethodGet, "/explorer/kg_triples?chat_session_id=sess-ne", nil)
	rec = httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("explorer/kg_triples status = %d, want 200", rec.Code)
	}

	// Test retrieval-index snapshot
	req = httptest.NewRequest(http.MethodGet, "/retrieval-index/sess-ne", nil)
	rec = httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("retrieval-index status = %d, want 200", rec.Code)
	}
}

// Test 11: GET /kg/recall without session returns empty items
func TestKGRecallGetWithoutSessionReturnsEmpty(t *testing.T) {
	cfg := config.Default()
	srv := NewServer(cfg)

	mux := http.NewServeMux()
	srv.RegisterRoutes(mux)

	req := httptest.NewRequest(http.MethodGet, "/kg/recall", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	var resp map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	items, ok := resp["items"].([]any)
	if !ok {
		t.Fatalf("items is not an array: %T", resp["items"])
	}
	if len(items) != 0 {
		t.Errorf("items count = %d, want 0", len(items))
	}
}

// Test 13: GET /explorer/chat_logs returns Store-backed data
func TestExplorerChatLogsReturnsStoreData(t *testing.T) {
	ts, _ := time.Parse(time.RFC3339, "2026-01-15T12:00:00Z")
	fake := &memoryFakeStore{
		chatLogs: []store.ChatLog{
			{ID: 1, ChatSessionID: "sess-cl", TurnIndex: 1, Role: "user", Content: "Hello there", CreatedAt: ts},
			{ID: 2, ChatSessionID: "sess-cl", TurnIndex: 2, Role: "assistant", Content: "Hi! How can I help you today?", CreatedAt: ts},
		},
	}
	cfg := config.Default()
	srv := NewServer(cfg)
	srv.Store = fake

	mux := http.NewServeMux()
	srv.RegisterRoutes(mux)

	req := httptest.NewRequest(http.MethodGet, "/explorer/chat_logs?chat_session_id=sess-cl", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200: %s", rec.Code, rec.Body.String())
	}
	var resp map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	items, ok := resp["items"].([]any)
	if !ok {
		t.Fatalf("items is not an array: %T", resp["items"])
	}
	if len(items) != 2 {
		t.Errorf("items count = %d, want 2", len(items))
	}
	if resp["total"].(float64) != 2 {
		t.Errorf("total = %v, want 2", resp["total"])
	}
	first, ok := items[0].(map[string]any)
	if !ok {
		t.Fatalf("first item not object: %T", items[0])
	}
	if first["role"] != "assistant" {
		t.Errorf("first item role = %v, want assistant", first["role"])
	}
	if first["preview"] != "Hi! How can I help you today?" {
		t.Errorf("first item preview = %v, want Hi! How can I help you today?", first["preview"])
	}
}

// Test 14: GET /explorer/chat_logs without session returns empty
func TestExplorerChatLogsWithoutSessionReturnsEmpty(t *testing.T) {
	cfg := config.Default()
	srv := NewServer(cfg)

	mux := http.NewServeMux()
	srv.RegisterRoutes(mux)

	req := httptest.NewRequest(http.MethodGet, "/explorer/chat_logs", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	var resp map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	items, ok := resp["items"].([]any)
	if !ok {
		t.Fatalf("items is not an array: %T", resp["items"])
	}
	if len(items) != 0 {
		t.Errorf("items count = %d, want 0", len(items))
	}
}

// Test 15: GET /explorer/chat_logs respects limit/offset pagination
func TestExplorerChatLogsLimitOffset(t *testing.T) {
	ts, _ := time.Parse(time.RFC3339, "2026-01-15T12:00:00Z")
	logs := make([]store.ChatLog, 5)
	for i := 0; i < 5; i++ {
		logs[i] = store.ChatLog{ID: int64(i + 1), ChatSessionID: "sess-page", TurnIndex: i + 1, Role: "user", Content: "msg", CreatedAt: ts}
	}
	fake := &memoryFakeStore{chatLogs: logs}
	cfg := config.Default()
	srv := NewServer(cfg)
	srv.Store = fake

	mux := http.NewServeMux()
	srv.RegisterRoutes(mux)

	req := httptest.NewRequest(http.MethodGet, "/explorer/chat_logs?chat_session_id=sess-page&limit=2&offset=1", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200: %s", rec.Code, rec.Body.String())
	}
	var resp map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	items := resp["items"].([]any)
	if len(items) != 2 {
		t.Errorf("items count = %d, want 2", len(items))
	}
	if resp["total"].(float64) != 5 {
		t.Errorf("total = %v, want 5", resp["total"])
	}
	if resp["has_more"] != true {
		t.Errorf("has_more = %v, want true", resp["has_more"])
	}
}

// Test 16: GET /explorer/chapter_summaries returns chapter data from Store
func TestExplorerChapterSummariesReturnsChapterData(t *testing.T) {
	fake := &memoryFakeStore{
		chapters: []store.ChapterSummary{
			{ID: 100, ChatSessionID: "sess-ch", FromTurn: 1, ToTurn: 10, ChapterIndex: 1, ChapterTitle: "Chapter One", SummaryText: "Chapter one summary", ResumeText: "Resume chapter one"},
		},
	}
	cfg := config.Default()
	srv := NewServer(cfg)
	srv.Store = fake

	mux := http.NewServeMux()
	srv.RegisterRoutes(mux)

	req := httptest.NewRequest(http.MethodGet, "/explorer/chapter_summaries?chat_session_id=sess-ch", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200: %s", rec.Code, rec.Body.String())
	}
	var resp map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	items, ok := resp["items"].([]any)
	if !ok {
		t.Fatalf("items is not an array: %T", resp["items"])
	}
	if len(items) != 1 {
		t.Errorf("items count = %d, want 1", len(items))
	}
	first, ok := items[0].(map[string]any)
	if !ok {
		t.Fatalf("first item not object: %T", items[0])
	}
	if first["source"] != "chapter_summary" {
		t.Errorf("first item source = %v, want chapter_summary", first["source"])
	}
	if first["chapter_title"] != "Chapter One" {
		t.Errorf("first item chapter_title = %v, want Chapter One", first["chapter_title"])
	}
	if first["summary_text"] != "Chapter one summary" {
		t.Errorf("first item summary_text = %v, want Chapter one summary", first["summary_text"])
	}
}

func TestExplorerArcSummariesReturnsArcData(t *testing.T) {
	fake := &memoryFakeStore{
		arcs: []store.ArcSummary{
			{ID: 300, ChatSessionID: "sess-arc", FromTurn: 1, ToTurn: 40, ArcIndex: 2, ArcName: "Bridge Siege", ArcStatus: "active", CoreConflict: "Escape route is closing", ArcResumeText: "Resume arc"},
		},
	}
	cfg := config.Default()
	srv := NewServer(cfg)
	srv.Store = fake

	mux := http.NewServeMux()
	srv.RegisterRoutes(mux)

	req := httptest.NewRequest(http.MethodGet, "/explorer/arc_summaries?chat_session_id=sess-arc", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200: %s", rec.Code, rec.Body.String())
	}
	var resp map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	items := resp["items"].([]any)
	if len(items) != 1 {
		t.Fatalf("items count = %d, want 1", len(items))
	}
	first := items[0].(map[string]any)
	if first["source"] != "arc_summary" {
		t.Errorf("first item source = %v, want arc_summary", first["source"])
	}
	if first["arc_name"] != "Bridge Siege" {
		t.Errorf("first item arc_name = %v, want Bridge Siege", first["arc_name"])
	}
}

func TestExplorerSagaDigestsReturnsSagaData(t *testing.T) {
	fake := &memoryFakeStore{
		sagas: []store.SagaDigest{
			{ID: 400, ChatSessionID: "sess-saga", FromTurn: 1, ToTurn: 120, EraLabel: "Airport Era", SagaSummary: "The group establishes a fragile base.", ResumePackText: "Resume saga"},
		},
	}
	cfg := config.Default()
	srv := NewServer(cfg)
	srv.Store = fake

	mux := http.NewServeMux()
	srv.RegisterRoutes(mux)

	req := httptest.NewRequest(http.MethodGet, "/explorer/saga_digests?chat_session_id=sess-saga", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200: %s", rec.Code, rec.Body.String())
	}
	var resp map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	items := resp["items"].([]any)
	if len(items) != 1 {
		t.Fatalf("items count = %d, want 1", len(items))
	}
	first := items[0].(map[string]any)
	if first["source"] != "saga_digest" {
		t.Errorf("first item source = %v, want saga_digest", first["source"])
	}
	if first["era_label"] != "Airport Era" {
		t.Errorf("first item era_label = %v, want Airport Era", first["era_label"])
	}
}

// Test 17: GET /explorer/chapter_summaries falls back to GetResumePack chapter
func TestExplorerChapterSummariesResumePackFallback(t *testing.T) {
	chapterTime, _ := time.Parse(time.RFC3339, "2026-02-01T00:00:00Z")
	fake := &memoryFakeStore{
		resumePack: &store.ResumePack{
			PackStatus: "ok",
			Trigger:    "resume",
			Chapter: &store.ChapterSummary{
				ID:           200,
				FromTurn:     1,
				ToTurn:       20,
				ChapterIndex: 3,
				ChapterTitle: "The Beginning",
				ResumeText:   "Characters meet",
				SummaryText:  "Full chapter summary",
				CreatedAt:    &chapterTime,
			},
		},
	}
	cfg := config.Default()
	srv := NewServer(cfg)
	srv.Store = fake

	mux := http.NewServeMux()
	srv.RegisterRoutes(mux)

	req := httptest.NewRequest(http.MethodGet, "/explorer/chapter_summaries?chat_session_id=sess-rp", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200: %s", rec.Code, rec.Body.String())
	}
	var resp map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	items, ok := resp["items"].([]any)
	if !ok {
		t.Fatalf("items is not an array: %T", resp["items"])
	}
	if len(items) != 1 {
		t.Errorf("items count = %d, want 1", len(items))
	}
	first, ok := items[0].(map[string]any)
	if !ok {
		t.Fatalf("first item not object: %T", items[0])
	}
	if first["source"] != "resume_pack_chapter" {
		t.Errorf("first item source = %v, want resume_pack_chapter", first["source"])
	}
	if first["chapter_title"] != "The Beginning" {
		t.Errorf("first item chapter_title = %v, want The Beginning", first["chapter_title"])
	}
}

// Test 18: GET /explorer/chapter_summaries without session returns empty
func TestExplorerChapterSummariesWithoutSessionReturnsEmpty(t *testing.T) {
	cfg := config.Default()
	srv := NewServer(cfg)

	mux := http.NewServeMux()
	srv.RegisterRoutes(mux)

	req := httptest.NewRequest(http.MethodGet, "/explorer/chapter_summaries", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	var resp map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	items, ok := resp["items"].([]any)
	if !ok {
		t.Fatalf("items is not an array: %T", resp["items"])
	}
	if len(items) != 0 {
		t.Errorf("items count = %d, want 0", len(items))
	}
}

// Test 19: ErrNotEnabled on chat_logs/chapter_summaries returns safe 200
type notEnabledChatStore struct {
	memoryFakeStore
}

func (n *notEnabledChatStore) ListChatLogs(ctx context.Context, sid string, from, to int) ([]store.ChatLog, error) {
	return nil, store.ErrNotEnabled
}
func (n *notEnabledChatStore) ListEpisodeSummaries(ctx context.Context, chatSessionID string, limit, fromTurn, toTurn int) ([]store.EpisodeSummary, error) {
	return nil, store.ErrNotEnabled
}
func (n *notEnabledChatStore) GetResumePack(ctx context.Context, sid, trigger string) (*store.ResumePack, error) {
	return nil, store.ErrNotEnabled
}

func TestErrNotEnabledChatLogsAndChapterSummariesReturnsSafe200(t *testing.T) {
	cfg := config.Default()
	srv := NewServer(cfg)
	srv.Store = &notEnabledChatStore{}

	mux := http.NewServeMux()
	srv.RegisterRoutes(mux)

	// chat_logs
	req := httptest.NewRequest(http.MethodGet, "/explorer/chat_logs?chat_session_id=sess-ne2", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("explorer/chat_logs status = %d, want 200", rec.Code)
	}

	// chapter_summaries
	req = httptest.NewRequest(http.MethodGet, "/explorer/chapter_summaries?chat_session_id=sess-ne2", nil)
	rec = httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("explorer/chapter_summaries status = %d, want 200", rec.Code)
	}
}

// Test 20: POST /search excludes chat_log_count and effective_input_count (Python 0.8 parity)
func TestSearchExcludesChatLogAndEffectiveInputCounts(t *testing.T) {
	ts, _ := time.Parse(time.RFC3339, "2026-01-15T12:00:00Z")
	fake := &memoryFakeStore{
		chatLogs: []store.ChatLog{
			{ID: 1, ChatSessionID: "sess-src", TurnIndex: 1, Role: "user", Content: "hi", CreatedAt: ts},
			{ID: 2, ChatSessionID: "sess-src", TurnIndex: 2, Role: "assistant", Content: "hello", CreatedAt: ts},
		},
		effectiveInput: &store.EffectiveInput{ID: 1, ChatSessionID: "sess-src", TurnIndex: 1, EffectiveInput: "greet"},
	}
	cfg := config.Default()
	srv := NewServer(cfg)
	srv.Store = fake

	mux := http.NewServeMux()
	srv.RegisterRoutes(mux)

	body := `{"user_input":"hello","chat_session_id":"sess-src","top_k":5}`
	req := httptest.NewRequest(http.MethodPost, "/search", strings.NewReader(body))
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
	if _, ok := resp["chat_log_count"]; ok {
		t.Errorf("chat_log_count should not be present in Python-compatible response")
	}
	if _, ok := resp["effective_input_count"]; ok {
		t.Errorf("effective_input_count should not be present in Python-compatible response")
	}
}
func TestRetrievalIndexSnapshotIncludesChatLogCount(t *testing.T) {
	ts, _ := time.Parse(time.RFC3339, "2026-01-15T12:00:00Z")
	fake := &memoryFakeStore{
		chatLogs: []store.ChatLog{
			{ID: 1, ChatSessionID: "sess-ri", TurnIndex: 1, Role: "user", Content: "hi", CreatedAt: ts},
		},
	}
	cfg := config.Default()
	srv := NewServer(cfg)
	srv.Store = fake

	mux := http.NewServeMux()
	srv.RegisterRoutes(mux)

	req := httptest.NewRequest(http.MethodGet, "/retrieval-index/sess-ri", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200: %s", rec.Code, rec.Body.String())
	}
	var resp map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	stc, ok := resp["source_type_counts"].(map[string]any)
	if !ok {
		t.Fatalf("source_type_counts is not an object: %T", resp["source_type_counts"])
	}
	if stc["chat_logs"].(float64) != 1 {
		t.Errorf("source_type_counts.chat_logs = %v, want 1", stc["chat_logs"])
	}
	// Status should be "ok" when chat_logs exist
	if resp["status"] != "ok" {
		t.Errorf("status = %v, want ok", resp["status"])
	}
}

// Test 12: GET /kg/recall with session returns Store data
func TestKGRecallGetWithSessionReturnsStoreData(t *testing.T) {
	fake := &memoryFakeStore{
		kgTriples: []store.KGTriple{
			{ID: 60, ChatSessionID: "sess-kg", Subject: "X", Predicate: "rel", Object: "Y"},
		},
	}
	cfg := config.Default()
	srv := NewServer(cfg)
	srv.Store = fake

	mux := http.NewServeMux()
	srv.RegisterRoutes(mux)

	req := httptest.NewRequest(http.MethodGet, "/kg/recall?chat_session_id=sess-kg", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200: %s", rec.Code, rec.Body.String())
	}
	var resp map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	items, ok := resp["items"].([]any)
	if !ok {
		t.Fatalf("items is not an array: %T", resp["items"])
	}
	if len(items) != 1 {
		t.Errorf("items count = %d, want 1", len(items))
	}
}

// fakeVectorStore implements vector.VectorStore for chroma-shadow R1 tests.
type fakeVectorStore struct {
	healthSnapshot vector.HealthSnapshot
	healthErr      error
	countResult    int
	countErr       error
	searchResults  []vector.VectorDocument
	searchErr      error
	searchCalls    int
	searchLimit    int
	searchFilter   string
	searchVector   []float32
	deleteDocIDs   []string
	deleteDocErr   error
}

func (f *fakeVectorStore) Search(ctx context.Context, sessionID string, v []float32, limit int, filter string) ([]vector.VectorDocument, error) {
	f.searchCalls++
	f.searchLimit = limit
	f.searchFilter = filter
	f.searchVector = append([]float32(nil), v...)
	if f.searchErr != nil {
		return nil, f.searchErr
	}
	if f.searchResults != nil {
		return f.searchResults, nil
	}
	return nil, vector.ErrNotEnabled
}
func (f *fakeVectorStore) Upsert(ctx context.Context, sessionID string, docs []vector.VectorDocument) error {
	return vector.ErrNotEnabled
}
func (f *fakeVectorStore) DeleteSession(ctx context.Context, sessionID string) error {
	return vector.ErrNotEnabled
}
func (f *fakeVectorStore) DeleteDocuments(ctx context.Context, ids []string) error {
	f.deleteDocIDs = append(f.deleteDocIDs, ids...)
	return f.deleteDocErr
}
func (f *fakeVectorStore) Rebuild(ctx context.Context, sessionID string) error {
	return vector.ErrNotEnabled
}
func (f *fakeVectorStore) Health(ctx context.Context) (vector.HealthSnapshot, error) {
	return f.healthSnapshot, f.healthErr
}
func (f *fakeVectorStore) Count(ctx context.Context, sessionID string) (int, error) {
	return f.countResult, f.countErr
}
func (f *fakeVectorStore) Close(ctx context.Context) error {
	return nil
}

// Test: backfill-dry-run returns Store-backed evidence
func TestChromaBackfillDryRunReturnsStoreEvidence(t *testing.T) {
	ts, _ := time.Parse(time.RFC3339, "2026-01-15T12:00:00Z")
	fake := &memoryFakeStore{
		memories: []store.Memory{
			{ID: 1, ChatSessionID: "sess-bf", TurnIndex: 1, SummaryJSON: `{}`, CreatedAt: ts},
			{ID: 2, ChatSessionID: "sess-bf", TurnIndex: 2, SummaryJSON: `{}`, CreatedAt: ts},
		},
		evidenceItems: []store.DirectEvidence{
			{ID: 10, ChatSessionID: "sess-bf", CreatedAt: ts},
		},
		kgTriples: []store.KGTriple{
			{ID: 20, ChatSessionID: "sess-bf", Subject: "A", Predicate: "B", Object: "C"},
		},
	}
	cfg := config.Default()
	srv := NewServer(cfg)
	srv.Store = fake
	srv.Vector = &fakeVectorStore{countResult: 1, countErr: nil}

	mux := http.NewServeMux()
	srv.RegisterRoutes(mux)

	body := `{"chat_session_id":"sess-bf"}`
	req := httptest.NewRequest(http.MethodPost, "/chroma-shadow/backfill-dry-run", strings.NewReader(body))
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
	ev, ok := resp["evidence"].(map[string]any)
	if !ok {
		t.Fatalf("evidence is not an object: %T", resp["evidence"])
	}
	if ev["memory_count"].(float64) != 2 {
		t.Errorf("evidence.memory_count = %v, want 2", ev["memory_count"])
	}
	if ev["evidence_count"].(float64) != 1 {
		t.Errorf("evidence.evidence_count = %v, want 1", ev["evidence_count"])
	}
	if ev["kg_triple_count"].(float64) != 1 {
		t.Errorf("evidence.kg_triple_count = %v, want 1", ev["kg_triple_count"])
	}
	if ev["vector_count"].(float64) != 1 {
		t.Errorf("evidence.vector_count = %v, want 1", ev["vector_count"])
	}
	ts2, ok := resp["trace_summary"].(map[string]any)
	if !ok {
		t.Fatalf("trace_summary is not an object: %T", resp["trace_summary"])
	}
	if ts2["step"] != "17-C1-r1" {
		t.Errorf("trace_summary.step = %v, want 17-C1-r1", ts2["step"])
	}
	if ts2["source"] != "shadow" {
		t.Errorf("trace_summary.source = %v, want shadow", ts2["source"])
	}
}

// Test: reembed-audit returns embedding model distribution
func TestChromaReembedAuditReturnsEmbeddingModels(t *testing.T) {
	ts, _ := time.Parse(time.RFC3339, "2026-01-15T12:00:00Z")
	fake := &memoryFakeStore{
		memories: []store.Memory{
			{ID: 1, ChatSessionID: "sess-rea", TurnIndex: 1, Embedding: "[0.1,0.2]", EmbeddingModel: "text-embedding-3-small", CreatedAt: ts},
			{ID: 2, ChatSessionID: "sess-rea", TurnIndex: 2, Embedding: "", EmbeddingModel: "", CreatedAt: ts},
		},
	}
	cfg := config.Default()
	srv := NewServer(cfg)
	srv.Store = fake
	srv.Vector = &fakeVectorStore{countResult: 1, countErr: nil}

	mux := http.NewServeMux()
	srv.RegisterRoutes(mux)

	body := `{"chat_session_id":"sess-rea"}`
	req := httptest.NewRequest(http.MethodPost, "/chroma-shadow/reembed-audit", strings.NewReader(body))
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
	ev, ok := resp["evidence"].(map[string]any)
	if !ok {
		t.Fatalf("evidence is not an object: %T", resp["evidence"])
	}
	if ev["memory_count"].(float64) != 2 {
		t.Errorf("evidence.memory_count = %v, want 2", ev["memory_count"])
	}
	if ev["memories_with_embedding"].(float64) != 1 {
		t.Errorf("evidence.memories_with_embedding = %v, want 1", ev["memories_with_embedding"])
	}
	models, ok := ev["memory_embedding_models"].(map[string]any)
	if !ok {
		t.Fatalf("memory_embedding_models is not an object: %T", ev["memory_embedding_models"])
	}
	if models["text-embedding-3-small"].(float64) != 1 {
		t.Errorf("models[text-embedding-3-small] = %v, want 1", models["text-embedding-3-small"])
	}
	if models["none"].(float64) != 1 {
		t.Errorf("models[none] = %v, want 1", models["none"])
	}
}

// Test: fallback-runbook returns Store stats
func TestChromaFallbackRunbookReturnsStoreStats(t *testing.T) {
	fake := &memoryFakeStore{}
	cfg := config.Default()
	srv := NewServer(cfg)
	srv.Store = fake
	srv.Vector = nil

	mux := http.NewServeMux()
	srv.RegisterRoutes(mux)

	body := `{"chat_session_id":"sess-fb"}`
	req := httptest.NewRequest(http.MethodPost, "/chroma-shadow/fallback-runbook", strings.NewReader(body))
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
	ev, ok := resp["evidence"].(map[string]any)
	if !ok {
		t.Fatalf("evidence is not an object: %T", resp["evidence"])
	}
	if ev["store_enabled"].(bool) != true {
		t.Errorf("evidence.store_enabled = %v, want true", ev["store_enabled"])
	}
	if ev["vector_error"] != "unavailable" {
		t.Errorf("evidence.vector_error = %v, want unavailable", ev["vector_error"])
	}
}

// Test: release-hygiene counts tombstoned evidence
func TestChromaReleaseHygieneCountsTombstoned(t *testing.T) {
	ts, _ := time.Parse(time.RFC3339, "2026-01-15T12:00:00Z")
	fake := &memoryFakeStore{
		evidenceItems: []store.DirectEvidence{
			{ID: 1, ChatSessionID: "sess-rh", Tombstoned: false, CreatedAt: ts},
			{ID: 2, ChatSessionID: "sess-rh", Tombstoned: true, CreatedAt: ts},
			{ID: 3, ChatSessionID: "sess-rh", Tombstoned: true, CreatedAt: ts},
		},
		chatLogs: []store.ChatLog{
			{ID: 1, ChatSessionID: "sess-rh", TurnIndex: 1, Role: "user", Content: "hi", CreatedAt: ts},
		},
	}
	cfg := config.Default()
	srv := NewServer(cfg)
	srv.Store = fake

	mux := http.NewServeMux()
	srv.RegisterRoutes(mux)

	body := `{"chat_session_id":"sess-rh"}`
	req := httptest.NewRequest(http.MethodPost, "/chroma-shadow/release-hygiene", strings.NewReader(body))
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
	ev, ok := resp["evidence"].(map[string]any)
	if !ok {
		t.Fatalf("evidence is not an object: %T", resp["evidence"])
	}
	if ev["evidence_count"].(float64) != 3 {
		t.Errorf("evidence.evidence_count = %v, want 3", ev["evidence_count"])
	}
	if ev["tombstoned_count"].(float64) != 2 {
		t.Errorf("evidence.tombstoned_count = %v, want 2", ev["tombstoned_count"])
	}
	if ev["chat_log_count"].(float64) != 1 {
		t.Errorf("evidence.chat_log_count = %v, want 1", ev["chat_log_count"])
	}
}

// Test: visibility-guard computes gap
func TestChromaVisibilityGuardComputesGap(t *testing.T) {
	ts, _ := time.Parse(time.RFC3339, "2026-01-15T12:00:00Z")
	fake := &memoryFakeStore{
		memories: []store.Memory{
			{ID: 1, ChatSessionID: "sess-vg", TurnIndex: 1, CreatedAt: ts},
			{ID: 2, ChatSessionID: "sess-vg", TurnIndex: 2, CreatedAt: ts},
			{ID: 3, ChatSessionID: "sess-vg", TurnIndex: 3, CreatedAt: ts},
		},
		evidenceItems: []store.DirectEvidence{
			{ID: 10, ChatSessionID: "sess-vg", CreatedAt: ts},
		},
	}
	cfg := config.Default()
	srv := NewServer(cfg)
	srv.Store = fake
	srv.Vector = &fakeVectorStore{countResult: 2, countErr: nil}

	mux := http.NewServeMux()
	srv.RegisterRoutes(mux)

	body := `{"chat_session_id":"sess-vg"}`
	req := httptest.NewRequest(http.MethodPost, "/chroma-shadow/visibility-guard", strings.NewReader(body))
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
	ev, ok := resp["evidence"].(map[string]any)
	if !ok {
		t.Fatalf("evidence is not an object: %T", resp["evidence"])
	}
	// storeTotal = 3 memories + 1 evidence + 0 kg = 4, vectorCount = 2, gap = 2
	if ev["visibility_gap"].(float64) != 2 {
		t.Errorf("evidence.visibility_gap = %v, want 2", ev["visibility_gap"])
	}
	if ev["vector_count"].(float64) != 2 {
		t.Errorf("evidence.vector_count = %v, want 2", ev["vector_count"])
	}
}

// Test: health-probe returns vector health and store status
func TestChromaHealthProbeReturnsVectorHealth(t *testing.T) {
	cfg := config.Default()
	srv := NewServer(cfg)
	srv.Store = &memoryFakeStore{}
	srv.Vector = &fakeVectorStore{
		healthSnapshot: vector.HealthSnapshot{
			Status:     "shadow",
			Collection: "archive_center_shadow",
			TotalCount: 42,
			ModelReady: true,
		},
		healthErr:   nil,
		countResult: 5,
		countErr:    nil,
	}

	mux := http.NewServeMux()
	srv.RegisterRoutes(mux)

	body := `{"chat_session_id":"sess-hp"}`
	req := httptest.NewRequest(http.MethodPost, "/chroma-shadow/health-probe", strings.NewReader(body))
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
	ev, ok := resp["evidence"].(map[string]any)
	if !ok {
		t.Fatalf("evidence is not an object: %T", resp["evidence"])
	}
	if ev["store_enabled"].(bool) != true {
		t.Errorf("evidence.store_enabled = %v, want true", ev["store_enabled"])
	}
	if ev["vector_health_status"] != "shadow" {
		t.Errorf("evidence.vector_health_status = %v, want shadow", ev["vector_health_status"])
	}
	if ev["vector_count"].(float64) != 5 {
		t.Errorf("evidence.vector_count = %v, want 5", ev["vector_count"])
	}
	vh, ok := ev["vector_health"].(map[string]any)
	if !ok {
		t.Fatalf("vector_health is not an object: %T", ev["vector_health"])
	}
	if vh["total_count"].(float64) != 42 {
		t.Errorf("vector_health.total_count = %v, want 42", vh["total_count"])
	}
	if vh["model_ready"].(bool) != true {
		t.Errorf("vector_health.model_ready = %v, want true", vh["model_ready"])
	}
}

// Test: ErrNotEnabled from Vector.Count returns safe 200 with not_enabled
func TestChromaBackfillDryRunVectorNotEnabledReturnsSafe200(t *testing.T) {
	fake := &memoryFakeStore{
		memories: []store.Memory{
			{ID: 1, ChatSessionID: "sess-ne", TurnIndex: 1, SummaryJSON: `{}`},
		},
	}
	cfg := config.Default()
	srv := NewServer(cfg)
	srv.Store = fake
	srv.Vector = &fakeVectorStore{countResult: 0, countErr: vector.ErrNotEnabled}

	mux := http.NewServeMux()
	srv.RegisterRoutes(mux)

	body := `{"chat_session_id":"sess-ne"}`
	req := httptest.NewRequest(http.MethodPost, "/chroma-shadow/backfill-dry-run", strings.NewReader(body))
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
	ev := resp["evidence"].(map[string]any)
	if ev["vector_error"] != "not_enabled" {
		t.Errorf("evidence.vector_error = %v, want not_enabled", ev["vector_error"])
	}
	if ev["memory_count"].(float64) != 1 {
		t.Errorf("evidence.memory_count = %v, want 1", ev["memory_count"])
	}
}

// Test: no chat_session_id returns safe 200 with empty evidence
func TestChromaHealthProbeNoSessionIDReturnsSafe200(t *testing.T) {
	cfg := config.Default()
	srv := NewServer(cfg)

	mux := http.NewServeMux()
	srv.RegisterRoutes(mux)

	req := httptest.NewRequest(http.MethodPost, "/chroma-shadow/health-probe", strings.NewReader(`{}`))
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
	ev := resp["evidence"].(map[string]any)
	if ev["store_enabled"].(bool) != false {
		t.Errorf("evidence.store_enabled = %v, want false (no session)", ev["store_enabled"])
	}
}

func TestSeq123P90CanonicalRowToVectorSyncScopeMarkers(t *testing.T) {
	ts, _ := time.Parse(time.RFC3339, "2026-01-15T12:00:00Z")
	fake := &memoryFakeStore{
		memories: []store.Memory{
			{ID: 1, ChatSessionID: "sess-p90", TurnIndex: 1, SummaryJSON: `{"summary":"memory row"}`, CreatedAt: ts},
		},
		evidenceItems: []store.DirectEvidence{
			{ID: 10, ChatSessionID: "sess-p90", CreatedAt: ts},
		},
		kgTriples: []store.KGTriple{
			{ID: 20, ChatSessionID: "sess-p90", Subject: "A", Predicate: "knows", Object: "B"},
		},
		episodes: []store.EpisodeSummary{
			{ID: 30, ChatSessionID: "sess-p90", FromTurn: 1, ToTurn: 2, SummaryText: "episode row"},
		},
	}
	cfg := config.Default()
	srv := NewServer(cfg)
	srv.Store = fake
	srv.Vector = &fakeVectorStore{countResult: 1}

	mux := http.NewServeMux()
	srv.RegisterRoutes(mux)

	req := httptest.NewRequest(http.MethodPost, "/chroma-shadow/backfill-dry-run", strings.NewReader(`{"chat_session_id":"sess-p90"}`))
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
	ev := resp["evidence"].(map[string]any)
	if ev["sync_scope"] != "selected_tiers" {
		t.Fatalf("sync_scope = %v, want selected_tiers", ev["sync_scope"])
	}
	if ev["primary_source"] != "canonical_row" || ev["vector_role"] != "shadow_backfill" {
		t.Fatalf("unexpected sync contract: %#v", ev)
	}
	tiers := ev["allowed_tiers"].([]any)
	want := map[string]bool{"memory": false, "evidence": false, "kg_triple": false, "episode": false}
	for _, tier := range tiers {
		if _, ok := want[tier.(string)]; ok {
			want[tier.(string)] = true
		}
	}
	for tier, seen := range want {
		if !seen {
			t.Fatalf("allowed_tiers missing %q: %#v", tier, tiers)
		}
	}
}

func TestSeq123P91SummaryReembedUpsertRuleMarkers(t *testing.T) {
	fake := &memoryFakeStore{
		memories: []store.Memory{
			{ID: 1, ChatSessionID: "sess-p91", TurnIndex: 1, Embedding: "[0.1]", EmbeddingModel: "model-a"},
			{ID: 2, ChatSessionID: "sess-p91", TurnIndex: 2, Embedding: "", EmbeddingModel: ""},
		},
	}
	cfg := config.Default()
	srv := NewServer(cfg)
	srv.Store = fake
	srv.Vector = &fakeVectorStore{countResult: 1}

	mux := http.NewServeMux()
	srv.RegisterRoutes(mux)

	req := httptest.NewRequest(http.MethodPost, "/chroma-shadow/reembed-audit", strings.NewReader(`{"chat_session_id":"sess-p91"}`))
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
	ev := resp["evidence"].(map[string]any)
	if ev["reembed_rule"] != "summary_edit_triggers_upsert" {
		t.Fatalf("reembed_rule = %v, want summary_edit_triggers_upsert", ev["reembed_rule"])
	}
	if ev["memories_with_embedding"].(float64) != 1 {
		t.Fatalf("memories_with_embedding = %v, want 1", ev["memories_with_embedding"])
	}
	models := ev["memory_embedding_models"].(map[string]any)
	if models["model-a"].(float64) != 1 || models["none"].(float64) != 1 {
		t.Fatalf("unexpected model distribution: %#v", models)
	}
}

func TestSeq123P92StaleVectorDeleteRollbackMergeMarkers(t *testing.T) {
	fake := &memoryFakeStore{
		evidenceItems: []store.DirectEvidence{
			{ID: 1, ChatSessionID: "sess-p92", Tombstoned: false},
			{ID: 2, ChatSessionID: "sess-p92", Tombstoned: true},
		},
		chatLogs: []store.ChatLog{
			{ID: 1, ChatSessionID: "sess-p92", TurnIndex: 1, Role: "user", Content: "delete marker"},
		},
	}
	cfg := config.Default()
	srv := NewServer(cfg)
	srv.Store = fake

	mux := http.NewServeMux()
	srv.RegisterRoutes(mux)

	req := httptest.NewRequest(http.MethodPost, "/chroma-shadow/release-hygiene", strings.NewReader(`{"chat_session_id":"sess-p92"}`))
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
	ev := resp["evidence"].(map[string]any)
	if ev["stale_vector_policy"] != "tombstone_before_delete" {
		t.Fatalf("stale_vector_policy = %v", ev["stale_vector_policy"])
	}
	if ev["delete_policy"] != "canonical_row_first" || ev["rollback_policy"] != "vector_doc_rollback_with_id" || ev["merge_policy"] != "merge_stale_vectors_to_tombstone" {
		t.Fatalf("unexpected stale vector policies: %#v", ev)
	}
	if ev["tombstoned_count"].(float64) != 1 {
		t.Fatalf("tombstoned_count = %v, want 1", ev["tombstoned_count"])
	}
}

func TestSeq123P93TargetedPartialFullRebuildOwnerMarkers(t *testing.T) {
	cfg := config.Default()
	srv := NewServer(cfg)

	mux := http.NewServeMux()
	srv.RegisterRoutes(mux)

	req := httptest.NewRequest(http.MethodPost, "/chroma-shadow/rebuild-drill", strings.NewReader(`{"chat_session_id":"sess-p93"}`))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("status = %d, want 503: %s", rec.Code, rec.Body.String())
	}
	var resp map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if resp["code"] != CodeShadowGuard {
		t.Fatalf("code = %v, want %s", resp["code"], CodeShadowGuard)
	}
	trace := resp["trace_summary"].(map[string]any)
	if trace["rebuild_owner"] != "chroma_shadow_orchestrator" {
		t.Fatalf("rebuild_owner = %v, want chroma_shadow_orchestrator", trace["rebuild_owner"])
	}
	modes := trace["rebuild_modes"].([]any)
	want := map[string]bool{"targeted": false, "partial": false, "full": false}
	for _, mode := range modes {
		if _, ok := want[mode.(string)]; ok {
			want[mode.(string)] = true
		}
	}
	for mode, seen := range want {
		if !seen {
			t.Fatalf("rebuild_modes missing %q: %#v", mode, modes)
		}
	}
}

// Test: GET /retrieval-index/runtime-config returns Python 0.8 compact shape
func TestRetrievalIndexRuntimeConfigGetR1EvidenceShape(t *testing.T) {
	fake := &memoryFakeStore{}
	cfg := config.Default()
	srv := NewServer(cfg)
	srv.Store = fake
	srv.Vector = &fakeVectorStore{
		healthSnapshot: vector.HealthSnapshot{
			Status:     "shadow",
			Collection: "archive_center_shadow",
			TotalCount: 42,
			ModelReady: true,
		},
		healthErr: nil,
	}

	mux := http.NewServeMux()
	srv.RegisterRoutes(mux)

	req := httptest.NewRequest(http.MethodGet, "/retrieval-index/runtime-config", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200: %s", rec.Code, rec.Body.String())
	}
	var resp map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}

	if resp["mode"] != "shadow" {
		t.Errorf("mode = %v, want shadow", resp["mode"])
	}
	if resp["shadow_write_enabled"] != true {
		t.Errorf("shadow_write_enabled = %v, want true", resp["shadow_write_enabled"])
	}
	if resp["reason"] != "default" {
		t.Errorf("reason = %v, want default", resp["reason"])
	}
	if resp["session_count"] != float64(0) {
		t.Errorf("session_count = %v, want 0", resp["session_count"])
	}
	if resp["index_version"] != "q1e.v1" {
		t.Errorf("index_version = %v, want q1e.v1", resp["index_version"])
	}
	if _, ok := resp["updated_at"].(string); !ok {
		t.Errorf("updated_at = %T, want string", resp["updated_at"])
	}
	for _, forbidden := range []string{"status", "runtime_mode", "evidence", "source", "store_mode", "vector_mode"} {
		if _, ok := resp[forbidden]; ok {
			t.Fatalf("unexpected Go-only key %q in Python-compatible response: %#v", forbidden, resp)
		}
	}
}
func TestIntentRoutingRuntimeConfigGetR1EvidenceShape(t *testing.T) {
	fake := &memoryFakeStore{}
	cfg := config.Default()
	srv := NewServer(cfg)
	srv.Store = fake

	mux := http.NewServeMux()
	srv.RegisterRoutes(mux)

	req := httptest.NewRequest(http.MethodGet, "/intent-routing/runtime-config", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200: %s", rec.Code, rec.Body.String())
	}
	var resp map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}

	if resp["mode"] != "single_query_shared" {
		t.Errorf("mode = %v, want single_query_shared", resp["mode"])
	}
	if resp["reason"] != "default" {
		t.Errorf("reason = %v, want default", resp["reason"])
	}
	if resp["version"] != "v0c.v1" {
		t.Errorf("version = %v, want v0c.v1", resp["version"])
	}
	if _, ok := resp["updated_at"].(string); !ok {
		t.Errorf("updated_at = %T, want string", resp["updated_at"])
	}
	modes, ok := resp["supported_modes"].([]any)
	if !ok || len(modes) != 2 {
		t.Fatalf("supported_modes = %#v, want two-item array", resp["supported_modes"])
	}
	for _, forbidden := range []string{"status", "runtime_mode", "evidence", "source", "store_mode", "vector_mode"} {
		if _, ok := resp[forbidden]; ok {
			t.Fatalf("unexpected Go-only key %q in Python-compatible response: %#v", forbidden, resp)
		}
	}
}

// Test: GET /retrieval-index/runtime-config with ErrNotEnabled returns safe 200
func TestRetrievalIndexRuntimeConfigGetStoreNotEnabledReturnsSafe200(t *testing.T) {
	fake := &memoryFakeStore{
		statsErr: store.ErrNotEnabled,
	}
	cfg := config.Default()
	srv := NewServer(cfg)
	srv.Store = fake
	srv.Vector = &fakeVectorStore{healthErr: vector.ErrNotEnabled}

	mux := http.NewServeMux()
	srv.RegisterRoutes(mux)

	req := httptest.NewRequest(http.MethodGet, "/retrieval-index/runtime-config", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	var resp map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if resp["mode"] != "shadow" {
		t.Errorf("mode = %v, want shadow", resp["mode"])
	}
	if resp["shadow_write_enabled"] != true {
		t.Errorf("shadow_write_enabled = %v, want true", resp["shadow_write_enabled"])
	}
	if resp["session_count"] != float64(0) {
		t.Errorf("session_count = %v, want 0", resp["session_count"])
	}
}

// Test: GET /retrieval-index/runtime-config without Store/Vector returns safe 200
func TestRetrievalIndexRuntimeConfigGetNoStoreNoVectorReturnsSafe200(t *testing.T) {
	cfg := config.Default()
	srv := NewServer(cfg)
	srv.Store = nil
	srv.Vector = nil

	mux := http.NewServeMux()
	srv.RegisterRoutes(mux)

	req := httptest.NewRequest(http.MethodGet, "/retrieval-index/runtime-config", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	var resp map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if resp["mode"] != "shadow" {
		t.Errorf("mode = %v, want shadow", resp["mode"])
	}
	if resp["shadow_write_enabled"] != true {
		t.Errorf("shadow_write_enabled = %v, want true", resp["shadow_write_enabled"])
	}
	if resp["session_count"] != float64(0) {
		t.Errorf("session_count = %v, want 0", resp["session_count"])
	}
	if _, ok := resp["updated_at"].(string); !ok {
		t.Errorf("updated_at = %T, want string", resp["updated_at"])
	}
}

func TestExplorerPatchMemoryWritesAuditAndChangedAt(t *testing.T) {
	fake := &memoryFakeStore{
		memories: []store.Memory{
			{
				ID:            42,
				ChatSessionID: "sess-edit",
				TurnIndex:     2,
				SummaryJSON:   `{"summary":"old"}`,
				Importance:    0.3,
				PlaceWing:     "old-wing",
				PlaceRoom:     "old-room",
				CreatedAt:     time.Date(2026, 5, 30, 1, 2, 3, 0, time.UTC),
			},
		},
	}
	cfg := config.Default()
	cfg.StoreMode = config.StoreModeMariaDBAuthority
	srv := NewServer(cfg)
	srv.Store = fake
	srv.StoreOpenError = nil

	mux := http.NewServeMux()
	srv.RegisterRoutes(mux)

	body := []byte(`{"chat_session_id":"sess-edit","summary_json":"{\"summary\":\"new\"}","importance":0.91,"archive_wing":"A","archive_room":"R"}`)
	req := httptest.NewRequest(http.MethodPatch, "/explorer/memories/42", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200: %s", rec.Code, rec.Body.String())
	}
	if len(fake.updatedMemory) != 1 {
		t.Fatalf("updatedMemory len = %d, want 1", len(fake.updatedMemory))
	}
	if got := fake.memories[0].Importance; got != 0.91 {
		t.Fatalf("importance = %.2f, want 0.91", got)
	}
	if len(fake.auditLogs) == 0 {
		t.Fatal("expected manual_edit audit log")
	}
	audit := fake.auditLogs[0]
	if audit.EventType != "manual_edit" || audit.TargetType != "memory" || audit.TargetID != 42 {
		t.Fatalf("unexpected audit: %#v", audit)
	}
	if !strings.Contains(audit.DetailsJSON, "changed_at") || !strings.Contains(audit.DetailsJSON, "updated_fields") {
		t.Fatalf("audit details missing history fields: %s", audit.DetailsJSON)
	}
	var resp map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if resp["audit_written"] != true || resp["changed_at"] == "" {
		t.Fatalf("response missing audit/changed_at: %#v", resp)
	}
}

func TestExplorerPatchMemoryRejectsInvalidSummaryJSON(t *testing.T) {
	fake := &memoryFakeStore{
		memories: []store.Memory{
			{ID: 42, ChatSessionID: "sess-edit", SummaryJSON: `{"summary":"old"}`},
		},
	}
	cfg := config.Default()
	cfg.StoreMode = config.StoreModeMariaDBAuthority
	srv := NewServer(cfg)
	srv.Store = fake
	srv.StoreOpenError = nil

	mux := http.NewServeMux()
	srv.RegisterRoutes(mux)

	body := []byte(`{"chat_session_id":"sess-edit","summary_json":"{\"summary\": "}`)
	req := httptest.NewRequest(http.MethodPatch, "/explorer/memories/42", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400: %s", rec.Code, rec.Body.String())
	}
	if len(fake.updatedMemory) != 0 {
		t.Fatalf("invalid summary_json should not update memory, got %d updates", len(fake.updatedMemory))
	}
	if len(fake.auditLogs) != 0 {
		t.Fatalf("invalid summary_json should not write audit, got %d logs", len(fake.auditLogs))
	}
	if !strings.Contains(rec.Body.String(), "summary_json") {
		t.Fatalf("error body should name summary_json: %s", rec.Body.String())
	}
}

func TestExplorerPatchMemoryRejectsForeignSession(t *testing.T) {
	fake := &memoryFakeStore{
		memories: []store.Memory{{ID: 42, ChatSessionID: "other-session", Importance: 0.3}},
	}
	cfg := config.Default()
	cfg.StoreMode = config.StoreModeMariaDBAuthority
	srv := NewServer(cfg)
	srv.Store = fake
	srv.StoreOpenError = nil

	mux := http.NewServeMux()
	srv.RegisterRoutes(mux)
	req := httptest.NewRequest(http.MethodPatch, "/explorer/memories/42", bytes.NewReader([]byte(`{"chat_session_id":"sess-edit","importance":0.91}`)))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want 404: %s", rec.Code, rec.Body.String())
	}
	if len(fake.updatedMemory) != 0 {
		t.Fatalf("foreign session should not update memory, got %d updates", len(fake.updatedMemory))
	}
	if len(fake.auditLogs) != 0 {
		t.Fatalf("foreign session should not write audit, got %d logs", len(fake.auditLogs))
	}
}

func TestExplorerPatchMemoryShadowGuardOutsideWriteMode(t *testing.T) {
	srv := NewServer(config.Default())
	srv.Store = &memoryFakeStore{}

	mux := http.NewServeMux()
	srv.RegisterRoutes(mux)
	req := httptest.NewRequest(http.MethodPatch, "/explorer/memories/42", bytes.NewReader([]byte(`{"chat_session_id":"sess-edit","importance":0.91}`)))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("status = %d, want 503: %s", rec.Code, rec.Body.String())
	}
}

func TestExplorerPatchKGTripleWritesAuditAndChangedAt(t *testing.T) {
	fake := &memoryFakeStore{
		kgTriples: []store.KGTriple{
			{ID: 7, ChatSessionID: "sess-edit", Subject: "old-a", Predicate: "old-p", Object: "old-b"},
		},
	}
	cfg := config.Default()
	cfg.StoreMode = config.StoreModeMariaDBAuthority
	srv := NewServer(cfg)
	srv.Store = fake
	srv.StoreOpenError = nil

	mux := http.NewServeMux()
	srv.RegisterRoutes(mux)

	body := []byte(`{"chat_session_id":"sess-edit","subject":"new-a","predicate":"new-p","object":"new-b","valid_from":2,"valid_to":null}`)
	req := httptest.NewRequest(http.MethodPatch, "/explorer/kg_triples/7", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200: %s", rec.Code, rec.Body.String())
	}
	if len(fake.updatedKG) != 1 {
		t.Fatalf("updatedKG len = %d, want 1", len(fake.updatedKG))
	}
	if got := fake.kgTriples[0].Predicate; got != "new-p" {
		t.Fatalf("predicate = %q, want new-p", got)
	}
	if fake.kgTriples[0].ValidFrom != 2 {
		t.Fatalf("valid_from = %#v, want 2", fake.kgTriples[0].ValidFrom)
	}
	if len(fake.auditLogs) == 0 {
		t.Fatal("expected manual_edit audit log")
	}
	audit := fake.auditLogs[0]
	if audit.EventType != "manual_edit" || audit.TargetType != "kg_triple" || audit.TargetID != 7 {
		t.Fatalf("unexpected audit: %#v", audit)
	}
	if !strings.Contains(audit.DetailsJSON, "changed_at") || !strings.Contains(audit.DetailsJSON, "updated_fields") {
		t.Fatalf("audit details missing history fields: %s", audit.DetailsJSON)
	}
}

func TestExplorerPatchEvidenceReviewWritesAuditAndChangedAt(t *testing.T) {
	fake := &memoryFakeStore{
		evidenceItems: []store.DirectEvidence{
			{
				ID:                  9,
				ChatSessionID:       "sess-edit",
				ArchiveState:        "candidate",
				CaptureVerification: "pending",
				CommittedGate:       "none",
			},
		},
	}
	cfg := config.Default()
	cfg.StoreMode = config.StoreModeMariaDBAuthority
	srv := NewServer(cfg)
	srv.Store = fake
	srv.StoreOpenError = nil

	mux := http.NewServeMux()
	srv.RegisterRoutes(mux)

	body := []byte(`{"chat_session_id":"sess-edit","capture_verification":"verified","review_note":"manual check"}`)
	req := httptest.NewRequest(http.MethodPatch, "/explorer/direct-evidence/9/review", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200: %s", rec.Code, rec.Body.String())
	}
	if len(fake.updatedEvidence) != 1 {
		t.Fatalf("updatedEvidence len = %d, want 1", len(fake.updatedEvidence))
	}
	if got := fake.evidenceItems[0].CaptureVerification; got != "verified" {
		t.Fatalf("capture_verification = %q, want verified", got)
	}
	if len(fake.auditLogs) == 0 {
		t.Fatal("expected manual_edit audit log")
	}
	audit := fake.auditLogs[0]
	if audit.EventType != "manual_edit" || audit.TargetType != "direct_evidence" || audit.TargetID != 9 {
		t.Fatalf("unexpected audit: %#v", audit)
	}
	if !strings.Contains(audit.DetailsJSON, "changed_at") || !strings.Contains(audit.DetailsJSON, "manual check") {
		t.Fatalf("audit details missing history fields: %s", audit.DetailsJSON)
	}
}

func TestExplorerPatchEvidenceRevalidateCommitsGateAndClearsRepair(t *testing.T) {
	fake := &memoryFakeStore{
		evidenceItems: []store.DirectEvidence{
			{
				ID:                  10,
				ChatSessionID:       "sess-edit",
				ArchiveState:        "repair_queue",
				CaptureVerification: "needs_review",
				CommittedGate:       "recovery",
				RepairNeeded:        true,
			},
		},
	}
	cfg := config.Default()
	cfg.StoreMode = config.StoreModeMariaDBAuthority
	srv := NewServer(cfg)
	srv.Store = fake
	srv.StoreOpenError = nil

	mux := http.NewServeMux()
	srv.RegisterRoutes(mux)

	body := []byte(`{"chat_session_id":"sess-edit","review_note":"revalidated by operator"}`)
	req := httptest.NewRequest(http.MethodPatch, "/explorer/direct-evidence/10/revalidate", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200: %s", rec.Code, rec.Body.String())
	}
	if len(fake.updatedEvidence) != 1 {
		t.Fatalf("updatedEvidence len = %d, want 1", len(fake.updatedEvidence))
	}
	ev := fake.evidenceItems[0]
	if ev.CaptureVerification != "verified" || ev.ArchiveState != "committed" || ev.CommittedGate != "manual_revalidate" || ev.RepairNeeded {
		t.Fatalf("revalidated evidence = %#v, want verified/committed/manual_revalidate/not repair-needed", ev)
	}
	if len(fake.auditLogs) == 0 {
		t.Fatal("expected manual_edit audit log")
	}
	audit := fake.auditLogs[0]
	if audit.EventType != "manual_edit" || audit.TargetType != "direct_evidence" || audit.TargetID != 10 {
		t.Fatalf("unexpected audit: %#v", audit)
	}
	for _, needle := range []string{"revalidate", "manual_revalidate", "revalidated by operator", "changed_at"} {
		if !strings.Contains(audit.DetailsJSON, needle) {
			t.Fatalf("audit details missing %q: %s", needle, audit.DetailsJSON)
		}
	}
}

func TestExplorerPatchEvidenceTombstoneAndSupersedeWriteAudit(t *testing.T) {
	fake := &memoryFakeStore{
		evidenceItems: []store.DirectEvidence{
			{ID: 11, ChatSessionID: "sess-edit", ArchiveState: "verified_direct", CaptureVerification: "verified"},
			{ID: 12, ChatSessionID: "sess-edit", ArchiveState: "verified_direct", CaptureVerification: "verified"},
		},
	}
	cfg := config.Default()
	cfg.StoreMode = config.StoreModeMariaDBAuthority
	srv := NewServer(cfg)
	srv.Store = fake
	srv.StoreOpenError = nil

	mux := http.NewServeMux()
	srv.RegisterRoutes(mux)

	tombstoneReq := httptest.NewRequest(http.MethodPatch, "/explorer/direct-evidence/11/tombstone", bytes.NewReader([]byte(`{"chat_session_id":"sess-edit","review_note":"deleted turn rollback"}`)))
	tombstoneReq.Header.Set("Content-Type", "application/json")
	tombstoneRec := httptest.NewRecorder()
	mux.ServeHTTP(tombstoneRec, tombstoneReq)
	if tombstoneRec.Code != http.StatusOK {
		t.Fatalf("tombstone status = %d, want 200: %s", tombstoneRec.Code, tombstoneRec.Body.String())
	}
	if !fake.evidenceItems[0].Tombstoned || fake.evidenceItems[0].ArchiveState != "tombstoned" {
		t.Fatalf("tombstoned evidence = %#v, want tombstoned archive state", fake.evidenceItems[0])
	}

	supersedeReq := httptest.NewRequest(http.MethodPatch, "/explorer/direct-evidence/12/supersede", bytes.NewReader([]byte(`{"chat_session_id":"sess-edit","superseded_by_id":11,"review_note":"newer fact wins"}`)))
	supersedeReq.Header.Set("Content-Type", "application/json")
	supersedeRec := httptest.NewRecorder()
	mux.ServeHTTP(supersedeRec, supersedeReq)
	if supersedeRec.Code != http.StatusOK {
		t.Fatalf("supersede status = %d, want 200: %s", supersedeRec.Code, supersedeRec.Body.String())
	}
	if fake.evidenceItems[1].SupersededByID != 11 {
		t.Fatalf("superseded_by_id = %d, want 11", fake.evidenceItems[1].SupersededByID)
	}
	if len(fake.updatedEvidence) != 2 {
		t.Fatalf("updatedEvidence len = %d, want 2", len(fake.updatedEvidence))
	}
	if len(fake.auditLogs) < 2 {
		t.Fatalf("auditLogs len = %d, want >= 2", len(fake.auditLogs))
	}
	combined := fake.auditLogs[0].DetailsJSON + "\n" + fake.auditLogs[1].DetailsJSON
	for _, needle := range []string{"tombstone", "supersede", "superseded_by_id", "changed_at"} {
		if !strings.Contains(combined, needle) {
			t.Fatalf("combined audit details missing %q: %s", needle, combined)
		}
	}
}

func TestExplorerDeleteMemoryWritesAuditAndScopesSession(t *testing.T) {
	fake := &memoryFakeStore{
		memories: []store.Memory{
			{
				ID:            42,
				ChatSessionID: "sess-delete",
				TurnIndex:     3,
				SummaryJSON:   `{"summary":"delete me"}`,
				Importance:    0.72,
				PlaceWing:     "wing",
				PlaceRoom:     "room",
				CreatedAt:     time.Date(2026, 6, 1, 1, 2, 3, 0, time.UTC),
			},
			{ID: 42, ChatSessionID: "other-session", SummaryJSON: `{"summary":"keep me"}`},
		},
	}
	cfg := config.Default()
	cfg.StoreMode = config.StoreModeMariaDBAuthority
	cfg.ChromaEndpoint = "http://127.0.0.1:8000"
	srv := NewServer(cfg)
	srv.Store = fake
	srv.StoreOpenError = nil
	vec := &fakeVectorStore{}
	srv.Vector = vec

	mux := http.NewServeMux()
	srv.RegisterRoutes(mux)

	req := httptest.NewRequest(http.MethodPost, "/explorer/memories/42/delete?chat_session_id=sess-delete", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200: %s", rec.Code, rec.Body.String())
	}
	if fake.deletedMemoryID != 42 {
		t.Fatalf("deletedMemoryID = %d, want 42", fake.deletedMemoryID)
	}
	if len(fake.memories) != 1 || fake.memories[0].ChatSessionID != "other-session" {
		t.Fatalf("memory delete was not session-scoped: %#v", fake.memories)
	}
	if len(vec.deleteDocIDs) != 1 || vec.deleteDocIDs[0] != "memory:sess-delete:42" {
		t.Fatalf("vector delete IDs = %#v, want memory:sess-delete:42", vec.deleteDocIDs)
	}
	if len(fake.auditLogs) == 0 {
		t.Fatal("expected manual_delete audit log")
	}
	audit := fake.auditLogs[0]
	if audit.EventType != "manual_delete" || audit.TargetType != "memory" || audit.TargetID != 42 {
		t.Fatalf("unexpected audit: %#v", audit)
	}
	if !strings.Contains(audit.DetailsJSON, "changed_at") || !strings.Contains(audit.DetailsJSON, "delete me") || !strings.Contains(audit.DetailsJSON, "memory:sess-delete:42") {
		t.Fatalf("audit details missing deletion history: %s", audit.DetailsJSON)
	}
	var resp map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if resp["deleted"] != true || resp["audit_written"] != true {
		t.Fatalf("response missing delete/audit proof: %#v", resp)
	}
	cleanup, ok := resp["vector_cleanup"].(map[string]any)
	if !ok || cleanup["attempted"] != true || cleanup["ok"] != true || cleanup["deleted_ids"] != float64(1) {
		t.Fatalf("response missing vector cleanup proof: %#v", resp)
	}
}

func TestExplorerDeleteDirectEvidenceWritesAuditAndScopesSession(t *testing.T) {
	fake := &memoryFakeStore{
		evidenceItems: []store.DirectEvidence{
			{
				ID:                  51,
				ChatSessionID:       "sess-delete",
				EvidenceKind:        "turn_excerpt",
				EvidenceText:        "delete this evidence",
				ArchiveState:        "verified_direct",
				CaptureVerification: "verified",
				TurnAnchor:          4,
				SourceTurnStart:     4,
				SourceTurnEnd:       4,
				CreatedAt:           time.Date(2026, 6, 1, 1, 2, 3, 0, time.UTC),
			},
			{ID: 51, ChatSessionID: "other-session", EvidenceText: "keep this evidence"},
		},
	}
	cfg := config.Default()
	cfg.StoreMode = config.StoreModeMariaDBAuthority
	srv := NewServer(cfg)
	srv.Store = fake
	srv.StoreOpenError = nil

	mux := http.NewServeMux()
	srv.RegisterRoutes(mux)

	req := httptest.NewRequest(http.MethodPost, "/explorer/direct-evidence/51/delete?chat_session_id=sess-delete", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200: %s", rec.Code, rec.Body.String())
	}
	if fake.deletedEvidenceID != 51 {
		t.Fatalf("deletedEvidenceID = %d, want 51", fake.deletedEvidenceID)
	}
	if len(fake.evidenceItems) != 1 || fake.evidenceItems[0].ChatSessionID != "other-session" {
		t.Fatalf("direct evidence delete was not session-scoped: %#v", fake.evidenceItems)
	}
	if len(fake.auditLogs) == 0 {
		t.Fatal("expected manual_delete audit log")
	}
	audit := fake.auditLogs[0]
	if audit.EventType != "manual_delete" || audit.TargetType != "direct_evidence" || audit.TargetID != 51 {
		t.Fatalf("unexpected audit: %#v", audit)
	}
	if !strings.Contains(audit.DetailsJSON, "changed_at") || !strings.Contains(audit.DetailsJSON, "delete this evidence") {
		t.Fatalf("audit details missing deletion history: %s", audit.DetailsJSON)
	}
	var resp map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if resp["deleted"] != true || resp["audit_written"] != true {
		t.Fatalf("response missing delete/audit proof: %#v", resp)
	}
}

func TestExplorerDeleteKGTripleWritesAuditAndScopesSession(t *testing.T) {
	fake := &memoryFakeStore{
		kgTriples: []store.KGTriple{
			{ID: 7, ChatSessionID: "sess-delete", Subject: "A", Predicate: "knows", Object: "B"},
			{ID: 7, ChatSessionID: "other-session", Subject: "Other", Predicate: "keeps", Object: "B"},
		},
	}
	cfg := config.Default()
	cfg.StoreMode = config.StoreModeMariaDBAuthority
	srv := NewServer(cfg)
	srv.Store = fake
	srv.StoreOpenError = nil

	mux := http.NewServeMux()
	srv.RegisterRoutes(mux)

	req := httptest.NewRequest(http.MethodDelete, "/explorer/kg_triples/7?chat_session_id=sess-delete", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200: %s", rec.Code, rec.Body.String())
	}
	if fake.deletedKGID != 7 {
		t.Fatalf("deletedKGID = %d, want 7", fake.deletedKGID)
	}
	if len(fake.kgTriples) != 1 || fake.kgTriples[0].ChatSessionID != "other-session" {
		t.Fatalf("KG delete was not session-scoped: %#v", fake.kgTriples)
	}
	if len(fake.auditLogs) == 0 {
		t.Fatal("expected manual_delete audit log")
	}
	audit := fake.auditLogs[0]
	if audit.EventType != "manual_delete" || audit.TargetType != "kg_triple" || audit.TargetID != 7 {
		t.Fatalf("unexpected audit: %#v", audit)
	}
	if !strings.Contains(audit.DetailsJSON, "changed_at") || !strings.Contains(audit.DetailsJSON, "knows") {
		t.Fatalf("audit details missing deletion history: %s", audit.DetailsJSON)
	}
}

func TestSeq123P97CanonicalToVectorDriftMarkers(t *testing.T) {
	fake := &memoryFakeStore{
		memories: []store.Memory{
			{ID: 1, ChatSessionID: "sess-p97", TurnIndex: 1, SummaryJSON: "{\"summary\":\"m1\"}"},
			{ID: 2, ChatSessionID: "sess-p97", TurnIndex: 2, SummaryJSON: "{\"summary\":\"m2\"}"},
		},
		evidenceItems: []store.DirectEvidence{
			{ID: 10, ChatSessionID: "sess-p97"},
		},
	}
	cfg := config.Default()
	srv := NewServer(cfg)
	srv.Store = fake
	srv.Vector = &fakeVectorStore{countResult: 1}

	mux := http.NewServeMux()
	srv.RegisterRoutes(mux)

	req := httptest.NewRequest(http.MethodPost, "/chroma-shadow/visibility-guard", strings.NewReader("{\"chat_session_id\":\"sess-p97\"}"))
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
	ev := resp["evidence"].(map[string]any)
	if ev["drift_policy"] != "shadow_degraded" {
		t.Fatalf("drift_policy = %v, want shadow_degraded", ev["drift_policy"])
	}
	if ev["drift_status"] != "drift_detected" {
		t.Fatalf("drift_status = %v, want drift_detected", ev["drift_status"])
	}
	if ev["canonical_count"] != float64(3) {
		t.Fatalf("canonical_count = %v, want 3", ev["canonical_count"])
	}
	if ev["canonical_to_vector_gap"] != float64(2) {
		t.Fatalf("canonical_to_vector_gap = %v, want 2", ev["canonical_to_vector_gap"])
	}
	if ev["drift_action"] != "keep_canonical_baseline" {
		t.Fatalf("drift_action = %v, want keep_canonical_baseline", ev["drift_action"])
	}
}

func TestSeq123P98FallbackFailOpenDegradedVocabularyMarkers(t *testing.T) {
	fake := &memoryFakeStore{}
	cfg := config.Default()
	srv := NewServer(cfg)
	srv.Store = fake
	srv.Vector = nil

	mux := http.NewServeMux()
	srv.RegisterRoutes(mux)

	req := httptest.NewRequest(http.MethodPost, "/chroma-shadow/fallback-runbook", strings.NewReader("{\"chat_session_id\":\"sess-p98\"}"))
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
	ev := resp["evidence"].(map[string]any)
	if ev["fallback_policy"] != "store_first_then_vector" {
		t.Fatalf("fallback_policy = %v, want store_first_then_vector", ev["fallback_policy"])
	}
	if ev["degraded_mode"] != "canonical_baseline" {
		t.Fatalf("degraded_mode = %v, want canonical_baseline", ev["degraded_mode"])
	}
	if ev["fail_open_baseline"] != true {
		t.Fatalf("fail_open_baseline = %v, want true", ev["fail_open_baseline"])
	}
	if ev["retrieval_baseline"] != "sqlite_canonical" {
		t.Fatalf("retrieval_baseline = %v, want sqlite_canonical", ev["retrieval_baseline"])
	}
}

func TestSeq123P99RetrievalObservabilityMarkers(t *testing.T) {
	fake := &memoryFakeStore{
		memories: []store.Memory{
			{ID: 1, ChatSessionID: "sess-p99", TurnIndex: 1, SummaryJSON: "{\"summary\":\"memory only\"}"},
		},
		chatLogs: []store.ChatLog{
			{ID: 1, ChatSessionID: "sess-p99", TurnIndex: 1, Role: "user", Content: "fallback log"},
		},
	}
	cfg := config.Default()
	srv := NewServer(cfg)
	srv.Store = fake

	mux := http.NewServeMux()
	srv.RegisterRoutes(mux)

	req := httptest.NewRequest(http.MethodPost, "/search", strings.NewReader("{\"chat_session_id\":\"sess-p99\",\"user_input\":\"memory\",\"top_k\":5}"))
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
	if resp["observability_status"] != "shadow_r1" {
		t.Fatalf("observability_status = %v, want shadow_r1", resp["observability_status"])
	}
	if _, ok := resp["fallback_rate_metric"]; !ok {
		t.Fatal("missing fallback_rate_metric")
	}
	if resp["stale_hit_metric"] != float64(0) {
		t.Fatalf("stale_hit_metric = %v, want 0", resp["stale_hit_metric"])
	}
	if resp["no_candidate_metric"] != float64(0) {
		t.Fatalf("no_candidate_metric = %v, want 0", resp["no_candidate_metric"])
	}
	if resp["hydration_miss_metric"] != float64(0) {
		t.Fatalf("hydration_miss_metric = %v, want 0", resp["hydration_miss_metric"])
	}
}

func TestSeq123P100MultiTierLiveCutoverPrerequisiteMarkers(t *testing.T) {
	cfg := config.Default()
	srv := NewServer(cfg)

	mux := http.NewServeMux()
	srv.RegisterRoutes(mux)

	req := httptest.NewRequest(http.MethodPost, "/chroma-shadow/adoption-gate", strings.NewReader("{\"chat_session_id\":\"sess-p100\"}"))
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
	if resp["live_cutover_allowed"] != false {
		t.Fatalf("live_cutover_allowed = %v, want false", resp["live_cutover_allowed"])
	}
	prereq, ok := resp["cutover_prerequisites"].([]any)
	if !ok || len(prereq) == 0 {
		t.Fatalf("missing cutover_prerequisites: %#v", resp["cutover_prerequisites"])
	}
	gates, ok := resp["required_green_gates"].([]any)
	if !ok || len(gates) == 0 {
		t.Fatalf("missing required_green_gates: %#v", resp["required_green_gates"])
	}
	if resp["multi_tier_cutover_scope"] != "memory_only" {
		t.Fatalf("multi_tier_cutover_scope = %v, want memory_only", resp["multi_tier_cutover_scope"])
	}
	if resp["adoption_gate_state"] != "closed" {
		t.Fatalf("adoption_gate_state = %v, want closed", resp["adoption_gate_state"])
	}
}
