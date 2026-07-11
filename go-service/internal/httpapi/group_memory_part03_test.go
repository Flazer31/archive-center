package httpapi

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/risulongmemory/archive-center-go/internal/config"
	"github.com/risulongmemory/archive-center-go/internal/store"
	"github.com/risulongmemory/archive-center-go/internal/vector"
)

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
