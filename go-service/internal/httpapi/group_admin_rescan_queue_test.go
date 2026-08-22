package httpapi

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/risulongmemory/archive-center-go/internal/config"
	"github.com/risulongmemory/archive-center-go/internal/store"
	"github.com/risulongmemory/archive-center-go/internal/vector"
)

type adminDuplicateReprocessingStore struct {
	store.Store
	source       store.MemorySourceRevision
	logs         []store.ChatLog
	memories     []store.Memory
	enqueuedJobs []*store.MemoryReprocessingJob
	auditLogs    []store.AuditLog
	enqueueNew   bool
}

func (f *adminDuplicateReprocessingStore) SaveCriticInputSnapshot(
	_ context.Context,
	_ string,
	revision string,
	snapshotJSON string,
	snapshotHash string,
	_ time.Time,
) error {
	if f.source.SourceRevision == revision {
		f.source.CriticInputSnapshotJSON = snapshotJSON
		f.source.CriticInputSnapshotHash = snapshotHash
	}
	return nil
}

func newAdminDuplicateReprocessingStore() *adminDuplicateReprocessingStore {
	source := store.MemorySourceRevision{
		SourceRevision:          "revision",
		ChatSessionID:           "session",
		LogicalTurnID:           "turn:4",
		TurnIndex:               4,
		UserContent:             "Mina asks about the brass key.",
		AssistantContent:        "Mina finds the brass key under the desk.",
		LifecycleState:          "active",
		DerivedAdmissionState:   "committed",
		DerivedAdmissionVersion: store.MemoryAdmissionContract,
		DerivedExtractorVersion: completeTurnCriticPipelineVersion,
		DerivedIndexVersion:     memoryAdmissionIndexVersion,
		DerivedResultHash:       "previous-result",
		DerivedResultJSON:       `{"turn_summary":"previous"}`,
	}
	return &adminDuplicateReprocessingStore{
		Store:  store.NewNoopStore(),
		source: source,
		logs: []store.ChatLog{
			{ChatSessionID: source.ChatSessionID, TurnIndex: source.TurnIndex, Role: "user", Content: source.UserContent},
			{ChatSessionID: source.ChatSessionID, TurnIndex: source.TurnIndex, Role: "assistant", Content: source.AssistantContent},
		},
	}
}

func (f *adminDuplicateReprocessingStore) MemoryDerivationLifecycleEnabled() bool {
	return true
}

func (f *adminDuplicateReprocessingStore) ListChatLogs(context.Context, string, int, int) ([]store.ChatLog, error) {
	return append([]store.ChatLog(nil), f.logs...), nil
}

func (f *adminDuplicateReprocessingStore) ListMemories(context.Context, string, int, int) ([]store.Memory, error) {
	return append([]store.Memory(nil), f.memories...), nil
}

func (f *adminDuplicateReprocessingStore) ListActiveSourceRevisions(context.Context, string, int, int) ([]store.MemorySourceRevision, error) {
	return []store.MemorySourceRevision{f.source}, nil
}

func (f *adminDuplicateReprocessingStore) RegisterAcceptedSourceRevision(
	context.Context,
	*store.MemorySourceRevision,
) (store.SourceRevisionRegistration, error) {
	return store.SourceRevisionRegistration{Idempotent: true}, nil
}

func (f *adminDuplicateReprocessingStore) GetSourceRevision(
	context.Context,
	string,
	string,
) (*store.MemorySourceRevision, error) {
	copySource := f.source
	return &copySource, nil
}

func (f *adminDuplicateReprocessingStore) IsSourceRevisionActive(
	context.Context,
	string,
	string,
) (bool, error) {
	return f.source.LifecycleState == "active", nil
}

func (f *adminDuplicateReprocessingStore) InvalidateSourceRevisions(
	context.Context,
	string,
	int,
	string,
	string,
	time.Time,
) error {
	return nil
}

func (f *adminDuplicateReprocessingStore) EnqueueMemoryReprocessingJob(_ context.Context, job *store.MemoryReprocessingJob) (bool, error) {
	f.enqueuedJobs = append(f.enqueuedJobs, job)
	return f.enqueueNew, nil
}

func (f *adminDuplicateReprocessingStore) ClaimMemoryReprocessingJob(context.Context, string, time.Time, time.Duration) (*store.MemoryReprocessingJob, error) {
	return nil, store.ErrNotFound
}

func (f *adminDuplicateReprocessingStore) CompleteMemoryReprocessingJob(context.Context, int64, string, time.Time) error {
	return nil
}

func (f *adminDuplicateReprocessingStore) FailMemoryReprocessingJob(context.Context, int64, string, time.Time, time.Time, bool, string) error {
	return nil
}

func (f *adminDuplicateReprocessingStore) SaveAuditLog(_ context.Context, item *store.AuditLog) error {
	if item != nil {
		f.auditLogs = append(f.auditLogs, *item)
	}
	return nil
}

func (f *adminDuplicateReprocessingStore) ListAuditLogs(
	_ context.Context,
	sid string,
	eventType string,
	limit int,
) ([]store.AuditLog, error) {
	out := []store.AuditLog{}
	for _, item := range f.auditLogs {
		if sid != "" && item.ChatSessionID != sid {
			continue
		}
		if eventType != "" && item.EventType != eventType {
			continue
		}
		out = append(out, item)
		if limit > 0 && len(out) >= limit {
			break
		}
	}
	return out, nil
}

type adminReopenableReprocessingStore struct {
	*adminDuplicateReprocessingStore
	reopenCalls int
	reopenKey   string
	reopenSID   string
	reopenRev   string
}

type adminCanonicalRawReprocessingStore struct {
	*adminDuplicateReprocessingStore
	registered []store.MemorySourceRevision
	admissions []*store.MemoryAdmission
}

func (f *adminCanonicalRawReprocessingStore) SaveCriticInputSnapshot(
	_ context.Context,
	_ string,
	revision string,
	snapshotJSON string,
	snapshotHash string,
	_ time.Time,
) error {
	for index := range f.registered {
		if f.registered[index].SourceRevision == revision {
			f.registered[index].CriticInputSnapshotJSON = snapshotJSON
			f.registered[index].CriticInputSnapshotHash = snapshotHash
			return nil
		}
	}
	return store.ErrNotFound
}

func (f *adminCanonicalRawReprocessingStore) ListActiveSourceRevisions(
	_ context.Context,
	_ string,
	_ int,
	_ int,
) ([]store.MemorySourceRevision, error) {
	return append([]store.MemorySourceRevision(nil), f.registered...), nil
}

func (f *adminCanonicalRawReprocessingStore) RegisterAcceptedSourceRevision(
	_ context.Context,
	source *store.MemorySourceRevision,
) (store.SourceRevisionRegistration, error) {
	if source == nil {
		return store.SourceRevisionRegistration{}, store.ErrSourceRevisionConflict
	}
	for _, existing := range f.registered {
		if existing.SourceRevision == source.SourceRevision &&
			existing.ChatSessionID == source.ChatSessionID &&
			existing.TurnIndex == source.TurnIndex &&
			existing.UserContent == source.UserContent &&
			existing.AssistantContent == source.AssistantContent {
			return store.SourceRevisionRegistration{Idempotent: true}, nil
		}
		if existing.ChatSessionID == source.ChatSessionID &&
			existing.TurnIndex == source.TurnIndex {
			return store.SourceRevisionRegistration{}, store.ErrSourceRevisionConflict
		}
	}
	copySource := *source
	f.registered = append(f.registered, copySource)
	return store.SourceRevisionRegistration{Inserted: true}, nil
}

func (f *adminCanonicalRawReprocessingStore) GetSourceRevision(
	_ context.Context,
	_ string,
	sourceRevision string,
) (*store.MemorySourceRevision, error) {
	for _, source := range f.registered {
		if source.SourceRevision == sourceRevision {
			copySource := source
			return &copySource, nil
		}
	}
	return nil, store.ErrNotFound
}

func (f *adminCanonicalRawReprocessingStore) IsSourceRevisionActive(
	ctx context.Context,
	sid string,
	sourceRevision string,
) (bool, error) {
	source, err := f.GetSourceRevision(ctx, sid, sourceRevision)
	if err != nil {
		return false, err
	}
	return source.LifecycleState == "active", nil
}

func (f *adminCanonicalRawReprocessingStore) InvalidateSourceRevisions(
	context.Context,
	string,
	int,
	string,
	string,
	time.Time,
) error {
	return nil
}

func (f *adminCanonicalRawReprocessingStore) MemoryAdmissionWritesEnabled() bool {
	return true
}

func (f *adminCanonicalRawReprocessingStore) CommitMemoryAdmission(
	_ context.Context,
	item *store.MemoryAdmission,
) (store.MemoryAdmissionResult, error) {
	f.admissions = append(f.admissions, item)
	if item != nil && item.Memory != nil {
		f.memories = append(f.memories, *item.Memory)
	}
	return store.MemoryAdmissionResult{
		MemoryInserted:   item != nil && item.Memory != nil,
		EvidenceInserted: len(item.Evidence),
		PreciseInserted:  len(item.PreciseUnits),
		VectorOperations: len(item.Vectors),
	}, nil
}

func (f *adminReopenableReprocessingStore) ReopenMemoryReprocessingJob(
	_ context.Context,
	idempotencyKey string,
	chatSessionID string,
	sourceRevision string,
	_ time.Time,
) (bool, error) {
	f.reopenCalls++
	f.reopenKey = idempotencyKey
	f.reopenSID = chatSessionID
	f.reopenRev = sourceRevision
	return true, nil
}

func TestAdminRescanDuplicateJobIsDeferredInsteadOfReportedSuccessful(t *testing.T) {
	st := newAdminDuplicateReprocessingStore()
	srv := &Server{Cfg: config.Default(), Store: st, Vector: vector.NewFakeVectorStore()}

	response, err := srv.runAdminRescan(
		context.Background(),
		st.source.ChatSessionID,
		adminRescanRequest{TurnIndices: []int{st.source.TurnIndex}},
	)
	if err != nil {
		t.Fatal(err)
	}
	if intFromAny(response["succeeded"], -1) != 0 ||
		intFromAny(response["queued"], -1) != 0 ||
		intFromAny(response["deferred"], -1) != 1 ||
		response["status"] != "deferred" ||
		len(st.enqueuedJobs) != 1 {
		t.Fatalf("response=%+v enqueued=%d", response, len(st.enqueuedJobs))
	}
	deferred := response["deferred_turns"].([]map[string]any)
	if len(deferred) != 1 || deferred[0]["reason"] != "reprocessing_job_already_pending" {
		t.Fatalf("deferred=%+v", deferred)
	}
}

func TestAdminRescanNewJobIsDeferredInsteadOfReportedSuccessful(t *testing.T) {
	st := newAdminDuplicateReprocessingStore()
	st.enqueueNew = true
	srv := &Server{Cfg: config.Default(), Store: st, Vector: vector.NewFakeVectorStore()}

	response, err := srv.runAdminRescan(
		context.Background(),
		st.source.ChatSessionID,
		adminRescanRequest{TurnIndices: []int{st.source.TurnIndex}},
	)
	if err != nil {
		t.Fatal(err)
	}
	if intFromAny(response["succeeded"], -1) != 0 ||
		intFromAny(response["deferred"], -1) != 1 ||
		intFromAny(response["queued"], -1) != 1 ||
		response["status"] != "deferred" ||
		len(st.enqueuedJobs) != 1 {
		t.Fatalf("response=%+v enqueued=%d", response, len(st.enqueuedJobs))
	}
	deferred := response["deferred_turns"].([]map[string]any)
	if len(deferred) != 1 || deferred[0]["reason"] != "reprocessing_queued" {
		t.Fatalf("deferred=%+v", deferred)
	}
}

func TestAdminRescanCanonicalRawPairRebuildsThroughSharedSourceOwner(t *testing.T) {
	base := newAdminDuplicateReprocessingStore()
	base.source.UserContent = "Mina asks about the brass key. <thinking>hidden chain</thinking>"
	base.logs[0].Content = base.source.UserContent
	st := &adminCanonicalRawReprocessingStore{
		adminDuplicateReprocessingStore: base,
	}
	srv := &Server{Cfg: config.Default(), Store: st, Vector: vector.NewFakeVectorStore()}
	oldClient := proxyHTTPClient
	criticCalls := 0
	criticRequest := ""
	criticContent := criticWireJSONForTest(map[string]any{"turn_summary": "Mina found the brass key.", "importance_score": 7})
	proxyHTTPClient = &http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
		criticCalls++
		body, _ := io.ReadAll(req.Body)
		criticRequest = string(body)
		payload, _ := json.Marshal(map[string]any{
			"choices": []any{map[string]any{
				"message": map[string]any{
					"content": criticContent,
				},
			}},
		})
		return &http.Response{
			StatusCode: http.StatusOK,
			Header:     make(http.Header),
			Body:       io.NopCloser(strings.NewReader(string(payload))),
		}, nil
	})}
	defer func() { proxyHTTPClient = oldClient }()

	response, err := srv.runAdminRescan(
		context.Background(),
		base.source.ChatSessionID,
		adminRescanRequest{
			TurnIndices:        []int{base.source.TurnIndex},
			CanonicalRawReplay: true,
			ClientMeta: map[string]any{
				"critic": map[string]any{
					"api_key":    "test-key",
					"endpoint":   "https://api.example.com/v1",
					"model":      "critic",
					"provider":   "openai",
					"timeout_ms": 45000,
				},
			},
		},
	)
	if err != nil {
		t.Fatal(err)
	}
	if response["status"] != "ok" ||
		intFromAny(response["succeeded"], -1) != 1 ||
		intFromAny(response["failed"], -1) != 0 ||
		intFromAny(response["deferred"], -1) != 0 ||
		intFromAny(response["queued"], -1) != 0 ||
		len(st.registered) != 1 ||
		len(st.enqueuedJobs) != 0 ||
		len(st.admissions) != 1 ||
		criticCalls != 1 {
		t.Fatalf(
			"response=%+v registered=%d enqueued=%d admissions=%d critic_calls=%d",
			response, len(st.registered), len(st.enqueuedJobs), len(st.admissions), criticCalls,
		)
	}
	source := st.registered[0]
	if source.UserContent != base.source.UserContent ||
		!strings.Contains(source.UserContent, "hidden chain") ||
		strings.Contains(criticRequest, "hidden chain") {
		t.Fatalf("source raw=%q critic request=%q", source.UserContent, criticRequest)
	}
	earlier := adminRescanCanonicalRawSourceRevision(
		source.ChatSessionID,
		source.TurnIndex,
		source.UserContent,
		source.AssistantContent,
		time.Unix(1, 0).UTC(),
	)
	later := adminRescanCanonicalRawSourceRevision(
		source.ChatSessionID,
		source.TurnIndex,
		source.UserContent,
		source.AssistantContent,
		time.Unix(2, 0).UTC(),
	)
	repeated := adminRescanCanonicalRawSourceRevision(
		source.ChatSessionID,
		source.TurnIndex,
		source.UserContent,
		source.AssistantContent,
		time.Unix(1, 0).UTC(),
	)
	if earlier == nil || later == nil || repeated == nil ||
		earlier.SourceRevision != repeated.SourceRevision ||
		earlier.LogicalTurnID != repeated.LogicalTurnID ||
		earlier.SourceRevision == later.SourceRevision ||
		earlier.LogicalTurnID == later.LogicalTurnID ||
		source.SourceMessageID != "" ||
		source.SourceGenerationID != "" ||
		source.BranchState != "not_exposed" {
		t.Fatalf("source=%+v earlier=%+v later=%+v repeated=%+v", source, earlier, later, repeated)
	}
	artifacts, _ := response["artifact_counts"].(map[string]int)
	if artifacts["memories"] != 1 {
		t.Fatalf("artifacts=%+v response=%+v", artifacts, response)
	}
}

func TestAdminRescanCanonicalRawReplayReprojectsCommittedExtractionWithoutCritic(t *testing.T) {
	base := newAdminDuplicateReprocessingStore()
	extraction := map[string]any{
		"turn_summary":      "Mina finds the brass key under the desk.",
		"importance_score":  7,
		"evidence_excerpts": []any{"Mina finds the brass key under the desk."},
		"kg_triples": []any{map[string]any{
			"subject":   "Mina",
			"predicate": "found",
			"object":    "brass key",
		}},
	}
	committed := base.source
	committed.DerivedAdmissionState = "committed"
	committed.DerivedAdmissionVersion = store.MemoryAdmissionContract
	committed.DerivedExtractorVersion = completeTurnCriticPipelineVersion
	committed.DerivedIndexVersion = memoryAdmissionIndexVersion
	committed.DerivedResultJSON = mustCompactJSON(normalizePreciseMemoryValue(extraction))
	committed.DerivedResultHash = memoryAdmissionResultHash(
		committed.SourceRevision,
		extraction,
		store.MemoryAdmissionContract,
		completeTurnCriticPipelineVersion,
		memoryAdmissionIndexVersion,
	)
	st := &adminCanonicalRawReprocessingStore{
		adminDuplicateReprocessingStore: base,
		registered:                      []store.MemorySourceRevision{committed},
	}
	srv := &Server{Cfg: config.Default(), Store: st, Vector: vector.NewFakeVectorStore()}

	response, err := srv.runAdminRescan(
		context.Background(),
		base.source.ChatSessionID,
		adminRescanRequest{
			TurnIndices:        []int{base.source.TurnIndex},
			CanonicalRawReplay: true,
		},
	)
	if err != nil {
		t.Fatal(err)
	}
	if response["status"] != "ok" ||
		intFromAny(response["candidate_count"], -1) != 1 ||
		intFromAny(response["succeeded"], -1) != 1 ||
		intFromAny(response["failed"], -1) != 0 ||
		len(st.admissions) != 1 {
		t.Fatalf("response=%+v admissions=%d", response, len(st.admissions))
	}
	if st.admissions[0].ResultJSON != committed.DerivedResultJSON {
		t.Fatalf("admission=%+v", st.admissions[0])
	}
}

func TestAdminRescanCanonicalRawReplaySkipsOnlySourceScopedCompletedProjection(t *testing.T) {
	base := newAdminDuplicateReprocessingStore()
	extraction := map[string]any{
		"turn_summary": "Mina finds the brass key under the desk.",
	}
	committed := base.source
	committed.DerivedAdmissionState = "committed"
	committed.DerivedAdmissionVersion = store.MemoryAdmissionContract
	committed.DerivedExtractorVersion = completeTurnCriticPipelineVersion
	committed.DerivedIndexVersion = memoryAdmissionIndexVersion
	committed.DerivedResultJSON = mustCompactJSON(normalizePreciseMemoryValue(extraction))
	committed.DerivedResultHash = memoryAdmissionResultHash(
		committed.SourceRevision,
		extraction,
		store.MemoryAdmissionContract,
		completeTurnCriticPipelineVersion,
		memoryAdmissionIndexVersion,
	)
	base.auditLogs = append(base.auditLogs, store.AuditLog{
		ChatSessionID: committed.ChatSessionID,
		EventType:     "critic_ingest_trace",
		TargetType:    "turn",
		TargetID:      int64(committed.TurnIndex),
		DetailsJSON: mustCompactJSON(map[string]any{
			"pipeline_complete":  true,
			"source_revision":    committed.SourceRevision,
			"derivation_version": store.MemoryAdmissionContract,
			"extractor_version":  completeTurnCriticPipelineVersion,
			"index_version":      memoryAdmissionIndexVersion,
		}),
	})
	st := &adminCanonicalRawReprocessingStore{
		adminDuplicateReprocessingStore: base,
		registered:                      []store.MemorySourceRevision{committed},
	}
	srv := &Server{Cfg: config.Default(), Store: st, Vector: vector.NewFakeVectorStore()}

	response, err := srv.runAdminRescan(
		context.Background(),
		base.source.ChatSessionID,
		adminRescanRequest{
			TurnIndices:        []int{base.source.TurnIndex},
			CanonicalRawReplay: true,
		},
	)
	if err != nil {
		t.Fatal(err)
	}
	skipped := response["skipped_turns"].([]map[string]any)
	if response["status"] != "ok" ||
		intFromAny(response["candidate_count"], -1) != 1 ||
		intFromAny(response["succeeded"], -1) != 0 ||
		intFromAny(response["skipped"], -1) != 1 ||
		len(skipped) != 1 ||
		skipped[0]["reason"] != "derived_projection_complete" ||
		len(st.admissions) != 0 {
		t.Fatalf("response=%+v admissions=%d", response, len(st.admissions))
	}
}

func TestAdminRescanCanonicalRawReplayRejectsActiveSourceContentMismatch(t *testing.T) {
	base := newAdminDuplicateReprocessingStore()
	mismatched := base.source
	mismatched.AssistantContent = "This source no longer matches canonical raw."
	st := &adminCanonicalRawReprocessingStore{
		adminDuplicateReprocessingStore: base,
		registered:                      []store.MemorySourceRevision{mismatched},
	}
	srv := &Server{Cfg: config.Default(), Store: st, Vector: vector.NewFakeVectorStore()}
	oldClient := proxyHTTPClient
	criticCalls := 0
	proxyHTTPClient = &http.Client{Transport: roundTripFunc(func(*http.Request) (*http.Response, error) {
		criticCalls++
		return nil, context.Canceled
	})}
	defer func() { proxyHTTPClient = oldClient }()

	response, err := srv.runAdminRescan(
		context.Background(),
		base.source.ChatSessionID,
		adminRescanRequest{
			TurnIndices:        []int{base.source.TurnIndex},
			CanonicalRawReplay: true,
		},
	)
	if err != nil {
		t.Fatal(err)
	}
	failed := response["failed_turns"].([]map[string]any)
	if response["status"] != "partial_error" ||
		intFromAny(response["failed"], -1) != 1 ||
		len(failed) != 1 ||
		failed[0]["reason"] != "active_source_raw_mismatch" ||
		criticCalls != 0 ||
		len(st.admissions) != 0 {
		t.Fatalf(
			"response=%+v critic_calls=%d admissions=%d",
			response, criticCalls, len(st.admissions),
		)
	}
}

func TestAdminRescanForceDerivedRebuildReopensDuplicateJobAndAudits(t *testing.T) {
	base := newAdminDuplicateReprocessingStore()
	st := &adminReopenableReprocessingStore{adminDuplicateReprocessingStore: base}
	srv := &Server{Cfg: config.Default(), Store: st, Vector: vector.NewFakeVectorStore()}

	response, err := srv.runAdminRescan(
		context.Background(),
		st.source.ChatSessionID,
		adminRescanRequest{
			TurnIndices: []int{st.source.TurnIndex},
			ClientMeta:  map[string]any{"force_derived_rebuild": true},
		},
	)
	if err != nil {
		t.Fatal(err)
	}
	if intFromAny(response["succeeded"], -1) != 0 ||
		intFromAny(response["deferred"], -1) != 1 ||
		intFromAny(response["queued"], -1) != 1 ||
		intFromAny(response["reopened"], -1) != 1 ||
		response["status"] != "deferred" ||
		st.reopenCalls != 1 {
		t.Fatalf("response=%+v reopen_calls=%d", response, st.reopenCalls)
	}
	expectedKey := completeTurnReprocessingIdempotencyKey(
		st.source.ChatSessionID,
		st.source.SourceRevision,
		store.MemoryAdmissionContract,
		completeTurnCriticPipelineVersion,
		memoryAdmissionIndexVersion,
	)
	if st.reopenKey != expectedKey ||
		st.reopenSID != st.source.ChatSessionID ||
		st.reopenRev != st.source.SourceRevision {
		t.Fatalf("reopen key/sid/rev=%q/%q/%q", st.reopenKey, st.reopenSID, st.reopenRev)
	}
	if len(st.auditLogs) != 1 ||
		st.auditLogs[0].EventType != "memory_reprocessing_reopened" {
		t.Fatalf("audit_logs=%+v", st.auditLogs)
	}
}

func TestAdminRescanForceDerivedRebuildWithoutReopenerIsHonestSkip(t *testing.T) {
	st := newAdminDuplicateReprocessingStore()
	srv := &Server{Cfg: config.Default(), Store: st, Vector: vector.NewFakeVectorStore()}

	response, err := srv.runAdminRescan(
		context.Background(),
		st.source.ChatSessionID,
		adminRescanRequest{
			TurnIndices: []int{st.source.TurnIndex},
			ClientMeta:  map[string]any{"force_derived_rebuild": true},
		},
	)
	if err != nil {
		t.Fatal(err)
	}
	if intFromAny(response["succeeded"], -1) != 0 ||
		intFromAny(response["queued"], -1) != 0 ||
		intFromAny(response["skipped"], -1) != 1 {
		t.Fatalf("response=%+v", response)
	}
	skipped := response["skipped_turns"].([]map[string]any)
	if len(skipped) != 1 || skipped[0]["reason"] != "reprocessing_reopen_unavailable" {
		t.Fatalf("skipped=%+v", skipped)
	}
}
