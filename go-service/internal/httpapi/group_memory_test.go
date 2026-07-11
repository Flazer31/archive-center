package httpapi

import (
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
