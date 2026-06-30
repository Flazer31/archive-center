package vector

import "context"

// milvusStore is a disabled skeleton for the live Milvus Lite implementation.
// It satisfies VectorStore but rejects every operation with ErrNotEnabled.
// Actual Milvus SDK import, connection logic, and persist dir creation are
// explicitly omitted in R0/R1.
type milvusStore struct {
	path string
}

// OpenMilvusLite returns a VectorStore backed by Milvus Lite.
// In R0/R1 this function always returns a non-nil stub; all methods on the
// stub return ErrNotEnabled. No connection to Milvus Lite is attempted.
func OpenMilvusLite(path string) (VectorStore, error) {
	return &milvusStore{path: path}, nil
}

func (m *milvusStore) Search(ctx context.Context, sessionID string, vector []float32, limit int, filter string) ([]VectorDocument, error) {
	return nil, ErrNotEnabled
}

func (m *milvusStore) Upsert(ctx context.Context, sessionID string, docs []VectorDocument) error {
	return ErrNotEnabled
}

func (m *milvusStore) DeleteSession(ctx context.Context, sessionID string) error {
	return ErrNotEnabled
}

func (m *milvusStore) DeleteDocuments(ctx context.Context, ids []string) error {
	return ErrNotEnabled
}

func (m *milvusStore) Rebuild(ctx context.Context, sessionID string) error {
	return ErrNotEnabled
}

func (m *milvusStore) Health(ctx context.Context) (HealthSnapshot, error) {
	return HealthSnapshot{}, ErrNotEnabled
}

func (m *milvusStore) Count(ctx context.Context, sessionID string) (int, error) {
	return 0, ErrNotEnabled
}

func (m *milvusStore) Close(ctx context.Context) error { return nil }
