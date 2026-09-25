package httpapi

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"github.com/risulongmemory/archive-center-go/internal/config"
	"github.com/risulongmemory/archive-center-go/internal/vector"
)

// Only the external vector store is substituted. Startup, readiness, settings
// admission, missing embedding requests and recovery orchestration are real.
type managementRecoveryVector struct {
	vector.VectorStore
	release   chan struct{}
	entered   chan struct{}
	docs      []vector.VectorDocument
	stored    []vector.VectorDocument
	healthy   atomic.Bool
	snapshots atomic.Int32
}

func (v *managementRecoveryVector) Health(context.Context) (vector.HealthSnapshot, error) {
	if v.healthy.Load() {
		return vector.HealthSnapshot{Status: "ok", ModelReady: true}, nil
	}
	return vector.HealthSnapshot{}, errors.New("Error loading hnsw index")
}
func (v *managementRecoveryVector) ResumeIndexRecovery(context.Context, string) error { return nil }
func (v *managementRecoveryVector) RecoverySnapshot(ctx context.Context, _ string) ([]vector.VectorDocument, int, error) {
	if v.snapshots.Add(1) == 1 && v.entered != nil {
		close(v.entered)
	}
	if v.release != nil {
		select {
		case <-v.release:
		case <-ctx.Done():
			return nil, 0, ctx.Err()
		}
	}
	return append([]vector.VectorDocument(nil), v.docs...), 2, nil
}
func (v *managementRecoveryVector) RecoverIndex(_ context.Context, _ string, fill func(vector.VectorStore) error) (string, error) {
	if err := fill(v); err != nil {
		return "", err
	}
	v.healthy.Store(true)
	return "retained-original", nil
}
func (v *managementRecoveryVector) Upsert(_ context.Context, _ string, docs []vector.VectorDocument) error {
	v.stored = append(v.stored, docs...)
	return nil
}
func (v *managementRecoveryVector) GetDocuments(_ context.Context, ids []string) ([]vector.VectorDocument, error) {
	out := []vector.VectorDocument{}
	for _, id := range ids {
		for _, doc := range v.stored {
			if doc.ID == id {
				out = append(out, doc)
			}
		}
	}
	return out, nil
}
func (v *managementRecoveryVector) Search(context.Context, string, []float32, int, string) ([]vector.VectorDocument, error) {
	return v.stored, nil
}

func managementRecoveryServer(t *testing.T, v *managementRecoveryVector) (*Server, context.Context) {
	t.Helper()
	t.Setenv("ARCHIVE_CENTER_DATA_DIR", t.TempDir())
	t.Setenv("AC_LOG_DIR", t.TempDir())
	s := setupTestServer()
	s.Cfg.StoreMode = config.StoreModeMariaDBAuthority
	s.Cfg.VectorMode = config.VectorModeBundled
	s.Cfg.ChromaEnabled = true
	s.Cfg.ChromaEndpoint = "http://127.0.0.1:8000"
	s.Vector = v
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	return s, ctx
}

func TestStartupRecoveryDoesNotBlockManagement(t *testing.T) {
	for _, cancelled := range []bool{false, true} {
		t.Run(map[bool]string{false: "completed", true: "cancelled"}[cancelled], func(t *testing.T) {
			v := &managementRecoveryVector{VectorStore: vector.NewFakeVectorStore(), release: make(chan struct{}), entered: make(chan struct{})}
			s, base := managementRecoveryServer(t, v)
			ctx, cancel := context.WithCancel(base)
			defer cancel()
			returned := make(chan error, 1)
			go func() { returned <- s.ValidateRuntimeDependencies(ctx) }()
			select {
			case err := <-returned:
				if err != nil {
					t.Fatal(err)
				}
			case <-time.After(time.Second):
				t.Fatal("management startup waited for vector recovery")
			}
			<-v.entered
			mux := http.NewServeMux()
			s.RegisterRoutes(mux)
			health := httptest.NewRecorder()
			mux.ServeHTTP(health, httptest.NewRequest("GET", "/health", nil))
			if health.Code != 200 {
				t.Fatal("management liveness unavailable")
			}
			ready := httptest.NewRecorder()
			mux.ServeHTTP(ready, httptest.NewRequest("GET", "/ready", nil))
			var before readyResponse
			_ = json.Unmarshal(ready.Body.Bytes(), &before)
			if ready.Code != 503 || before.Ready || before.Checks["chromadb_recovery"] != "running" {
				t.Fatalf("false readiness: %s", ready.Body.String())
			}
			if s.StartMemoryWorkers(ctx) {
				t.Fatal("workers started during recovery")
			}
			if cancelled {
				cancel()
				waitForIndexRecoveryState(t, s, 3)
				return
			}
			close(v.release)
			waitForIndexRecoveryState(t, s, 0)
			ready = httptest.NewRecorder()
			mux.ServeHTTP(ready, httptest.NewRequest("GET", "/ready", nil))
			var after readyResponse
			_ = json.Unmarshal(ready.Body.Bytes(), &after)
			if ready.Code != 200 || !after.Ready {
				t.Fatalf("recovery did not restore readiness: %s", ready.Body.String())
			}
		})
	}
}

func TestStartupRecoverySettingsWaitResumesThroughConfigAPI(t *testing.T) {
	v := &managementRecoveryVector{VectorStore: vector.NewFakeVectorStore(), docs: []vector.VectorDocument{{ID: "synthetic", DocumentText: "Synthetic repair memory"}}}
	s, ctx := managementRecoveryServer(t, v)
	if err := s.ValidateRuntimeDependencies(ctx); err != nil {
		t.Fatal(err)
	}
	waitForIndexRecoveryState(t, s, 1)
	mux := http.NewServeMux()
	s.RegisterRoutes(mux)
	ready := httptest.NewRecorder()
	mux.ServeHTTP(ready, httptest.NewRequest("GET", "/ready", nil))
	var before readyResponse
	_ = json.Unmarshal(ready.Body.Bytes(), &before)
	if before.Ready || before.Checks["chromadb_recovery"] != "waiting" {
		t.Fatalf("missing settings state: %s", ready.Body.String())
	}
	var requests atomic.Int32
	embed := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests.Add(1)
		var req map[string]any
		_ = json.NewDecoder(r.Body).Decode(&req)
		if req["input"] != "Synthetic repair memory" {
			t.Errorf("unexpected embedding input: %v", req["input"])
		}
		_, _ = w.Write([]byte(`{"data":[{"index":0,"embedding":[1,0]}]}`))
	}))
	defer embed.Close()
	body, _ := json.Marshal(map[string]any{"embeddingProvider": "openai", "embeddingApiKey": "synthetic", "embeddingEndpoint": embed.URL + "/v1/embeddings", "embeddingModel": "synthetic-model", "embeddingTimeout": 30})
	response := httptest.NewRecorder()
	mux.ServeHTTP(response, httptest.NewRequest("POST", "/config/update", bytes.NewReader(body)))
	if response.Code != 200 {
		t.Fatalf("management settings unavailable: %s", response.Body.String())
	}
	waitForIndexRecoveryState(t, s, 0)
	if requests.Load() != 1 || len(v.stored) != 1 || len(v.stored[0].Embedding) != 2 {
		t.Fatal("settings did not resume missing-vector recovery")
	}
	mux.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest("POST", "/config/update", bytes.NewReader(body)))
	if requests.Load() != 1 || v.snapshots.Load() != 2 {
		t.Fatal("unchanged settings repeated recovery")
	}
}
