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
	docs            []VectorDocument

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
	for _, doc := range docs {
		cp := doc
		if cp.ChatSessionID == "" {
			cp.ChatSessionID = sessionID
		}
		f.docs = append(f.docs, cp)
	}
	return nil
}

func (f *fakeVectorStore) DeleteSession(ctx context.Context, sessionID string) error {
	f.deleteCalled = true
	f.deleteSessionID = sessionID
	out := f.docs[:0]
	for _, doc := range f.docs {
		if doc.ChatSessionID != sessionID {
			out = append(out, doc)
		}
	}
	f.docs = out
	return nil
}

func (f *fakeVectorStore) DeleteDocuments(ctx context.Context, ids []string) error {
	f.deleteDocumentIDs = append(f.deleteDocumentIDs, ids...)
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
	if sessionID == "" {
		return len(f.docs), nil
	}
	count := 0
	for _, doc := range f.docs {
		if doc.ChatSessionID == sessionID {
			count++
		}
	}
	return count, nil
}

func (f *fakeVectorStore) ListDocuments(ctx context.Context, sessionID string) ([]VectorDocument, error) {
	out := []VectorDocument{}
	for _, doc := range f.docs {
		if sessionID == "" || doc.ChatSessionID == sessionID {
			out = append(out, doc)
		}
	}
	return out, nil
}

func (f *fakeVectorStore) Close(ctx context.Context) error { return nil }
