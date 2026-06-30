package vector

import (
	"context"
	"errors"
	"testing"
)

// ---------------------------------------------------------------------------
// Fake vector store tests
// ---------------------------------------------------------------------------

func TestFakeVectorStoreImplementsInterface(t *testing.T) {
	var _ VectorStore = NewFakeVectorStore()
}

func TestFakeVectorStoreSearch(t *testing.T) {
	f := NewFakeVectorStore().(*fakeVectorStore)
	docs, err := f.Search(context.Background(), "sess-1", []float32{0.1, 0.2, 0.3}, 5, `tier == "memory"`)
	if err != ErrNotFound {
		t.Errorf("expected ErrNotFound, got %v", err)
	}
	if docs != nil {
		t.Errorf("expected nil, got %v", docs)
	}
	if !f.searchCalled {
		t.Error("search was not called")
	}
	if f.searchSessionID != "sess-1" {
		t.Errorf("searchSessionID = %q, want sess-1", f.searchSessionID)
	}
	if f.searchVectorLen != 3 {
		t.Errorf("searchVectorLen = %d, want 3", f.searchVectorLen)
	}
	if f.searchLimit != 5 {
		t.Errorf("searchLimit = %d, want 5", f.searchLimit)
	}
	if f.searchFilter != `tier == "memory"` {
		t.Errorf("searchFilter = %q", f.searchFilter)
	}
}

func TestFakeVectorStoreUpsert(t *testing.T) {
	f := NewFakeVectorStore().(*fakeVectorStore)
	if err := f.Upsert(context.Background(), "sess-1", []VectorDocument{
		{ID: "doc-1", Tier: "memory", ChatSessionID: "sess-1"},
		{ID: "doc-2", Tier: "episode", ChatSessionID: "sess-1"},
	}); err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if !f.upsertCalled {
		t.Error("upsert was not called")
	}
	if f.upsertDocCount != 2 {
		t.Errorf("upsertDocCount = %d, want 2", f.upsertDocCount)
	}
}

func TestFakeVectorStoreDeleteSession(t *testing.T) {
	f := NewFakeVectorStore().(*fakeVectorStore)
	if err := f.DeleteSession(context.Background(), "sess-1"); err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if !f.deleteCalled {
		t.Error("delete was not called")
	}
	if f.deleteSessionID != "sess-1" {
		t.Errorf("deleteSessionID = %q, want sess-1", f.deleteSessionID)
	}
}

func TestFakeVectorStoreDeleteDocuments(t *testing.T) {
	f := NewFakeVectorStore().(*fakeVectorStore)
	if err := f.DeleteDocuments(context.Background(), []string{"memory:sess-1:1"}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(f.deleteDocumentIDs) != 1 || f.deleteDocumentIDs[0] != "memory:sess-1:1" {
		t.Fatalf("deleteDocumentIDs = %#v", f.deleteDocumentIDs)
	}
}

func TestFakeVectorStoreRebuild(t *testing.T) {
	f := NewFakeVectorStore().(*fakeVectorStore)
	if err := f.Rebuild(context.Background(), "sess-1"); err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if !f.rebuildCalled {
		t.Error("rebuild was not called")
	}
	if f.rebuildSessionID != "sess-1" {
		t.Errorf("rebuildSessionID = %q, want sess-1", f.rebuildSessionID)
	}
}

func TestFakeVectorStoreHealth(t *testing.T) {
	f := NewFakeVectorStore().(*fakeVectorStore)
	snap, err := f.Health(context.Background())
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if !f.healthCalled {
		t.Error("health was not called")
	}
	if snap.Status != "shadow" {
		t.Errorf("status = %q, want shadow", snap.Status)
	}
	if snap.ModelReady {
		t.Error("ModelReady should be false in shadow")
	}
}

func TestFakeVectorStoreCount(t *testing.T) {
	f := NewFakeVectorStore().(*fakeVectorStore)
	n, err := f.Count(context.Background(), "sess-1")
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if n != 0 {
		t.Errorf("count = %d, want 0", n)
	}
	if !f.countCalled {
		t.Error("count was not called")
	}
}

// ---------------------------------------------------------------------------
// Milvus disabled stub tests
// ---------------------------------------------------------------------------

func TestMilvusOpenReturnsDisabledStub(t *testing.T) {
	vs, err := OpenMilvusLite("any-path")
	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
	if vs == nil {
		t.Fatal("expected non-nil VectorStore, got nil")
	}
	// All operations on the stub must return ErrNotEnabled without connecting.
	ctx := context.Background()
	if _, err := vs.Search(ctx, "", nil, 0, ""); !errors.Is(err, ErrNotEnabled) {
		t.Errorf("Search: expected ErrNotEnabled, got %v", err)
	}
	if err := vs.Upsert(ctx, "", nil); !errors.Is(err, ErrNotEnabled) {
		t.Errorf("Upsert: expected ErrNotEnabled, got %v", err)
	}
	if err := vs.DeleteSession(ctx, ""); !errors.Is(err, ErrNotEnabled) {
		t.Errorf("DeleteSession: expected ErrNotEnabled, got %v", err)
	}
	if err := vs.Rebuild(ctx, ""); !errors.Is(err, ErrNotEnabled) {
		t.Errorf("Rebuild: expected ErrNotEnabled, got %v", err)
	}
	if _, err := vs.Health(ctx); !errors.Is(err, ErrNotEnabled) {
		t.Errorf("Health: expected ErrNotEnabled, got %v", err)
	}
	if _, err := vs.Count(ctx, ""); !errors.Is(err, ErrNotEnabled) {
		t.Errorf("Count: expected ErrNotEnabled, got %v", err)
	}
}

func TestMilvusStoreImplementsInterface(t *testing.T) {
	// Compile-time check.
	var _ VectorStore = &milvusStore{}
}

func TestMilvusStoreAllMethodsDisabled(t *testing.T) {
	m := &milvusStore{}
	ctx := context.Background()

	if _, err := m.Search(ctx, "", nil, 0, ""); !errors.Is(err, ErrNotEnabled) {
		t.Errorf("Search: expected ErrNotEnabled, got %v", err)
	}
	if err := m.Upsert(ctx, "", nil); !errors.Is(err, ErrNotEnabled) {
		t.Errorf("Upsert: expected ErrNotEnabled, got %v", err)
	}
	if err := m.DeleteSession(ctx, ""); !errors.Is(err, ErrNotEnabled) {
		t.Errorf("DeleteSession: expected ErrNotEnabled, got %v", err)
	}
	if err := m.Rebuild(ctx, ""); !errors.Is(err, ErrNotEnabled) {
		t.Errorf("Rebuild: expected ErrNotEnabled, got %v", err)
	}
	if _, err := m.Health(ctx); !errors.Is(err, ErrNotEnabled) {
		t.Errorf("Health: expected ErrNotEnabled, got %v", err)
	}
	if _, err := m.Count(ctx, ""); !errors.Is(err, ErrNotEnabled) {
		t.Errorf("Count: expected ErrNotEnabled, got %v", err)
	}
}
