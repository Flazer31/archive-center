package httpapi

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/risulongmemory/archive-center-go/internal/config"
	"github.com/risulongmemory/archive-center-go/internal/store"
	"github.com/risulongmemory/archive-center-go/internal/vector"
)

type recoveryCacheFixture struct{ store.Store }

func (*recoveryCacheFixture) ReadVectorRecoveryCache(context.Context, []string) ([]string, error) {
	return []string{`{"ID":"precise:session:1","DocumentText":"same text","Embedding":[1,0],"Metadata":{"embedding_model":"saved-model"}}`}, nil
}

func TestRecoveryReusesMariaDBMaterialization(t *testing.T) {
	s := setupTestServer()
	s.Store = &recoveryCacheFixture{Store: s.Store}
	docs := []vector.VectorDocument{{ID: "precise:session:1", DocumentText: "same text", Metadata: map[string]any{"embedding_model": "saved-model"}}}
	if err := s.fillRecoveryEmbeddings(context.Background(), docs); err != nil {
		t.Fatal(err)
	}
	if len(docs[0].Embedding) != 2 {
		t.Fatal("saved materialization not reused")
	}
	if count, err := s.generateMissingRecoveryEmbeddings(context.Background(), docs); err != nil || count != 0 {
		t.Fatalf("unexpected new embedding: %d %v", count, err)
	}
}

func TestRecoveryEmbeddingOnlyRequestsMissingDocuments(t *testing.T) {
	var requests atomic.Int32
	api := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests.Add(1)
		var req struct {
			Input any    `json:"input"`
			Model string `json:"model"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Error(err)
		}
		if req.Model != "saved-model" || req.Input != "missing vector text" {
			t.Errorf("unexpected embedding request: %+v", req)
		}
		_, _ = w.Write([]byte(`{"data":[{"index":0,"embedding":[1,0]}],"model":"saved-model"}`))
	}))
	defer api.Close()
	s := setupTestServer()
	s.updateRuntimeConfig(map[string]any{"embeddingProvider": "openai", "embeddingApiKey": "synthetic", "embeddingEndpoint": api.URL + "/v1/embeddings", "embeddingModel": "current-model", "embeddingTimeout": 30})
	docs := []vector.VectorDocument{
		{ID: "already-present", DocumentText: "cached vector text", Embedding: []float32{0, 1}},
		{ID: "missing", DocumentText: "missing vector text", Metadata: map[string]any{"embedding_model": "saved-model"}},
	}
	n, err := s.generateMissingRecoveryEmbeddings(context.Background(), docs)
	if err != nil || n != 1 || requests.Load() != 1 || len(docs[1].Embedding) != 2 {
		t.Fatalf("n=%d requests=%d docs=%+v err=%v", n, requests.Load(), docs, err)
	}
	if docs[0].Embedding[1] != 1 {
		t.Fatal("cached embedding changed")
	}
}

type startupIndexFailure struct {
	vector.VectorStore
	fail error
}

func (v *startupIndexFailure) Health(context.Context) (vector.HealthSnapshot, error) {
	return vector.HealthSnapshot{}, v.fail
}
func (v *startupIndexFailure) ResumeIndexRecovery(context.Context, string) error { return nil }
func (v *startupIndexFailure) RecoverySnapshot(context.Context, string) ([]vector.VectorDocument, int, error) {
	return nil, 0, errors.New("unavailable fixture snapshot")
}
func (v *startupIndexFailure) RecoverIndex(context.Context, string, func(vector.VectorStore) error) (string, error) {
	return "", errors.New("unexpected rebuild")
}

func TestStartupIndexFailureKeepsManagementAvailable(t *testing.T) {
	t.Setenv("ARCHIVE_CENTER_DATA_DIR", t.TempDir())
	t.Setenv("AC_LOG_DIR", t.TempDir())
	s := setupTestServer()
	s.Cfg.StoreMode = config.StoreModeMariaDBAuthority
	s.Cfg.ChromaEnabled = true
	s.Cfg.ChromaEndpoint = "http://127.0.0.1:8000"
	s.Vector = &startupIndexFailure{VectorStore: vector.NewFakeVectorStore(), fail: errors.New("Error loading hnsw index")}
	if err := s.ValidateRuntimeDependencies(context.Background()); err != nil {
		t.Fatal(err)
	}
	waitForIndexRecoveryState(t, s, 3)
	if s.indexRecoveryState.Load() != 3 {
		t.Fatal("incomplete recovery not reported")
	}
	if s.StartMemoryWorkers(context.Background()) {
		t.Fatal("workers started on a damaged index")
	}
	mux := http.NewServeMux()
	s.RegisterRoutes(mux)
	resp := httptest.NewRecorder()
	mux.ServeHTTP(resp, httptest.NewRequest(http.MethodGet, "/ready", nil))
	if !strings.Contains(resp.Body.String(), "Error loading hnsw index") {
		t.Fatalf("readiness hid index failure: %s", resp.Body.String())
	}
	s.Vector = &startupIndexFailure{VectorStore: vector.NewFakeVectorStore(), fail: errors.New("connection refused")}
	if err := s.ValidateRuntimeDependencies(context.Background()); err == nil {
		t.Fatal("unrelated startup failure was silently ignored")
	}
}

func TestStartupIndexRecoveryPreservesReadOnlyMode(t *testing.T) {
	s := setupTestServer()
	s.Cfg.StoreMode = config.StoreModeMariaDBReadShadow
	s.Cfg.ChromaEnabled = true
	s.Cfg.ChromaEndpoint = "http://127.0.0.1:8000"
	s.Vector = &startupIndexFailure{VectorStore: vector.NewFakeVectorStore(), fail: errors.New("Error loading hnsw index")}
	if err := s.ValidateRuntimeDependencies(context.Background()); !vector.IsIndexLoadError(err) {
		t.Fatalf("read-only startup failure changed: %v", err)
	}
	if s.indexRecoveryState.Load() != 0 {
		t.Fatal("read-only mode entered automatic recovery")
	}
}

// The fixture driver starts a disposable real Chroma 1.5.9, corrupts its HNSW
// header while it is stopped and passes only the fixture endpoint/data directory.
func TestStartupChromaRecoveryIntegration(t *testing.T) {
	endpoint := os.Getenv("AC_CHROMA_RECOVERY_INTEGRATION_ENDPOINT")
	if endpoint == "" {
		t.Skip("isolated corrupted Chroma fixture not configured")
	}
	s := setupTestServer()
	s.Cfg.StoreMode = config.StoreModeMariaDBAuthority
	s.Cfg.ChromaEnabled = true
	s.Cfg.ChromaEndpoint = endpoint
	s.Cfg.ChromaCollection = "ac_recovery_live"
	raw, err := vector.NewChromaStore(endpoint, s.Cfg.ChromaCollection, "/api/v2")
	if err != nil {
		t.Fatal(err)
	}
	s.Vector = vector.NewMutationFencedStore(raw)
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()
	if _, err := s.Vector.Health(ctx); !vector.IsIndexLoadError(err) {
		t.Fatalf("fixture did not reproduce reported failure: %v", err)
	}
	if err := s.ValidateRuntimeDependencies(ctx); err != nil {
		t.Fatal(err)
	}
	if os.Getenv("AC_CHROMA_RECOVERY_MISSING") == "1" {
		waitForIndexRecoveryState(t, s, 1)
		if s.indexRecoveryState.Load() != 1 {
			t.Fatal("missing embedding should wait for Host configuration")
		}
		var requests atomic.Int32
		api := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			requests.Add(1)
			var req map[string]any
			_ = json.NewDecoder(r.Body).Decode(&req)
			if req["input"] != "Synthetic bridge repair memory 0" {
				t.Errorf("unexpected recovery input: %v", req["input"])
			}
			_, _ = w.Write([]byte(`{"data":[{"index":0,"embedding":[1,0.05,0]}]}`))
		}))
		defer api.Close()
		mux := http.NewServeMux()
		s.RegisterRoutes(mux)
		body, _ := json.Marshal(map[string]any{"embeddingProvider": "openai", "embeddingApiKey": "synthetic", "embeddingEndpoint": api.URL + "/v1/embeddings", "embeddingModel": "synthetic-model", "embeddingTimeout": 30})
		resp := httptest.NewRecorder()
		mux.ServeHTTP(resp, httptest.NewRequest(http.MethodPost, "/config/update", bytes.NewReader(body)))
		if resp.Code != 200 {
			t.Fatalf("config sync: %s", resp.Body.String())
		}
		for s.indexRecoveryState.Load() != 0 && ctx.Err() == nil {
			time.Sleep(10 * time.Millisecond)
		}
		if requests.Load() != 1 {
			t.Fatalf("embedding requests=%d, want only the missing document", requests.Load())
		}
		mux.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodPost, "/config/update", bytes.NewReader(body)))
		if requests.Load() != 1 {
			t.Fatal("unchanged settings repeated recovery")
		}
	}
	waitForIndexRecoveryState(t, s, 0)
	if s.indexRecoveryState.Load() != 0 {
		t.Fatal("recovery did not finish")
	}
	count, err := s.Vector.Count(ctx, "")
	if err != nil || count != 12 {
		t.Fatalf("count=%d err=%v", count, err)
	}
	results, err := s.Vector.Search(ctx, "session-a", []float32{1, 0, 0}, 2, "")
	if err != nil || len(results) != 2 {
		t.Fatalf("search=%+v err=%v", results, err)
	}
	for _, doc := range results {
		if doc.ChatSessionID != "session-a" {
			t.Fatalf("session leaked: %+v", doc)
		}
	}
	fresh, _ := vector.NewChromaStore(endpoint, s.Cfg.ChromaCollection, "/api/v2")
	if count, err := fresh.Count(ctx, ""); err != nil || count != 12 {
		t.Fatalf("reopen count=%d err=%v", count, err)
	}
	other, _ := vector.NewChromaStore(endpoint, "ac_recovery_untouched", "/api/v2")
	if count, err := other.Count(ctx, ""); err != nil || count != 1 {
		t.Fatalf("other collection changed: %d %v", count, err)
	}
}

func waitForIndexRecoveryState(t *testing.T, s *Server, want int32) {
	t.Helper()
	deadline := time.After(10 * time.Second)
	ticker := time.NewTicker(time.Millisecond)
	defer ticker.Stop()
	for s.indexRecoveryState.Load() != want {
		select {
		case <-deadline:
			t.Fatalf("recovery state=%d, want %d", s.indexRecoveryState.Load(), want)
		case <-ticker.C:
		}
	}
}
