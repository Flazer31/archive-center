package httpapi

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
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
	sources            map[string]*store.MemorySourceRevision
	jobs               map[string]*store.MemoryReprocessingJob
	enqueueErr         error
	failEffectiveInput bool
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
	if f.enqueueErr != nil {
		return false, f.enqueueErr
	}
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

func (f *completeTurnReprocessingStore) SaveEffectiveInput(ctx context.Context, input *store.EffectiveInput) error {
	if f.failEffectiveInput {
		return errors.New("effective input store unavailable")
	}
	return f.turnRecordingStore.SaveEffectiveInput(ctx, input)
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
			StatusCode: http.StatusOK,
			Header:     make(http.Header),
			Body:       io.NopCloser(strings.NewReader(`{"choices":[{"message":{"content":"{\"turn_summary\":\"broken\",]"}}],"model":"critic"}`)),
		}, nil
	})}
	defer func() { proxyHTTPClient = oldClient }()

	reqBody := completeTurnAnchoredAcceptanceTestRequest(
		"session-reprocess", 1, "user source", "assistant source",
		1000, "generation-1", "not_streaming", 0, 1, 2,
	)
	reqBody.ClientMeta["turn_workflow_request_id"] = "critic-hud-recovery"
	srv.TurnWorkflows.begin("critic-hud-recovery", "session-reprocess", 1)
	reqBody.ClientMeta["critic"] = map[string]any{
		"api_key": "test-key", "endpoint": "https://api.example.com/v1",
		"model": "critic", "provider": "openai", "timeout_ms": 45000,
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
	var response map[string]any
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatalf("decode complete-turn response: %v", err)
	}
	criticFailure := mapFromAny(response["critic_failure"])
	if stringFromMap(criticFailure, "code") != "CRITIC_JSON_PARSE_FAILED" {
		t.Fatalf("critic failure=%#v", criticFailure)
	}
	hud := mapFromAny(response["turn_workflow_hud"])
	hudError := mapFromAny(hud["error"])
	recoveryActions := sliceFromAny(hudError["recovery_actions"])
	if len(recoveryActions) != 1 || stringFromMap(mapFromAny(recoveryActions[0]), "id") != turnWorkflowHUDRecoveryRetryDerivedTurn {
		t.Fatalf("critic recovery actions=%#v hud=%#v", recoveryActions, hud)
	}
	detailValues := map[string]string{}
	for _, item := range sliceFromAny(hudError["details"]) {
		detail := mapFromAny(item)
		detailValues[stringFromMap(detail, "key")] = stringFromMap(detail, "value")
	}
	if detailValues["pipeline_stage"] != "json_parse" ||
		detailValues["provider"] != "openai" ||
		detailValues["model"] != "critic" ||
		detailValues["reprocessing"] != "queued" ||
		!strings.Contains(detailValues["cause"], "critic_json_mismatched_brackets") ||
		detailValues["raw_preview"] == "" {
		t.Fatalf("critic failure details=%#v", detailValues)
	}
	for _, job := range recording.jobs {
		if job.SourceRevision == "" || !strings.Contains(job.LastError, "CRITIC_JSON_PARSE_FAILED") ||
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

func TestCompleteTurnSchemaInvalidJobWaitsForManualRecovery(t *testing.T) {
	recording := &completeTurnReprocessingStore{turnRecordingStore: &turnRecordingStore{}}
	cfg := config.Default()
	cfg.StoreMode = config.StoreModeMariaDBAuthority
	srv := NewServer(cfg)
	srv.Store = recording
	srv.StoreOpenError = nil

	oldClient := proxyHTTPClient
	providerCalls := 0
	proxyHTTPClient = &http.Client{Transport: roundTripFunc(func(*http.Request) (*http.Response, error) {
		providerCalls++
		return &http.Response{
			StatusCode: http.StatusOK,
			Header:     make(http.Header),
			Body: io.NopCloser(strings.NewReader(
				`{"choices":[{"message":{"content":"{\"turn_summary\":\"broken schema\",\"importance_score\":5,\"evidence_excerpts\":[{\"quote\":\"not a string\"}]}"}}],"model":"critic"}`,
			)),
		}, nil
	})}
	defer func() { proxyHTTPClient = oldClient }()

	reqBody := completeTurnAnchoredAcceptanceTestRequest(
		"session-schema-invalid", 1, "user source", "assistant source",
		1000, "generation-1", "not_streaming", 0, 1, 2,
	)
	reqBody.ClientMeta["critic"] = map[string]any{
		"api_key": "test-key", "endpoint": "https://api.example.com/v1",
		"model": "critic", "provider": "openai", "timeout_ms": 45000,
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
	if providerCalls != 1 || len(recording.jobs) != 1 {
		t.Fatalf("provider calls=%d jobs=%d", providerCalls, len(recording.jobs))
	}
	var response map[string]any
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatal(err)
	}
	if stringFromMap(mapFromAny(response["critic_failure"]), "code") != "CRITIC_SCHEMA_INVALID" {
		t.Fatalf("critic failure=%#v", response["critic_failure"])
	}
	for _, job := range recording.jobs {
		if job.Status != "permanent" || !strings.Contains(job.LastError, "CRITIC_SCHEMA_INVALID") {
			t.Fatalf("schema-invalid job must remain durable without automatic retry: %+v", job)
		}
	}
}

func TestCompleteTurnSuccessfulCriticDerivedWriteFailureEnqueuesDurableRevisionJob(t *testing.T) {
	recording := &completeTurnReprocessingStore{
		turnRecordingStore: &turnRecordingStore{},
		failEffectiveInput: true,
	}
	response := runCompleteTurnDerivedFailureReprocessingTest(t, recording)
	for field, want := range map[string]any{
		"critic_triggered":        true,
		"derived_retry_required":  true,
		"reconciliation_required": true,
		"queue_action":            "discard",
		"retryable":               false,
	} {
		if got := response[field]; got != want {
			t.Fatalf("%s=%v, want %v; response=%#v", field, got, want, response)
		}
	}
	if len(recording.jobs) != 1 {
		t.Fatalf("durable reprocessing jobs=%d, want 1", len(recording.jobs))
	}
	queue, _ := response["memory_reprocessing_queue"].(map[string]any)
	if queue["durable_or_existing"] != true || queue["reason_code"] != "derived_persist_failed" {
		t.Fatalf("memory_reprocessing_queue=%#v", queue)
	}
	pipeline := mapFromAny(response["persistence_pipeline"])
	derived := mapFromAny(pipeline["derived"])
	if _, ok := derived["attempted"]; !ok {
		t.Fatalf("derived attempted count missing: %#v", derived)
	}
	if intFromAny(derived["committed"], -1) != 0 ||
		stringFromMap(derived, "rollback_state") != "atomic_rollback" ||
		len(sliceFromAny(derived["error_diagnostics"])) == 0 {
		t.Fatalf("derived diagnostics=%#v", derived)
	}
	for _, job := range recording.jobs {
		if !strings.Contains(job.LastError, "operation=CommitMemoryAdmission") ||
			!strings.Contains(job.LastError, "cause=common writer is unavailable") {
			t.Fatalf("job detail=%q", job.LastError)
		}
	}
}

func TestCompleteTurnDerivedRecoveryEnqueueFailureDoesNotClaimBackendOwnership(t *testing.T) {
	recording := &completeTurnReprocessingStore{
		turnRecordingStore: &turnRecordingStore{},
		failEffectiveInput: true,
		enqueueErr:         errors.New("reprocessing queue unavailable"),
	}
	response := runCompleteTurnDerivedFailureReprocessingTest(t, recording)
	for field, want := range map[string]any{
		"critic_triggered":        true,
		"derived_retry_required":  false,
		"reconciliation_required": true,
		"queue_action":            "retry",
		"retryable":               true,
	} {
		if got := response[field]; got != want {
			t.Fatalf("%s=%v, want %v; response=%#v", field, got, want, response)
		}
	}
	if len(recording.jobs) != 0 {
		t.Fatalf("non-durable reprocessing jobs=%d, want 0", len(recording.jobs))
	}
	queue, _ := response["memory_reprocessing_queue"].(map[string]any)
	if queue["durable_or_existing"] != false || queue["reason_code"] != "derived_persist_failed" {
		t.Fatalf("memory_reprocessing_queue=%#v", queue)
	}
}

func runCompleteTurnDerivedFailureReprocessingTest(
	t *testing.T,
	recording *completeTurnReprocessingStore,
) map[string]any {
	t.Helper()
	cfg := config.Default()
	cfg.StoreMode = config.StoreModeMariaDBAuthority
	srv := NewServer(cfg)
	srv.Store = recording
	srv.StoreOpenError = nil

	oldClient := proxyHTTPClient
	proxyHTTPClient = &http.Client{Transport: roundTripFunc(func(*http.Request) (*http.Response, error) {
		return &http.Response{
			StatusCode: http.StatusOK,
			Header:     make(http.Header),
			Body: io.NopCloser(strings.NewReader(
				`{"choices":[{"message":{"content":"{\"turn_summary\":\"accepted final summary\",\"importance_score\":1}"}}]}`,
			)),
		}, nil
	})}
	defer func() { proxyHTTPClient = oldClient }()

	reqBody := completeTurnAnchoredAcceptanceTestRequest(
		"session-derived-reprocess", 1, "user source", "assistant source",
		1000, "generation-1", "not_streaming", 0, 1, 2,
	)
	reqBody.ClientMeta["critic"] = map[string]any{
		"api_key": "test-key", "endpoint": "https://api.example.com/v1",
		"model": "critic", "provider": "openai", "timeout_ms": 45000,
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
	var response map[string]any
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	return response
}
