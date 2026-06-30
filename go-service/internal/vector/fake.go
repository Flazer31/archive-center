package vector

import "context"

// fakeVectorStore is an R0/R1 no-op implementation.
// It records every call so tests can assert the boundary without real persistence.
type fakeVectorStore struct {
	searchCalled    bool
	searchSessionID string
	searchVectorLen int
	searchLimit     int
	searchFilter    string

	upsertCalled    bool
	upsertSessionID string
	upsertDocCount  int

	deleteCalled    bool
	deleteSessionID string

	deleteDocumentIDs []string

	resetAllCalled bool

	rebuildCalled    bool
	rebuildSessionID string

	healthCalled bool

	countCalled    bool
	countSessionID string
}

// NewFakeVectorStore returns a Store that records calls but does nothing.
func NewFakeVectorStore() VectorStore { return &fakeVectorStore{} }

func (f *fakeVectorStore) Search(ctx context.Context, sessionID string, vector []float32, limit int, filter string) ([]VectorDocument, error) {
	f.searchCalled = true
	f.searchSessionID = sessionID
	f.searchVectorLen = len(vector)
	f.searchLimit = limit
	f.searchFilter = filter
	return nil, ErrNotFound
}

func (f *fakeVectorStore) Upsert(ctx context.Context, sessionID string, docs []VectorDocument) error {
	f.upsertCalled = true
	f.upsertSessionID = sessionID
	f.upsertDocCount = len(docs)
	return nil
}

func (f *fakeVectorStore) DeleteSession(ctx context.Context, sessionID string) error {
	f.deleteCalled = true
	f.deleteSessionID = sessionID
	return nil
}

func (f *fakeVectorStore) DeleteDocuments(ctx context.Context, ids []string) error {
	f.deleteDocumentIDs = append(f.deleteDocumentIDs, ids...)
	return nil
}

func (f *fakeVectorStore) ResetAll(ctx context.Context) error {
	f.resetAllCalled = true
	return nil
}

func (f *fakeVectorStore) Rebuild(ctx context.Context, sessionID string) error {
	f.rebuildCalled = true
	f.rebuildSessionID = sessionID
	return nil
}

func (f *fakeVectorStore) Health(ctx context.Context) (HealthSnapshot, error) {
	f.healthCalled = true
	return HealthSnapshot{
		Status:          "shadow",
		Collection:      "archive_center_lite",
		PersistDir:      "",
		TotalCount:      0,
		ModelReady:      false,
		PreflightIssues: []string{"shadow_mode"},
	}, nil
}

func (f *fakeVectorStore) Count(ctx context.Context, sessionID string) (int, error) {
	f.countCalled = true
	f.countSessionID = sessionID
	return 0, nil
}

func (f *fakeVectorStore) Close(ctx context.Context) error { return nil }
