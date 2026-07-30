package vector

import (
	"context"
	"testing"
	"time"
)

type mutationFenceProbeStore struct {
	VectorStore
	listEntered  chan struct{}
	releaseList  chan struct{}
	upsertCalled chan struct{}
}

func (s *mutationFenceProbeStore) ListDocuments(context.Context, string) ([]VectorDocument, error) {
	close(s.listEntered)
	<-s.releaseList
	return nil, nil
}

func (s *mutationFenceProbeStore) Upsert(context.Context, string, []VectorDocument) error {
	close(s.upsertCalled)
	return nil
}

func TestMutationFenceBlocksConcurrentVectorWriterAcrossMigrationProofWindow(t *testing.T) {
	probe := &mutationFenceProbeStore{
		VectorStore:  NewFakeVectorStore(),
		listEntered:  make(chan struct{}),
		releaseList:  make(chan struct{}),
		upsertCalled: make(chan struct{}),
	}
	wrapped := NewMutationFencedStore(probe)
	fencer, ok := wrapped.(MutationFencer)
	if !ok {
		t.Fatal("mutation-fenced store does not expose fence callback")
	}
	fenceDone := make(chan error, 1)
	go func() {
		fenceDone <- fencer.WithExclusiveMutationFence(context.Background(), func(raw VectorStore) error {
			_, err := raw.(DocumentLister).ListDocuments(context.Background(), "target")
			return err
		})
	}()
	<-probe.listEntered
	upsertDone := make(chan error, 1)
	go func() {
		upsertDone <- wrapped.Upsert(context.Background(), "target", []VectorDocument{{ID: "late"}})
	}()
	select {
	case <-probe.upsertCalled:
		t.Fatal("concurrent vector writer crossed the exclusive proof fence")
	case <-time.After(25 * time.Millisecond):
	}
	close(probe.releaseList)
	if err := <-fenceDone; err != nil {
		t.Fatal(err)
	}
	if err := <-upsertDone; err != nil {
		t.Fatal(err)
	}
}
