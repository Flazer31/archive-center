package httpapi

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/risulongmemory/archive-center-go/internal/config"
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
