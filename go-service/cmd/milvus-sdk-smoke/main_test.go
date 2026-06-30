package main

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/risulongmemory/archive-center-go/internal/vector"
)

// mockStore is a test double for vector.VectorStore.
type mockStore struct {
	upsertErr    error
	searchDocs   []vector.VectorDocument
	searchErr    error
	healthSnap   vector.HealthSnapshot
	healthErr    error
	countResult  int
	upsertDocs   []vector.VectorDocument
	searchCalls  []mockSearchCall
	ensureCalled bool
	ensureErr    error
	closeCalled  bool
}

type mockSearchCall struct {
	sessionID string
	limit     int
	filter    string
}

func (m *mockStore) Search(ctx context.Context, sessionID string, vector []float32, limit int, filter string) ([]vector.VectorDocument, error) {
	m.searchCalls = append(m.searchCalls, mockSearchCall{sessionID: sessionID, limit: limit, filter: filter})
	if m.searchErr != nil {
		return nil, m.searchErr
	}
	return m.searchDocs, nil
}

func (m *mockStore) Upsert(ctx context.Context, sessionID string, docs []vector.VectorDocument) error {
	m.upsertDocs = append([]vector.VectorDocument(nil), docs...)
	return m.upsertErr
}

func (m *mockStore) DeleteSession(ctx context.Context, sessionID string) error { return nil }
func (m *mockStore) Rebuild(ctx context.Context, sessionID string) error       { return nil }

func (m *mockStore) Health(ctx context.Context) (vector.HealthSnapshot, error) {
	if m.healthErr != nil {
		return vector.HealthSnapshot{}, m.healthErr
	}
	return m.healthSnap, nil
}

func (m *mockStore) Count(ctx context.Context, sessionID string) (int, error) {
	return m.countResult, nil
}

func (m *mockStore) EnsureCollection(ctx context.Context, dimension int) error {
	m.ensureCalled = true
	return m.ensureErr
}

func (m *mockStore) Close(ctx context.Context) error {
	m.closeCalled = true
	return nil
}

func TestRunGuardedWithoutExecute(t *testing.T) {
	panicFactory := func(string) (vector.VectorStore, error) {
		panic("should not connect when not executing")
	}
	report, code := run("localhost:19530", false, false, "archive_center_vectors", 4, "", panicFactory)
	if code != 2 {
		t.Fatalf("exit code = %d, want 2", code)
	}
	if report.Status != "guarded" {
		t.Fatalf("status = %q, want guarded", report.Status)
	}
	if report.MilvusLiveEnabled {
		t.Error("MilvusLiveEnabled should be false")
	}
	if report.LiveRetrievalEnabled {
		t.Error("LiveRetrievalEnabled should be false")
	}
}

func TestRunFailedWithMissingEndpoint(t *testing.T) {
	panicFactory := func(string) (vector.VectorStore, error) {
		panic("should not connect when endpoint is missing")
	}
	report, code := run("", true, false, "archive_center_vectors", 4, "", panicFactory)
	if code != 2 {
		t.Fatalf("exit code = %d, want 2", code)
	}
	if report.Status != "failed" {
		t.Fatalf("status = %q, want failed", report.Status)
	}
}

func TestRunExecuteSuccess(t *testing.T) {
	mock := &mockStore{
		searchDocs: []vector.VectorDocument{
			{ID: "smoke-doc-1", DocumentText: "hello"},
			{ID: "smoke-doc-2", DocumentText: "world"},
		},
		healthSnap: vector.HealthSnapshot{
			Status:     "loaded",
			ModelReady: true,
		},
	}
	factory := func(endpoint string) (vector.VectorStore, error) {
		if endpoint == "" {
			return nil, vector.ErrNotEnabled
		}
		return mock, nil
	}

	report, code := run("grpc://localhost:19530", true, false, "archive_center_vectors", 4, "", factory)
	if code != 0 {
		t.Fatalf("exit code = %d, want 0", code)
	}
	if report.Status != "ok" {
		t.Fatalf("status = %q, want ok", report.Status)
	}
	if report.UpsertedCount != 2 {
		t.Errorf("upserted count = %d, want 2", report.UpsertedCount)
	}
	if report.SearchResultCount != 2 {
		t.Errorf("search result count = %d, want 2", report.SearchResultCount)
	}
	if len(report.TopIDs) != 2 || report.TopIDs[0] != "smoke-doc-1" || report.TopIDs[1] != "smoke-doc-2" {
		t.Errorf("top ids = %v, want [smoke-doc-1 smoke-doc-2]", report.TopIDs)
	}
	if report.HealthStatus != "loaded" {
		t.Errorf("health status = %q, want loaded", report.HealthStatus)
	}
	if !report.HealthModelReady {
		t.Error("health model ready should be true")
	}
	if !mock.closeCalled {
		t.Error("close should have been called")
	}
}

func TestRunExecuteConnectFailure(t *testing.T) {
	factory := func(endpoint string) (vector.VectorStore, error) {
		return nil, vector.ErrNotEnabled
	}
	report, code := run("bad-endpoint", true, false, "archive_center_vectors", 4, "", factory)
	if code != 1 {
		t.Fatalf("exit code = %d, want 1", code)
	}
	if report.Status != "failed" {
		t.Fatalf("status = %q, want failed", report.Status)
	}
}

func TestRunExecuteUpsertFailure(t *testing.T) {
	mock := &mockStore{upsertErr: vector.ErrNotEnabled}
	factory := func(endpoint string) (vector.VectorStore, error) {
		return mock, nil
	}
	report, code := run("grpc://localhost:19530", true, false, "archive_center_vectors", 4, "", factory)
	if code != 1 {
		t.Fatalf("exit code = %d, want 1", code)
	}
	if report.Status != "failed" {
		t.Fatalf("status = %q, want failed", report.Status)
	}
	if !mock.closeCalled {
		t.Error("close should be called after failed upsert")
	}
}

func TestRunExecuteSearchFailure(t *testing.T) {
	mock := &mockStore{searchErr: vector.ErrNotFound}
	factory := func(endpoint string) (vector.VectorStore, error) {
		return mock, nil
	}
	report, code := run("grpc://localhost:19530", true, false, "archive_center_vectors", 4, "", factory)
	if code != 1 {
		t.Fatalf("exit code = %d, want 1", code)
	}
	if report.Status != "failed" {
		t.Fatalf("status = %q, want failed", report.Status)
	}
}

func TestRunExecuteHealthOptional(t *testing.T) {
	mock := &mockStore{
		searchDocs: []vector.VectorDocument{
			{ID: "smoke-doc-1"},
		},
		healthErr: vector.ErrNotEnabled,
	}
	factory := func(endpoint string) (vector.VectorStore, error) {
		return mock, nil
	}
	report, code := run("grpc://localhost:19530", true, false, "archive_center_vectors", 4, "", factory)
	if code != 0 {
		t.Fatalf("exit code = %d, want 0", code)
	}
	if report.Status != "ok" {
		t.Fatalf("status = %q, want ok", report.Status)
	}
	if len(report.Errors) == 0 {
		t.Error("expected a health error recorded in report.Errors")
	}
}

func TestRunExecuteEnsuresCollection(t *testing.T) {
	mock := &mockStore{
		searchDocs: []vector.VectorDocument{{ID: "smoke-doc-1"}},
		healthSnap: vector.HealthSnapshot{
			Status:     "loaded",
			ModelReady: true,
		},
	}
	factory := func(endpoint string) (vector.VectorStore, error) {
		return mock, nil
	}
	report, code := run("grpc://localhost:19530", true, true, "archive_center_vectors", 4, "", factory)
	if code != 0 {
		t.Fatalf("exit code = %d, want 0", code)
	}
	if !report.CollectionEnsured {
		t.Fatal("expected collection_ensured=true")
	}
	if !mock.ensureCalled {
		t.Fatal("expected EnsureCollection to be called")
	}
}

func TestRunExecuteWithQuerySet(t *testing.T) {
	dir := t.TempDir()
	querySetPath := filepath.Join(dir, "query-set.json")
	if err := os.WriteFile(querySetPath, []byte(`{
	  "source_mode": "sqlite_real_memory_embeddings",
	  "result_limit": 3,
	  "queries": [
	    {
	      "query_id": "q1",
	      "source_id": "memories:1",
	      "embedding": [1.0, 0.0, 0.0, 0.0],
	      "tier": "memory",
	      "chat_session_id": "sess-1",
	      "source_table": "memories",
	      "source_row_id": "1",
	      "document_excerpt": "hello"
	    }
	  ]
	}`), 0644); err != nil {
		t.Fatal(err)
	}
	mock := &mockStore{
		searchDocs:  []vector.VectorDocument{{ID: "memories:1"}, {ID: "other"}},
		countResult: 1,
		healthSnap: vector.HealthSnapshot{
			Status:     "loaded",
			ModelReady: true,
		},
	}
	factory := func(endpoint string) (vector.VectorStore, error) {
		return mock, nil
	}

	report, code := run("grpc://localhost:19530", true, true, "archive_center_vectors", 4, querySetPath, factory)
	if code != 0 {
		t.Fatalf("exit code = %d, want 0; errors=%v", code, report.Errors)
	}
	if report.QuerySetSourceMode != "sqlite_real_memory_embeddings" {
		t.Fatalf("query set source mode = %q", report.QuerySetSourceMode)
	}
	if report.Dimension != 4 {
		t.Fatalf("dimension = %d, want 4", report.Dimension)
	}
	if len(mock.upsertDocs) != 1 || mock.upsertDocs[0].ID != "memories:1" {
		t.Fatalf("upsert docs = %+v", mock.upsertDocs)
	}
	if len(mock.searchCalls) != 1 || mock.searchCalls[0].filter != `chat_session_id == "sess-1"` {
		t.Fatalf("search calls = %+v", mock.searchCalls)
	}
	if len(report.Comparisons) != 1 || !report.Comparisons[0].Top1Match || !report.Comparisons[0].SelfFound {
		t.Fatalf("comparisons = %+v", report.Comparisons)
	}
}

func TestMakeSmokeDocs(t *testing.T) {
	docs := makeSmokeDocs("sess-1", 4)
	if len(docs) != 2 {
		t.Fatalf("expected 2 docs, got %d", len(docs))
	}
	if docs[0].ID != "smoke-doc-1" {
		t.Errorf("doc[0].ID = %q, want smoke-doc-1", docs[0].ID)
	}
	if docs[1].ID != "smoke-doc-2" {
		t.Errorf("doc[1].ID = %q, want smoke-doc-2", docs[1].ID)
	}
	if len(docs[0].Embedding) != 4 {
		t.Errorf("embedding len = %d, want 4", len(docs[0].Embedding))
	}
	if docs[0].Embedding[0] != 1.0 {
		t.Errorf("embedding[0] = %f, want 1.0", docs[0].Embedding[0])
	}
	if docs[1].Embedding[1] != 1.0 {
		t.Errorf("embedding[1][1] = %f, want 1.0", docs[1].Embedding[1])
	}
}
