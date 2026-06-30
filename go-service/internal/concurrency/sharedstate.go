package concurrency

import (
	"sync"
)

// SharedState holds process-level mutable state that is read-heavy.
// It mirrors 0.8 global settings / operator token / readiness flags.
// In R0/R1 only the shadow skeleton fields are populated.
type SharedState struct {
	mu sync.RWMutex

	// OperatorToken is the optional operator access token.
	OperatorToken string

	// Readiness flags (mirrors 0.8 readiness checks).
	MariaDBReady bool
	MilvusReady  bool

	// EmbeddingModel is the current active embedding model identifier.
	EmbeddingModel string

	// EmbeddingEndpoint is the current embedding API endpoint.
	EmbeddingEndpoint string
}

// NewSharedState creates a new shared state with safe defaults.
func NewSharedState() *SharedState {
	return &SharedState{}
}

// SetOperatorToken updates the operator token.
func (s *SharedState) SetOperatorToken(token string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.OperatorToken = token
}

// GetOperatorToken returns the current operator token.
func (s *SharedState) GetOperatorToken() string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.OperatorToken
}

// SetMariaDBReady sets the MariaDB readiness flag.
func (s *SharedState) SetMariaDBReady(ready bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.MariaDBReady = ready
}

// IsMariaDBReady returns the MariaDB readiness flag.
func (s *SharedState) IsMariaDBReady() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.MariaDBReady
}

// SetMilvusReady sets the Milvus readiness flag.
func (s *SharedState) SetMilvusReady(ready bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.MilvusReady = ready
}

// IsMilvusReady returns the Milvus readiness flag.
func (s *SharedState) IsMilvusReady() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.MilvusReady
}

// SetEmbeddingConfig updates the embedding model and endpoint.
func (s *SharedState) SetEmbeddingConfig(model, endpoint string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.EmbeddingModel = model
	s.EmbeddingEndpoint = endpoint
}

// GetEmbeddingConfig returns the current embedding model and endpoint.
func (s *SharedState) GetEmbeddingConfig() (string, string) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.EmbeddingModel, s.EmbeddingEndpoint
}
