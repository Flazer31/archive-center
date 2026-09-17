package vector

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"sync"
	"testing"
)

func TestChromaExactDocumentReadUsesRequestedIDsOnly(t *testing.T) {
	var getBody map[string]any
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch {
		case r.Method == http.MethodGet && strings.HasSuffix(r.URL.Path, "/collections/archive_center_vectors"):
			_, _ = w.Write([]byte(`{"id":"collection-1","name":"archive_center_vectors"}`))
		case r.Method == http.MethodPost && strings.HasSuffix(r.URL.Path, "/collections/collection-1/get"):
			if err := json.NewDecoder(r.Body).Decode(&getBody); err != nil {
				t.Fatalf("decode exact get: %v", err)
			}
			_, _ = w.Write([]byte(`{
				"ids":["memory:session:7"],
				"documents":["verified memory"],
				"metadatas":[{"chat_session_id":"session","tier":"memory"}]
			}`))
		default:
			http.Error(w, r.Method+" "+r.URL.Path, http.StatusNotFound)
		}
	}))
	defer ts.Close()

	raw, err := NewChromaStore(ts.URL, "archive_center_vectors", "/api/v2")
	if err != nil {
		t.Fatalf("NewChromaStore: %v", err)
	}
	reader, ok := raw.(ExactDocumentReader)
	if !ok {
		t.Fatal("Chroma store does not implement ExactDocumentReader")
	}
	documents, err := reader.GetDocuments(context.Background(), []string{"memory:session:7", "memory:session:7", " "})
	if err != nil {
		t.Fatalf("GetDocuments: %v", err)
	}
	if len(documents) != 1 || documents[0].ID != "memory:session:7" || documents[0].DocumentText != "verified memory" {
		t.Fatalf("unexpected exact readback: %+v", documents)
	}
	ids, _ := getBody["ids"].([]any)
	include, _ := getBody["include"].([]any)
	if len(ids) != 1 || ids[0] != "memory:session:7" ||
		!reflect.DeepEqual(include, []any{"metadatas", "documents"}) {
		t.Fatalf("exact get body = %#v", getBody)
	}
}

func TestChromaExactQueryPreservesRawRankDistanceAndQuerySensitivity(t *testing.T) {
	var queryBodies []map[string]any
	var upsertBody map[string]any
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch {
		case r.Method == http.MethodGet && strings.HasSuffix(r.URL.Path, "/collections/archive_center_reference_vectors"):
			_, _ = w.Write([]byte(`{"id":"reference-collection","name":"archive_center_reference_vectors"}`))
		case r.Method == http.MethodPost && strings.HasSuffix(r.URL.Path, "/collections/reference-collection/upsert"):
			_ = json.NewDecoder(r.Body).Decode(&upsertBody)
			_, _ = w.Write([]byte(`{"status":"ok"}`))
		case r.Method == http.MethodPost && strings.HasSuffix(r.URL.Path, "/collections/reference-collection/query"):
			var body map[string]any
			_ = json.NewDecoder(r.Body).Decode(&body)
			queryBodies = append(queryBodies, body)
			rows, _ := body["query_embeddings"].([]any)
			first, _ := rows[0].([]any)
			if first[0].(float64) > 0.5 {
				_, _ = w.Write([]byte(`{"ids":[["claim-a","claim-b"]],"documents":[["alpha","beta"]],"metadatas":[[{"work_id":"work-1","continuity_id":"main"},{"work_id":"work-1","continuity_id":"main"}]],"distances":[[0.23,1.17]],"embeddings":[[[1,0],[0,1]]]}`))
				return
			}
			_, _ = w.Write([]byte(`{"ids":[["claim-b","claim-a"]],"documents":[["beta","alpha"]],"metadatas":[[{"work_id":"work-1","continuity_id":"main"},{"work_id":"work-1","continuity_id":"main"}]],"distances":[[0.31,1.09]],"embeddings":[[[0,1],[1,0]]]}`))
		default:
			http.Error(w, r.Method+" "+r.URL.Path, http.StatusNotFound)
		}
	}))
	defer ts.Close()

	raw, err := NewChromaStore(ts.URL, "archive_center_reference_vectors", "/api/v2")
	if err != nil {
		t.Fatalf("NewChromaStore: %v", err)
	}
	if err := raw.Upsert(context.Background(), "work-1", []VectorDocument{{
		ID:            "reference_claim:claim-a",
		Embedding:     []float32{1, 0},
		Tier:          "reference_claim",
		ChatSessionID: "work-1",
		SourceTable:   "reference_claims",
		SourceRowID:   "claim-a",
		SchemaVersion: "reference.v1",
		DocumentText:  "alpha",
		Metadata: map[string]any{
			"work_id":         "work-1",
			"continuity_id":   "main",
			"tier":            "must-not-override",
			"chat_session_id": "must-not-override",
			"nested":          map[string]any{"ignored": true},
		},
	}}); err != nil {
		t.Fatalf("Upsert: %v", err)
	}
	metadatas := upsertBody["metadatas"].([]any)
	metadata := metadatas[0].(map[string]any)
	if metadata["work_id"] != "work-1" || metadata["continuity_id"] != "main" {
		t.Fatalf("custom scalar metadata missing: %#v", metadata)
	}
	if metadata["tier"] != "reference_claim" || metadata["chat_session_id"] != "work-1" {
		t.Fatalf("reserved metadata was overridden: %#v", metadata)
	}
	if _, exists := metadata["nested"]; exists {
		t.Fatalf("non-scalar metadata should not be sent to Chroma: %#v", metadata)
	}

	querier := raw.(ExactMetadataQuerier)
	where := map[string]any{"$and": []map[string]any{{"work_id": "work-1"}, {"continuity_id": "main"}}}
	firstResults, err := querier.QueryExact(context.Background(), ExactQuery{Embedding: []float32{1, 0}, Limit: 2, Where: where})
	if err != nil {
		t.Fatalf("QueryExact first: %v", err)
	}
	if len(firstResults) != 2 || firstResults[0].Document.ID != "claim-a" || firstResults[0].ChromaRank != 1 || firstResults[0].Distance != 0.23 {
		t.Fatalf("raw Chroma order/distance not preserved: %#v", firstResults)
	}
	if !firstResults[0].DistanceAvailable || !firstResults[0].CosineAvailable || firstResults[0].CosineSimilarity < 0.999 {
		t.Fatalf("real vector measurements missing: %#v", firstResults[0])
	}
	if firstResults[0].Document.SimilarityAvailable || firstResults[0].Document.SimilaritySource != "" {
		t.Fatalf("exact query must not synthesize generic similarity: %#v", firstResults[0].Document)
	}
	if firstResults[0].Document.Metadata["work_id"] != "work-1" {
		t.Fatalf("metadata round trip missing: %#v", firstResults[0].Document.Metadata)
	}

	secondResults, err := querier.QueryExact(context.Background(), ExactQuery{Embedding: []float32{0, 1}, Limit: 2, Where: where})
	if err != nil {
		t.Fatalf("QueryExact second: %v", err)
	}
	if secondResults[0].Document.ID != "claim-b" || secondResults[0].Distance != 0.31 {
		t.Fatalf("query change did not change Chroma result: %#v", secondResults)
	}
	if len(queryBodies) != 2 || int(queryBodies[0]["n_results"].(float64)) != 2 || !reflect.DeepEqual(queryBodies[0]["where"], map[string]any{"$and": []any{map[string]any{"work_id": "work-1"}, map[string]any{"continuity_id": "main"}}}) {
		t.Fatalf("exact query body changed: %#v", queryBodies)
	}
}

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
				"distances":[[0.1]],
				"embeddings":[[[0.1,0.2]]]
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
	if !docs[0].SimilarityAvailable || docs[0].Similarity < 0.999 || docs[0].SimilaritySource != "cosine_from_query_and_stored_embedding" {
		t.Fatalf("actual cosine similarity not preserved: %+v", docs[0])
	}
	state.mu.Lock()
	queryCandidateLimit := 0
	for _, body := range state.bodies {
		if value, ok := body["n_results"].(float64); ok {
			queryCandidateLimit = int(value)
		}
	}
	state.mu.Unlock()
	if queryCandidateLimit != 3 {
		t.Fatalf("query n_results = %d, want caller-requested limit 3", queryCandidateLimit)
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

func TestChromaSearchAcceptsLargeSuccessfulResponse(t *testing.T) {
	const count, dimensions = 128, 2048
	embedding := make([]float32, dimensions)
	for i := range embedding {
		embedding[i] = 0.1234567
	}
	ids, documents := make([]string, count), make([]string, count)
	metadatas := make([]map[string]any, count)
	embeddings := make([][]float32, count)
	for i := range ids {
		ids[i] = fmt.Sprintf("precise_memory:large-session:unit-%d", i)
		documents[i] = fmt.Sprintf("Complete canonical evidence %d", i)
		metadatas[i] = map[string]any{"chat_session_id": "large-session", "source_table": "precise_memory_units", "source_row_id": fmt.Sprintf("unit-%d", i)}
		embeddings[i] = embedding
	}
	payload, err := json.Marshal(map[string]any{
		"ids": [][]string{ids}, "documents": [][]string{documents},
		"metadatas": [][]map[string]any{metadatas}, "embeddings": [][][]float32{embeddings},
	})
	if err != nil || len(payload) <= 1<<20 {
		t.Fatalf("fixture must exceed the former response limit: bytes=%d err=%v", len(payload), err)
	}
	var queryBody map[string]any
	requests := []string{}
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests = append(requests, r.Method+" "+r.URL.Path)
		w.Header().Set("Content-Type", "application/json")
		switch {
		case r.Method == http.MethodGet && strings.HasSuffix(r.URL.Path, "/collections/archive_center_vectors"):
			_, _ = w.Write([]byte(`{"id":"large-collection","name":"archive_center_vectors"}`))
		case r.Method == http.MethodPost && strings.HasSuffix(r.URL.Path, "/collections/large-collection/query"):
			if err := json.NewDecoder(r.Body).Decode(&queryBody); err != nil {
				t.Errorf("decode query: %v", err)
				http.Error(w, "invalid query", http.StatusBadRequest)
				return
			}
			_, _ = w.Write(payload)
		default:
			t.Errorf("unexpected Chroma request: %s %s", r.Method, r.URL.Path)
			http.Error(w, "unexpected request", http.StatusNotFound)
		}
	}))
	defer ts.Close()
	store, err := NewChromaStore(ts.URL, "archive_center_vectors", "/api/v2")
	if err != nil {
		t.Fatalf("NewChromaStore: %v", err)
	}
	docs, err := store.Search(context.Background(), "large-session", embedding, count, `source_table == "precise_memory_units"`)
	if err != nil {
		t.Fatalf("Search must decode the full successful response (%d bytes): %v", len(payload), err)
	}
	if len(docs) != len(ids) {
		t.Fatalf("Search returned %d of %d candidates", len(docs), len(ids))
	}
	for i, doc := range docs {
		if doc.ID != ids[i] || doc.DocumentText != documents[i] || doc.SourceRowID != metadatas[i]["source_row_id"] || !reflect.DeepEqual(doc.Embedding, embedding) || !doc.SimilarityAvailable || doc.Similarity < 0.999 {
			t.Fatalf("candidate %d lost canonical text, metadata or embedding", i)
		}
	}
	wantWhere := map[string]any{"$and": []any{map[string]any{"chat_session_id": "large-session"}, map[string]any{"source_table": "precise_memory_units"}}}
	if len(requests) != 2 || queryBody["n_results"] != float64(count) || !reflect.DeepEqual(queryBody["where"], wantWhere) || !reflect.DeepEqual(queryBody["include"], []any{"metadatas", "documents", "distances", "embeddings"}) {
		t.Fatalf("search changed the requested scope, count or call count: requests=%v query=%#v", requests, queryBody)
	}
}

func TestChromaErrorResponseRemainsBounded(t *testing.T) {
	const tail = "error-body-tail-must-not-be-included"
	payload := strings.Repeat("e", 1<<20) + tail
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet || r.URL.Path != "/api/v2/failed-query" {
			t.Errorf("unexpected Chroma request: %s %s", r.Method, r.URL.Path)
		}
		w.WriteHeader(http.StatusServiceUnavailable)
		_, _ = w.Write([]byte(payload))
	}))
	defer ts.Close()
	raw, err := NewChromaStore(ts.URL, "archive_center_vectors", "/api/v2")
	if err != nil {
		t.Fatalf("NewChromaStore: %v", err)
	}
	status, err := raw.(*chromaStore).doJSON(context.Background(), http.MethodGet, "/failed-query", nil, nil, http.StatusOK)
	if status != http.StatusServiceUnavailable || err == nil {
		t.Fatalf("HTTP error was not preserved: status=%d err=%v", status, err)
	}
	want := "chroma store: GET /failed-query returned 503: " + strings.Repeat("e", 1<<20)
	if err.Error() != want || strings.Contains(err.Error(), tail) {
		t.Fatalf("HTTP error body limit changed: got %d bytes, want %d", len(err.Error()), len(want))
	}
}

func TestChromaWhereSupportsExistingPreciseMemorySourceTableMetadata(t *testing.T) {
	got := chromaWhere("session-1", `source_table == "precise_memory_units"`)
	want := map[string]any{"$and": []map[string]any{
		{"chat_session_id": "session-1"},
		{"source_table": "precise_memory_units"},
	}}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("precise-memory where = %#v, want %#v", got, want)
	}
}

func TestChromaStoreReranksReturnedCandidatesByActualCosine(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch {
		case r.Method == http.MethodGet && strings.HasSuffix(r.URL.Path, "/collections/archive_center_vectors"):
			_, _ = w.Write([]byte(`{"id":"collection-1","name":"archive_center_vectors"}`))
		case r.Method == http.MethodPost && strings.HasSuffix(r.URL.Path, "/collections/collection-1/query"):
			_, _ = w.Write([]byte(`{
				"ids":[["old-fixed","query-relevant"]],
				"documents":[["old fixed memory","query relevant memory"]],
				"metadatas":[[
					{"tier":"memory","chat_session_id":"sess-rerank"},
					{"tier":"memory","chat_session_id":"sess-rerank"}
				]],
				"distances":[[0.01,0.9]],
				"embeddings":[[[0,1],[1,0]]]
			}`))
		default:
			http.Error(w, r.Method+" "+r.URL.Path, http.StatusNotFound)
		}
	}))
	defer ts.Close()

	raw, err := NewChromaStore(ts.URL, "archive_center_vectors", "/api/v2")
	if err != nil {
		t.Fatalf("NewChromaStore: %v", err)
	}
	docs, err := raw.Search(context.Background(), "sess-rerank", []float32{1, 0}, 1, "tier == memory")
	if err != nil {
		t.Fatalf("Search: %v", err)
	}
	if len(docs) != 1 || docs[0].ID != "query-relevant" {
		t.Fatalf("actual cosine rerank did not replace fixed upstream order: %+v", docs)
	}
}

func TestChromaStoreResetAllDeletesCollection(t *testing.T) {
	var got []string
	deleted := false
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		got = append(got, r.Method+" "+r.URL.Path)
		if r.Method == http.MethodGet && r.URL.Path == "/api/v2/tenants/default_tenant/databases/default_database/collections/archive_center_vectors" {
			if deleted {
				http.Error(w, "not found", http.StatusNotFound)
				return
			}
			_, _ = w.Write([]byte(`{"id":"collection-1","name":"archive_center_vectors"}`))
			return
		}
		if r.Method == http.MethodDelete && r.URL.Path == "/api/v2/tenants/default_tenant/databases/default_database/collections/archive_center_vectors" {
			deleted = true
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
		"DELETE /api/v2/tenants/default_tenant/databases/default_database/collections/archive_center_vectors",
		"GET /api/v2/tenants/default_tenant/databases/default_database/collections/archive_center_vectors",
	}
	if strings.Join(got, "\n") != strings.Join(want, "\n") {
		t.Fatalf("requests = %#v, want %#v", got, want)
	}
}

func TestChromaStoreResetAllRejectsFallback404WhenCollectionStillExists(t *testing.T) {
	var got []string
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		got = append(got, r.Method+" "+r.URL.Path)
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/api/v2/tenants/default_tenant/databases/default_database/collections/archive_center_vectors":
			_, _ = w.Write([]byte(`{"id":"collection-1","name":"archive_center_vectors"}`))
		case r.Method == http.MethodDelete && r.URL.Path == "/api/v2/tenants/default_tenant/databases/default_database/collections/archive_center_vectors":
			http.Error(w, "name delete failed", http.StatusInternalServerError)
		case r.Method == http.MethodDelete && r.URL.Path == "/api/v2/tenants/default_tenant/databases/default_database/collections/collection-1":
			http.Error(w, "not found", http.StatusNotFound)
		default:
			http.Error(w, r.Method+" "+r.URL.Path, http.StatusNotFound)
		}
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
	if err := resetter.ResetAll(context.Background()); err == nil {
		t.Fatal("ResetAll succeeded even though fallback DELETE 404 left the named collection")
	}
	want := []string{
		"GET /api/v2/tenants/default_tenant/databases/default_database/collections/archive_center_vectors",
		"DELETE /api/v2/tenants/default_tenant/databases/default_database/collections/archive_center_vectors",
		"DELETE /api/v2/tenants/default_tenant/databases/default_database/collections/collection-1",
		"GET /api/v2/tenants/default_tenant/databases/default_database/collections/archive_center_vectors",
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
