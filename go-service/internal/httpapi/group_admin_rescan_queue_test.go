package httpapi

import (
	"context"
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

func (f *adminDuplicateReprocessingStore) EnqueueMemoryReprocessingJob(_ context.Context, job *store.MemoryReprocessingJob) (bool, error) {
	f.enqueuedJobs = append(f.enqueuedJobs, job)
	return false, nil
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

type adminReopenableReprocessingStore struct {
	*adminDuplicateReprocessingStore
	reopenCalls int
	reopenKey   string
	reopenSID   string
	reopenRev   string
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

func TestAdminRescanDuplicateJobIsSkippedInsteadOfReportedSuccessful(t *testing.T) {
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
		intFromAny(response["skipped"], -1) != 1 ||
		len(st.enqueuedJobs) != 1 {
		t.Fatalf("response=%+v enqueued=%d", response, len(st.enqueuedJobs))
	}
	skipped := response["skipped_turns"].([]map[string]any)
	if len(skipped) != 1 || skipped[0]["reason"] != "reprocessing_job_already_exists" {
		t.Fatalf("skipped=%+v", skipped)
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
	if intFromAny(response["succeeded"], -1) != 1 ||
		intFromAny(response["queued"], -1) != 1 ||
		intFromAny(response["reopened"], -1) != 1 ||
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
