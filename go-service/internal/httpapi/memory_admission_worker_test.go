package httpapi

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/risulongmemory/archive-center-go/internal/config"
	"github.com/risulongmemory/archive-center-go/internal/store"
	"github.com/risulongmemory/archive-center-go/internal/vector"
)

type memoryAdmissionWorkerStore struct {
	store.Store
	source          *store.MemorySourceRevision
	job             *store.MemoryReprocessingJob
	admissions      []*store.MemoryAdmission
	completedJobs   []int64
	failedJobs      []int64
	failedRetryAt   []time.Time
	failedPermanent []bool
	failedReasons   []string
	auditLogs       []*store.AuditLog
	enqueuedJobs    []*store.MemoryReprocessingJob
	logs            []store.ChatLog
	memories        []store.Memory
	legacyMemories  int
	legacyEvidence  int
	nextEvidenceID  int64
}

type memoryWorkerEventStore struct {
	*memoryAdmissionWorkerStore
	mu               sync.Mutex
	vectorItems      []*store.MemoryVectorOutboxItem
	vectorClaimed    map[int64]*store.MemoryVectorOutboxItem
	vectorClaims     int
	vectorCompleted  []int64
	vectorFailed     []int64
	claimObserved    chan struct{}
	completeObserved chan int64
	failObserved     chan int64
}

func (f *memoryWorkerEventStore) EnqueueMemoryVectorOperation(_ context.Context, item *store.MemoryVectorOutboxItem) (bool, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.vectorItems = append(f.vectorItems, item)
	return true, nil
}

func (f *memoryWorkerEventStore) ClaimMemoryVectorOperation(
	_ context.Context,
	owner string,
	now time.Time,
	lease time.Duration,
) (*store.MemoryVectorOutboxItem, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.vectorClaims++
	if f.claimObserved != nil {
		select {
		case f.claimObserved <- struct{}{}:
		default:
		}
	}
	if len(f.vectorItems) == 0 {
		return nil, store.ErrNotFound
	}
	item := f.vectorItems[0]
	f.vectorItems = f.vectorItems[1:]
	item.Attempts++
	item.Status = "leased"
	item.LeaseOwner = owner
	item.LeaseUntil = now.Add(lease)
	if f.vectorClaimed == nil {
		f.vectorClaimed = map[int64]*store.MemoryVectorOutboxItem{}
	}
	f.vectorClaimed[item.ID] = item
	copy := *item
	return &copy, nil
}

func (f *memoryWorkerEventStore) CompleteMemoryVectorOperation(
	_ context.Context,
	id int64,
	_ string,
	_ time.Time,
) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.vectorCompleted = append(f.vectorCompleted, id)
	delete(f.vectorClaimed, id)
	if f.completeObserved != nil {
		select {
		case f.completeObserved <- id:
		default:
		}
	}
	return nil
}

func (f *memoryWorkerEventStore) FailMemoryVectorOperation(
	_ context.Context,
	id int64,
	_ string,
	_ time.Time,
	retryAfter time.Time,
	permanent bool,
	failure string,
) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.vectorFailed = append(f.vectorFailed, id)
	item := f.vectorClaimed[id]
	delete(f.vectorClaimed, id)
	if item != nil && !permanent {
		item.Status = "retryable"
		item.RetryAfter = retryAfter
		item.LastError = failure
		f.vectorItems = append(f.vectorItems, item)
	}
	if f.failObserved != nil {
		select {
		case f.failObserved <- id:
		default:
		}
	}
	return nil
}

func (f *memoryWorkerEventStore) addVectorItems(items ...*store.MemoryVectorOutboxItem) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.vectorItems = append(f.vectorItems, items...)
}

func (f *memoryWorkerEventStore) vectorState() (claims int, completed, failed []int64, attempts []int) {
	f.mu.Lock()
	defer f.mu.Unlock()
	claims = f.vectorClaims
	completed = append([]int64(nil), f.vectorCompleted...)
	failed = append([]int64(nil), f.vectorFailed...)
	for _, item := range f.vectorItems {
		attempts = append(attempts, item.Attempts)
	}
	return
}

func (f *memoryAdmissionWorkerStore) MemoryDerivationLifecycleEnabled() bool {
	return true
}

func (f *memoryAdmissionWorkerStore) MemoryAdmissionWritesEnabled() bool {
	return true
}

func (f *memoryAdmissionWorkerStore) CommitMemoryAdmission(_ context.Context, item *store.MemoryAdmission) (store.MemoryAdmissionResult, error) {
	f.admissions = append(f.admissions, item)
	if item.Memory != nil {
		item.Memory.ID = 10
	}
	for _, evidence := range item.Evidence {
		if evidence == nil {
			continue
		}
		f.nextEvidenceID++
		evidence.ID = f.nextEvidenceID
	}
	return store.MemoryAdmissionResult{
		MemoryInserted:      item.Memory != nil,
		EvidenceInserted:    len(item.Evidence),
		PreciseInserted:     len(item.PreciseUnits),
		VectorOperations:    len(item.Vectors),
		CommittedResultHash: item.ResultHash,
	}, nil
}

func (f *memoryAdmissionWorkerStore) SaveMemory(context.Context, *store.Memory) error {
	f.legacyMemories++
	return nil
}

func (f *memoryAdmissionWorkerStore) SaveEvidence(context.Context, *store.DirectEvidence) error {
	f.legacyEvidence++
	return nil
}

func (f *memoryAdmissionWorkerStore) ListChatLogs(context.Context, string, int, int) ([]store.ChatLog, error) {
	return append([]store.ChatLog(nil), f.logs...), nil
}

func (f *memoryAdmissionWorkerStore) ListMemories(context.Context, string, int, int) ([]store.Memory, error) {
	return append([]store.Memory(nil), f.memories...), nil
}

func (f *memoryAdmissionWorkerStore) RegisterAcceptedSourceRevision(context.Context, *store.MemorySourceRevision) (store.SourceRevisionRegistration, error) {
	return store.SourceRevisionRegistration{}, nil
}

func (f *memoryAdmissionWorkerStore) GetSourceRevision(context.Context, string, string) (*store.MemorySourceRevision, error) {
	if f.source == nil {
		return nil, store.ErrNotFound
	}
	copy := *f.source
	return &copy, nil
}

func (f *memoryAdmissionWorkerStore) IsSourceRevisionActive(context.Context, string, string) (bool, error) {
	return f.source != nil && f.source.LifecycleState == "active", nil
}

func (f *memoryAdmissionWorkerStore) InvalidateSourceRevisions(context.Context, string, int, string, string, time.Time) error {
	return nil
}

func (f *memoryAdmissionWorkerStore) ListActiveSourceRevisions(context.Context, string, int, int) ([]store.MemorySourceRevision, error) {
	if f.source == nil {
		return nil, nil
	}
	return []store.MemorySourceRevision{*f.source}, nil
}

func (f *memoryAdmissionWorkerStore) EnqueueMemoryReprocessingJob(_ context.Context, item *store.MemoryReprocessingJob) (bool, error) {
	f.enqueuedJobs = append(f.enqueuedJobs, item)
	return true, nil
}

func (f *memoryAdmissionWorkerStore) ClaimMemoryReprocessingJob(_ context.Context, owner string, now time.Time, lease time.Duration) (*store.MemoryReprocessingJob, error) {
	if f.job == nil {
		return nil, store.ErrNotFound
	}
	item := *f.job
	f.job = nil
	item.Attempts++
	item.LeaseOwner = owner
	item.LeaseUntil = now.Add(lease)
	return &item, nil
}

func (f *memoryAdmissionWorkerStore) CompleteMemoryReprocessingJob(_ context.Context, id int64, _ string, _ time.Time) error {
	f.completedJobs = append(f.completedJobs, id)
	return nil
}

func (f *memoryAdmissionWorkerStore) FailMemoryReprocessingJob(_ context.Context, id int64, _ string, _ time.Time, retryAt time.Time, permanent bool, failure string) error {
	f.failedJobs = append(f.failedJobs, id)
	f.failedRetryAt = append(f.failedRetryAt, retryAt)
	f.failedPermanent = append(f.failedPermanent, permanent)
	f.failedReasons = append(f.failedReasons, failure)
	return nil
}

func (f *memoryAdmissionWorkerStore) SaveAuditLog(_ context.Context, item *store.AuditLog) error {
	if item == nil {
		return nil
	}
	copy := *item
	f.auditLogs = append(f.auditLogs, &copy)
	return nil
}

func TestAcceptedSourceUsesCommonAdmissionWriterWithoutLegacyParallelWrites(t *testing.T) {
	st := &memoryAdmissionWorkerStore{
		Store:          store.NewNoopStore(),
		nextEvidenceID: 100,
	}
	srv := &Server{Cfg: config.Default(), Store: st, Vector: vector.NewFakeVectorStore()}
	source := &store.MemorySourceRevision{
		SourceRevision: "revision", LogicalTurnID: "turn:3",
		SourceMessageID: "message:3", SourceGenerationID: "generation:3",
		CombinedContentHash: strings.Repeat("a", 64),
	}
	ctx := contextWithStoredMemorySource(context.Background(), source)
	content := "Mina looked under the desk.\nMina found the brass key."
	extraction := map[string]any{
		"turn_summary":      "Mina found the brass key.",
		"importance_score":  7,
		"evidence_excerpts": []any{"Mina found the brass key."},
	}
	result := srv.saveCriticExtractionArtifacts(
		ctx, "session", 3, extraction, content,
		completeTurnEmbeddingConfig{}, time.Unix(300, 0).UTC(),
	)
	if result.Errors != 0 || result.Memories != 1 || result.Evidence != 1 {
		t.Fatalf("result=%+v", result)
	}
	if len(st.admissions) != 1 || st.legacyMemories != 0 || st.legacyEvidence != 0 {
		t.Fatalf("admissions=%d legacy_memory=%d legacy_evidence=%d",
			len(st.admissions), st.legacyMemories, st.legacyEvidence)
	}
	admission := st.admissions[0]
	if admission.SourceRevision != "revision" ||
		admission.ContractVersion != store.MemoryAdmissionContract ||
		admission.Memory == nil || len(admission.Evidence) != 1 {
		t.Fatalf("admission=%+v", admission)
	}
	for _, unit := range admission.PreciseUnits {
		if unit.DerivationVersion != store.MemoryAdmissionContract ||
			unit.ExtractorVersion != completeTurnCriticPipelineVersion ||
			unit.IndexVersion != memoryAdmissionIndexVersion {
			t.Fatalf("precise unit versions=%+v", unit)
		}
	}
}

func TestLifecycleAuthorityRejectsSourceLessLegacyParallelWriter(t *testing.T) {
	st := &memoryAdmissionWorkerStore{
		Store:          store.NewNoopStore(),
		nextEvidenceID: 100,
	}
	srv := &Server{Cfg: config.Default(), Store: st, Vector: vector.NewFakeVectorStore()}
	result := srv.saveCriticExtractionArtifacts(
		context.Background(), "session", 3,
		map[string]any{
			"turn_summary":      "Mina found the brass key.",
			"importance_score":  7,
			"evidence_excerpts": []any{"Mina found the brass key."},
		},
		"Mina found the brass key.",
		completeTurnEmbeddingConfig{},
		time.Unix(300, 0).UTC(),
	)
	if result.Errors != 1 || len(st.admissions) != 0 ||
		st.legacyMemories != 0 || st.legacyEvidence != 0 {
		t.Fatalf("result=%+v admissions=%d legacy_memory=%d legacy_evidence=%d",
			result, len(st.admissions), st.legacyMemories, st.legacyEvidence)
	}
}

func TestCommittedAdmissionResultIsReusedBeforeSecondaryProjectionBuild(t *testing.T) {
	first := map[string]any{
		"turn_summary":      "Mina found the first key.",
		"importance_score":  7,
		"evidence_excerpts": []any{"Mina found the first key."},
	}
	firstJSON := mustCompactJSON(normalizePreciseMemoryValue(first))
	st := &memoryAdmissionWorkerStore{
		Store:          store.NewNoopStore(),
		nextEvidenceID: 100,
		source: &store.MemorySourceRevision{
			SourceRevision:          "revision",
			ChatSessionID:           "session",
			LogicalTurnID:           "turn:3",
			TurnIndex:               3,
			CombinedContentHash:     strings.Repeat("a", 64),
			LifecycleState:          "active",
			DerivedAdmissionState:   "committed",
			DerivedAdmissionVersion: store.MemoryAdmissionContract,
			DerivedExtractorVersion: completeTurnCriticPipelineVersion,
			DerivedIndexVersion:     memoryAdmissionIndexVersion,
			DerivedResultHash:       strings.Repeat("b", 64),
			DerivedResultJSON:       firstJSON,
		},
	}
	srv := &Server{Cfg: config.Default(), Store: st, Vector: vector.NewFakeVectorStore()}
	ctx := contextWithStoredMemorySource(context.Background(), st.source)
	result := srv.saveCriticExtractionArtifacts(
		ctx, "session", 3,
		map[string]any{
			"turn_summary":      "Mina found the second key.",
			"importance_score":  4,
			"evidence_excerpts": []any{"Mina found the second key."},
		},
		"Mina found the first key. Mina found the second key.",
		completeTurnEmbeddingConfig{},
		time.Unix(300, 0).UTC(),
	)
	if result.Errors != 0 || len(st.admissions) != 1 {
		t.Fatalf("result=%+v admissions=%d", result, len(st.admissions))
	}
	admission := st.admissions[0]
	if admission.ResultJSON != firstJSON ||
		admission.Memory == nil ||
		!strings.Contains(admission.Memory.SummaryJSON, "first key") ||
		strings.Contains(admission.Memory.SummaryJSON, "second key") {
		t.Fatalf("admission=%+v", admission)
	}
	if !stringSliceContains(result.Warnings, "memory_admission_committed_result_reused") {
		t.Fatalf("warnings=%v", result.Warnings)
	}
}

func TestAdminRescanHandsAcceptedSourceToDurableWorker(t *testing.T) {
	now := time.Now().UTC()
	st := newMemoryReprocessingWorkerStore(now)
	st.logs = []store.ChatLog{
		{ChatSessionID: "session", TurnIndex: 4, Role: "user", Content: st.source.UserContent},
		{ChatSessionID: "session", TurnIndex: 4, Role: "assistant", Content: st.source.AssistantContent},
	}
	srv := &Server{Cfg: config.Default(), Store: st, Vector: vector.NewFakeVectorStore()}
	response, err := srv.runAdminRescan(
		context.Background(), "session",
		adminRescanRequest{ChatSessionID: "session", TurnIndices: []int{4}},
	)
	if err != nil {
		t.Fatal(err)
	}
	if intFromAny(response["queued"], 0) != 1 ||
		len(st.enqueuedJobs) != 1 || len(st.admissions) != 0 ||
		st.legacyMemories != 0 || st.legacyEvidence != 0 {
		t.Fatalf("response=%+v jobs=%d admissions=%d legacy_memory=%d legacy_evidence=%d",
			response, len(st.enqueuedJobs), len(st.admissions),
			st.legacyMemories, st.legacyEvidence)
	}
	if st.enqueuedJobs[0].SourceRevision != st.source.SourceRevision ||
		st.enqueuedJobs[0].DerivationVersion != store.MemoryAdmissionContract {
		t.Fatalf("job=%+v", st.enqueuedJobs[0])
	}
	select {
	case <-srv.memoryWorkerWakeChannel():
	default:
		t.Fatal("reprocessing enqueue did not signal the memory worker")
	}
}

func TestExplorerRegenerationHandsAcceptedSourceToDurableWorker(t *testing.T) {
	now := time.Now().UTC()
	st := newMemoryReprocessingWorkerStore(now)
	st.job = nil
	st.logs = []store.ChatLog{
		{ChatSessionID: "session", TurnIndex: 4, Role: "user", Content: st.source.UserContent},
		{ChatSessionID: "session", TurnIndex: 4, Role: "assistant", Content: st.source.AssistantContent},
	}
	cfg := config.Default()
	cfg.StoreMode = config.StoreModeMariaDBAuthority
	srv := &Server{Cfg: cfg, Store: st, Vector: vector.NewFakeVectorStore()}
	req := httptest.NewRequest(
		http.MethodPost,
		"/explorer/memories/regenerate",
		strings.NewReader(`{"chat_session_id":"session","turn_index":4}`),
	)
	rec := httptest.NewRecorder()
	srv.handleRegenerateMemory(rec, req)
	var response map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &response); err != nil {
		t.Fatal(err)
	}
	if rec.Code != http.StatusOK ||
		response["code"] != "reprocessing_queued" ||
		len(st.enqueuedJobs) != 1 ||
		len(st.admissions) != 0 {
		t.Fatalf("status=%d response=%+v jobs=%d admissions=%d",
			rec.Code, response, len(st.enqueuedJobs), len(st.admissions))
	}
}

func TestHypaImportCannotBypassAcceptedSourceAdmission(t *testing.T) {
	st := &memoryAdmissionWorkerStore{Store: store.NewNoopStore()}
	cfg := config.Default()
	cfg.StoreMode = config.StoreModeMariaDBAuthority
	srv := &Server{Cfg: cfg, Store: st, Vector: vector.NewFakeVectorStore()}
	req := httptest.NewRequest(
		http.MethodPost,
		"/import/hypamemory",
		strings.NewReader(`{"chat_session_id":"session","summaries":[{"text":"Imported summary.","is_important":true}]}`),
	)
	rec := httptest.NewRecorder()
	srv.handleImportHypamemory(rec, req)
	var response map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &response); err != nil {
		t.Fatal(err)
	}
	if rec.Code != http.StatusConflict ||
		response["code"] != "external_import_source_admission_required" ||
		len(st.admissions) != 0 ||
		st.legacyMemories != 0 ||
		st.legacyEvidence != 0 {
		t.Fatalf("status=%d response=%+v admissions=%d legacy=%d/%d",
			rec.Code, response, len(st.admissions),
			st.legacyMemories, st.legacyEvidence)
	}
}

func TestMemoryReprocessingWorkerTerminatesWhenRetryLimitMissingOrInvalid(t *testing.T) {
	for _, maxAttempts := range []int{0, -1, 12} {
		t.Run(fmt.Sprintf("max_%d", maxAttempts), func(t *testing.T) {
			now := time.Now().UTC()
			st := newMemoryReprocessingWorkerStore(now)
			srv := &Server{
				Cfg: config.Default(), Store: st, Vector: vector.NewFakeVectorStore(),
				RuntimeConfig: RuntimeConfig{
					Synced: true, FailedQueueMaxAttempts: maxAttempts,
				},
			}
			result, err := srv.processMemoryReprocessingOnce(
				context.Background(), "worker", now, time.Minute,
			)
			if err != nil {
				t.Fatal(err)
			}
			if result.State != "terminal" ||
				result.Failure != criticRetryLimitUnconfigured ||
				len(st.failedJobs) != 1 ||
				len(st.failedPermanent) != 1 || !st.failedPermanent[0] ||
				len(st.completedJobs) != 0 ||
				len(st.failedRetryAt) != 1 || !st.failedRetryAt[0].IsZero() ||
				len(st.failedReasons) != 1 ||
				!strings.HasPrefix(st.failedReasons[0], criticRetryLimitUnconfigured) {
				t.Fatalf("result=%+v failed=%v permanent=%v reasons=%v completed=%v retry=%v",
					result, st.failedJobs, st.failedPermanent, st.failedReasons,
					st.completedJobs, st.failedRetryAt)
			}
		})
	}
}

func TestMemoryReprocessingWorkerRetriesBelowConfiguredLimit(t *testing.T) {
	now := time.Now().UTC()
	st := newMemoryReprocessingWorkerStore(now)
	srv := &Server{
		Cfg: config.Default(), Store: st, Vector: vector.NewFakeVectorStore(),
		RuntimeConfig: RuntimeConfig{
			Synced: true, FailedQueueMaxAttempts: 4,
		},
	}
	result, err := srv.processMemoryReprocessingOnce(
		context.Background(), "worker", now, time.Minute,
	)
	if err != nil {
		t.Fatal(err)
	}
	if result.State != "retryable" || result.Failure != "critic_config_missing" ||
		len(st.failedJobs) != 1 ||
		len(st.failedPermanent) != 1 || st.failedPermanent[0] ||
		len(st.completedJobs) != 0 || !st.failedRetryAt[0].Equal(now) {
		t.Fatalf("result=%+v failed=%v permanent=%v completed=%v retry=%v",
			result, st.failedJobs, st.failedPermanent, st.completedJobs, st.failedRetryAt)
	}
}

func TestMemoryReprocessingWorkerTerminatesAtConfiguredRetryLimit(t *testing.T) {
	now := time.Now().UTC()
	st := newMemoryReprocessingWorkerStore(now)
	st.job.Attempts = 3
	srv := &Server{
		Cfg: config.Default(), Store: st, Vector: vector.NewFakeVectorStore(),
		RuntimeConfig: RuntimeConfig{
			Synced: true, FailedQueueMaxAttempts: 4,
		},
	}
	result, err := srv.processMemoryReprocessingOnce(
		context.Background(), "worker", now, time.Minute,
	)
	if err != nil {
		t.Fatal(err)
	}
	if result.State != "terminal" ||
		result.Failure != criticRetryLimitReached ||
		len(st.failedPermanent) != 1 || !st.failedPermanent[0] ||
		len(st.failedRetryAt) != 1 || !st.failedRetryAt[0].IsZero() ||
		len(st.failedReasons) != 1 ||
		!strings.HasPrefix(st.failedReasons[0], criticRetryLimitReached) {
		t.Fatalf("result=%+v permanent=%v retry=%v reasons=%v",
			result, st.failedPermanent, st.failedRetryAt, st.failedReasons)
	}
}

func TestMemoryReprocessingWorkerDefersWithoutClaimBeforeRuntimeConfigSync(t *testing.T) {
	now := time.Now().UTC()
	st := newMemoryReprocessingWorkerStore(now)
	srv := &Server{
		Cfg: config.Default(), Store: st, Vector: vector.NewFakeVectorStore(),
	}
	result, err := srv.processMemoryReprocessingOnce(
		context.Background(), "worker", now, time.Minute,
	)
	if err != nil {
		t.Fatal(err)
	}
	if result.Processed ||
		result.State != "deferred_config_sync" ||
		result.Failure != memoryWorkerConfigDeferred ||
		st.job == nil ||
		len(st.failedJobs) != 0 ||
		len(st.completedJobs) != 0 {
		t.Fatalf(
			"result=%+v job=%+v failed=%v completed=%v",
			result, st.job, st.failedJobs, st.completedJobs,
		)
	}
}

func TestMemoryWorkerProductionLoopContainsNoPollingTicker(t *testing.T) {
	source, err := os.ReadFile("memory_reprocessing_worker.go")
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Contains(source, []byte("time.NewTicker")) ||
		bytes.Contains(source, []byte("memoryWorkerPollInterval")) {
		t.Fatal("memory worker regained a server-lifetime polling ticker")
	}
}

func TestMemoryWorkerWaitsForWakeAndDrainsAllDueItems(t *testing.T) {
	base := newMemoryReprocessingWorkerStore(time.Now().UTC())
	base.job = nil
	eventStore := &memoryWorkerEventStore{
		memoryAdmissionWorkerStore: base,
		claimObserved:              make(chan struct{}, 8),
		completeObserved:           make(chan int64, 8),
	}
	cfg := config.Default()
	cfg.StoreMode = config.StoreModeMariaDBAuthority
	srv := &Server{
		Cfg: cfg, Store: eventStore,
		Vector: &memoryVectorProcessorVector{VectorStore: vector.NewFakeVectorStore()},
		RuntimeConfig: RuntimeConfig{
			Synced: true, CriticTimeoutSec: 2, EmbeddingTimeoutSec: 3,
			FailedQueueMaxAttempts: 4,
		},
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	if !srv.StartMemoryWorkers(ctx) {
		t.Fatal("memory workers did not start")
	}
	select {
	case <-eventStore.claimObserved:
	case <-time.After(time.Second):
		t.Fatal("startup wake was not drained")
	}
	eventStore.addVectorItems(
		&store.MemoryVectorOutboxItem{
			ID: 21, Operation: "delete", ChatSessionID: "session",
			SourceRevision: "revision", DocumentID: "memory:21",
			EmbeddingReady: true, RequiredSourceState: "inactive", Status: "pending",
		},
		&store.MemoryVectorOutboxItem{
			ID: 22, Operation: "delete", ChatSessionID: "session",
			SourceRevision: "revision", DocumentID: "memory:22",
			EmbeddingReady: true, RequiredSourceState: "inactive", Status: "pending",
		},
	)
	select {
	case id := <-eventStore.completeObserved:
		t.Fatalf("vector item %d ran without a real wake", id)
	case <-time.After(25 * time.Millisecond):
	}
	srv.wakeMemoryWorkers()
	completed := []int64{}
	for len(completed) < 2 {
		select {
		case id := <-eventStore.completeObserved:
			completed = append(completed, id)
		case <-time.After(time.Second):
			t.Fatalf("single wake completed=%v, want both due items", completed)
		}
	}
	if completed[0] != 21 || completed[1] != 22 {
		t.Fatalf("completed=%v, want [21 22]", completed)
	}
}

func TestMemoryWorkerRetryWaitsForNextRealWake(t *testing.T) {
	base := newMemoryReprocessingWorkerStore(time.Now().UTC())
	base.job = nil
	item := &store.MemoryVectorOutboxItem{
		ID: 31, Operation: "delete", ChatSessionID: "session",
		SourceRevision: "revision", DocumentID: "memory:31",
		EmbeddingReady: true, RequiredSourceState: "inactive", Status: "pending",
	}
	eventStore := &memoryWorkerEventStore{
		memoryAdmissionWorkerStore: base,
		vectorItems:                []*store.MemoryVectorOutboxItem{item},
	}
	vec := &memoryVectorProcessorVector{
		VectorStore: vector.NewFakeVectorStore(),
		deleteErr:   errors.New("provider unavailable"),
	}
	srv := &Server{
		Cfg: config.Default(), Store: eventStore, Vector: vec,
		RuntimeConfig: RuntimeConfig{
			Synced: true, CriticTimeoutSec: 2, EmbeddingTimeoutSec: 3,
			FailedQueueMaxAttempts: 4,
		},
	}
	firstWake := time.Date(2026, 7, 30, 1, 0, 0, 0, time.UTC)
	srv.processMemoryWorkerWake(context.Background(), "worker", firstWake)
	claims, completed, failed, _ := eventStore.vectorState()
	if claims != 1 || len(completed) != 0 || len(failed) != 1 ||
		item.Attempts != 1 || item.Status != "retryable" ||
		!item.RetryAfter.Equal(firstWake) {
		t.Fatalf(
			"after first wake claims=%d completed=%v failed=%v item=%+v",
			claims, completed, failed, item,
		)
	}
	vec.deleteErr = nil
	secondWake := firstWake.Add(time.Second)
	srv.processMemoryWorkerWake(context.Background(), "worker", secondWake)
	claims, completed, failed, _ = eventStore.vectorState()
	if claims != 3 || len(completed) != 1 || completed[0] != item.ID ||
		len(failed) != 1 || item.Attempts != 2 {
		t.Fatalf(
			"after second wake claims=%d completed=%v failed=%v item=%+v",
			claims, completed, failed, item,
		)
	}
}

func TestRuntimeConfigSyncSignalsMemoryWorker(t *testing.T) {
	srv := &Server{}
	wake := srv.memoryWorkerWakeChannel()
	srv.updateRuntimeConfig(map[string]any{
		"criticTimeout":          2,
		"embeddingTimeout":       3,
		"failedQueueMaxAttempts": 4,
	})
	select {
	case <-wake:
	default:
		t.Fatal("runtime config sync did not signal the memory worker")
	}
}

func TestRuntimeConfigClampsFailedQueueMaxAttemptsWithoutHiddenDefault(t *testing.T) {
	srv := &Server{}
	srv.updateRuntimeConfig(map[string]any{"criticTimeout": 45})
	if srv.RuntimeConfig.FailedQueueMaxAttempts != 0 {
		t.Fatalf("missing retry setting gained hidden default %d", srv.RuntimeConfig.FailedQueueMaxAttempts)
	}
	srv.updateRuntimeConfig(map[string]any{"failedQueueMaxAttempts": 0})
	if srv.RuntimeConfig.FailedQueueMaxAttempts != 1 {
		t.Fatalf("lower clamp=%d, want 1", srv.RuntimeConfig.FailedQueueMaxAttempts)
	}
	srv.updateRuntimeConfig(map[string]any{"failedQueueMaxAttempts": 99})
	if srv.RuntimeConfig.FailedQueueMaxAttempts != 11 {
		t.Fatalf("upper clamp=%d, want 11", srv.RuntimeConfig.FailedQueueMaxAttempts)
	}
}

func TestMemoryReprocessingWorkerAuditsTypedCriticFailure(t *testing.T) {
	now := time.Now().UTC()
	st := newMemoryReprocessingWorkerStore(now)
	oldClient := proxyHTTPClient
	proxyHTTPClient = &http.Client{Transport: roundTripFunc(func(*http.Request) (*http.Response, error) {
		return &http.Response{
			StatusCode: http.StatusTooManyRequests,
			Header:     make(http.Header),
			Body: io.NopCloser(strings.NewReader(
				`{"error":{"message":"retry test-key later"}}`,
			)),
		}, nil
	})}
	defer func() { proxyHTTPClient = oldClient }()

	srv := &Server{
		Cfg: config.Default(), Store: st, Vector: vector.NewFakeVectorStore(),
		RuntimeConfig: RuntimeConfig{
			Synced: true, CriticProvider: "openai", CriticAPIKey: "test-key",
			CriticEndpoint: "https://example.invalid/v1", CriticModel: "critic-test",
			CriticTimeoutSec: 30, FailedQueueMaxAttempts: 4,
		},
	}
	result, err := srv.processMemoryReprocessingOnce(
		context.Background(), "worker", now, time.Minute,
	)
	if err != nil {
		t.Fatal(err)
	}
	if result.State != "retryable" || len(st.auditLogs) != 1 {
		t.Fatalf("result=%+v audit_count=%d", result, len(st.auditLogs))
	}
	audit := st.auditLogs[0]
	if audit.EventType != "critic_reprocessing_failed" ||
		audit.TargetType != "memory_reprocessing_job" ||
		audit.TargetID != st.failedJobs[0] {
		t.Fatalf("audit=%+v", audit)
	}
	if strings.Contains(audit.DetailsJSON, "test-key") {
		t.Fatalf("audit leaked API key: %s", audit.DetailsJSON)
	}
	var details map[string]any
	if err := json.Unmarshal([]byte(audit.DetailsJSON), &details); err != nil {
		t.Fatal(err)
	}
	failure, _ := details["failure"].(map[string]any)
	trace, _ := details["trace"].(map[string]any)
	if failure["code"] != "CRITIC_PROVIDER_HTTP_ERROR" ||
		failure["stage"] != "provider_response" ||
		failure["retryable"] != true ||
		failure["http_status"] != float64(http.StatusTooManyRequests) ||
		trace["provider"] != "openai" ||
		trace["model"] != "critic-test" ||
		trace["http_status"] != float64(http.StatusTooManyRequests) ||
		strings.TrimSpace(extractionStringFromAny(trace["raw_preview"])) == "" {
		t.Fatalf("details=%+v", details)
	}
	if details["job_id"] != float64(9) ||
		details["source_revision"] != "revision" ||
		details["turn_index"] != float64(4) ||
		details["attempt"] != float64(1) {
		t.Fatalf("job evidence=%+v", details)
	}
}

func TestMemoryReprocessingWorkerUsesSameAdmissionWriterAndCompletes(t *testing.T) {
	now := time.Now().UTC()
	st := newMemoryReprocessingWorkerStore(now)
	oldClient := proxyHTTPClient
	proxyHTTPClient = &http.Client{Transport: roundTripFunc(func(*http.Request) (*http.Response, error) {
		extraction, _ := json.Marshal(map[string]any{
			"turn_summary":      "Mina found the brass key.",
			"importance_score":  7,
			"evidence_excerpts": []any{"Mina found the brass key."},
		})
		payload, _ := json.Marshal(map[string]any{
			"model": "critic-test",
			"choices": []any{map[string]any{
				"message": map[string]any{"content": string(extraction)},
			}},
		})
		return &http.Response{
			StatusCode: http.StatusOK,
			Header:     make(http.Header),
			Body:       io.NopCloser(strings.NewReader(string(payload))),
		}, nil
	})}
	defer func() { proxyHTTPClient = oldClient }()

	srv := &Server{
		Cfg: config.Default(), Store: st, Vector: vector.NewFakeVectorStore(),
		RuntimeConfig: RuntimeConfig{
			Synced: true, CriticProvider: "openai", CriticAPIKey: "test-key",
			CriticEndpoint: "https://example.invalid/v1", CriticModel: "critic-test",
			CriticTimeoutSec: 30,
		},
	}
	result, err := srv.processMemoryReprocessingOnce(
		context.Background(), "worker", now, time.Minute,
	)
	if err != nil {
		t.Fatal(err)
	}
	if result.State != "completed" || len(st.completedJobs) != 1 ||
		len(st.failedJobs) != 0 || len(st.admissions) != 1 {
		t.Fatalf("result=%+v completed=%v failed=%v admissions=%d",
			result, st.completedJobs, st.failedJobs, len(st.admissions))
	}
	if st.legacyMemories != 0 || st.legacyEvidence != 0 {
		t.Fatalf("legacy parallel writes memory=%d evidence=%d",
			st.legacyMemories, st.legacyEvidence)
	}
}

func TestMemoryReprocessingWorkerPreservesRedactedRetryFailurePreview(t *testing.T) {
	now := time.Now().UTC()
	st := newMemoryReprocessingWorkerStore(now)
	st.source.AssistantContent = "The intimate scene involved penetration."
	oldClient := proxyHTTPClient
	callCount := 0
	proxyHTTPClient = &http.Client{Transport: roundTripFunc(func(*http.Request) (*http.Response, error) {
		callCount++
		marker := "first failure marker"
		if callCount == 2 {
			marker = "second failure marker"
		}
		return &http.Response{
			StatusCode: http.StatusTooManyRequests,
			Header:     make(http.Header),
			Body: io.NopCloser(strings.NewReader(
				`{"error":{"message":"` + marker + `"}}`,
			)),
		}, nil
	})}
	defer func() { proxyHTTPClient = oldClient }()

	srv := &Server{
		Cfg: config.Default(), Store: st, Vector: vector.NewFakeVectorStore(),
		RuntimeConfig: RuntimeConfig{
			Synced: true, CriticProvider: "openai", CriticAPIKey: "test-key",
			CriticEndpoint: "https://example.invalid/v1", CriticModel: "critic-test",
			CriticTimeoutSec: 30, LLMRetryCount: 1, FailedQueueMaxAttempts: 4,
		},
	}
	result, err := srv.processMemoryReprocessingOnce(
		context.Background(), "worker", now, time.Minute,
	)
	if err != nil {
		t.Fatal(err)
	}
	if result.State != "retryable" || callCount != 2 || len(st.auditLogs) != 1 {
		t.Fatalf("result=%+v calls=%d audits=%d", result, callCount, len(st.auditLogs))
	}
	var details map[string]any
	if err := json.Unmarshal([]byte(st.auditLogs[0].DetailsJSON), &details); err != nil {
		t.Fatal(err)
	}
	trace := mapFromAny(details["trace"])
	preview := stringFromMap(trace, "raw_preview")
	if !strings.Contains(preview, "second failure marker") {
		t.Fatalf("redacted retry failure preview was lost: %+v", details)
	}
}

func TestMemoryReprocessingWorkerDiscardsProviderResultAfterSourceInvalidation(t *testing.T) {
	now := time.Now().UTC()
	st := newMemoryReprocessingWorkerStore(now)
	requestStarted := make(chan struct{})
	oldClient := proxyHTTPClient
	proxyHTTPClient = &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		close(requestStarted)
		<-r.Context().Done()
		return nil, r.Context().Err()
	})}
	defer func() { proxyHTTPClient = oldClient }()

	srv := &Server{
		Cfg: config.Default(), Store: st, Vector: vector.NewFakeVectorStore(),
		RuntimeConfig: RuntimeConfig{
			Synced: true, CriticProvider: "openai", CriticAPIKey: "test-key",
			CriticEndpoint: "https://example.invalid/v1", CriticModel: "critic-test",
			CriticTimeoutSec: 30,
		},
	}
	type workerOutcome struct {
		result memoryReprocessingProcessResult
		err    error
	}
	done := make(chan workerOutcome, 1)
	workerCtx, cancelWorker := context.WithCancel(context.Background())
	defer cancelWorker()
	go func() {
		result, err := srv.processMemoryReprocessingOnce(
			workerCtx, "worker", now, time.Minute,
		)
		done <- workerOutcome{result: result, err: err}
	}()
	select {
	case <-requestStarted:
	case outcome := <-done:
		t.Fatalf("worker exited before provider request: result=%+v err=%v", outcome.result, outcome.err)
	case <-time.After(time.Second):
		t.Fatal("provider request did not start")
	}
	st.source.LifecycleState = "superseded"
	invalidationDone := make(chan struct{})
	go func() {
		srv.invalidateCompleteTurnSourceAcceptances(
			context.Background(), st.source.ChatSessionID, st.source.TurnIndex,
			"test_reroll", 1,
		)
		close(invalidationDone)
	}()
	select {
	case <-invalidationDone:
	case <-time.After(time.Second):
		t.Fatal("source invalidation did not cancel the provider request")
	}
	var outcome workerOutcome
	select {
	case outcome = <-done:
	case <-time.After(time.Second):
		t.Fatal("worker did not finish after source invalidation")
	}
	if outcome.err != nil {
		t.Fatal(outcome.err)
	}
	if outcome.result.State != "stale_rejected" ||
		outcome.result.Failure != "CRITIC_RESULT_SUPERSEDED" ||
		len(st.admissions) != 0 ||
		len(st.completedJobs) != 0 ||
		len(st.failedJobs) != 1 ||
		len(st.failedPermanent) != 1 || !st.failedPermanent[0] ||
		len(st.failedReasons) != 1 || st.failedReasons[0] != "CRITIC_RESULT_SUPERSEDED" ||
		len(st.auditLogs) != 0 {
		t.Fatalf(
			"result=%+v admissions=%d completed=%v failed=%v permanent=%v reasons=%v audits=%d",
			outcome.result, len(st.admissions), st.completedJobs, st.failedJobs,
			st.failedPermanent, st.failedReasons, len(st.auditLogs),
		)
	}
}

func newMemoryReprocessingWorkerStore(now time.Time) *memoryAdmissionWorkerStore {
	return &memoryAdmissionWorkerStore{
		Store:          store.NewNoopStore(),
		nextEvidenceID: 200,
		source: &store.MemorySourceRevision{
			SourceRevision: "revision", ChatSessionID: "session",
			LogicalTurnID: "turn:4", TurnIndex: 4,
			SourceMessageID: "message:4", SourceGenerationID: "generation:4",
			UserContent:         "Mina looked under the desk.",
			AssistantContent:    "Mina found the brass key.",
			CombinedContentHash: strings.Repeat("b", 64),
			LifecycleState:      "active",
		},
		job: &store.MemoryReprocessingJob{
			ID: 9, ChatSessionID: "session", SourceRevision: "revision",
			DerivationVersion: store.MemoryAdmissionContract,
			ExtractorVersion:  completeTurnCriticPipelineVersion,
			IndexVersion:      memoryAdmissionIndexVersion,
			Attempts:          0, CreatedAt: now,
		},
	}
}
