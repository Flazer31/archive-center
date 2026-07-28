package httpapi

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/risulongmemory/archive-center-go/internal/config"
	"github.com/risulongmemory/archive-center-go/internal/store"
)

type completeTurnReprocessingStore struct {
	*turnRecordingStore
	sources map[string]*store.MemorySourceRevision
	jobs    map[string]*store.MemoryReprocessingJob
}

func (f *completeTurnReprocessingStore) MemoryDerivationLifecycleEnabled() bool {
	return true
}

func (f *completeTurnReprocessingStore) RegisterAcceptedSourceRevision(_ context.Context, source *store.MemorySourceRevision) (store.SourceRevisionRegistration, error) {
	if f.sources == nil {
		f.sources = map[string]*store.MemorySourceRevision{}
	}
	if _, exists := f.sources[source.SourceRevision]; exists {
		return store.SourceRevisionRegistration{Idempotent: true}, nil
	}
	copy := *source
	f.sources[source.SourceRevision] = &copy
	return store.SourceRevisionRegistration{Inserted: true}, nil
}

func (f *completeTurnReprocessingStore) GetSourceRevision(_ context.Context, sid, revision string) (*store.MemorySourceRevision, error) {
	source := f.sources[revision]
	if source == nil || source.ChatSessionID != sid {
		return nil, store.ErrNotFound
	}
	copy := *source
	return &copy, nil
}

func (f *completeTurnReprocessingStore) IsSourceRevisionActive(_ context.Context, sid, revision string) (bool, error) {
	source := f.sources[revision]
	return source != nil && source.ChatSessionID == sid && source.LifecycleState == "active", nil
}

func (f *completeTurnReprocessingStore) InvalidateSourceRevisions(_ context.Context, sid string, fromTurn int, lifecycleState, reason string, invalidatedAt time.Time) error {
	for _, source := range f.sources {
		if source.ChatSessionID == sid && source.TurnIndex >= fromTurn {
			source.LifecycleState = lifecycleState
			source.InvalidationReason = reason
			source.UpdatedAt = invalidatedAt
		}
	}
	return nil
}

func (f *completeTurnReprocessingStore) EnqueueMemoryReprocessingJob(_ context.Context, job *store.MemoryReprocessingJob) (bool, error) {
	if f.jobs == nil {
		f.jobs = map[string]*store.MemoryReprocessingJob{}
	}
	if _, exists := f.jobs[job.IdempotencyKey]; exists {
		return false, nil
	}
	copy := *job
	f.jobs[job.IdempotencyKey] = &copy
	return true, nil
}

func (f *completeTurnReprocessingStore) ClaimMemoryReprocessingJob(context.Context, string, time.Time, time.Duration) (*store.MemoryReprocessingJob, error) {
	return nil, store.ErrNotFound
}

func (f *completeTurnReprocessingStore) CompleteMemoryReprocessingJob(context.Context, int64, string, time.Time) error {
	return store.ErrNotEnabled
}

func (f *completeTurnReprocessingStore) FailMemoryReprocessingJob(context.Context, int64, string, time.Time, time.Time, bool, string) error {
	return store.ErrNotEnabled
}

func TestCompleteTurnMemorySourceRevisionUsesOnlyHostObservation(t *testing.T) {
	now := time.Date(2026, 7, 28, 6, 0, 0, 0, time.UTC)
	decision := completeTurnSourceAcceptanceDecision{
		Enabled: true, Accepted: true, Revision: "sar_observed",
		LogicalTurnID: "lt_observed",
		Observation: completeTurnSourceObservation{
			ObservedAtMS: 1234, HostChatID: "chat", MessageIndex: 7,
			GenerationID: "generation", UserObservedContentHash: "user_hash",
			ObservedContentHash: "assistant_hash", HashAlgorithm: "djb2.v1",
		},
	}
	source, err := completeTurnMemorySourceRevision(decision, "session", 4, "raw user", "raw assistant", now)
	if err != nil {
		t.Fatal(err)
	}
	if source.BranchID != "" || source.BranchState != "not_exposed" {
		t.Fatalf("branch id/state = %q/%q", source.BranchID, source.BranchState)
	}
	if source.SourceMessageID != "chat:index:7" ||
		source.SourceGenerationID != "generation" ||
		source.UserContent != "raw user" ||
		source.AssistantContent != "raw assistant" {
		t.Fatalf("source=%+v", source)
	}
}

func TestCompleteTurnMemorySourceRevisionRejectsUnexposedLogicalTurn(t *testing.T) {
	_, err := completeTurnMemorySourceRevision(
		completeTurnSourceAcceptanceDecision{
			Enabled: true, Accepted: true, Revision: "sar_observed",
			Observation: completeTurnSourceObservation{ObservedAtMS: 1},
		},
		"session", 1, "user", "assistant", time.Now().UTC(),
	)
	if err == nil || err.Error() != "source_revision_logical_turn_not_exposed" {
		t.Fatalf("error=%v", err)
	}
}

func TestCompleteTurnCriticFailureEnqueuesDurableRevisionJob(t *testing.T) {
	base := &turnRecordingStore{}
	recording := &completeTurnReprocessingStore{turnRecordingStore: base}
	cfg := config.Default()
	cfg.StoreMode = config.StoreModeMariaDBAuthority
	srv := NewServer(cfg)
	srv.Store = recording
	srv.StoreOpenError = nil

	oldClient := proxyHTTPClient
	proxyHTTPClient = &http.Client{Transport: roundTripFunc(func(*http.Request) (*http.Response, error) {
		return &http.Response{
			StatusCode: http.StatusBadGateway,
			Header:     make(http.Header),
			Body:       io.NopCloser(strings.NewReader(`{"error":{"message":"provider unavailable"}}`)),
		}, nil
	})}
	defer func() { proxyHTTPClient = oldClient }()

	reqBody := completeTurnAnchoredAcceptanceTestRequest(
		"session-reprocess", 1, "user source", "assistant source",
		1000, "generation-1", "not_streaming", 0, 1, 2,
	)
	reqBody.ClientMeta["critic"] = map[string]any{
		"api_key": "test-key", "endpoint": "https://api.example.com/v1",
		"model": "critic", "provider": "openai",
	}
	raw, _ := json.Marshal(reqBody)
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/complete-turn", bytes.NewReader(raw))
	request.Header.Set("Content-Type", "application/json")
	mux := http.NewServeMux()
	srv.RegisterRoutes(mux)
	mux.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", recorder.Code, recorder.Body.String())
	}
	if len(recording.sources) != 1 || len(recording.jobs) != 1 {
		t.Fatalf("sources=%d jobs=%d, want one durable source and one retry job", len(recording.sources), len(recording.jobs))
	}
	for _, job := range recording.jobs {
		if job.SourceRevision == "" || job.LastError == "" ||
			job.SourceContract != completeTurnSourceAcceptanceContract ||
			job.Status != "pending" {
			t.Fatalf("job=%+v", job)
		}
		firstKey := job.IdempotencyKey
		inserted, err := srv.enqueueCompleteTurnReprocessingJob(
			context.Background(),
			completeTurnSourceAcceptanceDecision{
				Enabled: true, Accepted: true, Revision: job.SourceRevision,
			},
			job.ChatSessionID,
			job.LastError,
			job.CreatedAt,
		)
		if err != nil || inserted || len(recording.jobs) != 1 {
			t.Fatalf("idempotent enqueue inserted=%v err=%v jobs=%d", inserted, err, len(recording.jobs))
		}
		if _, ok := recording.jobs[firstKey]; !ok {
			t.Fatalf("idempotency key changed: %q", firstKey)
		}
	}
}
