package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/risulongmemory/archive-center-go/internal/config"
	"github.com/risulongmemory/archive-center-go/internal/store"
	"github.com/risulongmemory/archive-center-go/internal/vector"
)

type memoryVectorProcessorStore struct {
	store.Store
	items            []*store.MemoryVectorOutboxItem
	completed        []int64
	failed           []int64
	failureRetryAt   []time.Time
	failurePermanent []bool
	failureReasons   []string
	auditLogs        []*store.AuditLog
	completeErr      error
}

func (f *memoryVectorProcessorStore) EnqueueMemoryVectorOperation(context.Context, *store.MemoryVectorOutboxItem) (bool, error) {
	return false, errors.New("unexpected enqueue")
}

func (f *memoryVectorProcessorStore) ClaimMemoryVectorOperation(_ context.Context, owner string, now time.Time, lease time.Duration) (*store.MemoryVectorOutboxItem, error) {
	if len(f.items) == 0 {
		return nil, store.ErrNotFound
	}
	item := f.items[0]
	f.items = f.items[1:]
	item.LeaseOwner = owner
	item.LeaseUntil = now.Add(lease)
	item.Status = "leased"
	item.Attempts++
	return item, nil
}

func (f *memoryVectorProcessorStore) CompleteMemoryVectorOperation(_ context.Context, id int64, _ string, _ time.Time) error {
	f.completed = append(f.completed, id)
	return f.completeErr
}

func (f *memoryVectorProcessorStore) FailMemoryVectorOperation(_ context.Context, id int64, _ string, _ time.Time, retryAfter time.Time, permanent bool, failure string) error {
	f.failed = append(f.failed, id)
	f.failureRetryAt = append(f.failureRetryAt, retryAfter)
	f.failurePermanent = append(f.failurePermanent, permanent)
	f.failureReasons = append(f.failureReasons, failure)
	return nil
}

func (f *memoryVectorProcessorStore) SaveAuditLog(_ context.Context, item *store.AuditLog) error {
	if item == nil {
		return nil
	}
	copy := *item
	f.auditLogs = append(f.auditLogs, &copy)
	return nil
}

type memoryVectorProcessorVector struct {
	vector.VectorStore
	upsertErr error
	deleteErr error
	upserts   [][]vector.VectorDocument
	deletes   [][]string
}

func (f *memoryVectorProcessorVector) Upsert(_ context.Context, _ string, docs []vector.VectorDocument) error {
	f.upserts = append(f.upserts, docs)
	return f.upsertErr
}

func (f *memoryVectorProcessorVector) DeleteDocuments(_ context.Context, ids []string) error {
	f.deletes = append(f.deletes, append([]string(nil), ids...))
	return f.deleteErr
}

func TestMemoryVectorProcessorRecordsRetryAfterVectorFailure(t *testing.T) {
	now := time.Date(2026, 7, 28, 4, 0, 0, 0, time.UTC)
	document := vector.VectorDocument{
		ID: "precise_memory:session:unit", ChatSessionID: "session",
		SourceTable: "precise_memory_units", SourceRowID: "unit",
		SchemaVersion: store.PreciseMemoryUnitContract, DocumentText: "grounded",
		Embedding: []float32{0.1, 0.2},
	}
	documentJSON, err := materializedMemoryVectorDocumentJSON(document)
	if err != nil {
		t.Fatal(err)
	}
	st := &memoryVectorProcessorStore{
		Store: store.NewNoopStore(),
		items: []*store.MemoryVectorOutboxItem{{
			ID: 1, Operation: "upsert", ChatSessionID: "session",
			SourceRevision: "sar_active", DocumentID: document.ID,
			DocumentJSON: documentJSON, EmbeddingReady: true,
			RequiredSourceState: "active", Status: "pending",
		}},
	}
	vec := &memoryVectorProcessorVector{
		VectorStore: vector.NewFakeVectorStore(),
		upsertErr:   errors.New("provider unavailable"),
	}
	server := &Server{
		Store: st, Vector: vec,
		RuntimeConfig: RuntimeConfig{
			Synced: true, FailedQueueMaxAttempts: 4,
		},
	}
	result, err := server.processMemoryVectorOutboxOnce(context.Background(), "worker", now, time.Minute)
	if err != nil {
		t.Fatal(err)
	}
	if !result.Processed || result.CanonicalState != "retryable" ||
		len(st.failed) != 1 || len(st.completed) != 0 ||
		!st.failureRetryAt[0].Equal(now) {
		t.Fatalf("result=%+v failed=%v completed=%v retry=%v", result, st.failed, st.completed, st.failureRetryAt)
	}
}

func TestMemoryVectorProcessorDeleteReplayIsIdempotent(t *testing.T) {
	now := time.Date(2026, 7, 28, 4, 30, 0, 0, time.UTC)
	st := &memoryVectorProcessorStore{
		Store: store.NewNoopStore(),
		items: []*store.MemoryVectorOutboxItem{
			{ID: 2, Operation: "delete", ChatSessionID: "session", SourceRevision: "sar_old", DocumentID: "memory:session:7", EmbeddingReady: true, RequiredSourceState: "inactive"},
			{ID: 3, Operation: "delete", ChatSessionID: "session", SourceRevision: "sar_old", DocumentID: "memory:session:7", EmbeddingReady: true, RequiredSourceState: "inactive"},
		},
	}
	vec := &memoryVectorProcessorVector{VectorStore: vector.NewFakeVectorStore()}
	server := &Server{
		Store: st, Vector: vec,
		RuntimeConfig: RuntimeConfig{
			Synced: true, FailedQueueMaxAttempts: 4,
		},
	}
	for range 2 {
		result, err := server.processMemoryVectorOutboxOnce(context.Background(), "worker", now, time.Minute)
		if err != nil || result.CanonicalState != "completed" {
			t.Fatalf("result=%+v err=%v", result, err)
		}
	}
	if len(vec.deletes) != 2 || len(st.completed) != 2 || len(st.failed) != 0 {
		t.Fatalf("deletes=%v completed=%v failed=%v", vec.deletes, st.completed, st.failed)
	}
}

func TestMemoryVectorProcessorCompensatesStaleUpsertWithoutResurrection(t *testing.T) {
	now := time.Date(2026, 7, 28, 5, 0, 0, 0, time.UTC)
	document := vector.VectorDocument{
		ID: "precise_memory:session:unit", ChatSessionID: "session",
		Embedding: []float32{0.3}, DocumentText: "stale",
	}
	documentJSON, err := materializedMemoryVectorDocumentJSON(document)
	if err != nil {
		t.Fatal(err)
	}
	st := &memoryVectorProcessorStore{
		Store:       store.NewNoopStore(),
		completeErr: store.ErrSourceRevisionStale,
		items: []*store.MemoryVectorOutboxItem{{
			ID: 4, Operation: "upsert", ChatSessionID: "session",
			SourceRevision: "sar_superseded", DocumentID: document.ID,
			DocumentJSON: documentJSON, EmbeddingReady: true,
			RequiredSourceState: "active",
		}},
	}
	vec := &memoryVectorProcessorVector{VectorStore: vector.NewFakeVectorStore()}
	server := &Server{
		Store: st, Vector: vec,
		RuntimeConfig: RuntimeConfig{
			Synced: true, FailedQueueMaxAttempts: 4,
		},
	}
	result, err := server.processMemoryVectorOutboxOnce(context.Background(), "worker", now, time.Minute)
	if err != nil {
		t.Fatal(err)
	}
	if result.CanonicalState != "stale_rejected" || len(vec.upserts) != 1 ||
		len(vec.deletes) != 1 || vec.deletes[0][0] != document.ID {
		t.Fatalf("result=%+v upserts=%v deletes=%v", result, vec.upserts, vec.deletes)
	}
}

func TestMemoryVectorProcessorMaterializesDeferredEmbedding(t *testing.T) {
	now := time.Now().UTC()
	document := vector.VectorDocument{
		ID: "evidence:session:7", ChatSessionID: "session",
		SourceTable: "direct_evidence_records", SourceRowID: "7",
		SchemaVersion: "direct_evidence.v1", DocumentText: "Mina found the key.",
	}
	documentJSON, err := json.Marshal(document)
	if err != nil {
		t.Fatal(err)
	}
	st := &memoryVectorProcessorStore{
		Store: store.NewNoopStore(),
		items: []*store.MemoryVectorOutboxItem{{
			ID: 5, Operation: "upsert", ChatSessionID: "session",
			SourceRevision: "sar_active", DocumentID: document.ID,
			DocumentJSON: string(documentJSON), EmbeddingReady: false,
			RequiredSourceState: "active", Status: "needs_embedding",
		}},
	}
	vec := &memoryVectorProcessorVector{VectorStore: vector.NewFakeVectorStore()}
	oldClient := proxyHTTPClient
	proxyHTTPClient = &http.Client{Transport: roundTripFunc(func(*http.Request) (*http.Response, error) {
		payload := `{"model":"embedding-test","data":[{"embedding":[0.1,0.2]}]}`
		return &http.Response{
			StatusCode: http.StatusOK,
			Header:     make(http.Header),
			Body:       io.NopCloser(strings.NewReader(payload)),
		}, nil
	})}
	defer func() { proxyHTTPClient = oldClient }()
	server := &Server{
		Store: st, Vector: vec,
		RuntimeConfig: RuntimeConfig{
			Synced: true, EmbeddingProvider: "openai",
			EmbeddingAPIKey: "test-key", EmbeddingEndpoint: "https://example.invalid/v1",
			EmbeddingModel:         "embedding-test",
			EmbeddingTimeoutSec:    30,
			FailedQueueMaxAttempts: 4,
		},
	}
	result, err := server.processMemoryVectorOutboxOnce(
		context.Background(), "worker", now, time.Minute,
	)
	if err != nil {
		t.Fatal(err)
	}
	if result.CanonicalState != "completed" || len(st.completed) != 1 ||
		len(vec.upserts) != 1 || len(vec.upserts[0]) != 1 ||
		len(vec.upserts[0][0].Embedding) != 2 {
		t.Fatalf("result=%+v completed=%v upserts=%+v", result, st.completed, vec.upserts)
	}
}

func TestMemoryVectorProcessorTerminatesAtConfiguredRetryLimitWithTypedAudit(t *testing.T) {
	now := time.Date(2026, 7, 28, 6, 0, 0, 0, time.UTC)
	document := vector.VectorDocument{
		ID: "precise_memory:session:unit", ChatSessionID: "session",
		SourceTable: "precise_memory_units", SourceRowID: "unit",
		SchemaVersion: store.PreciseMemoryUnitContract, DocumentText: "grounded",
		Embedding: []float32{0.1, 0.2},
	}
	documentJSON, err := materializedMemoryVectorDocumentJSON(document)
	if err != nil {
		t.Fatal(err)
	}
	st := &memoryVectorProcessorStore{
		Store: store.NewNoopStore(),
		items: []*store.MemoryVectorOutboxItem{{
			ID: 6, Operation: "upsert", ChatSessionID: "session",
			SourceRevision: "sar_active", DocumentID: document.ID,
			DocumentJSON: documentJSON, EmbeddingReady: true,
			RequiredSourceState: "active", Status: "retryable", Attempts: 2,
		}},
	}
	vec := &memoryVectorProcessorVector{
		VectorStore: vector.NewFakeVectorStore(),
		upsertErr:   errors.New("provider unavailable"),
	}
	server := &Server{
		Cfg: config.Default(), Store: st, Vector: vec,
		RuntimeConfig: RuntimeConfig{
			Synced: true, FailedQueueMaxAttempts: 3,
		},
	}
	result, err := server.processMemoryVectorOutboxOnce(
		context.Background(), "worker", now, time.Minute,
	)
	if err != nil {
		t.Fatal(err)
	}
	if result.CanonicalState != "permanent" ||
		result.Failure != memoryVectorRetryLimitReached ||
		len(st.failed) != 1 ||
		len(st.failurePermanent) != 1 || !st.failurePermanent[0] ||
		len(st.failureRetryAt) != 1 || !st.failureRetryAt[0].IsZero() ||
		len(st.failureReasons) != 1 ||
		!strings.HasPrefix(st.failureReasons[0], memoryVectorRetryLimitReached) ||
		len(st.auditLogs) != 1 {
		t.Fatalf(
			"result=%+v failed=%v permanent=%v retry=%v reasons=%v audits=%d",
			result, st.failed, st.failurePermanent, st.failureRetryAt,
			st.failureReasons, len(st.auditLogs),
		)
	}
	audit := st.auditLogs[0]
	if audit.EventType != "memory_vector_outbox_permanent" ||
		audit.TargetType != "memory_vector_outbox" ||
		audit.TargetID != 6 ||
		!strings.Contains(audit.DetailsJSON, memoryVectorRetryLimitReached) {
		t.Fatalf("audit=%+v", audit)
	}
}
