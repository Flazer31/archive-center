package vector

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
)

type chromaTestServerState struct {
	mu       sync.Mutex
	requests []string
	bodies   []map[string]any
}

func newChromaTestServer(t *testing.T, state *chromaTestServerState) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		state.mu.Lock()
		state.requests = append(state.requests, r.Method+" "+r.URL.Path)
		if r.Body != nil {
			var body map[string]any
			_ = json.NewDecoder(r.Body).Decode(&body)
			if len(body) > 0 {
				state.bodies = append(state.bodies, body)
			}
		}
		state.mu.Unlock()

		w.Header().Set("Content-Type", "application/json")
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/api/v2/heartbeat":
			_, _ = w.Write([]byte(`{"nanosecond heartbeat":1}`))
		case r.Method == http.MethodGet && r.URL.Path == "/api/v2/tenants/default_tenant/databases/default_database/collections/archive_center_vectors":
			_, _ = w.Write([]byte(`{"id":"collection-1","name":"archive_center_vectors"}`))
		case r.Method == http.MethodPost && r.URL.Path == "/api/v2/tenants/default_tenant/databases/default_database/collections/collection-1/upsert":
			_, _ = w.Write([]byte(`{"status":"ok"}`))
		case r.Method == http.MethodPost && r.URL.Path == "/api/v2/tenants/default_tenant/databases/default_database/collections/collection-1/query":
			_, _ = w.Write([]byte(`{
				"ids":[["doc-1"]],
				"documents":[["hello memory"]],
				"metadatas":[[{
					"tier":"memory",
					"chat_session_id":"sess-1",
					"source_table":"memories",
					"source_row_id":"7",
					"schema_version":"q1a.v1"
				}]],
				"distances":[[0.1]]
			}`))
		case r.Method == http.MethodPost && r.URL.Path == "/api/v2/tenants/default_tenant/databases/default_database/collections/collection-1/get":
			_, _ = w.Write([]byte(`{"ids":["doc-1","doc-2"]}`))
		case r.Method == http.MethodGet && r.URL.Path == "/api/v2/tenants/default_tenant/databases/default_database/collections/collection-1/count":
			_, _ = w.Write([]byte(`{"count":2}`))
		case r.Method == http.MethodPost && r.URL.Path == "/api/v2/tenants/default_tenant/databases/default_database/collections/collection-1/delete":
			_, _ = w.Write([]byte(`{"status":"ok"}`))
		default:
			http.Error(w, r.Method+" "+r.URL.Path, http.StatusNotFound)
		}
	}))
}

func TestChromaStoreUpsertSearchCountDelete(t *testing.T) {
	state := &chromaTestServerState{}
	ts := newChromaTestServer(t, state)
	defer ts.Close()

	raw, err := NewChromaStore(ts.URL, "archive_center_vectors", "/api/v2")
	if err != nil {
		t.Fatalf("NewChromaStore: %v", err)
	}
	store := raw.(interface {
		VectorStore
		DocumentDeleter
	})

	ctx := context.Background()
	if err := store.Upsert(ctx, "sess-1", []VectorDocument{{
		ID:            "doc-1",
		Embedding:     []float32{0.1, 0.2},
		Tier:          "memory",
		ChatSessionID: "sess-1",
		SourceTable:   "memories",
		SourceRowID:   "7",
		SchemaVersion: "q1a.v1",
		DocumentText:  "hello memory",
	}}); err != nil {
		t.Fatalf("Upsert: %v", err)
	}

	docs, err := store.Search(ctx, "sess-1", []float32{0.1, 0.2}, 3, "tier == memory")
	if err != nil {
		t.Fatalf("Search: %v", err)
	}
	if len(docs) != 1 || docs[0].ID != "doc-1" || docs[0].DocumentText != "hello memory" {
		t.Fatalf("unexpected docs: %+v", docs)
	}
	if docs[0].ChatSessionID != "sess-1" || docs[0].SourceTable != "memories" {
		t.Fatalf("metadata not mapped: %+v", docs[0])
	}

	total, err := store.Count(ctx, "")
	if err != nil || total != 2 {
		t.Fatalf("Count all = %d, %v", total, err)
	}
	sessionTotal, err := store.Count(ctx, "sess-1")
	if err != nil || sessionTotal != 2 {
		t.Fatalf("Count session = %d, %v", sessionTotal, err)
	}
	if err := store.DeleteSession(ctx, "sess-1"); err != nil {
		t.Fatalf("DeleteSession: %v", err)
	}
	if err := store.DeleteDocuments(ctx, []string{"doc-1"}); err != nil {
		t.Fatalf("DeleteDocuments: %v", err)
	}
	health, err := store.Health(ctx)
	if err != nil {
		t.Fatalf("Health: %v", err)
	}
	if health.Status != "ok" || health.Collection != "archive_center_vectors" || !health.ModelReady {
		t.Fatalf("unexpected health: %+v", health)
	}

	state.mu.Lock()
	defer state.mu.Unlock()
	joined := strings.Join(state.requests, "\n")
	for _, want := range []string{
		"GET /api/v2/tenants/default_tenant/databases/default_database/collections/archive_center_vectors",
		"POST /api/v2/tenants/default_tenant/databases/default_database/collections/collection-1/upsert",
		"POST /api/v2/tenants/default_tenant/databases/default_database/collections/collection-1/query",
		"POST /api/v2/tenants/default_tenant/databases/default_database/collections/collection-1/get",
		"GET /api/v2/tenants/default_tenant/databases/default_database/collections/collection-1/count",
		"POST /api/v2/tenants/default_tenant/databases/default_database/collections/collection-1/delete",
		"GET /api/v2/heartbeat",
	} {
		if !strings.Contains(joined, want) {
			t.Fatalf("missing request %q in:\n%s", want, joined)
		}
	}
}

func TestChromaStoreResetAllDeletesCollection(t *testing.T) {
	var got []string
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		got = append(got, r.Method+" "+r.URL.Path)
		if r.Method == http.MethodGet && r.URL.Path == "/api/v2/tenants/default_tenant/databases/default_database/collections/archive_center_vectors" {
			_, _ = w.Write([]byte(`{"id":"collection-1","name":"archive_center_vectors"}`))
			return
		}
		if r.Method == http.MethodDelete && r.URL.Path == "/api/v2/tenants/default_tenant/databases/default_database/collections/collection-1" {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		http.Error(w, r.Method+" "+r.URL.Path, http.StatusNotFound)
	}))
	defer ts.Close()

	raw, err := NewChromaStore(ts.URL, "archive_center_vectors", "/api/v2")
	if err != nil {
		t.Fatalf("NewChromaStore: %v", err)
	}
	resetter, ok := raw.(CollectionResetter)
	if !ok {
		t.Fatal("chroma store should implement CollectionResetter")
	}
	if err := resetter.ResetAll(context.Background()); err != nil {
		t.Fatalf("ResetAll: %v", err)
	}
	want := []string{
		"GET /api/v2/tenants/default_tenant/databases/default_database/collections/archive_center_vectors",
		"DELETE /api/v2/tenants/default_tenant/databases/default_database/collections/collection-1",
	}
	if strings.Join(got, "\n") != strings.Join(want, "\n") {
		t.Fatalf("requests = %#v, want %#v", got, want)
	}
}

func TestChromaStoreCreatesCollectionWhenV2ReturnsInvalidCollection(t *testing.T) {
	var got []string
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		got = append(got, r.Method+" "+r.URL.Path)
		w.Header().Set("Content-Type", "application/json")
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/api/v2/tenants/default_tenant/databases/default_database/collections/archive_center_vectors":
			w.WriteHeader(http.StatusBadRequest)
			_, _ = w.Write([]byte(`{"error":"InvalidCollection","message":"Collection archive_center_vectors does not exist."}`))
		case r.Method == http.MethodPost && r.URL.Path == "/api/v2/tenants/default_tenant/databases/default_database/collections":
			_, _ = w.Write([]byte(`{"id":"collection-created","name":"archive_center_vectors"}`))
		case r.Method == http.MethodGet && r.URL.Path == "/api/v2/tenants/default_tenant/databases/default_database/collections/collection-created/count":
			_, _ = w.Write([]byte(`{"count":0}`))
		default:
			http.Error(w, r.Method+" "+r.URL.Path, http.StatusNotFound)
		}
	}))
	defer ts.Close()

	raw, err := NewChromaStore(ts.URL, "archive_center_vectors", "/api/v2")
	if err != nil {
		t.Fatalf("NewChromaStore: %v", err)
	}
	count, err := raw.Count(context.Background(), "")
	if err != nil {
		t.Fatalf("Count should create missing collection after InvalidCollection: %v", err)
	}
	if count != 0 {
		t.Fatalf("count = %d, want 0", count)
	}
	joined := strings.Join(got, "\n")
	for _, want := range []string{
		"GET /api/v2/tenants/default_tenant/databases/default_database/collections/archive_center_vectors",
		"POST /api/v2/tenants/default_tenant/databases/default_database/collections",
		"GET /api/v2/tenants/default_tenant/databases/default_database/collections/collection-created/count",
	} {
		if !strings.Contains(joined, want) {
			t.Fatalf("missing request %q in:\n%s", want, joined)
		}
	}
}

func TestChromaStoreUpsertReportsDimensionMismatch(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/api/v2/tenants/default_tenant/databases/default_database/collections/archive_center_vectors":
			_, _ = w.Write([]byte(`{"id":"collection-1","name":"archive_center_vectors"}`))
		case r.Method == http.MethodPost && r.URL.Path == "/api/v2/tenants/default_tenant/databases/default_database/collections/collection-1/upsert":
			w.WriteHeader(http.StatusBadRequest)
			_, _ = w.Write([]byte(`{"error":"InvalidArgumentError","message":"Collection expecting embedding with dimension of 3072, got 1024"}`))
		default:
			http.Error(w, r.Method+" "+r.URL.Path, http.StatusNotFound)
		}
	}))
	defer ts.Close()

	raw, err := NewChromaStore(ts.URL, "archive_center_vectors", "/api/v2")
	if err != nil {
		t.Fatalf("NewChromaStore: %v", err)
	}
	err = raw.Upsert(context.Background(), "sess-1", []VectorDocument{{
		ID:            "memory:sess-1:1",
		Embedding:     make([]float32, 1024),
		Tier:          "memory",
		ChatSessionID: "sess-1",
		SourceTable:   "memories",
		SourceRowID:   "1",
		SchemaVersion: "memory.v2",
		DocumentText:  "hello",
	}})
	if err == nil {
		t.Fatal("expected dimension mismatch error")
	}
	text := err.Error()
	if !strings.Contains(text, "chroma collection dimension mismatch") || !strings.Contains(text, "current embedding dimension=1024") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestChromaWhereBuildsSessionAndTierFilter(t *testing.T) {
	where := chromaWhere("sess-1", `tier == "memory"`)
	raw, _ := json.Marshal(where)
	text := string(raw)
	for _, want := range []string{`"$and"`, `"chat_session_id":"sess-1"`, `"tier":"memory"`} {
		if !strings.Contains(text, want) {
			t.Fatalf("where %s missing %s", text, want)
		}
	}
}

func TestNewChromaStoreRequiresEndpoint(t *testing.T) {
	if _, err := NewChromaStore("", "", ""); err == nil {
		t.Fatal("expected endpoint error")
	}
}
