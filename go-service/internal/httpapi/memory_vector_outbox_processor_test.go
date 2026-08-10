package httpapi

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
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

func (f *memoryVectorProcessorStore) ClaimMemoryVectorOperations(_ context.Context, owner string, now time.Time, lease time.Duration) ([]*store.MemoryVectorOutboxItem, error) {
	if len(f.items) == 0 {
		return nil, store.ErrNotFound
	}
	item := f.items[0]
	f.items = f.items[1:]
	items := []*store.MemoryVectorOutboxItem{item}
	if item.Operation == "upsert" && !item.EmbeddingReady {
		remaining := f.items[:0]
		for _, candidate := range f.items {
			if candidate.Operation == "upsert" && !candidate.EmbeddingReady && candidate.SourceRevision == item.SourceRevision {
				items = append(items, candidate)
				continue
			}
			remaining = append(remaining, candidate)
		}
		f.items = remaining
	}
	for _, claimed := range items {
		claimed.LeaseOwner = owner
		claimed.LeaseUntil = now.Add(lease)
		claimed.Status = "leased"
		claimed.Attempts++
	}
	return items, nil
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
	upsertErr           error
	upsertErrs          []error
	deleteErr           error
	upserts             [][]vector.VectorDocument
	deletes             [][]string
	documents           map[string]vector.VectorDocument
	readbackOverride    []vector.VectorDocument
	readbackOverrideSet bool
	readbackErr         error
	retainDeletes       bool
}

type blockingMemoryVectorProcessorVector struct {
	vector.VectorStore
}

func (f *blockingMemoryVectorProcessorVector) Upsert(ctx context.Context, _ string, _ []vector.VectorDocument) error {
	<-ctx.Done()
	return ctx.Err()
}

func (f *blockingMemoryVectorProcessorVector) GetDocuments(context.Context, []string) ([]vector.VectorDocument, error) {
	return nil, nil
}

func (f *memoryVectorProcessorVector) Upsert(_ context.Context, _ string, docs []vector.VectorDocument) error {
	f.upserts = append(f.upserts, docs)
	if len(f.upsertErrs) > 0 {
		err := f.upsertErrs[0]
		f.upsertErrs = f.upsertErrs[1:]
		if err != nil {
			return err
		}
	} else if f.upsertErr != nil {
		return f.upsertErr
	}
	if f.documents == nil {
		f.documents = map[string]vector.VectorDocument{}
	}
	for _, document := range docs {
		f.documents[document.ID] = document
	}
	return nil
}

func (f *memoryVectorProcessorVector) DeleteDocuments(_ context.Context, ids []string) error {
	f.deletes = append(f.deletes, append([]string(nil), ids...))
	if f.deleteErr != nil {
		return f.deleteErr
	}
	if !f.retainDeletes {
		for _, id := range ids {
			delete(f.documents, id)
		}
	}
	return nil
}

func (f *memoryVectorProcessorVector) GetDocuments(_ context.Context, ids []string) ([]vector.VectorDocument, error) {
	if f.readbackErr != nil {
		return nil, f.readbackErr
	}
	if f.readbackOverrideSet {
		return append([]vector.VectorDocument(nil), f.readbackOverride...), nil
	}
	out := make([]vector.VectorDocument, 0, len(ids))
	for _, id := range ids {
		if document, ok := f.documents[id]; ok {
			out = append(out, document)
		}
	}
	return out, nil
}

func verifiedMemoryVectorProcessorDocument(document vector.VectorDocument, sourceRevision string) vector.VectorDocument {
	document.Metadata = map[string]any{
		"source_revision":     sourceRevision,
		"source_contract":     store.MemorySourceRevisionContract,
		"index_identity":      "memory-vector-index-v1",
		"content_fingerprint": fmt.Sprintf("%x", sha256.Sum256([]byte(document.DocumentText))),
	}
	return document
}

func deferredVoyageMemoryVectorOutboxItem(t *testing.T, id int64, sourceRevision, documentID, documentText string, inputs []string, index int) *store.MemoryVectorOutboxItem {
	t.Helper()
	document := verifiedMemoryVectorProcessorDocument(vector.VectorDocument{
		ID: documentID, ChatSessionID: "session", SourceTable: "precise_memory_units",
		SourceRowID: fmt.Sprint(id), SchemaVersion: store.PreciseMemoryUnitContract,
		DocumentText: documentText,
	}, sourceRevision)
	document.Metadata["contextualized_embedding_inputs"] = append([]string(nil), inputs...)
	document.Metadata["contextualized_embedding_index"] = index
	documentJSON, err := json.Marshal(document)
	if err != nil {
		t.Fatal(err)
	}
	return &store.MemoryVectorOutboxItem{
		ID: id, Operation: "upsert", ChatSessionID: "session", SourceRevision: sourceRevision,
		DocumentID: documentID, DocumentJSON: string(documentJSON), EmbeddingReady: false,
		RequiredSourceState: "active", Status: "needs_embedding",
	}
}

func TestMemoryVectorProcessorRecordsRetryAfterVectorFailure(t *testing.T) {
	now := time.Date(2026, 7, 28, 4, 0, 0, 0, time.UTC)
	document := vector.VectorDocument{
		ID: "precise_memory:session:unit", ChatSessionID: "session",
		SourceTable: "precise_memory_units", SourceRowID: "unit",
		SchemaVersion: store.PreciseMemoryUnitContract, DocumentText: "grounded",
		Embedding: []float32{0.1, 0.2},
	}
	document = verifiedMemoryVectorProcessorDocument(document, "sar_active")
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

func TestMemoryVectorOutboxRetryReusesFullVoyageContextGroupAndStableIndex(t *testing.T) {
	oldClient := proxyHTTPClient
	calls := 0
	proxyHTTPClient = &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		calls++
		var request map[string]any
		if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
			t.Fatalf("decode request: %v", err)
		}
		groups := sliceFromAny(request["inputs"])
		if len(groups) != 1 || len(sliceFromAny(groups[0])) != 2 {
			t.Fatalf("retry request did not preserve sibling group: %#v", groups)
		}
		return &http.Response{StatusCode: http.StatusOK, Status: "200 OK", Header: make(http.Header), Body: io.NopCloser(strings.NewReader(`{"data":[{"index":0,"data":[{"index":0,"embedding":[1,0]},{"index":1,"embedding":[2,0]}]}],"model":"voyage-context-4"}`))}, nil
	})}
	defer func() { proxyHTTPClient = oldClient }()

	now := time.Date(2026, 8, 6, 12, 0, 0, 0, time.UTC)
	document := verifiedMemoryVectorProcessorDocument(vector.VectorDocument{
		ID: "precise_memory:session:unit", ChatSessionID: "session",
		SourceTable: "precise_memory_units", SourceRowID: "unit",
		SchemaVersion: store.PreciseMemoryUnitContract, DocumentText: "second sibling",
	}, "revision-context-retry")
	document.Metadata["contextualized_embedding_inputs"] = []string{"first sibling", "second sibling"}
	document.Metadata["contextualized_embedding_index"] = 1
	documentJSONBytes, _ := json.Marshal(document)
	st := &memoryVectorProcessorStore{Store: store.NewNoopStore(), items: []*store.MemoryVectorOutboxItem{{
		ID: 90, Operation: "upsert", ChatSessionID: "session", SourceRevision: "revision-context-retry",
		DocumentID: document.ID, DocumentJSON: string(documentJSONBytes), RequiredSourceState: "active", Status: "needs_embedding",
	}}}
	vec := &memoryVectorProcessorVector{VectorStore: vector.NewFakeVectorStore()}
	server := &Server{Store: st, Vector: vec, RuntimeConfig: RuntimeConfig{
		Synced: true, FailedQueueMaxAttempts: 4, EmbeddingProvider: "voyageai",
		EmbeddingAPIKey: "key", EmbeddingEndpoint: "https://api.voyageai.com/v1/embeddings",
		EmbeddingModel: "voyage-context-4", EmbeddingTimeoutSec: 30,
	}}
	result, err := server.processMemoryVectorOutboxOnce(context.Background(), "worker", now, time.Minute)
	if err != nil || result.CanonicalState != "completed" || calls != 1 {
		t.Fatalf("result=%+v calls=%d err=%v", result, calls, err)
	}
	if len(vec.upserts) != 1 || len(vec.upserts[0]) != 1 || len(vec.upserts[0][0].Embedding) != 2 || vec.upserts[0][0].Embedding[0] != 2 {
		t.Fatalf("stable chunk index was not mapped to the target document: %#v", vec.upserts)
	}
	if _, leaked := vec.upserts[0][0].Metadata["contextualized_embedding_inputs"]; leaked {
		t.Fatalf("retry-only sibling payload leaked into Chroma metadata")
	}
}

func TestMemoryVectorOutboxVoyageClaimsTurnSiblingsForOneContextualizedCall(t *testing.T) {
	oldClient := proxyHTTPClient
	calls := 0
	proxyHTTPClient = &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		calls++
		var request map[string]any
		if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
			t.Fatal(err)
		}
		groups := sliceFromAny(request["inputs"])
		chunks := sliceFromAny(groups[0])
		if len(groups) != 1 || len(chunks) != 5 {
			t.Fatalf("Voyage request groups=%#v", groups)
		}
		rows := make([]map[string]any, 0, len(chunks))
		for index := range chunks {
			rows = append(rows, map[string]any{"index": index, "embedding": []float64{float64(index + 1), 0}})
		}
		body, _ := json.Marshal(map[string]any{"data": []any{map[string]any{"index": 0, "data": rows}}, "model": "voyage-context-4"})
		return &http.Response{StatusCode: http.StatusOK, Status: "200 OK", Header: make(http.Header), Body: io.NopCloser(strings.NewReader(string(body)))}, nil
	})}
	defer func() { proxyHTTPClient = oldClient }()

	inputs := []string{"raw user", "raw assistant", "artifact one", "artifact two", "artifact three"}
	st := &memoryVectorProcessorStore{Store: store.NewNoopStore(), items: []*store.MemoryVectorOutboxItem{
		deferredVoyageMemoryVectorOutboxItem(t, 101, "revision-turn", "precise_memory:session:one", "artifact one", inputs, 2),
		deferredVoyageMemoryVectorOutboxItem(t, 102, "revision-turn", "precise_memory:session:two", "artifact two", inputs, 3),
		deferredVoyageMemoryVectorOutboxItem(t, 103, "revision-turn", "precise_memory:session:three", "artifact three", inputs, 4),
	}}
	vec := &memoryVectorProcessorVector{VectorStore: vector.NewFakeVectorStore()}
	server := &Server{Store: st, Vector: vec, RuntimeConfig: RuntimeConfig{
		Synced: true, FailedQueueMaxAttempts: 4, EmbeddingProvider: "voyageai",
		EmbeddingAPIKey: "key", EmbeddingEndpoint: "https://api.voyageai.com/v1/embeddings",
		EmbeddingModel: "voyage-context-4", EmbeddingTimeoutSec: 30,
	}}
	results := server.processMemoryVectorOutboxBatch(context.Background(), "worker", time.Now().UTC(), time.Minute, 0)
	if len(results) != 3 || calls != 1 {
		t.Fatalf("results=%+v calls=%d", results, calls)
	}
	for _, result := range results {
		if result.CanonicalState != "completed" {
			t.Fatalf("results=%+v", results)
		}
	}
	if len(st.completed) != 3 || len(st.failed) != 0 || len(vec.upserts) != 3 {
		t.Fatalf("completed=%v failed=%v upserts=%#v", st.completed, st.failed, vec.upserts)
	}
	for index, upsert := range vec.upserts {
		if len(upsert) != 1 || len(upsert[0].Embedding) != 2 || upsert[0].Embedding[0] != float32(index+3) {
			t.Fatalf("upsert %d did not receive stable contextualized index: %#v", index, upsert)
		}
	}
}

func TestMemoryVectorOutboxVoyageSeparatesRevisionDeleteAndEmbeddingReady(t *testing.T) {
	oldClient := proxyHTTPClient
	documents := [][]string{}
	proxyHTTPClient = &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		var request map[string]any
		if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
			t.Fatal(err)
		}
		chunksAny := sliceFromAny(sliceFromAny(request["inputs"])[0])
		chunks := make([]string, 0, len(chunksAny))
		rows := make([]map[string]any, 0, len(chunksAny))
		for index, chunk := range chunksAny {
			chunks = append(chunks, extractionStringFromAny(chunk))
			rows = append(rows, map[string]any{"index": index, "embedding": []float64{float64(index + 1), 0}})
		}
		documents = append(documents, chunks)
		body, _ := json.Marshal(map[string]any{"data": []any{map[string]any{"index": 0, "data": rows}}, "model": "voyage-context-4"})
		return &http.Response{StatusCode: http.StatusOK, Status: "200 OK", Header: make(http.Header), Body: io.NopCloser(strings.NewReader(string(body)))}, nil
	})}
	defer func() { proxyHTTPClient = oldClient }()

	ready := verifiedMemoryVectorProcessorDocument(vector.VectorDocument{
		ID: "memory:session:ready", ChatSessionID: "session", DocumentText: "already ready", Embedding: []float32{9, 0},
	}, "revision-a")
	readyJSON, err := materializedMemoryVectorDocumentJSON(ready)
	if err != nil {
		t.Fatal(err)
	}
	st := &memoryVectorProcessorStore{Store: store.NewNoopStore(), items: []*store.MemoryVectorOutboxItem{
		deferredVoyageMemoryVectorOutboxItem(t, 111, "revision-a", "memory:session:a", "artifact a", []string{"user a", "assistant a", "artifact a"}, 2),
		deferredVoyageMemoryVectorOutboxItem(t, 112, "revision-b", "memory:session:b", "artifact b", []string{"user b", "assistant b", "artifact b"}, 2),
		{ID: 113, Operation: "delete", ChatSessionID: "session", SourceRevision: "revision-a", DocumentID: "memory:session:old", EmbeddingReady: true, RequiredSourceState: "inactive", Status: "pending"},
		{ID: 114, Operation: "upsert", ChatSessionID: "session", SourceRevision: "revision-a", DocumentID: ready.ID, DocumentJSON: readyJSON, EmbeddingReady: true, RequiredSourceState: "active", Status: "pending"},
	}}
	vec := &memoryVectorProcessorVector{VectorStore: vector.NewFakeVectorStore()}
	server := &Server{Store: st, Vector: vec, RuntimeConfig: RuntimeConfig{
		Synced: true, FailedQueueMaxAttempts: 4, EmbeddingProvider: "voyageai",
		EmbeddingAPIKey: "key", EmbeddingEndpoint: "https://api.voyageai.com/v1/embeddings",
		EmbeddingModel: "voyage-context-4", EmbeddingTimeoutSec: 30,
	}}
	for range 4 {
		result, err := server.processMemoryVectorOutboxOnce(context.Background(), "worker", time.Now().UTC(), time.Minute)
		if err != nil || result.CanonicalState != "completed" {
			t.Fatalf("result=%+v err=%v", result, err)
		}
	}
	if len(documents) != 2 || strings.Join(documents[0], "\n") == strings.Join(documents[1], "\n") ||
		len(st.completed) != 4 || len(vec.deletes) != 1 || len(vec.upserts) != 3 {
		t.Fatalf("documents=%#v completed=%v deletes=%v upserts=%#v", documents, st.completed, vec.deletes, vec.upserts)
	}
}

func TestMemoryVectorOutboxVoyageGroupEmbeddingFailureRetriesEverySibling(t *testing.T) {
	oldClient := proxyHTTPClient
	calls := 0
	proxyHTTPClient = &http.Client{Transport: roundTripFunc(func(*http.Request) (*http.Response, error) {
		calls++
		return &http.Response{StatusCode: http.StatusServiceUnavailable, Status: "503 Service Unavailable", Header: make(http.Header), Body: io.NopCloser(strings.NewReader(`{"error":"unavailable"}`))}, nil
	})}
	defer func() { proxyHTTPClient = oldClient }()

	inputs := []string{"raw user", "raw assistant", "artifact one", "artifact two", "artifact three"}
	st := &memoryVectorProcessorStore{Store: store.NewNoopStore(), items: []*store.MemoryVectorOutboxItem{
		deferredVoyageMemoryVectorOutboxItem(t, 121, "revision-failure", "memory:session:one", "artifact one", inputs, 2),
		deferredVoyageMemoryVectorOutboxItem(t, 122, "revision-failure", "memory:session:two", "artifact two", inputs, 3),
		deferredVoyageMemoryVectorOutboxItem(t, 123, "revision-failure", "memory:session:three", "artifact three", inputs, 4),
	}}
	vec := &memoryVectorProcessorVector{VectorStore: vector.NewFakeVectorStore()}
	now := time.Now().UTC()
	server := &Server{Store: st, Vector: vec, RuntimeConfig: RuntimeConfig{
		Synced: true, FailedQueueMaxAttempts: 4, EmbeddingProvider: "voyageai",
		EmbeddingAPIKey: "key", EmbeddingEndpoint: "https://api.voyageai.com/v1/embeddings",
		EmbeddingModel: "voyage-context-4", EmbeddingTimeoutSec: 30,
	}}
	result, err := server.processMemoryVectorOutboxOnce(context.Background(), "worker", now, time.Minute)
	if err != nil || result.CanonicalState != "retryable" || calls != 1 {
		t.Fatalf("result=%+v calls=%d err=%v", result, calls, err)
	}
	if len(st.failed) != 3 || len(st.completed) != 0 || len(st.failurePermanent) != 3 || len(vec.upserts) != 0 {
		t.Fatalf("failed=%v completed=%v permanent=%v upserts=%#v", st.failed, st.completed, st.failurePermanent, vec.upserts)
	}
	for index := range st.failed {
		if st.failurePermanent[index] || !st.failureRetryAt[index].Equal(now) {
			t.Fatalf("failure %d permanent=%v retry=%v", index, st.failurePermanent[index], st.failureRetryAt[index])
		}
	}
}

func TestMemoryVectorProcessorUnboundedDrainDoesNotLetRetryBlockLaterItem(t *testing.T) {
	now := time.Date(2026, 7, 30, 4, 10, 0, 0, time.UTC)
	makeItem := func(id int64) *store.MemoryVectorOutboxItem {
		sourceRevision := fmt.Sprintf("sar_%d", id)
		document := vector.VectorDocument{
			ID:            fmt.Sprintf("precise_memory:session:%d", id),
			ChatSessionID: "session", SourceTable: "precise_memory_units",
			SourceRowID: fmt.Sprint(id), SchemaVersion: store.PreciseMemoryUnitContract,
			DocumentText: "grounded", Embedding: []float32{0.1, 0.2},
		}
		document = verifiedMemoryVectorProcessorDocument(document, sourceRevision)
		documentJSON, err := materializedMemoryVectorDocumentJSON(document)
		if err != nil {
			t.Fatal(err)
		}
		return &store.MemoryVectorOutboxItem{
			ID: id, Operation: "upsert", ChatSessionID: "session",
			SourceRevision: sourceRevision, DocumentID: document.ID,
			DocumentJSON: documentJSON, EmbeddingReady: true,
			RequiredSourceState: "active", Status: "pending",
		}
	}
	st := &memoryVectorProcessorStore{
		Store: store.NewNoopStore(),
		items: []*store.MemoryVectorOutboxItem{makeItem(1), makeItem(2)},
	}
	vec := &memoryVectorProcessorVector{
		VectorStore: vector.NewFakeVectorStore(),
		upsertErrs:  []error{errors.New("provider unavailable"), nil},
	}
	server := &Server{
		Store: st, Vector: vec,
		RuntimeConfig: RuntimeConfig{
			Synced: true, FailedQueueMaxAttempts: 4,
		},
	}
	results := server.processMemoryVectorOutboxBatch(
		context.Background(), "worker", now, time.Minute, 0,
	)
	if len(results) != 2 ||
		results[0].CanonicalState != "retryable" ||
		results[1].CanonicalState != "completed" ||
		len(st.failed) != 1 || st.failed[0] != 1 ||
		len(st.completed) != 1 || st.completed[0] != 2 {
		t.Fatalf("results=%+v failed=%v completed=%v", results, st.failed, st.completed)
	}
}

func TestMemoryVectorProcessorBoundsBlockingVectorCallByLease(t *testing.T) {
	now := time.Date(2026, 7, 30, 4, 0, 0, 0, time.UTC)
	document := vector.VectorDocument{
		ID: "precise_memory:session:bounded", ChatSessionID: "session",
		SourceTable: "precise_memory_units", SourceRowID: "bounded",
		SchemaVersion: store.PreciseMemoryUnitContract, DocumentText: "grounded",
		Embedding: []float32{0.1, 0.2},
	}
	document = verifiedMemoryVectorProcessorDocument(document, "sar_active")
	documentJSON, err := materializedMemoryVectorDocumentJSON(document)
	if err != nil {
		t.Fatal(err)
	}
	st := &memoryVectorProcessorStore{
		Store: store.NewNoopStore(),
		items: []*store.MemoryVectorOutboxItem{{
			ID: 9, Operation: "upsert", ChatSessionID: "session",
			SourceRevision: "sar_active", DocumentID: document.ID,
			DocumentJSON: documentJSON, EmbeddingReady: true,
			RequiredSourceState: "active", Status: "pending",
		}},
	}
	server := &Server{
		Store:  st,
		Vector: &blockingMemoryVectorProcessorVector{VectorStore: vector.NewFakeVectorStore()},
		RuntimeConfig: RuntimeConfig{
			Synced: true, FailedQueueMaxAttempts: 4,
		},
	}
	started := time.Now()
	result, err := server.processMemoryVectorOutboxOnce(
		context.Background(), "worker", now, 20*time.Millisecond,
	)
	if err != nil {
		t.Fatal(err)
	}
	if result.CanonicalState != "retryable" || len(st.failed) != 1 {
		t.Fatalf("result=%+v failed=%v", result, st.failed)
	}
	if elapsed := time.Since(started); elapsed > time.Second {
		t.Fatalf("blocking vector call exceeded lease deadline: %s", elapsed)
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

func TestMemoryVectorProcessorDoesNotCompleteMismatchedUpsertReadback(t *testing.T) {
	now := time.Date(2026, 7, 31, 1, 0, 0, 0, time.UTC)
	document := verifiedMemoryVectorProcessorDocument(vector.VectorDocument{
		ID: "memory:session:9", ChatSessionID: "session",
		SourceTable: "memories", SourceRowID: "9",
		SchemaVersion: "memory.v1", DocumentText: "current memory",
		Embedding: []float32{0.1, 0.2},
	}, "sar_active")
	documentJSON, err := materializedMemoryVectorDocumentJSON(document)
	if err != nil {
		t.Fatal(err)
	}
	st := &memoryVectorProcessorStore{
		Store: store.NewNoopStore(),
		items: []*store.MemoryVectorOutboxItem{{
			ID: 10, Operation: "upsert", ChatSessionID: "session",
			SourceRevision: "sar_active", DocumentID: document.ID,
			DocumentJSON: documentJSON, EmbeddingReady: true,
			RequiredSourceState: "active", Status: "pending",
		}},
	}
	mismatch := document
	mismatch.DocumentText = "stale memory"
	vec := &memoryVectorProcessorVector{
		VectorStore:         vector.NewFakeVectorStore(),
		readbackOverrideSet: true,
		readbackOverride:    []vector.VectorDocument{mismatch},
	}
	server := &Server{
		Store: st, Vector: vec,
		RuntimeConfig: RuntimeConfig{Synced: true, FailedQueueMaxAttempts: 4},
	}
	result, err := server.processMemoryVectorOutboxOnce(context.Background(), "worker", now, time.Minute)
	if err != nil {
		t.Fatal(err)
	}
	if result.CanonicalState != "retryable" || result.VectorApplied ||
		len(st.completed) != 0 || len(st.failed) != 1 ||
		!strings.Contains(result.Failure, "readback metadata or content mismatch") {
		t.Fatalf("result=%+v completed=%v failed=%v", result, st.completed, st.failed)
	}
}

func TestMemoryVectorProcessorDoesNotCompleteDeleteWhileDocumentStillExists(t *testing.T) {
	now := time.Date(2026, 7, 31, 1, 30, 0, 0, time.UTC)
	documentID := "memory:session:9"
	st := &memoryVectorProcessorStore{
		Store: store.NewNoopStore(),
		items: []*store.MemoryVectorOutboxItem{{
			ID: 11, Operation: "delete", ChatSessionID: "session",
			SourceRevision: "sar_old", DocumentID: documentID,
			RequiredSourceState: "inactive", Status: "pending",
		}},
	}
	vec := &memoryVectorProcessorVector{
		VectorStore:   vector.NewFakeVectorStore(),
		retainDeletes: true,
		documents: map[string]vector.VectorDocument{
			documentID: {ID: documentID, ChatSessionID: "session"},
		},
	}
	server := &Server{
		Store: st, Vector: vec,
		RuntimeConfig: RuntimeConfig{Synced: true, FailedQueueMaxAttempts: 4},
	}
	result, err := server.processMemoryVectorOutboxOnce(context.Background(), "worker", now, time.Minute)
	if err != nil {
		t.Fatal(err)
	}
	if result.CanonicalState != "retryable" || result.VectorApplied ||
		len(st.completed) != 0 || len(st.failed) != 1 ||
		!strings.Contains(result.Failure, "still contains document") {
		t.Fatalf("result=%+v completed=%v failed=%v", result, st.completed, st.failed)
	}
}

func TestMemoryVectorProcessorCompensatesStaleUpsertWithoutResurrection(t *testing.T) {
	now := time.Date(2026, 7, 28, 5, 0, 0, 0, time.UTC)
	document := vector.VectorDocument{
		ID: "precise_memory:session:unit", ChatSessionID: "session",
		Embedding: []float32{0.3}, DocumentText: "stale",
	}
	document = verifiedMemoryVectorProcessorDocument(document, "sar_superseded")
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
	document = verifiedMemoryVectorProcessorDocument(document, "sar_active")
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
	document = verifiedMemoryVectorProcessorDocument(document, "sar_active")
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
