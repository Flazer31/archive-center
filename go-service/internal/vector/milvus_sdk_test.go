package vector

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/milvus-io/milvus/client/v2/column"
	"github.com/milvus-io/milvus/client/v2/entity"
	"github.com/milvus-io/milvus/client/v2/milvusclient"
)

// mockMilvusClient is a test double for the Milvus SDK v2 client boundary.
type mockMilvusClient struct {
	searchCalls           []mockSearchCall
	queryCalls            []mockQueryCall
	upsertCalls           []mockUpsertCall
	deleteCalls           []mockDeleteCall
	createCollectionCalls []milvusclient.CreateCollectionOption
	loadState             entity.LoadState
	loadStateErr          error
	describeCollection    *entity.Collection
	describeErr           error
	searchResult          []milvusclient.ResultSet
	searchErr             error
	queryResult           milvusclient.ResultSet
	queryErr              error
	upsertResult          milvusclient.UpsertResult
	upsertErr             error
	deleteResult          milvusclient.DeleteResult
	deleteErr             error
	createCollectionErr   error
}

type mockSearchCall struct {
	collectionName string
	vectors        []entity.Vector
	limit          int
	expr           string
	outputFields   []string
}

type mockQueryCall struct {
	collectionName string
	expr           string
	outputFields   []string
	limit          int
}

type mockUpsertCall struct {
	collectionName string
	columns        []column.Column
}

type mockDeleteCall struct {
	collectionName string
	expr           string
}

func (m *mockMilvusClient) Search(ctx context.Context, collectionName string, vectors []entity.Vector, limit int, expr string, outputFields []string) ([]milvusclient.ResultSet, error) {
	m.searchCalls = append(m.searchCalls, mockSearchCall{
		collectionName: collectionName,
		vectors:        vectors,
		limit:          limit,
		expr:           expr,
		outputFields:   outputFields,
	})
	if m.searchErr != nil {
		return nil, m.searchErr
	}
	return m.searchResult, nil
}

func (m *mockMilvusClient) Query(ctx context.Context, collectionName string, expr string, outputFields []string, limit int) (milvusclient.ResultSet, error) {
	m.queryCalls = append(m.queryCalls, mockQueryCall{
		collectionName: collectionName,
		expr:           expr,
		outputFields:   outputFields,
		limit:          limit,
	})
	if m.queryErr != nil {
		return milvusclient.ResultSet{}, m.queryErr
	}
	return m.queryResult, nil
}

func (m *mockMilvusClient) Upsert(ctx context.Context, collectionName string, columns []column.Column) (milvusclient.UpsertResult, error) {
	m.upsertCalls = append(m.upsertCalls, mockUpsertCall{
		collectionName: collectionName,
		columns:        columns,
	})
	if m.upsertErr != nil {
		return milvusclient.UpsertResult{}, m.upsertErr
	}
	return m.upsertResult, nil
}

func (m *mockMilvusClient) Delete(ctx context.Context, collectionName string, expr string) (milvusclient.DeleteResult, error) {
	m.deleteCalls = append(m.deleteCalls, mockDeleteCall{
		collectionName: collectionName,
		expr:           expr,
	})
	if m.deleteErr != nil {
		return milvusclient.DeleteResult{}, m.deleteErr
	}
	return m.deleteResult, nil
}

func (m *mockMilvusClient) GetLoadState(ctx context.Context, collectionName string) (entity.LoadState, error) {
	if m.loadStateErr != nil {
		return entity.LoadState{}, m.loadStateErr
	}
	return m.loadState, nil
}

func (m *mockMilvusClient) DescribeCollection(ctx context.Context, collectionName string) (*entity.Collection, error) {
	if m.describeErr != nil {
		return nil, m.describeErr
	}
	if m.describeCollection != nil {
		return m.describeCollection, nil
	}
	return nil, errors.New("not found")
}

func (m *mockMilvusClient) CreateCollection(ctx context.Context, option milvusclient.CreateCollectionOption) error {
	m.createCollectionCalls = append(m.createCollectionCalls, option)
	return m.createCollectionErr
}

func (m *mockMilvusClient) DropCollection(ctx context.Context, collectionName string) error {
	return errors.New("not implemented in mock")
}

func (m *mockMilvusClient) Close(ctx context.Context) error {
	return nil
}

// ---------------------------------------------------------------------------
// SDK vector store tests
// ---------------------------------------------------------------------------

func TestSDKVectorStoreSearch(t *testing.T) {
	mock := &mockMilvusClient{
		searchResult: []milvusclient.ResultSet{
			{
				ResultCount: 1,
				IDs:         column.NewColumnVarChar("id", []string{"doc-1"}),
				Fields: milvusclient.DataSet{
					column.NewColumnVarChar("tier", []string{"memory"}),
					column.NewColumnVarChar("chat_session_id", []string{"sess-1"}),
					column.NewColumnVarChar("source_table", []string{"turns"}),
					column.NewColumnVarChar("source_row_id", []string{"row-1"}),
					column.NewColumnVarChar("schema_version", []string{"v1"}),
					column.NewColumnVarChar("document_text", []string{"hello world"}),
				},
				Scores: []float32{0.95},
			},
		},
	}
	store := &sdkVectorStore{client: mock}

	docs, err := store.Search(context.Background(), "sess-1", []float32{0.1, 0.2, 0.3}, 5, `tier == "memory"`)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(docs) != 1 {
		t.Fatalf("expected 1 doc, got %d", len(docs))
	}
	if docs[0].ID != "doc-1" {
		t.Errorf("ID = %q, want doc-1", docs[0].ID)
	}
	if docs[0].Tier != "memory" {
		t.Errorf("Tier = %q, want memory", docs[0].Tier)
	}
	if docs[0].ChatSessionID != "sess-1" {
		t.Errorf("ChatSessionID = %q, want sess-1", docs[0].ChatSessionID)
	}
	if docs[0].DocumentText != "hello world" {
		t.Errorf("DocumentText = %q, want hello world", docs[0].DocumentText)
	}

	if len(mock.searchCalls) != 1 {
		t.Fatalf("expected 1 search call, got %d", len(mock.searchCalls))
	}
	call := mock.searchCalls[0]
	if call.collectionName != milvusCollectionName {
		t.Errorf("collectionName = %q, want %q", call.collectionName, milvusCollectionName)
	}
	if call.limit != 5 {
		t.Errorf("limit = %d, want 5", call.limit)
	}
	if call.expr != `tier == "memory"` {
		t.Errorf("expr = %q, want tier == \"memory\"", call.expr)
	}
	if len(call.vectors) != 1 {
		t.Errorf("vectors count = %d, want 1", len(call.vectors))
	}
	fv, ok := call.vectors[0].(entity.FloatVector)
	if !ok {
		t.Errorf("vector type = %T, want entity.FloatVector", call.vectors[0])
	}
	if len(fv) != 3 {
		t.Errorf("vector dim = %d, want 3", len(fv))
	}
	expectedOutputs := []string{"id", "tier", "chat_session_id", "source_table", "source_row_id", "schema_version", "document_text"}
	if len(call.outputFields) != len(expectedOutputs) {
		t.Errorf("outputFields count = %d, want %d", len(call.outputFields), len(expectedOutputs))
	}
}

func TestArchiveCenterSearchOptionUsesL2Metric(t *testing.T) {
	opt := newArchiveCenterSearchOption(
		milvusCollectionName,
		[]entity.Vector{entity.FloatVector([]float32{0.1, 0.2})},
		5,
		`tier == "memory"`,
		[]string{"id"},
	)
	req, err := opt.Request()
	if err != nil {
		t.Fatalf("Request() error = %v", err)
	}
	params := map[string]string{}
	for _, kv := range req.GetSearchParams() {
		params[kv.GetKey()] = kv.GetValue()
	}
	if params["metric_type"] != string(entity.L2) {
		t.Errorf("metric_type = %q, want %q", params["metric_type"], entity.L2)
	}
	if params["anns_field"] != milvusVectorField {
		t.Errorf("anns_field = %q, want %q", params["anns_field"], milvusVectorField)
	}
}

func TestSDKVectorStoreSearchReturnsErrNotFoundWhenEmpty(t *testing.T) {
	mock := &mockMilvusClient{searchResult: []milvusclient.ResultSet{}}
	store := &sdkVectorStore{client: mock}

	_, err := store.Search(context.Background(), "sess-1", []float32{0.1}, 5, "")
	if !errors.Is(err, ErrNotFound) {
		t.Errorf("expected ErrNotFound, got %v", err)
	}
}

func TestSDKVectorStoreUpsert(t *testing.T) {
	mock := &mockMilvusClient{}
	store := &sdkVectorStore{client: mock}

	docs := []VectorDocument{
		{
			ID:            "doc-1",
			Embedding:     []float32{0.1, 0.2},
			Tier:          "memory",
			ChatSessionID: "sess-1",
			SourceTable:   "turns",
			SourceRowID:   "r1",
			SchemaVersion: "v1",
			DocumentText:  "hello",
		},
		{
			ID:            "doc-2",
			Embedding:     []float32{0.3, 0.4},
			Tier:          "episode",
			ChatSessionID: "sess-1",
			SourceTable:   "turns",
			SourceRowID:   "r2",
			SchemaVersion: "v1",
			DocumentText:  "world",
		},
	}
	if err := store.Upsert(context.Background(), "sess-1", docs); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(mock.upsertCalls) != 1 {
		t.Fatalf("expected 1 upsert call, got %d", len(mock.upsertCalls))
	}
	call := mock.upsertCalls[0]
	if call.collectionName != milvusCollectionName {
		t.Errorf("collectionName = %q, want %q", call.collectionName, milvusCollectionName)
	}
	if len(call.columns) != 8 {
		t.Fatalf("expected 8 columns, got %d", len(call.columns))
	}

	var idCol column.Column
	for _, col := range call.columns {
		if col.Name() == "id" {
			idCol = col
			break
		}
	}
	if idCol == nil {
		t.Fatal("missing id column")
	}
	if idCol.Len() != 2 {
		t.Errorf("id column len = %d, want 2", idCol.Len())
	}
	v0, _ := idCol.GetAsString(0)
	if v0 != "doc-1" {
		t.Errorf("id[0] = %q, want doc-1", v0)
	}
	v1, _ := idCol.GetAsString(1)
	if v1 != "doc-2" {
		t.Errorf("id[1] = %q, want doc-2", v1)
	}
}

func TestSDKVectorStoreUpsertEmptyDocs(t *testing.T) {
	mock := &mockMilvusClient{}
	store := &sdkVectorStore{client: mock}

	if err := store.Upsert(context.Background(), "sess-1", nil); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(mock.upsertCalls) != 0 {
		t.Errorf("expected 0 upsert calls, got %d", len(mock.upsertCalls))
	}
}

func TestSDKVectorStoreEnsureCollectionSkipsExistingCollection(t *testing.T) {
	mock := &mockMilvusClient{describeCollection: &entity.Collection{Name: milvusCollectionName}}
	store := &sdkVectorStore{client: mock}

	if err := store.EnsureCollection(context.Background(), 4); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(mock.createCollectionCalls) != 0 {
		t.Fatalf("expected no create call for existing collection, got %d", len(mock.createCollectionCalls))
	}
}

func TestSDKVectorStoreEnsureCollectionCreatesMissingCollection(t *testing.T) {
	mock := &mockMilvusClient{describeErr: errors.New("collection not found")}
	store := &sdkVectorStore{client: mock}

	if err := store.EnsureCollection(context.Background(), 4); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(mock.createCollectionCalls) != 1 {
		t.Fatalf("expected 1 create call, got %d", len(mock.createCollectionCalls))
	}
}

func TestSDKVectorStoreCount(t *testing.T) {
	mock := &mockMilvusClient{
		queryResult: milvusclient.ResultSet{
			ResultCount: 2,
			IDs:         column.NewColumnVarChar("id", []string{"doc-1", "doc-2"}),
		},
	}
	store := &sdkVectorStore{client: mock}

	count, err := store.Count(context.Background(), "sess-1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if count != 2 {
		t.Fatalf("count = %d, want 2", count)
	}
	if len(mock.queryCalls) != 1 {
		t.Fatalf("expected 1 query call, got %d", len(mock.queryCalls))
	}
	call := mock.queryCalls[0]
	if call.collectionName != milvusCollectionName {
		t.Errorf("collectionName = %q, want %q", call.collectionName, milvusCollectionName)
	}
	if call.expr != `chat_session_id == "sess-1"` {
		t.Errorf("expr = %q", call.expr)
	}
	if call.limit != milvusCountLimit {
		t.Errorf("limit = %d, want %d", call.limit, milvusCountLimit)
	}
}

func TestSDKVectorStoreHealth(t *testing.T) {
	mock := &mockMilvusClient{
		loadState: entity.LoadState{State: entity.LoadStateLoaded},
	}
	store := &sdkVectorStore{client: mock}

	snap, err := store.Health(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if snap.Status != "loaded" {
		t.Errorf("status = %q, want loaded", snap.Status)
	}
	if !snap.ModelReady {
		t.Error("ModelReady should be true when loaded")
	}
	if snap.Collection != milvusCollectionName {
		t.Errorf("collection = %q, want %q", snap.Collection, milvusCollectionName)
	}
}

func TestSDKVectorStoreHealthLoading(t *testing.T) {
	mock := &mockMilvusClient{
		loadState: entity.LoadState{State: entity.LoadStateLoading},
	}
	store := &sdkVectorStore{client: mock}

	snap, err := store.Health(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if snap.Status != "loading" {
		t.Errorf("status = %q, want loading", snap.Status)
	}
	if snap.ModelReady {
		t.Error("ModelReady should be false when loading")
	}
}

func TestSDKVectorStoreDeleteSession(t *testing.T) {
	mock := &mockMilvusClient{}
	store := &sdkVectorStore{client: mock}

	if err := store.DeleteSession(context.Background(), "sess-1"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(mock.deleteCalls) != 1 {
		t.Fatalf("expected 1 delete call, got %d", len(mock.deleteCalls))
	}
	call := mock.deleteCalls[0]
	if call.collectionName != milvusCollectionName {
		t.Errorf("collectionName = %q, want %q", call.collectionName, milvusCollectionName)
	}
	wantExpr := `chat_session_id == "sess-1"`
	if call.expr != wantExpr {
		t.Errorf("expr = %q, want %q", call.expr, wantExpr)
	}
}

func TestSDKVectorStoreDeleteDocuments(t *testing.T) {
	mock := &mockMilvusClient{}
	store := &sdkVectorStore{client: mock}

	if err := store.DeleteDocuments(context.Background(), []string{"memory:sess-1:41", "memory:sess-1:42"}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(mock.deleteCalls) != 1 {
		t.Fatalf("expected 1 delete call, got %d", len(mock.deleteCalls))
	}
	call := mock.deleteCalls[0]
	if call.collectionName != milvusCollectionName {
		t.Errorf("collectionName = %q, want %q", call.collectionName, milvusCollectionName)
	}
	wantExpr := `id in ["memory:sess-1:41","memory:sess-1:42"]`
	if call.expr != wantExpr {
		t.Errorf("expr = %q, want %q", call.expr, wantExpr)
	}
}

func TestSDKVectorStoreRebuildReturnsNotImplemented(t *testing.T) {
	mock := &mockMilvusClient{}
	store := &sdkVectorStore{client: mock}

	err := store.Rebuild(context.Background(), "sess-1")
	if !errors.Is(err, ErrNotImplemented) {
		t.Errorf("expected ErrNotImplemented, got %v", err)
	}
}

func TestSDKVectorStoreCountReturnsQueryError(t *testing.T) {
	mock := &mockMilvusClient{queryErr: errors.New("query boom")}
	store := &sdkVectorStore{client: mock}

	_, err := store.Count(context.Background(), "sess-1")
	if err == nil || !strings.Contains(err.Error(), "milvus sdk count failed") {
		t.Errorf("expected wrapped query error, got %v", err)
	}
}

func TestNewMilvusSDKStoreRequiresEndpoint(t *testing.T) {
	_, err := NewMilvusSDKStore("")
	if err == nil {
		t.Fatal("expected error for empty endpoint")
	}
	if !errors.Is(err, errors.New("milvus sdk store: endpoint is required")) {
		// Exact match not required; just verify message is present
		if err.Error() != "milvus sdk store: endpoint is required" {
			t.Errorf("unexpected error message: %q", err.Error())
		}
	}
}

func TestSDKVectorStoreImplementsInterface(t *testing.T) {
	var _ VectorStore = &sdkVectorStore{client: &mockMilvusClient{}}
	var _ DocumentDeleter = &sdkVectorStore{client: &mockMilvusClient{}}
}
