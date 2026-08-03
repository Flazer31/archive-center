package vector

import (
	"context"
	"sync"
)

// MutationFencer serializes a multi-step integrity check with every mutation
// performed through the same process-wide VectorStore wrapper. The callback
// receives the unwrapped delegate so it can list and mutate without re-entering
// the fence.
type MutationFencer interface {
	WithExclusiveMutationFence(ctx context.Context, fn func(VectorStore) error) error
}

type mutationFencedStore struct {
	mu       sync.RWMutex
	delegate VectorStore
}

// NewMutationFencedStore wraps the process-owned vector store. Archive Center's
// managed runtime is single-process; all server vector access must use the
// returned wrapper for the fence to be complete.
func NewMutationFencedStore(delegate VectorStore) VectorStore {
	if delegate == nil {
		return nil
	}
	if _, alreadyFenced := delegate.(*mutationFencedStore); alreadyFenced {
		return delegate
	}
	return &mutationFencedStore{delegate: delegate}
}

func (s *mutationFencedStore) WithExclusiveMutationFence(
	ctx context.Context,
	fn func(VectorStore) error,
) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := ctx.Err(); err != nil {
		return err
	}
	return fn(s.delegate)
}

func (s *mutationFencedStore) Search(ctx context.Context, sessionID string, embedding []float32, limit int, filter string) ([]VectorDocument, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.delegate.Search(ctx, sessionID, embedding, limit, filter)
}

func (s *mutationFencedStore) Upsert(ctx context.Context, sessionID string, docs []VectorDocument) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.delegate.Upsert(ctx, sessionID, docs)
}

func (s *mutationFencedStore) DeleteSession(ctx context.Context, sessionID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.delegate.DeleteSession(ctx, sessionID)
}

func (s *mutationFencedStore) Rebuild(ctx context.Context, sessionID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.delegate.Rebuild(ctx, sessionID)
}

func (s *mutationFencedStore) Health(ctx context.Context) (HealthSnapshot, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.delegate.Health(ctx)
}

func (s *mutationFencedStore) Count(ctx context.Context, sessionID string) (int, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.delegate.Count(ctx, sessionID)
}

func (s *mutationFencedStore) Close(ctx context.Context) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.delegate.Close(ctx)
}

func (s *mutationFencedStore) DeleteDocuments(ctx context.Context, ids []string) error {
	deleter, ok := s.delegate.(DocumentDeleter)
	if !ok {
		return ErrNotEnabled
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	return deleter.DeleteDocuments(ctx, ids)
}

func (s *mutationFencedStore) ListDocuments(ctx context.Context, sessionID string) ([]VectorDocument, error) {
	lister, ok := s.delegate.(DocumentLister)
	if !ok {
		return nil, ErrNotEnabled
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	return lister.ListDocuments(ctx, sessionID)
}

func (s *mutationFencedStore) GetDocuments(ctx context.Context, ids []string) ([]VectorDocument, error) {
	reader, ok := s.delegate.(ExactDocumentReader)
	if !ok {
		return nil, ErrNotEnabled
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	return reader.GetDocuments(ctx, ids)
}

func (s *mutationFencedStore) QueryExact(ctx context.Context, query ExactQuery) ([]ExactQueryResult, error) {
	querier, ok := s.delegate.(ExactMetadataQuerier)
	if !ok {
		return nil, ErrNotEnabled
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	return querier.QueryExact(ctx, query)
}

func (s *mutationFencedStore) ResetAll(ctx context.Context) error {
	resetter, ok := s.delegate.(CollectionResetter)
	if !ok {
		return ErrNotEnabled
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	return resetter.ResetAll(ctx)
}
