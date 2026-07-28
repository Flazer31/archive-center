package httpapi

import (
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
	"github.com/risulongmemory/archive-center-go/internal/vector"
)

type memoryAdmissionWorkerStore struct {
	store.Store
	source         *store.MemorySourceRevision
	job            *store.MemoryReprocessingJob
	admissions     []*store.MemoryAdmission
	completedJobs  []int64
	failedJobs     []int64
	failedRetryAt  []time.Time
	enqueuedJobs   []*store.MemoryReprocessingJob
	logs           []store.ChatLog
	memories       []store.Memory
	legacyMemories int
	legacyEvidence int
	nextEvidenceID int64
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

func (f *memoryAdmissionWorkerStore) FailMemoryReprocessingJob(_ context.Context, id int64, _ string, _ time.Time, retryAt time.Time, _ bool, _ string) error {
	f.failedJobs = append(f.failedJobs, id)
	f.failedRetryAt = append(f.failedRetryAt, retryAt)
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

func TestMemoryReprocessingWorkerRetriesWithoutRuntimeCriticConfig(t *testing.T) {
	now := time.Now().UTC()
	st := newMemoryReprocessingWorkerStore(now)
	srv := &Server{Cfg: config.Default(), Store: st, Vector: vector.NewFakeVectorStore()}
	result, err := srv.processMemoryReprocessingOnce(
		context.Background(), "worker", now, time.Minute,
	)
	if err != nil {
		t.Fatal(err)
	}
	if result.State != "retryable" || len(st.failedJobs) != 1 ||
		len(st.completedJobs) != 0 || !st.failedRetryAt[0].After(now) {
		t.Fatalf("result=%+v failed=%v completed=%v retry=%v",
			result, st.failedJobs, st.completedJobs, st.failedRetryAt)
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
