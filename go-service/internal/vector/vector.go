// Package vector defines the vector search contract for Archive Center 2.0.
package vector

import (
	"context"
	"errors"
)

// Common errors.
var (
	ErrNotFound   = errors.New("no vector results found")
	ErrNotEnabled = errors.New("vector store is not enabled")
)

// VectorStore defines the core vector search and management contract.
// This mirrors the Chroma shadow operations analyzed in 0.8.
type VectorStore interface {
	// Search returns the top-k most similar vectors for a session.
	// Filter is a metadata expression (e.g. `tier == "memory"`).
	Search(ctx context.Context, sessionID string, vector []float32, limit int, filter string) ([]VectorDocument, error)

	// Upsert inserts or updates documents into the vector store.
	Upsert(ctx context.Context, sessionID string, docs []VectorDocument) error

	// DeleteSession removes all vectors for a session.
	DeleteSession(ctx context.Context, sessionID string) error

	// Rebuild creates a new collection from MariaDB canonical truth,
	// validates it with a sample query, then atomically swaps.
	Rebuild(ctx context.Context, sessionID string) error

	// Health returns a diagnostic snapshot of the vector store.
	Health(ctx context.Context) (HealthSnapshot, error)

	// Count returns the number of vectors for a session.
	Count(ctx context.Context, sessionID string) (int, error)

	// Close releases underlying connections and resources.
	Close(ctx context.Context) error
}

// DocumentDeleter is an optional extension for removing specific vector docs.
// It is used by turn rollback when canonical row IDs are known before deletion.
type DocumentDeleter interface {
	DeleteDocuments(ctx context.Context, ids []string) error
}

// DocumentLister is an optional diagnostic extension for full vector integrity
// audits. It returns stored vector metadata without changing runtime recall.
type DocumentLister interface {
	ListDocuments(ctx context.Context, sessionID string) ([]VectorDocument, error)
}

// CollectionResetter is an explicit operator/debug-only extension for clearing
// all vector documents while preserving service configuration.
type CollectionResetter interface {
	ResetAll(ctx context.Context) error
}

// VectorDocument maps to a single upserted row in the vector store.
type VectorDocument struct {
	ID                    string
	Embedding             []float32
	Distance              float64
	Similarity            float64
	SimilarityAvailable   bool
	SimilaritySource      string
	Tier                  string
	ChatSessionID         string
	SourceTable           string
	SourceRowID           string
	SchemaVersion         string
	DocumentText          string
	SearchTextPolicy      string
	RawLanguage           string
	SummaryLanguage       string
	SessionOutputLanguage string
	AliasCount            int
	MigrationID           int64
	MigratedFromSessionID string
}

// HealthSnapshot is the diagnostic shape returned by Health.
type HealthSnapshot struct {
	Status          string
	Collection      string
	PersistDir      string
	TotalCount      int
	ProjectModel    string
	ModelReady      bool
	PreflightIssues []string
}
