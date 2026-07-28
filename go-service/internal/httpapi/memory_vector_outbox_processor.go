package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/risulongmemory/archive-center-go/internal/store"
	"github.com/risulongmemory/archive-center-go/internal/vector"
)

type memoryVectorProcessResult struct {
	Processed      bool
	OutboxID       int64
	Operation      string
	DocumentID     string
	CanonicalState string
	VectorApplied  bool
	Failure        string
}

// processMemoryVectorOutboxOnce is the production MariaDB-to-vector
// orchestrator. MariaDB completion is authoritative: a provider success is
// compensated or left behind a queued delete if the source fence changes
// before commit.
func (s *Server) processMemoryVectorOutboxOnce(
	ctx context.Context,
	leaseOwner string,
	now time.Time,
	leaseDuration time.Duration,
) (memoryVectorProcessResult, error) {
	var result memoryVectorProcessResult
	if s == nil || s.Store == nil {
		return result, store.ErrNotEnabled
	}
	outbox, ok := s.Store.(store.MemoryVectorOutboxStore)
	if !ok {
		return result, store.ErrNotEnabled
	}
	item, err := outbox.ClaimMemoryVectorOperation(ctx, leaseOwner, now, leaseDuration)
	if errors.Is(err, store.ErrNotFound) {
		return result, nil
	}
	if err != nil {
		return result, err
	}
	result.Processed = true
	result.OutboxID = item.ID
	result.Operation = item.Operation
	result.DocumentID = item.DocumentID
	if s.Vector == nil {
		result.CanonicalState = "retryable"
		result.Failure = "vector store is not configured"
		return result, s.retryMemoryVectorOperation(ctx, outbox, item, leaseOwner, time.Now().UTC(), "vector store is not configured")
	}

	switch item.Operation {
	case "delete":
		deleter, ok := s.Vector.(vector.DocumentDeleter)
		if !ok {
			result.CanonicalState = "retryable"
			result.Failure = "vector store does not support document deletion"
			return result, s.retryMemoryVectorOperation(ctx, outbox, item, leaseOwner, time.Now().UTC(), "vector store does not support document deletion")
		}
		if err := deleter.DeleteDocuments(ctx, []string{item.DocumentID}); err != nil {
			result.CanonicalState = "retryable"
			result.Failure = err.Error()
			return result, s.retryMemoryVectorOperation(ctx, outbox, item, leaseOwner, time.Now().UTC(), err.Error())
		}
	case "upsert":
		var document vector.VectorDocument
		if err := json.Unmarshal([]byte(item.DocumentJSON), &document); err != nil {
			result.CanonicalState = "permanent"
			result.Failure = err.Error()
			return result, s.failMemoryVectorOperationPermanently(ctx, outbox, item, leaseOwner, time.Now().UTC(), "materialized vector document is invalid: "+err.Error())
		}
		if strings.TrimSpace(document.ID) == "" {
			document.ID = item.DocumentID
		}
		if strings.TrimSpace(document.ChatSessionID) == "" {
			document.ChatSessionID = item.ChatSessionID
		}
		if len(document.Embedding) == 0 {
			embeddingCfg := s.completeTurnExtractionConfig(nil).Embedder
			if !embeddingCfg.hasConfig() {
				result.CanonicalState = "retryable"
				result.Failure = "embedding configuration is not available"
				return result, s.retryMemoryVectorOperation(ctx, outbox, item, leaseOwner, time.Now().UTC(), result.Failure)
			}
			if strings.TrimSpace(document.DocumentText) == "" {
				result.CanonicalState = "permanent"
				result.Failure = "materialized vector document has no searchable text"
				return result, s.failMemoryVectorOperationPermanently(ctx, outbox, item, leaseOwner, time.Now().UTC(), result.Failure)
			}
			embeddingJSON, _, embedErr := callEmbedding(ctx, embeddingCfg, document.DocumentText)
			if embedErr != nil {
				result.CanonicalState = "retryable"
				result.Failure = "embedding materialization failed"
				return result, s.retryMemoryVectorOperation(ctx, outbox, item, leaseOwner, time.Now().UTC(), result.Failure)
			}
			document.Embedding = parseFloat32JSONList(embeddingJSON)
			if len(document.Embedding) == 0 {
				result.CanonicalState = "retryable"
				result.Failure = "embedding materialization returned no vector"
				return result, s.retryMemoryVectorOperation(ctx, outbox, item, leaseOwner, time.Now().UTC(), result.Failure)
			}
		}
		if err := s.Vector.Upsert(ctx, item.ChatSessionID, []vector.VectorDocument{document}); err != nil {
			result.CanonicalState = "retryable"
			result.Failure = err.Error()
			return result, s.retryMemoryVectorOperation(ctx, outbox, item, leaseOwner, time.Now().UTC(), err.Error())
		}
	default:
		result.CanonicalState = "permanent"
		result.Failure = "unknown vector operation"
		return result, s.failMemoryVectorOperationPermanently(ctx, outbox, item, leaseOwner, time.Now().UTC(), "unknown vector operation")
	}
	result.VectorApplied = true
	if err := outbox.CompleteMemoryVectorOperation(ctx, item.ID, leaseOwner, time.Now().UTC()); err != nil {
		if errors.Is(err, store.ErrSourceRevisionStale) && item.Operation == "upsert" {
			if deleter, ok := s.Vector.(vector.DocumentDeleter); ok {
				_ = deleter.DeleteDocuments(ctx, []string{item.DocumentID})
			}
			result.CanonicalState = "stale_rejected"
			return result, nil
		}
		return result, err
	}
	result.CanonicalState = "completed"
	return result, nil
}

func (s *Server) processMemoryVectorOutboxBatch(
	ctx context.Context,
	leaseOwner string,
	now time.Time,
	leaseDuration time.Duration,
	limit int,
) []memoryVectorProcessResult {
	if limit <= 0 {
		limit = 16
	}
	results := make([]memoryVectorProcessResult, 0, limit)
	for len(results) < limit {
		batchNow := now
		if len(results) > 0 {
			batchNow = time.Now().UTC()
		}
		result, err := s.processMemoryVectorOutboxOnce(ctx, leaseOwner, batchNow, leaseDuration)
		if err != nil || !result.Processed {
			break
		}
		results = append(results, result)
	}
	return results
}

func (s *Server) retryMemoryVectorOperation(
	ctx context.Context,
	outbox store.MemoryVectorOutboxStore,
	item *store.MemoryVectorOutboxItem,
	leaseOwner string,
	now time.Time,
	failure string,
) error {
	retryAfter := now.Add(30 * time.Second)
	if err := outbox.FailMemoryVectorOperation(ctx, item.ID, leaseOwner, now, retryAfter, false, failure); err != nil {
		return err
	}
	return nil
}

func (s *Server) failMemoryVectorOperationPermanently(
	ctx context.Context,
	outbox store.MemoryVectorOutboxStore,
	item *store.MemoryVectorOutboxItem,
	leaseOwner string,
	now time.Time,
	failure string,
) error {
	if err := outbox.FailMemoryVectorOperation(ctx, item.ID, leaseOwner, now, time.Time{}, true, failure); err != nil {
		return err
	}
	return nil
}

func materializedMemoryVectorDocumentJSON(document vector.VectorDocument) (string, error) {
	if strings.TrimSpace(document.ID) == "" || strings.TrimSpace(document.ChatSessionID) == "" {
		return "", fmt.Errorf("materialized vector identity is required")
	}
	if len(document.Embedding) == 0 {
		return "", fmt.Errorf("materialized vector embedding is required")
	}
	encoded, err := json.Marshal(document)
	return string(encoded), err
}
