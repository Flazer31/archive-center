package httpapi

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/risulongmemory/archive-center-go/internal/store"
	"github.com/risulongmemory/archive-center-go/internal/vector"
)

const (
	memoryVectorRetryLimitUnconfigured = "MEMORY_VECTOR_RETRY_LIMIT_UNCONFIGURED"
	memoryVectorRetryLimitReached      = "MEMORY_VECTOR_RETRY_LIMIT_REACHED"
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
	if !s.runtimeConfigSnapshot().Synced {
		result.CanonicalState = "deferred_config_sync"
		result.Failure = memoryWorkerConfigDeferred
		return result, nil
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
	if leaseDuration <= 0 {
		result.CanonicalState = "retryable"
		result.Failure = "vector operation timeout is not configured"
		return result, s.retryMemoryVectorOperation(ctx, outbox, item, leaseOwner, now, &result, result.Failure)
	}
	vectorCtx, cancelVector := context.WithTimeout(ctx, leaseDuration)
	defer cancelVector()
	if s.Vector == nil {
		result.CanonicalState = "retryable"
		result.Failure = "vector store is not configured"
		return result, s.retryMemoryVectorOperation(ctx, outbox, item, leaseOwner, now, &result, "vector store is not configured")
	}
	switch item.Operation {
	case "delete":
		deleter, ok := s.Vector.(vector.DocumentDeleter)
		if !ok {
			result.CanonicalState = "retryable"
			result.Failure = "vector store does not support document deletion"
			return result, s.retryMemoryVectorOperation(ctx, outbox, item, leaseOwner, now, &result, "vector store does not support document deletion")
		}
		if err := deleter.DeleteDocuments(vectorCtx, []string{item.DocumentID}); err != nil {
			result.CanonicalState = "retryable"
			result.Failure = err.Error()
			return result, s.retryMemoryVectorOperation(ctx, outbox, item, leaseOwner, now, &result, err.Error())
		}
		reader, ok := s.Vector.(vector.ExactDocumentReader)
		if !ok {
			result.CanonicalState = "retryable"
			result.Failure = "vector exact readback is not supported"
			return result, s.retryMemoryVectorOperation(ctx, outbox, item, leaseOwner, now, &result, result.Failure)
		}
		readback, readErr := reader.GetDocuments(vectorCtx, []string{item.DocumentID})
		if readErr != nil {
			result.CanonicalState = "retryable"
			result.Failure = "vector delete readback failed: " + readErr.Error()
			return result, s.retryMemoryVectorOperation(ctx, outbox, item, leaseOwner, now, &result, result.Failure)
		}
		for _, document := range readback {
			if strings.TrimSpace(document.ID) == strings.TrimSpace(item.DocumentID) {
				result.CanonicalState = "retryable"
				result.Failure = "vector delete readback still contains document"
				return result, s.retryMemoryVectorOperation(ctx, outbox, item, leaseOwner, now, &result, result.Failure)
			}
		}
	case "upsert":
		var document vector.VectorDocument
		if err := json.Unmarshal([]byte(item.DocumentJSON), &document); err != nil {
			result.CanonicalState = "permanent"
			result.Failure = err.Error()
			return result, s.failMemoryVectorOperationPermanently(ctx, outbox, item, leaseOwner, now, "materialized vector document is invalid: "+err.Error())
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
				return result, s.retryMemoryVectorOperation(ctx, outbox, item, leaseOwner, now, &result, result.Failure)
			}
			if strings.TrimSpace(document.DocumentText) == "" {
				result.CanonicalState = "permanent"
				result.Failure = "materialized vector document has no searchable text"
				return result, s.failMemoryVectorOperationPermanently(ctx, outbox, item, leaseOwner, now, result.Failure)
			}
			embeddingJSON := ""
			var embedErr error
			if usesVoyageContextualizedEmbedding(embeddingCfg) {
				contextChunks := stringsFromAny(document.Metadata["contextualized_embedding_inputs"])
				contextIndex := intFromAny(document.Metadata["contextualized_embedding_index"], -1)
				if len(contextChunks) == 0 || contextIndex < 0 || contextIndex >= len(contextChunks) {
					result.CanonicalState = "permanent"
					result.Failure = "contextualized embedding group metadata is invalid"
					return result, s.failMemoryVectorOperationPermanently(ctx, outbox, item, leaseOwner, now, result.Failure)
				}
				var grouped []string
				grouped, _, embedErr = callDocumentEmbeddings(vectorCtx, embeddingCfg, contextChunks)
				if embedErr == nil {
					embeddingJSON = grouped[contextIndex]
				}
			} else {
				embeddingJSON, _, embedErr = callEmbedding(vectorCtx, embeddingCfg, document.DocumentText)
			}
			if embedErr != nil {
				result.CanonicalState = "retryable"
				result.Failure = "embedding materialization failed"
				return result, s.retryMemoryVectorOperation(ctx, outbox, item, leaseOwner, now, &result, result.Failure)
			}
			document.Embedding = parseFloat32JSONList(embeddingJSON)
			if len(document.Embedding) == 0 {
				result.CanonicalState = "retryable"
				result.Failure = "embedding materialization returned no vector"
				return result, s.retryMemoryVectorOperation(ctx, outbox, item, leaseOwner, now, &result, result.Failure)
			}
		}
		delete(document.Metadata, "contextualized_embedding_inputs")
		delete(document.Metadata, "contextualized_embedding_index")
		if err := s.Vector.Upsert(vectorCtx, item.ChatSessionID, []vector.VectorDocument{document}); err != nil {
			result.CanonicalState = "retryable"
			result.Failure = err.Error()
			return result, s.retryMemoryVectorOperation(ctx, outbox, item, leaseOwner, now, &result, err.Error())
		}
		reader, ok := s.Vector.(vector.ExactDocumentReader)
		if !ok {
			result.CanonicalState = "retryable"
			result.Failure = "vector exact readback is not supported"
			return result, s.retryMemoryVectorOperation(ctx, outbox, item, leaseOwner, now, &result, result.Failure)
		}
		readback, readErr := reader.GetDocuments(vectorCtx, []string{item.DocumentID})
		if readErr != nil {
			result.CanonicalState = "retryable"
			result.Failure = "vector upsert readback failed: " + readErr.Error()
			return result, s.retryMemoryVectorOperation(ctx, outbox, item, leaseOwner, now, &result, result.Failure)
		}
		if verifyErr := verifyMemoryVectorUpsertReadback(item, document, readback); verifyErr != nil {
			result.CanonicalState = "retryable"
			result.Failure = verifyErr.Error()
			return result, s.retryMemoryVectorOperation(ctx, outbox, item, leaseOwner, now, &result, result.Failure)
		}
	default:
		result.CanonicalState = "permanent"
		result.Failure = "unknown vector operation"
		return result, s.failMemoryVectorOperationPermanently(ctx, outbox, item, leaseOwner, now, "unknown vector operation")
	}
	result.VectorApplied = true
	if err := outbox.CompleteMemoryVectorOperation(ctx, item.ID, leaseOwner, now); err != nil {
		if errors.Is(err, store.ErrSourceRevisionStale) && item.Operation == "upsert" {
			if deleter, ok := s.Vector.(vector.DocumentDeleter); ok {
				_ = deleter.DeleteDocuments(vectorCtx, []string{item.DocumentID})
			}
			result.CanonicalState = "stale_rejected"
			return result, nil
		}
		return result, err
	}
	result.CanonicalState = "completed"
	return result, nil
}

func verifyMemoryVectorUpsertReadback(item *store.MemoryVectorOutboxItem, expected vector.VectorDocument, readback []vector.VectorDocument) error {
	if item == nil {
		return fmt.Errorf("vector upsert readback item is missing")
	}
	expectedMetadata := expected.Metadata
	expectedRevision := strings.TrimSpace(extractionStringFromAny(expectedMetadata["source_revision"]))
	expectedContract := strings.TrimSpace(extractionStringFromAny(expectedMetadata["source_contract"]))
	expectedIndexIdentity := strings.TrimSpace(extractionStringFromAny(expectedMetadata["index_identity"]))
	expectedFingerprint := strings.TrimSpace(extractionStringFromAny(expectedMetadata["content_fingerprint"]))
	computedExpectedFingerprint := fmt.Sprintf("%x", sha256.Sum256([]byte(expected.DocumentText)))
	if expectedRevision == "" || expectedRevision != strings.TrimSpace(item.SourceRevision) ||
		expectedContract == "" || expectedIndexIdentity == "" ||
		expectedFingerprint == "" || expectedFingerprint != computedExpectedFingerprint {
		return fmt.Errorf("vector upsert verification metadata is invalid")
	}
	matched := 0
	for _, actual := range readback {
		if strings.TrimSpace(actual.ID) != strings.TrimSpace(item.DocumentID) {
			continue
		}
		matched++
		actualFingerprint := fmt.Sprintf("%x", sha256.Sum256([]byte(actual.DocumentText)))
		if strings.TrimSpace(extractionStringFromAny(actual.Metadata["source_revision"])) != expectedRevision ||
			strings.TrimSpace(extractionStringFromAny(actual.Metadata["source_contract"])) != expectedContract ||
			strings.TrimSpace(extractionStringFromAny(actual.Metadata["index_identity"])) != expectedIndexIdentity ||
			strings.TrimSpace(extractionStringFromAny(actual.Metadata["content_fingerprint"])) != expectedFingerprint ||
			actualFingerprint != expectedFingerprint {
			return fmt.Errorf("vector upsert readback metadata or content mismatch")
		}
	}
	if matched != 1 {
		return fmt.Errorf("vector upsert readback exact document count is %d", matched)
	}
	return nil
}

func (s *Server) processMemoryVectorOutboxBatch(
	ctx context.Context,
	leaseOwner string,
	now time.Time,
	leaseDuration time.Duration,
	limit int,
) []memoryVectorProcessResult {
	capacity := limit
	if capacity < 0 {
		capacity = 0
	}
	results := make([]memoryVectorProcessResult, 0, capacity)
	seen := map[int64]struct{}{}
	for limit <= 0 || len(results) < limit {
		result, err := s.processMemoryVectorOutboxOnce(ctx, leaseOwner, now, leaseDuration)
		if err != nil || !result.Processed {
			break
		}
		if result.OutboxID > 0 {
			if _, duplicate := seen[result.OutboxID]; duplicate {
				break
			}
			seen[result.OutboxID] = struct{}{}
		}
		results = append(results, result)
		if result.CanonicalState == "retryable" {
			continue
		}
	}
	return results
}

func (s *Server) retryMemoryVectorOperation(
	ctx context.Context,
	outbox store.MemoryVectorOutboxStore,
	item *store.MemoryVectorOutboxItem,
	leaseOwner string,
	now time.Time,
	result *memoryVectorProcessResult,
	failure string,
) error {
	if item == nil {
		return fmt.Errorf("memory vector outbox item is missing")
	}
	maxAttempts := s.runtimeConfigSnapshot().FailedQueueMaxAttempts
	terminalCode := ""
	switch {
	case maxAttempts < 1 || maxAttempts > 11:
		terminalCode = memoryVectorRetryLimitUnconfigured
	case item.Attempts >= maxAttempts:
		terminalCode = memoryVectorRetryLimitReached
	}
	if terminalCode != "" {
		if result != nil {
			result.CanonicalState = "permanent"
			result.Failure = terminalCode
		}
		persistedFailure := terminalCode
		if cause := strings.TrimSpace(failure); cause != "" &&
			!strings.EqualFold(cause, terminalCode) {
			persistedFailure += ": " + cause
		}
		if err := outbox.FailMemoryVectorOperation(
			ctx, item.ID, leaseOwner, now, time.Time{}, true, persistedFailure,
		); err != nil {
			return err
		}
		s.recordMemoryVectorRetryTerminal(
			ctx, item, terminalCode, maxAttempts, failure, now,
		)
		return nil
	}
	if err := outbox.FailMemoryVectorOperation(ctx, item.ID, leaseOwner, now, now, false, failure); err != nil {
		return err
	}
	return nil
}

func (s *Server) recordMemoryVectorRetryTerminal(
	ctx context.Context,
	item *store.MemoryVectorOutboxItem,
	code string,
	maxAttempts int,
	failure string,
	now time.Time,
) {
	if s == nil || s.Store == nil || item == nil || ctx == nil {
		return
	}
	apiKey := s.runtimeConfigSnapshot().EmbeddingAPIKey
	safeFailure := truncateRunes(
		strings.TrimSpace(scrubCriticFailureText(failure, apiKey)), 1000,
	)
	_ = s.Store.SaveAuditLog(ctx, &store.AuditLog{
		ChatSessionID: item.ChatSessionID,
		EventType:     "memory_vector_outbox_permanent",
		TargetType:    "memory_vector_outbox",
		TargetID:      item.ID,
		Summary:       fmt.Sprintf("vector outbox item %d reached a terminal retry state", item.ID),
		DetailsJSON: mustCompactJSON(map[string]any{
			"code":             code,
			"attempt":          item.Attempts,
			"max_attempts":     maxAttempts,
			"operation":        item.Operation,
			"document_id":      item.DocumentID,
			"source_revision":  item.SourceRevision,
			"provider_failure": safeFailure,
		}),
		Source:    s.storeWriteSource(),
		CreatedAt: now,
	})
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
