package vector

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"

	"github.com/milvus-io/milvus/client/v2/column"
	"github.com/milvus-io/milvus/client/v2/entity"
	"github.com/milvus-io/milvus/client/v2/index"
	"github.com/milvus-io/milvus/client/v2/milvusclient"
)

// ErrNotImplemented is returned by SDK store methods that are not yet implemented.
var ErrNotImplemented = errors.New("milvus sdk store: operation not implemented in this slice")

const (
	milvusCollectionName = "archive_center_vectors"
	milvusVectorField    = "embedding"
	milvusCountLimit     = 16384
)

// MilvusSDKClient is a narrow interface over the Milvus SDK v2 Client.
// It exists to allow tests and smoke executors to mock the SDK boundary
// without a real server.
type MilvusSDKClient interface {
	Search(ctx context.Context, collectionName string, vectors []entity.Vector, limit int, expr string, outputFields []string) ([]milvusclient.ResultSet, error)
	Query(ctx context.Context, collectionName string, expr string, outputFields []string, limit int) (milvusclient.ResultSet, error)
	Upsert(ctx context.Context, collectionName string, columns []column.Column) (milvusclient.UpsertResult, error)
	Delete(ctx context.Context, collectionName string, expr string) (milvusclient.DeleteResult, error)
	GetLoadState(ctx context.Context, collectionName string) (entity.LoadState, error)
	DescribeCollection(ctx context.Context, collectionName string) (*entity.Collection, error)
	CreateCollection(ctx context.Context, option milvusclient.CreateCollectionOption) error
	DropCollection(ctx context.Context, collectionName string) error
	Close(ctx context.Context) error
}

// sdkVectorStore implements VectorStore using a real Milvus SDK v2 client.
type sdkVectorStore struct {
	client MilvusSDKClient
}

// NewMilvusSDKStore creates a VectorStore backed by the Milvus SDK v2.
// It connects to the provided endpoint. The collection must already exist.
// This function returns an error if the endpoint is empty or connection fails.
func NewMilvusSDKStore(endpoint string) (VectorStore, error) {
	if endpoint == "" {
		return nil, errors.New("milvus sdk store: endpoint is required")
	}
	cfg := &milvusclient.ClientConfig{
		Address: endpoint,
	}
	client, err := milvusclient.New(context.Background(), cfg)
	if err != nil {
		return nil, fmt.Errorf("milvus sdk store: failed to connect to %s: %w", endpoint, err)
	}
	return &sdkVectorStore{
		client: &realMilvusClient{client: client},
	}, nil
}

// NewMilvusSDKStoreWithClient creates a VectorStore from an injected SDK client.
// It is used by smoke tests and other executors that need mockability.
func NewMilvusSDKStoreWithClient(client MilvusSDKClient) VectorStore {
	return &sdkVectorStore{client: client}
}

// realMilvusClient adapts *milvusclient.Client to our internal MilvusSDKClient interface.
type realMilvusClient struct {
	client *milvusclient.Client
}

func (r *realMilvusClient) Search(ctx context.Context, collectionName string, vectors []entity.Vector, limit int, expr string, outputFields []string) ([]milvusclient.ResultSet, error) {
	opt := newArchiveCenterSearchOption(collectionName, vectors, limit, expr, outputFields)
	return r.client.Search(ctx, opt)
}

func newArchiveCenterSearchOption(collectionName string, vectors []entity.Vector, limit int, expr string, outputFields []string) milvusclient.SearchOption {
	return milvusclient.NewSearchOption(collectionName, limit, vectors).
		WithFilter(expr).
		WithOutputFields(outputFields...).
		WithANNSField(milvusVectorField).
		WithSearchParam("metric_type", string(entity.L2))
}

func (r *realMilvusClient) Query(ctx context.Context, collectionName string, expr string, outputFields []string, limit int) (milvusclient.ResultSet, error) {
	opt := milvusclient.NewQueryOption(collectionName).
		WithOutputFields(outputFields...)
	if strings.TrimSpace(expr) != "" {
		opt = opt.WithFilter(expr)
	}
	if limit > 0 {
		opt = opt.WithLimit(limit)
	}
	return r.client.Query(ctx, opt)
}

func (r *realMilvusClient) Upsert(ctx context.Context, collectionName string, columns []column.Column) (milvusclient.UpsertResult, error) {
	opt := milvusclient.NewColumnBasedInsertOption(collectionName, columns...)
	return r.client.Upsert(ctx, opt)
}

func (r *realMilvusClient) Delete(ctx context.Context, collectionName string, expr string) (milvusclient.DeleteResult, error) {
	opt := milvusclient.NewDeleteOption(collectionName).WithExpr(expr)
	return r.client.Delete(ctx, opt)
}

func (r *realMilvusClient) GetLoadState(ctx context.Context, collectionName string) (entity.LoadState, error) {
	opt := milvusclient.NewGetLoadStateOption(collectionName)
	return r.client.GetLoadState(ctx, opt)
}

func (r *realMilvusClient) DescribeCollection(ctx context.Context, collectionName string) (*entity.Collection, error) {
	opt := milvusclient.NewDescribeCollectionOption(collectionName)
	return r.client.DescribeCollection(ctx, opt)
}

func (r *realMilvusClient) CreateCollection(ctx context.Context, option milvusclient.CreateCollectionOption) error {
	return r.client.CreateCollection(ctx, option)
}

func (r *realMilvusClient) DropCollection(ctx context.Context, collectionName string) error {
	opt := milvusclient.NewDropCollectionOption(collectionName)
	return r.client.DropCollection(ctx, opt)
}

func (r *realMilvusClient) Close(ctx context.Context) error {
	return r.client.Close(ctx)
}

func (s *sdkVectorStore) Search(ctx context.Context, sessionID string, vector []float32, limit int, filter string) ([]VectorDocument, error) {
	if limit <= 0 {
		limit = 5
	}
	vectors := []entity.Vector{entity.FloatVector(vector)}
	outputFields := []string{"id", "tier", "chat_session_id", "source_table", "source_row_id", "schema_version", "document_text"}

	resultSets, err := s.client.Search(ctx, milvusCollectionName, vectors, limit, filter, outputFields)
	if err != nil {
		return nil, fmt.Errorf("milvus sdk search failed: %w", err)
	}

	var docs []VectorDocument
	for _, rs := range resultSets {
		for i := 0; i < rs.ResultCount; i++ {
			doc := VectorDocument{}
			if rs.IDs != nil {
				idStr, _ := rs.IDs.GetAsString(i)
				doc.ID = idStr
			}
			if tierCol := rs.GetColumn("tier"); tierCol != nil {
				doc.Tier, _ = tierCol.GetAsString(i)
			}
			if sidCol := rs.GetColumn("chat_session_id"); sidCol != nil {
				doc.ChatSessionID, _ = sidCol.GetAsString(i)
			}
			if stCol := rs.GetColumn("source_table"); stCol != nil {
				doc.SourceTable, _ = stCol.GetAsString(i)
			}
			if srCol := rs.GetColumn("source_row_id"); srCol != nil {
				doc.SourceRowID, _ = srCol.GetAsString(i)
			}
			if svCol := rs.GetColumn("schema_version"); svCol != nil {
				doc.SchemaVersion, _ = svCol.GetAsString(i)
			}
			if dtCol := rs.GetColumn("document_text"); dtCol != nil {
				doc.DocumentText, _ = dtCol.GetAsString(i)
			}
			docs = append(docs, doc)
		}
	}

	if len(docs) == 0 {
		return nil, ErrNotFound
	}
	return docs, nil
}

func (s *sdkVectorStore) Upsert(ctx context.Context, sessionID string, docs []VectorDocument) error {
	if len(docs) == 0 {
		return nil
	}

	n := len(docs)
	ids := make([]string, n)
	embeddings := make([][]float32, n)
	tiers := make([]string, n)
	sessionIDs := make([]string, n)
	sourceTables := make([]string, n)
	sourceRowIDs := make([]string, n)
	schemaVersions := make([]string, n)
	documentTexts := make([]string, n)

	for i, doc := range docs {
		ids[i] = doc.ID
		embeddings[i] = doc.Embedding
		tiers[i] = doc.Tier
		sessionIDs[i] = doc.ChatSessionID
		sourceTables[i] = doc.SourceTable
		sourceRowIDs[i] = doc.SourceRowID
		schemaVersions[i] = doc.SchemaVersion
		documentTexts[i] = doc.DocumentText
	}

	dim := 0
	if len(embeddings) > 0 && len(embeddings[0]) > 0 {
		dim = len(embeddings[0])
	}

	columns := []column.Column{
		column.NewColumnVarChar("id", ids),
		column.NewColumnFloatVector("embedding", dim, embeddings),
		column.NewColumnVarChar("tier", tiers),
		column.NewColumnVarChar("chat_session_id", sessionIDs),
		column.NewColumnVarChar("source_table", sourceTables),
		column.NewColumnVarChar("source_row_id", sourceRowIDs),
		column.NewColumnVarChar("schema_version", schemaVersions),
		column.NewColumnVarChar("document_text", documentTexts),
	}

	_, err := s.client.Upsert(ctx, milvusCollectionName, columns)
	if err != nil {
		return fmt.Errorf("milvus sdk upsert failed: %w", err)
	}
	return nil
}

// EnsureCollection creates the Archive Center vector collection when it is
// absent. It is used by guarded smoke/bootstrap paths so users do not need to
// pre-create Milvus schema by hand.
func (s *sdkVectorStore) EnsureCollection(ctx context.Context, dimension int) error {
	if dimension <= 0 {
		return errors.New("milvus sdk ensure collection: dimension must be positive")
	}
	if _, err := s.client.DescribeCollection(ctx, milvusCollectionName); err == nil {
		return nil
	}

	schema := entity.NewSchema().
		WithDynamicFieldEnabled(false).
		WithField(entity.NewField().WithName("id").WithDataType(entity.FieldTypeVarChar).WithMaxLength(512).WithIsPrimaryKey(true)).
		WithField(entity.NewField().WithName(milvusVectorField).WithDataType(entity.FieldTypeFloatVector).WithDim(int64(dimension))).
		WithField(entity.NewField().WithName("tier").WithDataType(entity.FieldTypeVarChar).WithMaxLength(64)).
		WithField(entity.NewField().WithName("chat_session_id").WithDataType(entity.FieldTypeVarChar).WithMaxLength(512)).
		WithField(entity.NewField().WithName("source_table").WithDataType(entity.FieldTypeVarChar).WithMaxLength(128)).
		WithField(entity.NewField().WithName("source_row_id").WithDataType(entity.FieldTypeVarChar).WithMaxLength(256)).
		WithField(entity.NewField().WithName("schema_version").WithDataType(entity.FieldTypeVarChar).WithMaxLength(128)).
		WithField(entity.NewField().WithName("document_text").WithDataType(entity.FieldTypeVarChar).WithMaxLength(8192))

	idx := milvusclient.NewCreateIndexOption(milvusCollectionName, milvusVectorField, index.NewAutoIndex(entity.L2)).
		WithIndexName(milvusVectorField)
	opt := milvusclient.NewCreateCollectionOption(milvusCollectionName, schema).
		WithIndexOptions(idx)
	return s.client.CreateCollection(ctx, opt)
}

func (s *sdkVectorStore) DeleteSession(ctx context.Context, sessionID string) error {
	expr := fmt.Sprintf("chat_session_id == %q", sessionID)
	_, err := s.client.Delete(ctx, milvusCollectionName, expr)
	if err != nil {
		return fmt.Errorf("milvus sdk delete session failed: %w", err)
	}
	return nil
}

func (s *sdkVectorStore) DeleteDocuments(ctx context.Context, ids []string) error {
	if len(ids) == 0 {
		return nil
	}
	quoted := make([]string, 0, len(ids))
	for _, id := range ids {
		if id == "" {
			continue
		}
		quoted = append(quoted, strconv.Quote(id))
	}
	if len(quoted) == 0 {
		return nil
	}
	expr := fmt.Sprintf("id in [%s]", strings.Join(quoted, ","))
	_, err := s.client.Delete(ctx, milvusCollectionName, expr)
	if err != nil {
		return fmt.Errorf("milvus sdk delete documents failed: %w", err)
	}
	return nil
}

func (s *sdkVectorStore) Rebuild(ctx context.Context, sessionID string) error {
	return ErrNotImplemented
}

func (s *sdkVectorStore) Health(ctx context.Context) (HealthSnapshot, error) {
	state, err := s.client.GetLoadState(ctx, milvusCollectionName)
	if err != nil {
		return HealthSnapshot{}, fmt.Errorf("milvus sdk health check failed: %w", err)
	}

	status := "unknown"
	modelReady := false
	switch state.State {
	case entity.LoadStateLoaded:
		status = "loaded"
		modelReady = true
	case entity.LoadStateLoading:
		status = "loading"
	case entity.LoadStateNotLoad:
		status = "not_loaded"
	default:
		status = "unknown"
	}

	return HealthSnapshot{
		Status:     status,
		Collection: milvusCollectionName,
		ModelReady: modelReady,
	}, nil
}

func (s *sdkVectorStore) Count(ctx context.Context, sessionID string) (int, error) {
	expr := ""
	if strings.TrimSpace(sessionID) != "" {
		expr = fmt.Sprintf("chat_session_id == %q", sessionID)
	}
	rs, err := s.client.Query(ctx, milvusCollectionName, expr, []string{"id"}, milvusCountLimit)
	if err != nil {
		return 0, fmt.Errorf("milvus sdk count failed: %w", err)
	}
	if rs.ResultCount > 0 {
		return rs.ResultCount, nil
	}
	if rs.IDs != nil {
		return rs.IDs.Len(), nil
	}
	return 0, nil
}

func (s *sdkVectorStore) Close(ctx context.Context) error {
	if s.client != nil {
		return s.client.Close(ctx)
	}
	return nil
}
