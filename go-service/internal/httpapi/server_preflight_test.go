package httpapi

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/risulongmemory/archive-center-go/internal/config"
	"github.com/risulongmemory/archive-center-go/internal/vector"
)

func TestValidateRuntimeDependenciesChecksChromaV2HeartbeatAndCollection(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/api/v2/heartbeat":
			_, _ = w.Write([]byte(`{"nanosecond heartbeat":1}`))
		case r.Method == http.MethodGet && r.URL.Path == "/api/v2/tenants/default_tenant/databases/default_database/collections/archive_center_vectors":
			_, _ = w.Write([]byte(`{"id":"collection-1","name":"archive_center_vectors"}`))
		case r.Method == http.MethodGet && r.URL.Path == "/api/v2/tenants/default_tenant/databases/default_database/collections/collection-1/count":
			_, _ = w.Write([]byte(`0`))
		case r.Method == http.MethodGet && r.URL.Path == "/api/v2/tenants/default_tenant/databases/default_database/collections/archive_center_reference_vectors":
			_, _ = w.Write([]byte(`{"id":"reference-collection-1","name":"archive_center_reference_vectors"}`))
		case r.Method == http.MethodGet && r.URL.Path == "/api/v2/tenants/default_tenant/databases/default_database/collections/reference-collection-1/count":
			_, _ = w.Write([]byte(`0`))
		default:
			http.Error(w, r.Method+" "+r.URL.Path, http.StatusNotFound)
		}
	}))
	defer server.Close()

	cfg := config.Default()
	cfg.ChromaEnabled = true
	cfg.ChromaEndpoint = server.URL
	cfg.ChromaAPIPath = "/api/v2"
	srv := NewServer(cfg)
	if err := srv.ValidateRuntimeDependencies(context.Background()); err != nil {
		t.Fatalf("ValidateRuntimeDependencies: %v", err)
	}
}

func TestValidateRuntimeDependenciesRejectsIncompatibleChromaAPI(t *testing.T) {
	server := httptest.NewServer(http.NotFoundHandler())
	defer server.Close()

	cfg := config.Default()
	cfg.ChromaEnabled = true
	cfg.ChromaEndpoint = server.URL
	cfg.ChromaAPIPath = "/api/v2"
	srv := NewServer(cfg)
	if err := srv.ValidateRuntimeDependencies(context.Background()); err == nil {
		t.Fatal("expected incompatible Chroma API to fail startup preflight")
	}
}

func TestValidateRuntimeDependenciesAllowsUnavailableReferenceCollection(t *testing.T) {
	server := newMainHealthyReferenceUnavailableChroma(t)
	defer server.Close()

	cfg := config.Default()
	cfg.ChromaEnabled = true
	cfg.ChromaEndpoint = server.URL
	cfg.ChromaAPIPath = "/api/v2"
	srv := NewServer(cfg)
	if err := srv.ValidateRuntimeDependencies(context.Background()); err != nil {
		t.Fatalf("reference-only Chroma failure blocked main startup: %v", err)
	}
}

func TestReadyReportsUnavailableReferenceCollectionWithoutBlockingMainReadiness(t *testing.T) {
	server := newMainHealthyReferenceUnavailableChroma(t)
	defer server.Close()

	cfg := config.Default()
	cfg.ChromaEnabled = true
	cfg.ChromaEndpoint = server.URL
	cfg.ChromaAPIPath = "/api/v2"
	cfg.RuntimeProfile = config.RuntimeProfileFullLocal
	cfg.VectorMode = config.VectorModeBundled
	srv := NewServer(cfg)
	mux := http.NewServeMux()
	srv.RegisterRoutes(mux)

	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/ready", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("ready status=%d body=%s", rec.Code, rec.Body.String())
	}
	var response readyResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &response); err != nil {
		t.Fatal(err)
	}
	if !response.Ready || !response.VectorReady || response.Degraded {
		t.Fatalf("main readiness changed by reference capability: %#v", response)
	}
	if response.ReferenceVectorReady || !response.ReferenceVectorDegraded {
		t.Fatalf("reference readiness was not isolated: %#v", response)
	}
	if response.Checks["reference_chromadb_vector"] != "health_error" || response.Checks["reference_chromadb_vector_error"] == "" || response.Checks["reference_chromadb_vector_error"] == "none" {
		t.Fatalf("reference readiness checks = %#v", response.Checks)
	}
}

func TestReadyLimitsSlowReferenceProbeWithoutBlockingMainReadiness(t *testing.T) {
	cfg := config.Default()
	cfg.ChromaEnabled = true
	cfg.ChromaEndpoint = "http://reference-probe.test"
	cfg.RuntimeProfile = config.RuntimeProfileFullLocal
	cfg.VectorMode = config.VectorModeBundled
	srv := NewServer(cfg)
	srv.Vector = &fakeVectorStore{healthSnapshot: vector.HealthSnapshot{Status: "ok", ModelReady: true}}
	srv.VectorOpenError = nil
	srv.ReferenceVector = &blockingHealthVectorStore{fakeVectorStore: &fakeVectorStore{}}
	srv.ReferenceVectorOpenError = nil
	mux := http.NewServeMux()
	srv.RegisterRoutes(mux)

	started := time.Now()
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/ready", nil))
	elapsed := time.Since(started)
	if rec.Code != http.StatusOK {
		t.Fatalf("ready status=%d body=%s", rec.Code, rec.Body.String())
	}
	if elapsed >= 1500*time.Millisecond {
		t.Fatalf("optional reference probe delayed main readiness for %s", elapsed)
	}
	var response readyResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &response); err != nil {
		t.Fatal(err)
	}
	if !response.Ready || !response.VectorReady || response.Degraded {
		t.Fatalf("main readiness changed by slow reference probe: %#v", response)
	}
	if response.ReferenceVectorReady || !response.ReferenceVectorDegraded {
		t.Fatalf("slow reference probe was not reported separately: %#v", response)
	}
}

type blockingHealthVectorStore struct {
	*fakeVectorStore
}

func (s *blockingHealthVectorStore) Health(ctx context.Context) (vector.HealthSnapshot, error) {
	<-ctx.Done()
	return vector.HealthSnapshot{}, ctx.Err()
}

func TestReferenceVectorFailureDoesNotBreakPrepareOrCompleteTurnStorage(t *testing.T) {
	cfg := config.Default()
	cfg.StoreMode = config.StoreModeMariaDBAuthority
	srv := NewServer(cfg)
	recordingStore := &turnRecordingStore{}
	srv.Store = recordingStore
	srv.StoreOpenError = nil
	srv.Vector = &fakeVectorStore{healthSnapshot: vector.HealthSnapshot{Status: "ok", ModelReady: true}}
	srv.ReferenceVector = &fakeVectorStore{healthErr: errors.New("reference collection unavailable")}
	srv.ReferenceVectorOpenError = errors.New("reference collection unavailable")
	mux := http.NewServeMux()
	srv.RegisterRoutes(mux)

	prepare := httptest.NewRecorder()
	mux.ServeHTTP(prepare, httptest.NewRequest(http.MethodPost, "/prepare-turn", bytes.NewBufferString(`{"chat_session_id":"session-1","raw_user_input":"continue","settings":{"injection_enabled":true,"max_injection_chars":1000}}`)))
	if prepare.Code != http.StatusOK {
		t.Fatalf("prepare-turn status=%d body=%s", prepare.Code, prepare.Body.String())
	}

	complete := httptest.NewRecorder()
	mux.ServeHTTP(complete, httptest.NewRequest(http.MethodPost, "/complete-turn", bytes.NewBufferString(`{"chat_session_id":"session-1","user_input":"continue","assistant_content":"continued"}`)))
	if complete.Code != http.StatusOK {
		t.Fatalf("complete-turn status=%d body=%s", complete.Code, complete.Body.String())
	}
	if len(recordingStore.savedChatLogs) != 2 || len(recordingStore.savedEffectiveInputs) != 1 {
		t.Fatalf("main turn storage was not preserved: logs=%d effective_inputs=%d", len(recordingStore.savedChatLogs), len(recordingStore.savedEffectiveInputs))
	}
}

func newMainHealthyReferenceUnavailableChroma(t *testing.T) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/api/v2/heartbeat":
			_, _ = w.Write([]byte(`{"nanosecond heartbeat":1}`))
		case r.Method == http.MethodGet && r.URL.Path == "/api/v2/tenants/default_tenant/databases/default_database/collections/archive_center_vectors":
			_, _ = w.Write([]byte(`{"id":"collection-1","name":"archive_center_vectors"}`))
		case r.Method == http.MethodGet && r.URL.Path == "/api/v2/tenants/default_tenant/databases/default_database/collections/collection-1/count":
			_, _ = w.Write([]byte(`0`))
		case r.Method == http.MethodGet && r.URL.Path == "/api/v2/tenants/default_tenant/databases/default_database/collections/archive_center_reference_vectors":
			http.Error(w, "reference collection unavailable", http.StatusServiceUnavailable)
		default:
			http.Error(w, r.Method+" "+r.URL.Path, http.StatusNotFound)
		}
	}))
}
