package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"

	"github.com/risulongmemory/archive-center-go/internal/config"
	"github.com/risulongmemory/archive-center-go/internal/store"
	"github.com/risulongmemory/archive-center-go/internal/vector"
)

type referenceVectorTestStore struct {
	docs         []vector.VectorDocument
	upserted     []vector.VectorDocument
	deletedIDs   []string
	exactQuery   vector.ExactQuery
	exactResults []vector.ExactQueryResult
	exactErr     error
	upsertErr    error
}

func (f *referenceVectorTestStore) Search(context.Context, string, []float32, int, string) ([]vector.VectorDocument, error) {
	return nil, vector.ErrNotFound
}

func (f *referenceVectorTestStore) QueryExact(_ context.Context, query vector.ExactQuery) ([]vector.ExactQueryResult, error) {
	f.exactQuery = query
	return append([]vector.ExactQueryResult(nil), f.exactResults...), f.exactErr
}

func (f *referenceVectorTestStore) Upsert(_ context.Context, sessionID string, docs []vector.VectorDocument) error {
	if f.upsertErr != nil {
		return f.upsertErr
	}
	f.upserted = append([]vector.VectorDocument(nil), docs...)
	byID := map[string]vector.VectorDocument{}
	for _, doc := range f.docs {
		byID[doc.ID] = doc
	}
	for _, doc := range docs {
		copy := doc
		if copy.ChatSessionID == "" {
			copy.ChatSessionID = sessionID
		}
		byID[copy.ID] = copy
	}
	f.docs = f.docs[:0]
	for _, doc := range byID {
		f.docs = append(f.docs, doc)
	}
	return nil
}

func (f *referenceVectorTestStore) DeleteSession(_ context.Context, sessionID string) error {
	out := f.docs[:0]
	for _, doc := range f.docs {
		if doc.ChatSessionID != sessionID {
			out = append(out, doc)
		}
	}
	f.docs = out
	return nil
}

func (f *referenceVectorTestStore) DeleteDocuments(_ context.Context, ids []string) error {
	f.deletedIDs = append(f.deletedIDs, ids...)
	remove := map[string]bool{}
	for _, id := range ids {
		remove[id] = true
	}
	out := f.docs[:0]
	for _, doc := range f.docs {
		if !remove[doc.ID] {
			out = append(out, doc)
		}
	}
	f.docs = out
	return nil
}

func (f *referenceVectorTestStore) ListDocuments(_ context.Context, sessionID string) ([]vector.VectorDocument, error) {
	out := []vector.VectorDocument{}
	for _, doc := range f.docs {
		if sessionID == "" || doc.ChatSessionID == sessionID {
			out = append(out, doc)
		}
	}
	return out, nil
}

func (f *referenceVectorTestStore) Rebuild(context.Context, string) error { return nil }
func (f *referenceVectorTestStore) Health(context.Context) (vector.HealthSnapshot, error) {
	return vector.HealthSnapshot{Status: "ok", Collection: "archive_center_reference_vectors", ModelReady: true, TotalCount: len(f.docs)}, nil
}
func (f *referenceVectorTestStore) Count(context.Context, string) (int, error) {
	return len(f.docs), nil
}
func (f *referenceVectorTestStore) Close(context.Context) error { return nil }

func referenceVectorEmbeddingServer(t *testing.T) (*httptest.Server, *int) {
	t.Helper()
	calls := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		var payload map[string]any
		_ = json.NewDecoder(r.Body).Decode(&payload)
		input := fmt.Sprint(payload["input"])
		first := float64(len(input)%7 + 1)
		w.Header().Set("Content-Type", "application/json")
		_, _ = fmt.Fprintf(w, `{"model":"embed-reference","data":[{"embedding":[%v,1]}]}`, first)
	}))
	return server, &calls
}

func referenceVectorFixtureStore() *referenceLibraryHTTPStore {
	fake := newReferenceLibraryHTTPStore()
	fake.works = []store.ReferenceWork{{WorkID: "work-1", Title: "Work", Status: "active"}}
	fake.continuities = []store.ReferenceContinuity{{ContinuityID: "continuity-1", WorkID: "work-1", Label: "Main", Status: "active"}}
	fake.timeline = []store.ReferenceTimelineNode{
		{NodeID: "node-approved", WorkID: "work-1", ContinuityID: "continuity-1", Label: "Hero arrives", NodeKind: "event", BranchKey: "main", Ordinal: 10, ReviewStatus: "approved", MetadataJSON: `{"evidence_excerpt":"The hero reached the city."}`},
		{NodeID: "node-pending", WorkID: "work-1", ContinuityID: "continuity-1", Label: "Unverified future", ReviewStatus: "pending"},
	}
	fake.entities = []store.ReferenceEntity{
		{EntityID: "entity-approved", WorkID: "work-1", ContinuityID: "continuity-1", EntityType: "character", CanonicalName: "Mina", DescriptionText: "A careful archivist.", ReviewStatus: "approved"},
		{EntityID: "entity-pending", WorkID: "work-1", ContinuityID: "continuity-1", EntityType: "character", CanonicalName: "Rumor", ReviewStatus: "pending"},
	}
	fake.aliases["entity-approved"] = []store.ReferenceEntityAlias{{EntityID: "entity-approved", AliasText: "Min"}}
	fake.claims = []store.ReferenceClaim{
		{ClaimID: "claim-approved", WorkID: "work-1", ContinuityID: "continuity-1", ClaimType: "world_rule", ClaimText: "Only marked gates open at night.", EvidenceExcerpt: "The eastern gate stayed shut.", BranchKey: "main", ReviewStatus: "approved", Confidence: 0.9},
		{ClaimID: "claim-pending", WorkID: "work-1", ContinuityID: "continuity-1", ClaimType: "event", ClaimText: "A pending spoiler.", ReviewStatus: "pending"},
	}
	return fake
}

func TestReferenceVectorReindexIndexesOnlyApprovedMaterialAndDeletesStaleAfterUpsert(t *testing.T) {
	fake := referenceVectorFixtureStore()
	embeddingServer, calls := referenceVectorEmbeddingServer(t)
	defer embeddingServer.Close()
	vectorStore := &referenceVectorTestStore{docs: []vector.VectorDocument{
		{ID: "reference_claim:stale", ChatSessionID: "work-1", Metadata: map[string]any{"work_id": "work-1", "continuity_id": "continuity-1"}},
		{ID: "reference_claim:other-flow", ChatSessionID: "work-1", Metadata: map[string]any{"work_id": "work-1", "continuity_id": "continuity-2"}},
	}}
	srv := &Server{
		Cfg:             config.Config{ReferenceChromaCollection: "archive_center_reference_vectors"},
		Store:           fake,
		ReferenceVector: vectorStore,
	}
	embedder := completeTurnEmbeddingConfig{Provider: "openai", APIKey: "key", Endpoint: embeddingServer.URL, Model: "embed-reference", TimeoutMs: 5000}
	result, err := srv.runReferenceVectorReindex(context.Background(), fake, "work-1", "continuity-1", embedder, func(map[string]any) {})
	if err != nil {
		t.Fatalf("runReferenceVectorReindex: %v", err)
	}
	if *calls != 3 || len(vectorStore.upserted) != 3 {
		t.Fatalf("embedding calls=%d upserted=%d, want approved 3 only", *calls, len(vectorStore.upserted))
	}
	for _, doc := range vectorStore.upserted {
		if strings.Contains(doc.DocumentText, "pending") || doc.Metadata["review_status"] != "approved" {
			t.Fatalf("unapproved material indexed: %#v", doc)
		}
		if doc.Metadata["work_id"] != "work-1" || doc.Metadata["continuity_id"] != "continuity-1" || doc.Metadata["embedding_model"] != "embed-reference" {
			t.Fatalf("reference metadata missing: %#v", doc.Metadata)
		}
	}
	if !reflect.DeepEqual(vectorStore.deletedIDs, []string{"reference_claim:stale"}) {
		t.Fatalf("deleted IDs = %#v, want stale current continuity only", vectorStore.deletedIDs)
	}
	if result["indexed"] != 3 || result["stale_deleted"] != 1 {
		t.Fatalf("result = %#v", result)
	}
}

func TestReferenceVectorReindexDoesNotDeleteExistingIndexWhenUpsertFails(t *testing.T) {
	fake := referenceVectorFixtureStore()
	embeddingServer, _ := referenceVectorEmbeddingServer(t)
	defer embeddingServer.Close()
	vectorStore := &referenceVectorTestStore{
		docs:      []vector.VectorDocument{{ID: "reference_claim:existing", ChatSessionID: "work-1", Metadata: map[string]any{"work_id": "work-1", "continuity_id": "continuity-1"}}},
		upsertErr: errors.New("upsert failed"),
	}
	srv := &Server{Cfg: config.Config{ReferenceChromaCollection: "archive_center_reference_vectors"}, Store: fake, ReferenceVector: vectorStore}
	embedder := completeTurnEmbeddingConfig{Provider: "openai", APIKey: "key", Endpoint: embeddingServer.URL, Model: "embed-reference", TimeoutMs: 5000}
	if _, err := srv.runReferenceVectorReindex(context.Background(), fake, "work-1", "continuity-1", embedder, func(map[string]any) {}); err == nil {
		t.Fatal("runReferenceVectorReindex error = nil, want upsert failure")
	}
	if len(vectorStore.deletedIDs) != 0 || len(vectorStore.docs) != 1 || vectorStore.docs[0].ID != "reference_claim:existing" {
		t.Fatalf("failed reindex changed existing index: deleted=%#v docs=%#v", vectorStore.deletedIDs, vectorStore.docs)
	}
}

func TestReferenceAutomaticVectorIndexIndexesApprovedMaterial(t *testing.T) {
	fake := referenceVectorFixtureStore()
	embeddingServer, calls := referenceVectorEmbeddingServer(t)
	defer embeddingServer.Close()
	vectorStore := &referenceVectorTestStore{}
	srv := &Server{Cfg: config.Config{ReferenceChromaCollection: "archive_center_reference_vectors"}, Store: fake, ReferenceVector: vectorStore}
	embedder := completeTurnEmbeddingConfig{Provider: "openai", APIKey: "key", Endpoint: embeddingServer.URL, Model: "embed-reference", TimeoutMs: 5000}

	result := srv.runReferenceAutomaticVectorIndex(context.Background(), fake, "work-1", "continuity-1", embedder, func(map[string]any) {})

	if result["status"] != "completed" || result["trigger"] != "automatic_after_review" || result["indexed"] != 3 {
		t.Fatalf("automatic index result = %#v", result)
	}
	if *calls != 3 || len(vectorStore.upserted) != 3 {
		t.Fatalf("embedding calls=%d upserted=%d, want 3", *calls, len(vectorStore.upserted))
	}
}

func TestReferenceAutomaticVectorIndexSkipsWithoutEmbeddingConfig(t *testing.T) {
	fake := referenceVectorFixtureStore()
	vectorStore := &referenceVectorTestStore{}
	srv := &Server{Cfg: config.Config{ReferenceChromaCollection: "archive_center_reference_vectors"}, Store: fake, ReferenceVector: vectorStore}

	result := srv.runReferenceAutomaticVectorIndex(context.Background(), fake, "work-1", "continuity-1", completeTurnEmbeddingConfig{}, func(map[string]any) {})

	if result["status"] != "skipped" || result["reason"] != "embedding_config_missing" {
		t.Fatalf("automatic index result = %#v", result)
	}
	if len(vectorStore.upserted) != 0 {
		t.Fatalf("unexpected vector upsert = %#v", vectorStore.upserted)
	}
}

func TestReferenceVectorSearchReturnsExactChromaMeasurementsWithoutFixedScores(t *testing.T) {
	fake := referenceVectorFixtureStore()
	embeddingServer, _ := referenceVectorEmbeddingServer(t)
	defer embeddingServer.Close()
	vectorStore := &referenceVectorTestStore{exactResults: []vector.ExactQueryResult{
		{Document: vector.VectorDocument{ID: "reference_claim:claim-approved", DocumentText: "gate rule", Metadata: map[string]any{"work_id": "work-1", "continuity_id": "continuity-1", "review_status": "approved", "reference_kind": "claim", "source_id": "claim-approved"}}, ChromaRank: 1, Distance: 0.427, DistanceAvailable: true, CosineSimilarity: 0.8123, CosineAvailable: true},
		{Document: vector.VectorDocument{ID: "reference_entity:entity-approved", DocumentText: "Mina", Metadata: map[string]any{"work_id": "work-1", "continuity_id": "continuity-1", "review_status": "approved", "reference_kind": "entity", "source_id": "entity-approved"}}, ChromaRank: 2, Distance: 0.913, DistanceAvailable: true, CosineSimilarity: 0.311, CosineAvailable: true},
	}}
	cfg := config.Default()
	cfg.ChromaEnabled = true
	cfg.ChromaEndpoint = "http://127.0.0.1:8000"
	srv := &Server{Cfg: cfg, Store: fake, ReferenceVector: vectorStore, AdminJobs: newAdminJobManager()}
	srv.RuntimeConfig.Synced = true
	srv.RuntimeConfig.EmbeddingProvider = "openai"
	srv.RuntimeConfig.EmbeddingAPIKey = "key"
	srv.RuntimeConfig.EmbeddingEndpoint = embeddingServer.URL
	srv.RuntimeConfig.EmbeddingModel = "embed-reference"
	srv.RuntimeConfig.EmbeddingTimeoutSec = 5
	mux := http.NewServeMux()
	srv.RegisterRoutes(mux)

	response := referenceLibraryTestRequest(t, mux, http.MethodPost, "/reference-works/work-1/vector/search", map[string]any{"continuity_id": "continuity-1", "query": "night gate", "limit": 2, "client_meta": map[string]any{}})
	results := response["results"].([]any)
	if len(results) != 2 {
		t.Fatalf("results = %#v", results)
	}
	first := results[0].(map[string]any)
	second := results[1].(map[string]any)
	if first["chroma_rank"] != float64(1) || first["distance"] != 0.427 || first["cosine_similarity"] != 0.8123 {
		t.Fatalf("first exact result changed: %#v", first)
	}
	if second["chroma_rank"] != float64(2) || second["distance"] != 0.913 || second["cosine_similarity"] != 0.311 {
		t.Fatalf("second exact result changed: %#v", second)
	}
	for _, item := range []map[string]any{first, second} {
		if _, exists := item["similarity"]; exists {
			t.Fatalf("synthetic similarity must not exist: %#v", item)
		}
		if _, exists := item["rerank_score"]; exists {
			t.Fatalf("rerank score must not exist: %#v", item)
		}
	}
	contract := response["score_contract"].(map[string]any)
	if contract["ranking"] != "chromadb_response_order" || contract["normalized_similarity"] != "not_generated" || contract["fixed_rank_scores"] != false || contract["client_rerank"] != false {
		t.Fatalf("score contract = %#v", contract)
	}
	wantWhere := map[string]any{"$and": []map[string]any{{"work_id": "work-1"}, {"continuity_id": "continuity-1"}, {"review_status": "approved"}}}
	if !reflect.DeepEqual(vectorStore.exactQuery.Where, wantWhere) || vectorStore.exactQuery.Limit != 2 || len(vectorStore.exactQuery.Embedding) != 2 {
		t.Fatalf("exact query = %#v", vectorStore.exactQuery)
	}
}

func TestReferenceVectorSearchFiltersNoLongerApprovedChromaDocumentWithoutReranking(t *testing.T) {
	fake := referenceVectorFixtureStore()
	embeddingServer, _ := referenceVectorEmbeddingServer(t)
	defer embeddingServer.Close()
	vectorStore := &referenceVectorTestStore{exactResults: []vector.ExactQueryResult{
		{Document: vector.VectorDocument{ID: "reference_claim:claim-pending", Metadata: map[string]any{"work_id": "work-1", "continuity_id": "continuity-1", "review_status": "approved", "reference_kind": "claim", "source_id": "claim-pending"}}, ChromaRank: 1, Distance: 0.1, DistanceAvailable: true},
		{Document: vector.VectorDocument{ID: "reference_entity:entity-approved", Metadata: map[string]any{"work_id": "work-1", "continuity_id": "continuity-1", "review_status": "approved", "reference_kind": "entity", "source_id": "entity-approved"}}, ChromaRank: 2, Distance: 0.2, DistanceAvailable: true},
	}}
	cfg := config.Default()
	cfg.ChromaEnabled = true
	cfg.ChromaEndpoint = "http://127.0.0.1:8000"
	srv := &Server{Cfg: cfg, Store: fake, ReferenceVector: vectorStore}
	srv.RuntimeConfig.Synced = true
	srv.RuntimeConfig.EmbeddingProvider = "openai"
	srv.RuntimeConfig.EmbeddingAPIKey = "key"
	srv.RuntimeConfig.EmbeddingEndpoint = embeddingServer.URL
	srv.RuntimeConfig.EmbeddingModel = "embed-reference"
	srv.RuntimeConfig.EmbeddingTimeoutSec = 5
	mux := http.NewServeMux()
	srv.RegisterRoutes(mux)

	response := referenceLibraryTestRequest(t, mux, http.MethodPost, "/reference-works/work-1/vector/search", map[string]any{"continuity_id": "continuity-1", "query": "Mina", "limit": 2, "client_meta": map[string]any{}})
	results := response["results"].([]any)
	if len(results) != 1 || results[0].(map[string]any)["chroma_rank"] != float64(2) {
		t.Fatalf("results = %#v, want surviving raw Chroma rank 2 without rerank", results)
	}
	if response["filtered_mismatch"] != float64(1) {
		t.Fatalf("filtered_mismatch = %#v, want 1", response["filtered_mismatch"])
	}
}

func TestReferenceVectorSearchRejectsDifferentIndexedEmbeddingSpace(t *testing.T) {
	fake := referenceVectorFixtureStore()
	embeddingServer, _ := referenceVectorEmbeddingServer(t)
	defer embeddingServer.Close()
	vectorStore := &referenceVectorTestStore{docs: []vector.VectorDocument{{
		ID:            "reference_entity:entity-approved",
		ChatSessionID: "work-1",
		Metadata: map[string]any{
			"work_id": "work-1", "continuity_id": "continuity-1", "reference_kind": "entity",
			"embedding_provider": "voyage", "embedding_model": "old-model",
		},
	}}}
	cfg := config.Default()
	cfg.ChromaEnabled = true
	cfg.ChromaEndpoint = "http://127.0.0.1:8000"
	srv := &Server{Cfg: cfg, Store: fake, ReferenceVector: vectorStore}
	srv.RuntimeConfig.Synced = true
	srv.RuntimeConfig.EmbeddingProvider = "openai"
	srv.RuntimeConfig.EmbeddingAPIKey = "key"
	srv.RuntimeConfig.EmbeddingEndpoint = embeddingServer.URL
	srv.RuntimeConfig.EmbeddingModel = "embed-reference"
	srv.RuntimeConfig.EmbeddingTimeoutSec = 5
	mux := http.NewServeMux()
	srv.RegisterRoutes(mux)

	body := `{"continuity_id":"continuity-1","query":"Mina","limit":2,"client_meta":{}}`
	req := httptest.NewRequest(http.MethodPost, "/reference-works/work-1/vector/search", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusConflict || !strings.Contains(rec.Body.String(), "reference_embedding_model_mismatch") {
		t.Fatalf("response = %d %s", rec.Code, rec.Body.String())
	}
	if len(vectorStore.exactQuery.Embedding) != 0 {
		t.Fatalf("Chroma query ran despite embedding-space mismatch: %#v", vectorStore.exactQuery)
	}
}

func TestReferenceVectorStatusReportsStaleAndMissingApprovedDocuments(t *testing.T) {
	fake := referenceVectorFixtureStore()
	vectorStore := &referenceVectorTestStore{docs: []vector.VectorDocument{
		{ID: "reference_entity:entity-approved", ChatSessionID: "work-1", Metadata: map[string]any{"work_id": "work-1", "continuity_id": "continuity-1", "reference_kind": "entity", "embedding_model": "embed-reference"}},
		{ID: "reference_claim:removed", ChatSessionID: "work-1", Metadata: map[string]any{"work_id": "work-1", "continuity_id": "continuity-1", "reference_kind": "claim", "embedding_model": "embed-reference"}},
		{ID: "reference_claim:other", ChatSessionID: "work-1", Metadata: map[string]any{"work_id": "work-1", "continuity_id": "continuity-2", "reference_kind": "claim"}},
	}}
	cfg := config.Default()
	cfg.ChromaEnabled = true
	cfg.ChromaEndpoint = "http://127.0.0.1:8000"
	srv := &Server{Cfg: cfg, Store: fake, ReferenceVector: vectorStore}
	mux := http.NewServeMux()
	srv.RegisterRoutes(mux)

	response := referenceLibraryTestRequest(t, mux, http.MethodGet, "/reference-works/work-1/vector/status?continuity_id=continuity-1", nil)
	if response["indexed"] != float64(1) || response["indexed_total"] != float64(2) || response["stale_indexed"] != float64(1) || response["missing_approved"] != float64(2) {
		t.Fatalf("status = %#v", response)
	}
	counts := response["counts"].(map[string]any)
	if counts["entity"] != float64(1) || counts["claim"] != float64(0) {
		t.Fatalf("counts = %#v", counts)
	}
}
