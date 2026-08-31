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
	"strings"
	"testing"
	"time"

	"github.com/risulongmemory/archive-center-go/internal/config"
	"github.com/risulongmemory/archive-center-go/internal/store"
	"github.com/risulongmemory/archive-center-go/internal/vector"
)

func (r *rollbackRecordingStore) SaveChapterSummary(context.Context, *store.ChapterSummary) error {
	return nil
}

func (r *rollbackRecordingStore) SearchChapterSummaries(context.Context, string, string, int, int, int) ([]store.ChapterSummary, error) {
	return r.chapterSummaries, nil
}

func (r *rollbackRecordingStore) SaveArcSummary(context.Context, string, *store.ArcSummary) error {
	return nil
}

func (r *rollbackRecordingStore) GetLatestArcSummary(context.Context, string) (*store.ArcSummary, error) {
	return nil, store.ErrNotFound
}

func (r *rollbackRecordingStore) ListArcSummaries(context.Context, string, string, int) ([]store.ArcSummary, error) {
	return r.arcSummaries, nil
}

func (r *rollbackRecordingStore) SearchArcSummaries(context.Context, string, string, int, int, int) ([]store.ArcSummary, error) {
	return r.arcSummaries, nil
}

func (r *rollbackRecordingStore) SaveSagaDigest(context.Context, string, *store.SagaDigest) error {
	return nil
}

func (r *rollbackRecordingStore) GetLatestSagaDigest(context.Context, string) (*store.SagaDigest, error) {
	return nil, store.ErrNotFound
}

func (r *rollbackRecordingStore) ListSagaDigests(context.Context, string, int) ([]store.SagaDigest, error) {
	return r.sagaDigests, nil
}

func (r *rollbackRecordingStore) SearchSagaDigests(context.Context, string, string, int, int, int) ([]store.SagaDigest, error) {
	return r.sagaDigests, nil
}

func (r *rollbackRecordingStore) DeleteTrustStates(ctx context.Context, sid string, fromTurn int) error {
	if r.deleteErr != nil {
		return r.deleteErr
	}
	r.deletes = append(r.deletes, fmt.Sprintf("trust_states:%s:%d", sid, fromTurn))
	return nil
}

func (r *rollbackRecordingStore) DeleteStorylines(ctx context.Context, sid string, fromTurn int) error {
	if r.deleteErr != nil {
		return r.deleteErr
	}
	r.deletes = append(r.deletes, fmt.Sprintf("storylines:%s:%d", sid, fromTurn))
	return nil
}

func (r *rollbackRecordingStore) DeleteWorldRules(ctx context.Context, sid string, fromTurn int) error {
	if r.deleteErr != nil {
		return r.deleteErr
	}
	r.deletes = append(r.deletes, fmt.Sprintf("world_rules:%s:%d", sid, fromTurn))
	return nil
}

func (r *rollbackRecordingStore) DeleteCharacterStates(ctx context.Context, sid string, fromTurn int) error {
	if r.deleteErr != nil {
		return r.deleteErr
	}
	r.deletes = append(r.deletes, fmt.Sprintf("character_states:%s:%d", sid, fromTurn))
	return nil
}

func (r *rollbackRecordingStore) DeletePendingThreads(ctx context.Context, sid string, fromTurn int) error {
	if r.deleteErr != nil {
		return r.deleteErr
	}
	r.deletes = append(r.deletes, fmt.Sprintf("pending_threads:%s:%d", sid, fromTurn))
	return nil
}

func (r *rollbackRecordingStore) DeleteActiveStates(ctx context.Context, sid string, fromTurn int) error {
	if r.deleteErr != nil {
		return r.deleteErr
	}
	r.deletes = append(r.deletes, fmt.Sprintf("active_states:%s:%d", sid, fromTurn))
	return nil
}

func (r *rollbackRecordingStore) DeleteCanonicalStateLayers(ctx context.Context, sid string, fromTurn int) error {
	if r.deleteErr != nil {
		return r.deleteErr
	}
	r.deletes = append(r.deletes, fmt.Sprintf("canonical_state_layers:%s:%d", sid, fromTurn))
	return nil
}

func (r *rollbackRecordingStore) DeleteEpisodeSummaries(ctx context.Context, sid string, fromTurn int) error {
	if r.deleteErr != nil {
		return r.deleteErr
	}
	r.deletes = append(r.deletes, fmt.Sprintf("episode_summaries:%s:%d", sid, fromTurn))
	return nil
}

func (r *rollbackRecordingStore) DeleteGuidancePlanState(ctx context.Context, sid string, fromTurn int) error {
	if r.deleteErr != nil {
		return r.deleteErr
	}
	r.deletes = append(r.deletes, fmt.Sprintf("guidance_plan_states:%s:%d", sid, fromTurn))
	return nil
}

func (r *rollbackRecordingStore) DeleteChapterSummaries(ctx context.Context, sid string, fromTurn int) error {
	if r.deleteErr != nil {
		return r.deleteErr
	}
	r.deletes = append(r.deletes, fmt.Sprintf("chapter_summaries:%s:%d", sid, fromTurn))
	return nil
}

func (r *rollbackRecordingStore) DeleteArcSummaries(ctx context.Context, sid string, fromTurn int) error {
	if r.deleteErr != nil {
		return r.deleteErr
	}
	r.deletes = append(r.deletes, fmt.Sprintf("arc_summaries:%s:%d", sid, fromTurn))
	return nil
}

func (r *rollbackRecordingStore) DeleteSagaDigests(ctx context.Context, sid string, fromTurn int) error {
	if r.deleteErr != nil {
		return r.deleteErr
	}
	r.deletes = append(r.deletes, fmt.Sprintf("saga_digests:%s:%d", sid, fromTurn))
	return nil
}

func (r *rollbackRecordingStore) DeleteSessionActiveScopes(ctx context.Context, sid string, fromTurn int) error {
	if r.deleteErr != nil {
		return r.deleteErr
	}
	r.deletes = append(r.deletes, fmt.Sprintf("session_active_scopes:%s:%d", sid, fromTurn))
	return nil
}

func (r *rollbackRecordingStore) DeleteProtagonistEntityMemories(ctx context.Context, sid string, fromTurn int) error {
	if r.deleteErr != nil {
		return r.deleteErr
	}
	r.deletes = append(r.deletes, fmt.Sprintf("subjective_entity_memories:%s:%d", sid, fromTurn))
	return nil
}

func (r *rollbackRecordingStore) DeleteConsequenceRecords(ctx context.Context, sid string, fromTurn int) error {
	if r.deleteErr != nil {
		return r.deleteErr
	}
	r.deletes = append(r.deletes, fmt.Sprintf("consequence_records:%s:%d", sid, fromTurn))
	return nil
}

func (r *rollbackRecordingStore) DeletePsychologyBranches(ctx context.Context, sid string, fromTurn int) error {
	if r.deleteErr != nil {
		return r.deleteErr
	}
	r.deletes = append(r.deletes, fmt.Sprintf("psychology_branches:%s:%d", sid, fromTurn))
	return nil
}

func (r *rollbackRecordingStore) DeleteThemeOffscreenCarries(ctx context.Context, sid string, fromTurn int) error {
	if r.deleteErr != nil {
		return r.deleteErr
	}
	r.deletes = append(r.deletes, fmt.Sprintf("theme_offscreen_carries:%s:%d", sid, fromTurn))
	return nil
}

func (r *rollbackRecordingStore) DeleteCaptureVerificationRecords(ctx context.Context, sid string, fromTurn int) error {
	if r.deleteErr != nil {
		return r.deleteErr
	}
	r.deletes = append(r.deletes, fmt.Sprintf("capture_verification_records:%s:%d", sid, fromTurn))
	return nil
}

func (r *rollbackRecordingStore) DeleteStatusCurrentValues(ctx context.Context, sid string, fromTurn int) error {
	if r.deleteErr != nil {
		return r.deleteErr
	}
	r.deletes = append(r.deletes, fmt.Sprintf("status_current_values:%s:%d", sid, fromTurn))
	return nil
}

func (r *rollbackRecordingStore) DeleteStatusChangeEvents(ctx context.Context, sid string, fromTurn int) error {
	if r.deleteErr != nil {
		return r.deleteErr
	}
	r.deletes = append(r.deletes, fmt.Sprintf("status_change_events:%s:%d", sid, fromTurn))
	return nil
}

func (r *rollbackRecordingStore) DeleteStatusEffects(ctx context.Context, sid string, fromTurn int) error {
	if r.deleteErr != nil {
		return r.deleteErr
	}
	r.deletes = append(r.deletes, fmt.Sprintf("status_effects:%s:%d", sid, fromTurn))
	return nil
}

func (r *rollbackRecordingStore) DeleteSession(ctx context.Context, sid string) error {
	if r.deleteErr != nil {
		return r.deleteErr
	}
	r.deletes = append(r.deletes, fmt.Sprintf("session:%s", sid))
	return nil
}

func (r *rollbackRecordingStore) SaveAuditLog(ctx context.Context, a *store.AuditLog) error {
	r.audits = append(r.audits, a)
	return nil
}

type rollbackLifecycleStore struct {
	*rollbackRecordingStore
	invalidationErr    error
	outboxOperation    string
	outbox             *store.MemoryVectorOutboxItem
	lastRollback       store.LogicalTurnRollback
	lastLifecycleState string
}

func (r *rollbackLifecycleStore) MemoryDerivationLifecycleEnabled() bool {
	return true
}

func (r *rollbackLifecycleStore) RegisterAcceptedSourceRevision(context.Context, *store.MemorySourceRevision) (store.SourceRevisionRegistration, error) {
	return store.SourceRevisionRegistration{}, store.ErrNotEnabled
}

func (r *rollbackLifecycleStore) GetSourceRevision(context.Context, string, string) (*store.MemorySourceRevision, error) {
	return nil, store.ErrNotFound
}

func (r *rollbackLifecycleStore) IsSourceRevisionActive(context.Context, string, string) (bool, error) {
	return false, nil
}

func (r *rollbackLifecycleStore) InvalidateSourceRevisions(_ context.Context, sid string, fromTurn int, lifecycleState, reason string, now time.Time) error {
	r.lastLifecycleState = lifecycleState
	if r.invalidationErr != nil {
		return r.invalidationErr
	}
	operation := r.outboxOperation
	if operation == "" {
		operation = "delete"
	}
	r.outbox = &store.MemoryVectorOutboxItem{
		ID: 1, Operation: operation, OperationKey: "rollback-delete",
		ChatSessionID: sid, SourceRevision: "old-revision",
		DocumentID: "memory:" + sid + ":41", EmbeddingReady: true,
		RequiredSourceState: "inactive", Status: "pending",
		CreatedAt: now, UpdatedAt: now,
	}
	return nil
}

func (r *rollbackLifecycleStore) RollbackCanonicalTail(ctx context.Context, rollback store.LogicalTurnRollback) error {
	r.lastRollback = rollback
	return r.InvalidateSourceRevisions(ctx, rollback.ChatSessionID, rollback.TurnIndex, rollback.LifecycleAction, rollback.Reason, rollback.CreatedAt)
}

func (r *rollbackLifecycleStore) DeleteSession(_ context.Context, sid string) error {
	if r.invalidationErr != nil {
		return r.invalidationErr
	}
	r.deletes = append(r.deletes, "session:"+sid)
	now := time.Now().UTC()
	r.outbox = &store.MemoryVectorOutboxItem{
		ID: 2, Operation: "delete", OperationKey: "session-delete",
		ChatSessionID: sid, SourceRevision: "deleted-revision",
		DocumentID: "memory:" + sid + ":all", EmbeddingReady: true,
		RequiredSourceState: "inactive", Status: "pending",
		CreatedAt: now, UpdatedAt: now,
	}
	return nil
}

func (r *rollbackLifecycleStore) EnqueueMemoryVectorOperation(_ context.Context, item *store.MemoryVectorOutboxItem) (bool, error) {
	r.outbox = item
	return true, nil
}

func (r *rollbackLifecycleStore) ClaimMemoryVectorOperations(_ context.Context, owner string, now time.Time, lease time.Duration) ([]*store.MemoryVectorOutboxItem, error) {
	if r.outbox == nil || (r.outbox.Status != "pending" && r.outbox.Status != "retryable") {
		return nil, store.ErrNotFound
	}
	if !r.outbox.RetryAfter.IsZero() && !r.outbox.RetryAfter.Before(now) {
		return nil, store.ErrNotFound
	}
	copy := *r.outbox
	copy.Status = "leased"
	copy.LeaseOwner = owner
	copy.LeaseUntil = now.Add(lease)
	r.outbox = &copy
	return []*store.MemoryVectorOutboxItem{&copy}, nil
}

func (r *rollbackLifecycleStore) CompleteMemoryVectorOperation(_ context.Context, _ int64, owner string, now time.Time) error {
	if r.outbox == nil || r.outbox.LeaseOwner != owner {
		return store.ErrLeaseExpired
	}
	r.outbox.Status = "completed"
	r.outbox.UpdatedAt = now
	return nil
}

func (r *rollbackLifecycleStore) FailMemoryVectorOperation(_ context.Context, _ int64, owner string, now, retryAfter time.Time, permanent bool, failure string) error {
	if r.outbox == nil || r.outbox.LeaseOwner != owner {
		return store.ErrLeaseExpired
	}
	r.outbox.Status = "retryable"
	if permanent {
		r.outbox.Status = "permanent"
	}
	r.outbox.RetryAfter = retryAfter
	r.outbox.LastError = failure
	r.outbox.LeaseOwner = ""
	r.outbox.LeaseUntil = time.Time{}
	r.outbox.UpdatedAt = now
	return nil
}

type sessionDeleteOutboxProbeStore struct {
	*rollbackLifecycleStore
	claimCalls           int
	unrelatedOutboxState string
}

func (s *sessionDeleteOutboxProbeStore) ClaimMemoryVectorOperations(
	ctx context.Context,
	owner string,
	now time.Time,
	lease time.Duration,
) ([]*store.MemoryVectorOutboxItem, error) {
	s.claimCalls++
	s.unrelatedOutboxState = "leased"
	return s.rollbackLifecycleStore.ClaimMemoryVectorOperations(ctx, owner, now, lease)
}

func TestRollbackLiveWriteExecutesDeletions(t *testing.T) {
	cfg := config.Default()
	cfg.StoreMode = config.StoreModeMariaDBAuthority

	vec := &turnRecordingVectorStore{}
	rec := &rollbackRecordingStore{
		Store: &turnRecordingStore{
			returnMemories: []store.Memory{
				{ID: 41, ChatSessionID: "sess-live", TurnIndex: 5},
				{ID: 42, ChatSessionID: "sess-live", TurnIndex: 6},
			},
			returnEpisodeSums: []store.EpisodeSummary{
				{ID: 51, ChatSessionID: "sess-live", FromTurn: 1, ToTurn: 4},
				{ID: 52, ChatSessionID: "sess-live", FromTurn: 1, ToTurn: 5},
			},
		},
		chapterSummaries: []store.ChapterSummary{
			{ID: 61, ChatSessionID: "sess-live", FromTurn: 1, ToTurn: 4},
			{ID: 62, ChatSessionID: "sess-live", FromTurn: 1, ToTurn: 5},
		},
		arcSummaries: []store.ArcSummary{
			{ID: 71, ChatSessionID: "sess-live", FromTurn: 1, ToTurn: 4},
			{ID: 72, ChatSessionID: "sess-live", FromTurn: 1, ToTurn: 5},
		},
		sagaDigests: []store.SagaDigest{
			{ID: 81, ChatSessionID: "sess-live", FromTurn: 1, ToTurn: 4},
			{ID: 82, ChatSessionID: "sess-live", FromTurn: 1, ToTurn: 5},
		},
	}
	srv := &Server{
		Cfg:            cfg,
		Store:          rec,
		StoreOpenError: nil,
		Vector:         vec,
	}

	mux := http.NewServeMux()
	srv.RegisterRoutes(mux)

	req := httptest.NewRequest(http.MethodDelete, "/rollback/5?chat_session_id=sess-live&req_source=auto_rollback", nil)
	recorder := httptest.NewRecorder()
	mux.ServeHTTP(recorder, req)

	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200: %s", recorder.Code, recorder.Body.String())
	}

	var resp map[string]any
	if err := json.Unmarshal(recorder.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	if resp["status"] != "ok" {
		t.Errorf("status = %v, want ok", resp["status"])
	}
	if resp["source"] != "mariadb_authority" {
		t.Errorf("source = %v, want mariadb_authority", resp["source"])
	}

	rb, ok := resp["rollback_plan"].(map[string]any)
	if !ok {
		t.Fatalf("rollback_plan is not an object")
	}
	if rb["status"] != "executed" {
		t.Errorf("rollback_plan.status = %v, want executed", rb["status"])
	}
	if rb["would_delete"] != true {
		t.Errorf("rollback_plan.would_delete = %v, want true", rb["would_delete"])
	}
	if rb["mutation_enabled"] != true {
		t.Errorf("rollback_plan.mutation_enabled = %v, want true", rb["mutation_enabled"])
	}
	if rb["turn_delete_policy"] != "tail_from_earliest_deleted_turn" {
		t.Errorf("rollback_plan.turn_delete_policy = %v", rb["turn_delete_policy"])
	}
	if rb["hierarchy_invalidation"] != "delete_overlapping_episode_chapter_arc_saga_ranges" {
		t.Errorf("rollback_plan.hierarchy_invalidation = %v", rb["hierarchy_invalidation"])
	}
	if rb["step23_invalidation"] != "delete_turn_scoped_support_records_from_from_turn" {
		t.Errorf("rollback_plan.step23_invalidation = %v", rb["step23_invalidation"])
	}
	hud, ok := resp["turn_workflow_hud"].(map[string]any)
	if !ok {
		t.Fatalf("turn_workflow_hud missing from executed rollback: %+v", resp)
	}
	if hud["display_mode"] != "notice" || hud["status"] != "completed" {
		t.Fatalf("delete HUD status = %+v", hud)
	}
	if hud["title_key"] != "turn_hud.notice.delete_confirmed" || hud["notice_code"] != "ASSISTANT_OUTPUT_DELETE_CONFIRMED" {
		t.Fatalf("delete HUD presentation = %+v", hud)
	}
	if hud["request_id"] != "rollback:sess-live:5:auto_rollback" {
		t.Fatalf("delete HUD request_id = %v", hud["request_id"])
	}
	hudFacts := map[string]map[string]any{}
	for _, rawFact := range hud["facts"].([]any) {
		fact := rawFact.(map[string]any)
		hudFacts[fact["key"].(string)] = fact
	}
	if hudFacts["raw_persistence"]["status"] != "deleted" ||
		hudFacts["derived_memory"]["status"] != "deleted" ||
		hudFacts["vector_index"]["status"] != "deleted" ||
		hudFacts["vector_index"]["count"] != float64(12) {
		t.Fatalf("delete HUD facts = %+v", hudFacts)
	}

	wantDeletes := []string{
		"chat_logs:sess-live:5",
		"effective_inputs:sess-live:5",
		"memories:sess-live:5",
		"direct_evidence:sess-live:5",
		"kg_triples:sess-live:5",
		"critic_feedback:sess-live:5",
		"character_events:sess-live:5",
		"entities:sess-live:5",
		"trust_states:sess-live:5",
		"storylines:sess-live:5",
		"world_rules:sess-live:5",
		"character_states:sess-live:5",
		"pending_threads:sess-live:5",
		"active_states:sess-live:5",
		"canonical_state_layers:sess-live:5",
		"episode_summaries:sess-live:5",
		"guidance_plan_states:sess-live:5",
		"chapter_summaries:sess-live:5",
		"arc_summaries:sess-live:5",
		"saga_digests:sess-live:5",
		"session_active_scopes:sess-live:5",
		"subjective_entity_memories:sess-live:5",
		"consequence_records:sess-live:5",
		"psychology_branches:sess-live:5",
		"theme_offscreen_carries:sess-live:5",
		"capture_verification_records:sess-live:5",
		"status_current_values:sess-live:5",
		"status_change_events:sess-live:5",
		"status_effects:sess-live:5",
	}
	if len(rec.deletes) != len(wantDeletes) {
		t.Errorf("delete call count = %d, want %d", len(rec.deletes), len(wantDeletes))
	}
	for i, want := range wantDeletes {
		if i >= len(rec.deletes) {
			break
		}
		if rec.deletes[i] != want {
			t.Errorf("delete[%d] = %s, want %s", i, rec.deletes[i], want)
		}
	}
	wantVectorIDs := []string{
		"memory:sess-live:41", "memory:41", "memory:sess-live:42", "memory:42",
		"episode:sess-live:52", "episode:52",
		"chapter:sess-live:62", "chapter:62",
		"arc:sess-live:72", "arc:72",
		"saga:sess-live:82", "saga:82",
	}
	if len(vec.deletedDocumentIDs) != len(wantVectorIDs) {
		t.Fatalf("deleted vector ids = %#v, want %#v", vec.deletedDocumentIDs, wantVectorIDs)
	}
	for i, want := range wantVectorIDs {
		if vec.deletedDocumentIDs[i] != want {
			t.Fatalf("deleted vector ids = %#v, want %#v", vec.deletedDocumentIDs, wantVectorIDs)
		}
	}
	if len(rec.audits) != 1 {
		t.Fatalf("audit count = %d, want 1", len(rec.audits))
	}
	if rec.audits[0].Source != "auto_rollback" {
		t.Fatalf("audit source = %q, want auto_rollback", rec.audits[0].Source)
	}
}

func TestRollbackLifecycleQueuesDurableOutboxWithoutWaitingForProvider(t *testing.T) {
	cfg := config.Default()
	cfg.StoreMode = config.StoreModeMariaDBAuthority
	base := &rollbackRecordingStore{Store: &turnRecordingStore{}}
	lifecycle := &rollbackLifecycleStore{rollbackRecordingStore: base}
	vec := &turnRecordingVectorStore{deleteErr: errors.New("chroma unavailable")}
	srv := &Server{
		Cfg: cfg, Store: lifecycle, Vector: vec,
		RuntimeConfig: RuntimeConfig{
			Synced: true, EmbeddingTimeoutSec: 30, FailedQueueMaxAttempts: 4,
		},
	}
	mux := http.NewServeMux()
	srv.RegisterRoutes(mux)

	request := httptest.NewRequest(http.MethodDelete, "/rollback/5?chat_session_id=sess-outbox&req_source=manual", nil)
	recorder := httptest.NewRecorder()
	mux.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", recorder.Code, recorder.Body.String())
	}
	var response map[string]any
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatal(err)
	}
	if response["status"] != "ok" {
		t.Fatalf("response=%+v", response)
	}
	deletions := response["deletions"].(map[string]any)
	vectors := deletions["vectors"].(map[string]any)
	if vectors["mode"] != "durable_outbox" || vectors["vector_cleanup"] != "queued" ||
		vectors["drain_attempted"] != false {
		t.Fatalf("vectors=%+v", vectors)
	}
	if lifecycle.outbox == nil || lifecycle.outbox.Status != "pending" {
		t.Fatalf("outbox=%+v", lifecycle.outbox)
	}
	if len(vec.deletedDocumentIDs) != 0 {
		t.Fatalf("rollback HTTP called vector provider synchronously: %v", vec.deletedDocumentIDs)
	}
	if lifecycle.lastLifecycleState != store.LogicalTurnLifecycleDeleted {
		t.Fatalf("typed delete lifecycle was not applied: %q", lifecycle.lastLifecycleState)
	}
	if len(base.deletes) == 0 {
		t.Fatal("canonical rollback did not run")
	}
	hud := response["turn_workflow_hud"].(map[string]any)
	facts := map[string]map[string]any{}
	for _, rawFact := range hud["facts"].([]any) {
		fact := rawFact.(map[string]any)
		facts[fact["key"].(string)] = fact
	}
	if facts["vector_index"]["status"] != "queued" ||
		facts["vector_index"]["disposition"] != "deferred" ||
		facts["vector_index"]["count"] != float64(0) {
		t.Fatalf("outbox rollback HUD facts=%+v", facts)
	}
}

func TestRollbackLifecycleDefersUnknownVectorOperationToWorker(t *testing.T) {
	cfg := config.Default()
	cfg.StoreMode = config.StoreModeMariaDBAuthority
	base := &rollbackRecordingStore{Store: &turnRecordingStore{}}
	lifecycle := &rollbackLifecycleStore{
		rollbackRecordingStore: base,
		outboxOperation:        "unsupported",
	}
	srv := &Server{
		Cfg: cfg, Store: lifecycle, Vector: &turnRecordingVectorStore{},
		RuntimeConfig: RuntimeConfig{
			Synced: true, EmbeddingTimeoutSec: 30, FailedQueueMaxAttempts: 4,
		},
	}
	mux := http.NewServeMux()
	srv.RegisterRoutes(mux)

	request := httptest.NewRequest(http.MethodDelete, "/rollback/5?chat_session_id=sess-permanent&req_source=manual", nil)
	recorder := httptest.NewRecorder()
	mux.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", recorder.Code, recorder.Body.String())
	}
	var response map[string]any
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatal(err)
	}
	if response["status"] != "ok" {
		t.Fatalf("response status=%v body=%s", response["status"], recorder.Body.String())
	}
	deletions := response["deletions"].(map[string]any)
	vectors := deletions["vectors"].(map[string]any)
	if vectors["ok"] != true || vectors["canonical_committed"] != true ||
		vectors["vector_cleanup"] != "queued" || vectors["drain_attempted"] != false {
		t.Fatalf("queued vector result=%+v", vectors)
	}
	hud := response["turn_workflow_hud"].(map[string]any)
	if hud["status"] != "completed" || hud["notice_code"] != "ASSISTANT_OUTPUT_DELETE_CONFIRMED" {
		t.Fatalf("queued vector HUD=%+v", hud)
	}
	facts := map[string]map[string]any{}
	for _, rawFact := range hud["facts"].([]any) {
		fact := rawFact.(map[string]any)
		facts[fact["key"].(string)] = fact
	}
	if facts["vector_index"]["status"] != "queued" ||
		facts["vector_index"]["disposition"] != "deferred" ||
		facts["vector_index"]["count"] != float64(0) {
		t.Fatalf("queued vector HUD fact=%+v", facts["vector_index"])
	}
}

func TestRollbackStopsBeforeCanonicalDeletionWhenSourceInvalidationFails(t *testing.T) {
	cfg := config.Default()
	cfg.StoreMode = config.StoreModeMariaDBAuthority
	base := &rollbackRecordingStore{Store: &turnRecordingStore{}}
	lifecycle := &rollbackLifecycleStore{
		rollbackRecordingStore: base,
		invalidationErr:        errors.New("source lock failed"),
	}
	srv := &Server{Cfg: cfg, Store: lifecycle, Vector: &turnRecordingVectorStore{}}
	mux := http.NewServeMux()
	srv.RegisterRoutes(mux)

	request := httptest.NewRequest(http.MethodDelete, "/rollback/5?chat_session_id=sess-stop&req_source=manual", nil)
	recorder := httptest.NewRecorder()
	mux.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusInternalServerError {
		t.Fatalf("status=%d body=%s", recorder.Code, recorder.Body.String())
	}
	if len(base.deletes) != 0 {
		t.Fatalf("canonical deletes ran after source invalidation failure: %+v", base.deletes)
	}
	var response map[string]any
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatal(err)
	}
	hud := response["turn_workflow_hud"].(map[string]any)
	if hud["request_id"] != "rollback:sess-stop:5:manual" {
		t.Fatalf("blocked rollback HUD request_id=%v", hud["request_id"])
	}
	facts := map[string]map[string]any{}
	for _, rawFact := range hud["facts"].([]any) {
		fact := rawFact.(map[string]any)
		facts[fact["key"].(string)] = fact
	}
	for _, key := range []string{"raw_persistence", "derived_memory", "vector_index"} {
		if facts[key]["status"] != "blocked" || facts[key]["severity"] != "error" {
			t.Fatalf("blocked rollback fact %s=%+v", key, facts[key])
		}
	}
	if facts["finality"]["status"] != "blocked" {
		t.Fatalf("blocked rollback finality=%+v", facts["finality"])
	}
}

func TestSessionDeleteLifecycleUsesOutboxInsteadOfDirectVectorSessionDelete(t *testing.T) {
	cfg := config.Default()
	cfg.StoreMode = config.StoreModeMariaDBAuthority
	base := &rollbackRecordingStore{Store: &turnRecordingStore{}}
	lifecycle := &sessionDeleteOutboxProbeStore{
		rollbackLifecycleStore: &rollbackLifecycleStore{rollbackRecordingStore: base},
		unrelatedOutboxState:   "pending",
	}
	vec := &turnRecordingVectorStore{deleteErr: errors.New("chroma unavailable")}
	srv := &Server{
		Cfg: cfg, Store: lifecycle, Vector: vec,
		RuntimeConfig: RuntimeConfig{
			Synced: true, EmbeddingTimeoutSec: 30, FailedQueueMaxAttempts: 4,
		},
	}
	mux := http.NewServeMux()
	srv.RegisterRoutes(mux)

	request := httptest.NewRequest(http.MethodDelete, "/sessions/sess-session-outbox?req_source=timeline_manual_delete", nil)
	recorder := httptest.NewRecorder()
	mux.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", recorder.Code, recorder.Body.String())
	}
	var response map[string]any
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatal(err)
	}
	if response["status"] != "ok" || response["deleted"] != true {
		t.Fatalf("response=%+v", response)
	}
	cleanup := response["vector_cleanup"].(map[string]any)
	if cleanup["mode"] != "durable_outbox" || cleanup["vector_cleanup"] != "queued" ||
		cleanup["queued"] != true || cleanup["canonical_committed"] != true ||
		cleanup["drain_attempted"] != false || cleanup["processed"] != float64(0) {
		t.Fatalf("cleanup=%+v", cleanup)
	}
	if vec.deleteSessionCalls != 0 {
		t.Fatalf("direct vector session delete calls=%d", vec.deleteSessionCalls)
	}
	if lifecycle.claimCalls != 0 || lifecycle.unrelatedOutboxState != "pending" {
		t.Fatalf("manual delete drained global outbox: claims=%d unrelated=%q", lifecycle.claimCalls, lifecycle.unrelatedOutboxState)
	}
	if lifecycle.outbox == nil || lifecycle.outbox.Status != "pending" || lifecycle.outbox.ChatSessionID != "sess-session-outbox" {
		t.Fatalf("outbox=%+v", lifecycle.outbox)
	}
	select {
	case <-srv.memoryWorkerWakeChannel():
	default:
		t.Fatal("session delete did not wake the existing bounded memory worker")
	}
}

func TestSessionDeleteLifecycleStoreFailureDoesNotReportSuccessOrWakeWorker(t *testing.T) {
	cfg := config.Default()
	cfg.StoreMode = config.StoreModeMariaDBAuthority
	base := &rollbackRecordingStore{Store: &turnRecordingStore{}}
	lifecycle := &sessionDeleteOutboxProbeStore{
		rollbackLifecycleStore: &rollbackLifecycleStore{
			rollbackRecordingStore: base,
			invalidationErr:        errors.New("database commit failed"),
		},
		unrelatedOutboxState: "pending",
	}
	vec := &turnRecordingVectorStore{}
	srv := &Server{Cfg: cfg, Store: lifecycle, Vector: vec}
	mux := http.NewServeMux()
	srv.RegisterRoutes(mux)

	request := httptest.NewRequest(http.MethodDelete, "/sessions/sess-session-db-failure?req_source=timeline_manual_delete", nil)
	recorder := httptest.NewRecorder()
	mux.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusInternalServerError {
		t.Fatalf("status=%d body=%s", recorder.Code, recorder.Body.String())
	}
	var response map[string]any
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatal(err)
	}
	if response["deleted"] == true || response["status"] == "ok" {
		t.Fatalf("database failure reported success: %+v", response)
	}
	if lifecycle.claimCalls != 0 || lifecycle.unrelatedOutboxState != "pending" || vec.deleteSessionCalls != 0 {
		t.Fatalf("database failure crossed cleanup boundary: claims=%d unrelated=%q direct_vector=%d",
			lifecycle.claimCalls, lifecycle.unrelatedOutboxState, vec.deleteSessionCalls)
	}
	select {
	case <-srv.memoryWorkerWakeChannel():
		t.Fatal("database failure woke the vector worker")
	default:
	}
}

func TestRollbackMinFromTurnClampsAttachedSessionDelete(t *testing.T) {
	cfg := config.Default()
	cfg.StoreMode = config.StoreModeMariaDBAuthority

	rec := &rollbackRecordingStore{Store: &turnRecordingStore{returnMemories: []store.Memory{
		{ID: 51, ChatSessionID: "sess-attached", TurnIndex: 6},
	}}}
	srv := &Server{
		Cfg:            cfg,
		Store:          rec,
		StoreOpenError: nil,
		Vector:         &turnRecordingVectorStore{},
	}

	mux := http.NewServeMux()
	srv.RegisterRoutes(mux)

	req := httptest.NewRequest(http.MethodDelete, "/rollback/1?chat_session_id=sess-attached&req_source=auto_rollback&min_from_turn=6&protected_before_turn=5&requested_turn_index=1", nil)
	recorder := httptest.NewRecorder()
	mux.ServeHTTP(recorder, req)

	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200: %s", recorder.Code, recorder.Body.String())
	}

	var resp map[string]any
	if err := json.Unmarshal(recorder.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if resp["turn_index"] != float64(6) {
		t.Fatalf("turn_index = %v, want clamped 6", resp["turn_index"])
	}
	rb, ok := resp["rollback_plan"].(map[string]any)
	if !ok {
		t.Fatalf("rollback_plan is not an object")
	}
	if rb["requested_turn_index"] != float64(1) || rb["min_from_turn"] != float64(6) || rb["protected_before_turn"] != float64(5) {
		t.Fatalf("rollback baseline fields mismatch: %#v", rb)
	}
	if rb["session_routing_baseline_clamped"] != true {
		t.Fatalf("session_routing_baseline_clamped = %v, want true", rb["session_routing_baseline_clamped"])
	}
	for _, got := range rec.deletes {
		if !strings.HasSuffix(got, ":sess-attached:6") {
			t.Fatalf("delete call used unsafe fromTurn: %s", got)
		}
	}
}

func TestRollbackLiveWritePartialError(t *testing.T) {
	cfg := config.Default()
	cfg.StoreMode = config.StoreModeMariaDBShadow

	rec := &rollbackRecordingStore{Store: store.NewNoopStore()}
	rec.deleteErr = errors.New("boom")
	srv := &Server{
		Cfg:            cfg,
		Store:          rec,
		StoreOpenError: nil,
		Vector:         vector.NewFakeVectorStore(),
	}

	mux := http.NewServeMux()
	srv.RegisterRoutes(mux)

	req := httptest.NewRequest(http.MethodDelete, "/rollback/3?chat_session_id=sess-partial", nil)
	recorder := httptest.NewRecorder()
	mux.ServeHTTP(recorder, req)

	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200: %s", recorder.Code, recorder.Body.String())
	}

	var resp map[string]any
	if err := json.Unmarshal(recorder.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	if resp["status"] != "partial_error" {
		t.Errorf("status = %v, want partial_error", resp["status"])
	}
	errs, ok := resp["errors"].([]any)
	if !ok || len(errs) == 0 {
		t.Errorf("expected errors, got %v", resp["errors"])
	}
}

func TestCompleteTurnCriticDeltaPromotesCanonicalStateLayersWithProvenance(t *testing.T) {
	fake := &turnRecordingStore{}
	srv := NewServer(config.Default())
	srv.Store = fake
	srv.StoreOpenError = nil

	extraction := normalizeCriticExtraction(map[string]any{
		"turn_summary":     "Mina kept the scene tense while a promise stayed open.",
		"importance_score": 7,
		"relationship_memory": map[string]any{
			"bond_and_distance": "Mina trusts Rowan after the rescue.",
			"confidence":        0.86,
			"verification":      "verified",
		},
		"state_deltas": map[string]any{
			"scene_state":  map[string]any{"mood": "tense", "location": "archive hall"},
			"confidence":   0.82,
			"verification": "verified",
		},
		"pending_threads": []any{map[string]any{
			"thread_type": "promise",
			"title":       "Rowan must answer Mina later",
			"confidence":  0.9,
		}},
	})

	result := srv.saveCriticExtractionArtifacts(context.Background(), "sess-hs", 12, extraction, "Mina trusts Rowan.", completeTurnEmbeddingConfig{}, time.Unix(1200, 0))
	if result.ActiveStates != 3 {
		t.Fatalf("active states saved = %d, want 3", result.ActiveStates)
	}
	if result.CanonicalStateLayers != 3 || len(fake.savedCanonicalLayers) != 3 {
		t.Fatalf("canonical layers saved = %d/%d, want 3", result.CanonicalStateLayers, len(fake.savedCanonicalLayers))
	}
	seen := map[string]store.CanonicalStateLayer{}
	for _, item := range fake.savedCanonicalLayers {
		seen[item.LayerType] = *item
		if item.SourceTurn != 12 || item.LastVerifiedTurn != 12 || item.Confidence < 0.7 {
			t.Fatalf("canonical provenance not preserved: %#v", item)
		}
	}
	for _, layerType := range []string{"relationship_state", "scene_state", "unresolved_threads"} {
		if _, ok := seen[layerType]; !ok {
			t.Fatalf("missing canonical layer %q in %#v", layerType, seen)
		}
	}
}

func TestLC1BCanonicalStateWriteCostMeasurementClassifiesModes(t *testing.T) {
	newServer := func(fake *turnRecordingStore) *Server {
		srv := NewServer(config.Default())
		srv.Store = fake
		srv.StoreOpenError = nil
		return srv
	}
	extractionForState := func(state map[string]any) map[string]any {
		return normalizeCriticExtraction(map[string]any{
			"turn_summary":     "The current scene state was verified.",
			"importance_score": 7,
			"state_deltas":     state,
		})
	}
	state := map[string]any{
		"scene_state":  map[string]any{"mood": "tense", "location": "archive hall"},
		"confidence":   0.91,
		"verification": "verified",
	}

	bootstrapFake := &turnRecordingStore{}
	bootstrapResult := newServer(bootstrapFake).saveCriticExtractionArtifacts(context.Background(), "sess-lc1b-bootstrap", 20, extractionForState(state), "verified scene", completeTurnEmbeddingConfig{}, time.Unix(2000, 0))
	if bootstrapResult.CanonicalStateWriteCost == nil {
		t.Fatal("bootstrap cost measurement missing")
	}
	if bootstrapResult.CanonicalStateWriteCost.PolicyVersion != "lc1b.v1" {
		t.Fatalf("bootstrap policy = %q, want lc1b.v1", bootstrapResult.CanonicalStateWriteCost.PolicyVersion)
	}
	if bootstrapResult.CanonicalStateWriteCost.StateWriteCount != 1 || bootstrapResult.CanonicalStateWriteCost.FullRewriteCount != 1 {
		t.Fatalf("bootstrap counts = %#v, want one full rewrite write", bootstrapResult.CanonicalStateWriteCost)
	}
	if got := bootstrapResult.CanonicalStateWriteCost.Items[0]["write_mode"]; got != "full_rewrite_bootstrap" {
		t.Fatalf("bootstrap write_mode = %v", got)
	}

	sanitizedState := sanitizeStateDeltasForParticipant(state)
	deltaFake := &turnRecordingStore{returnCanonicalLayers: []store.CanonicalStateLayer{{
		ChatSessionID: "sess-lc1b-delta",
		LayerType:     "scene_state",
		Content:       mustCompactJSON(sanitizedState),
		TurnIndex:     19,
	}}}
	deltaResult := newServer(deltaFake).saveCriticExtractionArtifacts(context.Background(), "sess-lc1b-delta", 21, extractionForState(state), "verified scene", completeTurnEmbeddingConfig{}, time.Unix(2100, 0))
	if deltaResult.CanonicalStateWriteCost == nil {
		t.Fatal("delta cost measurement missing")
	}
	if deltaResult.CanonicalStateWriteCost.StateWriteCount != 1 || deltaResult.CanonicalStateWriteCost.DeltaUpdateCount != 1 {
		t.Fatalf("delta counts = %#v, want one delta update write", deltaResult.CanonicalStateWriteCost)
	}
	if got := deltaResult.CanonicalStateWriteCost.Items[0]["write_mode"]; got != "delta_update" {
		t.Fatalf("delta write_mode = %v", got)
	}

	rewriteFake := &turnRecordingStore{returnCanonicalLayers: []store.CanonicalStateLayer{{
		ChatSessionID: "sess-lc1b-rewrite",
		LayerType:     "scene_state",
		Content:       `{"scene_state":{"mood":"peaceful","location":"sunlit garden"},"confidence":0.91,"verification":"verified"}`,
		TurnIndex:     19,
	}}}
	rewriteResult := newServer(rewriteFake).saveCriticExtractionArtifacts(context.Background(), "sess-lc1b-rewrite", 22, extractionForState(state), "verified scene", completeTurnEmbeddingConfig{}, time.Unix(2200, 0))
	if rewriteResult.CanonicalStateWriteCost == nil {
		t.Fatal("full rewrite cost measurement missing")
	}
	if rewriteResult.CanonicalStateWriteCost.StateWriteCount != 1 || rewriteResult.CanonicalStateWriteCost.FullRewriteCount != 1 {
		t.Fatalf("rewrite counts = %#v, want one full rewrite write", rewriteResult.CanonicalStateWriteCost)
	}
	if got := rewriteResult.CanonicalStateWriteCost.Items[0]["write_mode"]; got != "full_rewrite" {
		t.Fatalf("rewrite write_mode = %v", got)
	}
}

func TestCompleteTurnCanonicalStatePromotionRequiresVerifiedConfidence(t *testing.T) {
	fake := &turnRecordingStore{}
	srv := NewServer(config.Default())
	srv.Store = fake
	srv.StoreOpenError = nil

	extraction := normalizeCriticExtraction(map[string]any{
		"turn_summary":     "Low confidence state should stay active only.",
		"importance_score": 5,
		"state_deltas": map[string]any{
			"scene_state":  map[string]any{"mood": "uncertain"},
			"confidence":   0.95,
			"verification": "pending",
		},
		"pending_threads": []any{map[string]any{
			"thread_type": "open_question",
			"title":       "Maybe this is a thread",
			"confidence":  0.4,
		}},
	})

	result := srv.saveCriticExtractionArtifacts(context.Background(), "sess-hs-low", 13, extraction, "Maybe.", completeTurnEmbeddingConfig{}, time.Unix(1300, 0))
	if result.ActiveStates != 2 {
		t.Fatalf("active states saved = %d, want 2", result.ActiveStates)
	}
	if result.CanonicalStateLayers != 0 || len(fake.savedCanonicalLayers) != 0 {
		t.Fatalf("canonical layers should not promote pending/low-confidence state, got result=%d layers=%#v", result.CanonicalStateLayers, fake.savedCanonicalLayers)
	}
}

func TestCompleteTurnDualShadowWithCriticSavesAllArtifacts(t *testing.T) {
	fake := &turnRecordingStore{
		returnCharStates: []store.CharacterState{{CharacterName: "Alice"}},
	}
	vec := &turnRecordingVectorStore{}
	cfg := config.Default()
	cfg.StoreMode = config.StoreModeMariaDBShadow
	cfg.ChromaEndpoint = "http://127.0.0.1:8000"
	srv := NewServer(cfg)

	srv.Store = store.NewDualWriteStore(store.NewNoopStore(), fake)
	srv.StoreOpenError = nil
	srv.Vector = vec
	srv.VectorOpenError = nil

	extraction := map[string]any{
		"turn_summary":        "Alice decided to trust Bob after the rescue.",
		"importance_score":    8,
		"relationship_memory": map[string]any{"bond_and_distance": "Alice trusts Bob more after he helped her.", "trust": 0.8},
		"entities": map[string]any{"characters": []any{map[string]any{
			"name": "Alice", "aliases": []any{"I"}, "role": "protagonist", "status_emotion": "relieved",
			"reference_contract": "critic_entity_reference.v1", "reference_scope": "session_stable", "name_expression": "Alice", "evidence_excerpt": "Alice relaxed after Bob helped her.",
		}}},
		"kg_triples": []any{},
		"relationship_observations": []any{map[string]any{
			"source_entity": "Alice", "source_entity_expression": "I", "target_entity": "Bob", "target_entity_expression": "Bob",
			"domain": "trust", "domain_expression": "trust", "observation": "I trust Bob", "support_kind": "explicit_statement", "evidence_excerpt": "I trust Bob.",
		}},
		"archive_hint":           map[string]any{"wing": "wing_general", "room": "hall_relationships"},
		"evidence_excerpts":      []any{"I trust Bob."},
		"emotional_intensity":    0.7,
		"narrative_significance": 0.9,
		"state_deltas":           map[string]any{"scene_state": map[string]any{"mood": "warm"}},
		"character_deltas": []any{map[string]any{
			"name":               "Alice",
			"reference_contract": "critic_entity_reference.v1",
			"reference_scope":    "session_stable",
			"name_expression":    "Alice",
			"evidence_excerpt":   "Alice relaxed after Bob helped her.",
			"status":             map[string]any{"emotion": "relieved"},
			"events":             []any{map[string]any{"type": "relationship_shift", "detail": "Alice's trust in Bob increased."}},
		}},
		"pending_threads": []any{map[string]any{"thread_type": "promise", "title": "Alice thanks Bob later", "confidence": 0.85}},
		"world_rules":     []any{map[string]any{"scope": "session", "category": "relationship", "key": "trust_changes_need_evidence", "value": "Trust shifts should be grounded in visible actions."}},
	}
	extractionBytes := []byte(criticWireJSONForTest(extraction))
	chatResp, _ := json.Marshal(map[string]any{
		"model":   "critic-model",
		"choices": []any{map[string]any{"message": map[string]any{"content": string(extractionBytes)}}},
	})
	oldClient := proxyHTTPClient
	proxyHTTPClient = &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		if strings.HasSuffix(r.URL.Path, "/embeddings") {
			return &http.Response{
				StatusCode: http.StatusOK,
				Header:     make(http.Header),
				Body:       io.NopCloser(strings.NewReader(`{"model":"embed-model","data":[{"embedding":[0.1,0.2,0.3]}]}`)),
			}, nil
		}
		return &http.Response{
			StatusCode: http.StatusOK,
			Header:     make(http.Header),
			Body:       io.NopCloser(bytes.NewReader(chatResp)),
		}, nil
	})}
	defer func() { proxyHTTPClient = oldClient }()

	mux := http.NewServeMux()
	srv.RegisterRoutes(mux)

	body := map[string]any{
		"chat_session_id":   "sess-dual-shadow",
		"turn_index":        2,
		"user_input":        "I trust Bob.",
		"assistant_content": "Alice relaxed after Bob helped her.",
		"context_messages":  []any{},
		"improvement_trace": map[string]any{"score": 9},
		"client_meta":       map[string]any{"critic": map[string]any{"api_key": "sk-test", "endpoint": "https://api.example.com/v1", "model": "critic-model", "provider": "openai", "max_tokens": 1200, "timeout_ms": 45000}, "embedding": map[string]any{"api_key": "sk-test", "endpoint": "https://api.example.com/v1", "model": "embed-model", "provider": "openai", "timeout_ms": 30000}},
		"request_type":      "model",
	}
	raw, _ := json.Marshal(body)
	req := httptest.NewRequest(http.MethodPost, "/complete-turn", bytes.NewReader(raw))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200: %s", rec.Code, rec.Body.String())
	}
	var resp map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if resp["critic_triggered"] != true {
		t.Fatalf("critic_triggered = %v, want true", resp["critic_triggered"])
	}
	if len(fake.savedMemories) != 1 {
		t.Fatalf("expected one memory, got %d", len(fake.savedMemories))
	}
	if len(fake.savedEvidence) != 1 {
		t.Fatalf("expected one evidence, got %d", len(fake.savedEvidence))
	}
	if len(fake.savedKGTriples) != 0 {
		t.Fatalf("directional relationship must not enter generic KG, got %d", len(fake.savedKGTriples))
	}
	if len(fake.savedEntities) != 1 {
		t.Fatalf("expected one entity, got %d", len(fake.savedEntities))
	}
	if len(fake.savedTrusts) != 0 {
		t.Fatalf("legacy directionless trust state was persisted: %d", len(fake.savedTrusts))
	}
	if len(fake.savedCharacterEvents) != 0 {
		t.Fatalf("legacy relationship shift event was persisted: %d", len(fake.savedCharacterEvents))
	}
	if len(fake.savedCharacterStates) != 1 {
		t.Fatalf("expected one character state, got %d", len(fake.savedCharacterStates))
	}
	if len(fake.savedPendingThreads) != 1 {
		t.Fatalf("expected one pending thread, got %d", len(fake.savedPendingThreads))
	}
	if len(fake.savedActiveStates) == 0 {
		t.Fatalf("expected at least one active state, got %d", len(fake.savedActiveStates))
	}
	if len(fake.savedWorldRules) != 1 {
		t.Fatalf("expected one world rule, got %d", len(fake.savedWorldRules))
	}
	if len(fake.savedStorylines) != 1 {
		t.Fatalf("expected one storyline, got %d", len(fake.savedStorylines))
	}
	if len(vec.docs) != 3 {
		t.Fatalf("public relationship evidence was not retained for general recall; got %#v", vec.docs)
	}
	tiers := map[string]bool{}
	for _, doc := range vec.docs {
		tiers[doc.Tier] = true
	}
	if !tiers["memory"] || !tiers["evidence"] || !tiers["world_rule"] {
		t.Fatalf("expected public memory, evidence, and world-rule vectors, got %#v", vec.docs)
	}
}

func TestBuildRecallResultIntentExecutionShadow(t *testing.T) {
	docs := []map[string]any{
		{"tier": "memory", "document_id": "d1", "text": "hello"},
		{"tier": "episode", "document_id": "d2", "text": "world"},
		{"tier": "chapter", "document_id": "d3", "text": "foo"},
	}
	shadow := buildIntentExecutionShadow(docs, map[string]any{}, "long", q3PacketBudgetPolicy())
	if shadow["version"] != "p29a.v1" {
		t.Fatalf("version mismatch: %v", shadow["version"])
	}
	if shadow["routing_mode"] != "per_intent_shadow" {
		t.Fatalf("routing_mode mismatch: %v", shadow["routing_mode"])
	}
	if shadow["status"] != "ready" {
		t.Fatalf("status mismatch: %v", shadow["status"])
	}
	intents, ok := shadow["intents"].([]map[string]any)
	if !ok || len(intents) != 4 {
		t.Fatalf("intents mismatch: %v", shadow["intents"])
	}
	be, ok := shadow["budget_enforcement"].(map[string]any)
	if !ok || be["mode"] != "enforced_shadow" {
		t.Fatalf("budget_enforcement mismatch")
	}
	gt, ok := shadow["guarded_takeover"].(map[string]any)
	if !ok || gt["decision"] != "shadow_compare" {
		t.Fatalf("guarded_takeover mismatch")
	}
	et, ok := shadow["enforced_takeover"].(map[string]any)
	if !ok || et["decision"] != "enforced_shadow" {
		t.Fatalf("enforced_takeover mismatch")
	}
	tpv, ok := shadow["tier_priority_verification"].(map[string]any)
	if !ok {
		t.Fatalf("tier_priority_verification missing")
	}
	if tpv["version"] != "t1d.v1" {
		t.Fatalf("tier_priority_verification version = %v, want t1d.v1", tpv["version"])
	}
	if tpv["mode"] != "verification_only" {
		t.Fatalf("tier_priority_verification mode = %v, want verification_only", tpv["mode"])
	}
	if tpv["status"] != "ready" {
		t.Fatalf("tier_priority_verification status = %v, want ready", tpv["status"])
	}
	if tpv["priority_verdict"] != "tier_priority_verification_shadow" {
		t.Fatalf("tier_priority_verification priority_verdict = %v, want tier_priority_verification_shadow", tpv["priority_verdict"])
	}
	if tpv["requires_manual_review"] != false {
		t.Fatalf("tier_priority_verification requires_manual_review = %v, want false", tpv["requires_manual_review"])
	}
	if tpv["saga_collision_policy"] != "none" {
		t.Fatalf("tier_priority_verification saga_collision_policy = %v, want none", tpv["saga_collision_policy"])
	}
	te, ok := tpv["tier_events"].([]map[string]any)
	if !ok || len(te) != 4 {
		t.Fatalf("tier_events mismatch: %v", tpv["tier_events"])
	}
}

func TestBuildRecallResultIntentExecutionShadowFailOpen(t *testing.T) {
	shadow := buildIntentExecutionShadow([]map[string]any{}, map[string]any{}, "long", q3PacketBudgetPolicy())
	if shadow["status"] != "off" {
		t.Fatalf("status = %v, want off", shadow["status"])
	}
	gt := shadow["guarded_takeover"].(map[string]any)
	if gt["decision"] != "fail_open" {
		t.Fatalf("guarded_takeover decision = %v, want fail_open", gt["decision"])
	}
	et := shadow["enforced_takeover"].(map[string]any)
	if et["decision"] != "fail_open" {
		t.Fatalf("enforced_takeover decision = %v, want fail_open", et["decision"])
	}
	tpv := shadow["tier_priority_verification"].(map[string]any)
	if tpv["status"] != "off" {
		t.Fatalf("tier_priority_verification status = %v, want off", tpv["status"])
	}
	if tpv["saga_collision_policy"] != "none" {
		t.Fatalf("tier_priority_verification saga_collision_policy = %v, want none", tpv["saga_collision_policy"])
	}
}

func TestBuildRecallResultIntentContractTierPrioritySurface(t *testing.T) {
	contract := buildIntentContractQ3()
	if contract["version"] != "q3a.v1" {
		t.Fatalf("version = %v, want q3a.v1", contract["version"])
	}
	rstp, ok := contract["routing_shadow_tier_priority"].(map[string]any)
	if !ok {
		t.Fatal("routing_shadow_tier_priority missing")
	}
	if rstp["version"] != "t1d.v1" {
		t.Fatalf("version = %v, want t1d.v1", rstp["version"])
	}
	if rstp["mode"] != "verification_only" {
		t.Fatalf("mode = %v, want verification_only", rstp["mode"])
	}
	if rstp["status"] != "shadow_only" {
		t.Fatalf("status = %v, want shadow_only", rstp["status"])
	}
	if rstp["priority_verdict"] != "tier_priority_verification_shadow" {
		t.Fatalf("priority_verdict = %v, want tier_priority_verification_shadow", rstp["priority_verdict"])
	}
	if rstp["requires_manual_review"] != false {
		t.Fatalf("requires_manual_review = %v, want false", rstp["requires_manual_review"])
	}
	tc, ok := rstp["tier_counts"].(map[string]int)
	if !ok {
		t.Fatal("tier_counts missing")
	}
	if tc["memory"] != 3 {
		t.Fatalf("memory tier count = %v, want 3", tc["memory"])
	}
	if tc["episode"] != 2 {
		t.Fatalf("episode tier count = %v, want 2", tc["episode"])
	}
	if tc["chapter"] != 2 {
		t.Fatalf("chapter tier count = %v, want 2", tc["chapter"])
	}
	if tc["arc"] != 3 {
		t.Fatalf("arc tier count = %v, want 3", tc["arc"])
	}
	if tc["saga"] != 2 {
		t.Fatalf("saga tier count = %v, want 2", tc["saga"])
	}
}

func TestBuildRecallResultIntentExecutionShadowUltraProfile(t *testing.T) {
	docs := []map[string]any{
		{"tier": "memory", "document_id": "d1", "text": "hello"},
		{"tier": "saga", "document_id": "d2", "text": "world"},
	}
	shadow := buildIntentExecutionShadow(docs, map[string]any{}, "ultra", q3PacketBudgetPolicy())
	tpv, ok := shadow["tier_priority_verification"].(map[string]any)
	if !ok {
		t.Fatal("tier_priority_verification missing")
	}
	if tpv["saga_collision_policy"] != "saga_floor_reserve_v0d" {
		t.Fatalf("saga_collision_policy = %v, want saga_floor_reserve_v0d", tpv["saga_collision_policy"])
	}
	te, ok := tpv["tier_events"].([]map[string]any)
	if !ok || len(te) != 4 {
		t.Fatalf("tier_events len = %v, want 4", len(te))
	}
}

func TestBuildRecallResultIntentExecutionShadowExtremeProfile(t *testing.T) {
	docs := []map[string]any{
		{"tier": "memory", "document_id": "d1", "text": "hello"},
	}
	shadow := buildIntentExecutionShadow(docs, map[string]any{}, "extreme", q3PacketBudgetPolicy())
	tpv, ok := shadow["tier_priority_verification"].(map[string]any)
	if !ok {
		t.Fatal("tier_priority_verification missing")
	}
	if tpv["saga_collision_policy"] != "saga_floor_reserve_v0d" {
		t.Fatalf("saga_collision_policy = %v, want saga_floor_reserve_v0d", tpv["saga_collision_policy"])
	}
}

func TestBuildRecallResultHierarchyConsistencyTrace(t *testing.T) {
	episodeSums := []store.EpisodeSummary{
		{ID: 10, ChatSessionID: "s1", FromTurn: 1, ToTurn: 3, SummaryText: "ep1"},
	}
	resumePack := &store.ResumePack{
		Chapter: &store.ChapterSummary{
			ID: 20, ChatSessionID: "s1", FromTurn: 1, ToTurn: 5, ChapterIndex: 1, ChapterTitle: "Ch1", SummaryText: "ch1",
		},
		Saga: &store.SagaDigest{
			ID: 40, ChatSessionID: "s1", FromTurn: 1, ToTurn: 20, EraLabel: "E1", SagaSummary: "s1",
		},
	}
	trace := buildHierarchyConsistencyTrace(nil, resumePack, episodeSums)
	if trace["version"] != "p59a.v1" {
		t.Fatalf("version mismatch: %v", trace["version"])
	}
	if trace["episode_present"] != true {
		t.Fatal("episode_present should be true")
	}
	if trace["chapter_present"] != true {
		t.Fatal("chapter_present should be true")
	}
	if trace["saga_present"] != true {
		t.Fatal("saga_present should be true")
	}
	if trace["chapter_episode_aligned"] != true {
		t.Fatalf("chapter_episode_aligned = %v, want true", trace["chapter_episode_aligned"])
	}
	if trace["consistency_score"] != 0.75 {
		t.Fatalf("consistency_score = %v, want 0.75", trace["consistency_score"])
	}
}

func TestBuildRecallResultSummaryFailureStability(t *testing.T) {
	chatLogs := []store.ChatLog{
		{ID: 1, ChatSessionID: "s1", TurnIndex: 1, Role: "user", Content: "hello world"},
	}
	stab := buildSummaryFailureStability(false, chatLogs)
	if stab["version"] != "p46a.v1" {
		t.Fatalf("version mismatch: %v", stab["version"])
	}
	if stab["last_good_fallback"] != "hello world" {
		t.Fatalf("last_good_fallback = %v, want hello world", stab["last_good_fallback"])
	}
	if stab["retry_ready"] != true {
		t.Fatal("retry_ready should be true")
	}
	if stab["continuity_guard"] != "trace_only" {
		t.Fatalf("continuity_guard = %v, want trace_only", stab["continuity_guard"])
	}
	stabDeg := buildSummaryFailureStability(true, nil)
	if stabDeg["retry_ready"] != false {
		t.Fatal("retry_ready should be false when degraded")
	}
}

func TestBuildRecallResultANNTakeoverGuard(t *testing.T) {
	ann := map[string]any{
		"benchmark": map[string]any{
			"overlap_ratio": 0.4,
		},
	}
	guard := buildANNTakeoverGuard(ann, map[string]any{"profile": "wide"})
	if guard["version"] != "p33a.v1" {
		t.Fatalf("version mismatch: %v", guard["version"])
	}
	if guard["profile"] != "wide" {
		t.Fatalf("profile = %v, want wide", guard["profile"])
	}
	if guard["overlap_threshold"] != 0.25 {
		t.Fatalf("overlap_threshold = %v, want 0.25", guard["overlap_threshold"])
	}
	if guard["guard_decision"] != "shadow_compare" {
		t.Fatalf("guard_decision = %v, want shadow_compare", guard["guard_decision"])
	}
	ev := guard["evidence"].(map[string]any)
	if ev["threshold_met"] != true {
		t.Fatal("threshold_met should be true")
	}
}

func TestPrepareTurnIntentExecutionShadow(t *testing.T) {
	fake := &turnRecordingStore{}
	cfg := config.Default()
	cfg.StoreMode = config.StoreModeDualShadow
	srv := NewServer(cfg)
	srv.Store = fake
	mux := http.NewServeMux()
	srv.RegisterRoutes(mux)

	body := `{"chat_session_id":"test-intent-shadow","raw_user_input":"query","settings":{"injection_enabled":true,"input_context_enabled":true}}`
	req := httptest.NewRequest(http.MethodPost, "/prepare-turn", bytes.NewReader([]byte(body)))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d", rec.Code)
	}
	var resp map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatal(err)
	}
	recall, ok := resp["recall_result"].(map[string]any)
	if !ok {
		t.Fatal("recall_result missing")
	}
	shadow, ok := recall["intent_execution_shadow"].(map[string]any)
	if !ok {
		t.Fatal("intent_execution_shadow missing")
	}
	if shadow["routing_mode"] != "per_intent_shadow" {
		t.Fatalf("routing_mode = %v", shadow["routing_mode"])
	}
}

func TestPrepareTurnBudgetEnforcement(t *testing.T) {
	fake := &turnRecordingStore{}
	cfg := config.Default()
	cfg.StoreMode = config.StoreModeDualShadow
	srv := NewServer(cfg)
	srv.Store = fake
	mux := http.NewServeMux()
	srv.RegisterRoutes(mux)

	body := `{"chat_session_id":"test-budget","raw_user_input":"query","settings":{"injection_enabled":true}}`
	req := httptest.NewRequest(http.MethodPost, "/prepare-turn", bytes.NewReader([]byte(body)))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d", rec.Code)
	}
	var resp map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatal(err)
	}
	recall := resp["recall_result"].(map[string]any)
	shadow := recall["intent_execution_shadow"].(map[string]any)
	be := shadow["budget_enforcement"].(map[string]any)
	if be == nil {
		t.Fatal("budget_enforcement missing")
	}
	if be["mode"] != "enforced_shadow" {
		t.Fatalf("mode = %v", be["mode"])
	}
}

func TestPrepareTurnTakeover(t *testing.T) {
	fake := &turnRecordingStore{}
	cfg := config.Default()
	cfg.StoreMode = config.StoreModeDualShadow
	srv := NewServer(cfg)
	srv.Store = fake
	mux := http.NewServeMux()
	srv.RegisterRoutes(mux)

	body := `{"chat_session_id":"test-takeover","raw_user_input":"query"}`
	req := httptest.NewRequest(http.MethodPost, "/prepare-turn", bytes.NewReader([]byte(body)))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d", rec.Code)
	}
	var resp map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatal(err)
	}
	recall := resp["recall_result"].(map[string]any)
	shadow := recall["intent_execution_shadow"].(map[string]any)
	gt := shadow["guarded_takeover"].(map[string]any)
	if gt == nil {
		t.Fatal("guarded_takeover missing")
	}
	et := shadow["enforced_takeover"].(map[string]any)
	if et == nil {
		t.Fatal("enforced_takeover missing")
	}
}

func TestPrepareTurnHierarchy(t *testing.T) {
	fake := &turnRecordingStore{}
	cfg := config.Default()
	cfg.StoreMode = config.StoreModeDualShadow
	srv := NewServer(cfg)
	srv.Store = fake
	mux := http.NewServeMux()
	srv.RegisterRoutes(mux)

	body := `{"chat_session_id":"test-hierarchy","raw_user_input":"query"}`
	req := httptest.NewRequest(http.MethodPost, "/prepare-turn", bytes.NewReader([]byte(body)))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d", rec.Code)
	}
	var resp map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatal(err)
	}
	recall := resp["recall_result"].(map[string]any)
	trace := recall["hierarchy_consistency_trace"].(map[string]any)
	if trace == nil {
		t.Fatal("hierarchy_consistency_trace missing")
	}
	if trace["version"] != "p59a.v1" {
		t.Fatalf("version = %v", trace["version"])
	}
}

func TestPrepareTurnSummary(t *testing.T) {
	fake := &turnRecordingStore{}
	cfg := config.Default()
	cfg.StoreMode = config.StoreModeDualShadow
	srv := NewServer(cfg)
	srv.Store = fake
	mux := http.NewServeMux()
	srv.RegisterRoutes(mux)

	body := `{"chat_session_id":"test-summary","raw_user_input":"query"}`
	req := httptest.NewRequest(http.MethodPost, "/prepare-turn", bytes.NewReader([]byte(body)))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d", rec.Code)
	}
	var resp map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatal(err)
	}
	recall := resp["recall_result"].(map[string]any)
	stab := recall["summary_failure_stability"].(map[string]any)
	if stab == nil {
		t.Fatal("summary_failure_stability missing")
	}
	if stab["version"] != "p46a.v1" {
		t.Fatalf("version = %v", stab["version"])
	}
	if stab["continuity_guard"] != "trace_only" {
		t.Fatalf("continuity_guard = %v", stab["continuity_guard"])
	}
}

func TestPrepareTurnANN(t *testing.T) {
	fake := &turnRecordingStore{}
	cfg := config.Default()
	cfg.StoreMode = config.StoreModeDualShadow
	srv := NewServer(cfg)
	srv.Store = fake
	mux := http.NewServeMux()
	srv.RegisterRoutes(mux)

	body := `{"chat_session_id":"test-ann","raw_user_input":"query"}`
	req := httptest.NewRequest(http.MethodPost, "/prepare-turn", bytes.NewReader([]byte(body)))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d", rec.Code)
	}
	var resp map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatal(err)
	}
	recall := resp["recall_result"].(map[string]any)
	guard := recall["ann_default_takeover_guard"].(map[string]any)
	if guard == nil {
		t.Fatal("ann_default_takeover_guard missing")
	}
	if guard["version"] != "p33a.v1" {
		t.Fatalf("version = %v", guard["version"])
	}
}
