package httpapi

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/risulongmemory/archive-center-go/internal/config"
	"github.com/risulongmemory/archive-center-go/internal/store"
	"github.com/risulongmemory/archive-center-go/internal/vector"
)

// Store I/O is the boundary fixture. Routing, raw repair, source registration,
// Critic request/parsing, admission, job reuse and completion are production.
type worldlineRecoveryStore struct {
	*durableSessionIdentityBindingStore
	mu        sync.Mutex
	rows      []store.ChatLog
	revisions map[string]store.MemorySourceRevision
	audits    []store.AuditLog
	jobs      []store.MemoryReprocessingJob
}

func (f *worldlineRecoveryStore) ListChatLogs(_ context.Context, sid string, from, to int) ([]store.ChatLog, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	var out []store.ChatLog
	for _, row := range f.rows {
		if row.ChatSessionID == sid && row.TurnIndex >= from && (to == 0 || row.TurnIndex <= to) {
			out = append(out, row)
		}
	}
	return out, nil
}
func (f *worldlineRecoveryStore) SaveChatLog(_ context.Context, row *store.ChatLog) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	for _, old := range f.rows {
		if old.ChatSessionID == row.ChatSessionID && old.TurnIndex == row.TurnIndex && old.Role == row.Role {
			if old.Content != row.Content {
				return fmt.Errorf("raw conflict")
			}
			return nil
		}
	}
	f.rows = append(f.rows, *row)
	return nil
}
func (f *worldlineRecoveryStore) ListActiveSourceRevisions(_ context.Context, sid string, from, to int) ([]store.MemorySourceRevision, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	var out []store.MemorySourceRevision
	for _, item := range f.revisions {
		if item.ChatSessionID == sid && item.LifecycleState == "active" && item.TurnIndex >= from && (to == 0 || item.TurnIndex <= to) {
			out = append(out, item)
		}
	}
	return out, nil
}
func (f *worldlineRecoveryStore) GetSourceRevision(_ context.Context, sid, revision string) (*store.MemorySourceRevision, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	item, ok := f.revisions[revision]
	if !ok || item.ChatSessionID != sid {
		return nil, store.ErrNotFound
	}
	return &item, nil
}

func (f *worldlineRecoveryStore) ListSourceRevisions(ctx context.Context, sid string, from, to int) ([]store.MemorySourceRevision, error) {
	return f.ListActiveSourceRevisions(ctx, sid, from, to)
}
func (f *worldlineRecoveryStore) RegisterAcceptedSourceRevision(_ context.Context, source *store.MemorySourceRevision) (store.SourceRevisionRegistration, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	for _, old := range f.revisions {
		if old.ChatSessionID == source.ChatSessionID && old.TurnIndex == source.TurnIndex {
			if old.SourceRevision == source.SourceRevision {
				return store.SourceRevisionRegistration{Idempotent: true}, nil
			}
			return store.SourceRevisionRegistration{}, store.ErrSourceRevisionConflict
		}
	}
	f.revisions[source.SourceRevision] = *source
	return store.SourceRevisionRegistration{Inserted: true}, nil
}
func (f *worldlineRecoveryStore) IsSourceRevisionActive(ctx context.Context, sid, rev string) (bool, error) {
	s, e := f.GetSourceRevision(ctx, sid, rev)
	return s != nil && s.LifecycleState == "active", e
}
func (f *worldlineRecoveryStore) InvalidateSourceRevisions(context.Context, string, int, string, string, time.Time) error {
	return fmt.Errorf("unexpected invalidation")
}
func (f *worldlineRecoveryStore) SaveCriticInputSnapshot(_ context.Context, sid, rev, body, hash string, _ time.Time) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	s := f.revisions[rev]
	s.CriticInputSnapshotJSON, s.CriticInputSnapshotHash = body, hash
	f.revisions[rev] = s
	return nil
}
func (f *worldlineRecoveryStore) MemoryAdmissionWritesEnabled() bool     { return true }
func (f *worldlineRecoveryStore) MemoryDerivationLifecycleEnabled() bool { return true }
func (f *worldlineRecoveryStore) CommitMemoryAdmission(_ context.Context, item *store.MemoryAdmission) (store.MemoryAdmissionResult, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	s := f.revisions[item.SourceRevision]
	s.DerivedAdmissionState = "committed"
	s.DerivedAdmissionVersion = item.DerivationVersion
	s.DerivedExtractorVersion = item.ExtractorVersion
	s.DerivedIndexVersion = item.IndexVersion
	s.DerivedResultJSON = item.ResultJSON
	s.DerivedResultHash = item.ResultHash
	f.revisions[item.SourceRevision] = s
	return store.MemoryAdmissionResult{CommittedResultHash: item.ResultHash}, nil
}
func (f *worldlineRecoveryStore) SaveAuditLog(_ context.Context, item *store.AuditLog) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.audits = append(f.audits, *item)
	return nil
}
func (f *worldlineRecoveryStore) ListAuditLogs(_ context.Context, sid, event string, _ int) ([]store.AuditLog, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	var out []store.AuditLog
	for _, a := range f.audits {
		if a.ChatSessionID == sid && (event == "" || a.EventType == event) {
			out = append(out, a)
		}
	}
	return out, nil
}
func (f *worldlineRecoveryStore) EnqueueMemoryReprocessingJob(_ context.Context, j *store.MemoryReprocessingJob) (bool, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	for _, old := range f.jobs {
		if old.IdempotencyKey == j.IdempotencyKey {
			return false, nil
		}
	}
	f.jobs = append(f.jobs, *j)
	return true, nil
}
func (f *worldlineRecoveryStore) ClaimMemoryReprocessingJob(context.Context, string, time.Time, time.Duration) (*store.MemoryReprocessingJob, error) {
	return nil, store.ErrNotFound
}
func (f *worldlineRecoveryStore) CompleteMemoryReprocessingJob(context.Context, int64, string, time.Time) error {
	return nil
}
func (f *worldlineRecoveryStore) FailMemoryReprocessingJob(context.Context, int64, string, time.Time, time.Time, bool, string) error {
	return nil
}

func newWorldlineRecoveryFixture() (*Server, *worldlineRecoveryStore) {
	f := &worldlineRecoveryStore{durableSessionIdentityBindingStore: &durableSessionIdentityBindingStore{Store: store.NewNoopStore(), bindings: map[string]string{"stable\x00parent": "A", "stable\x00child": "B", "stable\x00sibling": "C"}}, revisions: map[string]store.MemorySourceRevision{}}
	s := &Server{Store: f, Cfg: config.Default(), Vector: vector.NewFakeVectorStore(), AdminJobs: newAdminJobManager(), SourceAcceptances: newCompleteTurnSourceAcceptanceLedger(), RuntimeConfig: RuntimeConfig{Synced: true, CriticProvider: "openai", CriticAPIKey: "synthetic", CriticEndpoint: "https://example.invalid/v1", CriticModel: "critic-fixture", CriticTimeoutSec: 30, FailedQueueMaxAttempts: 3}}
	return s, f
}
func recoveryRoute(t *testing.T, s *Server, req sessionRoutingTurnResolutionRequest) sessionRoutingTurnResolutionResponse {
	t.Helper()
	body, _ := json.Marshal(req)
	w := httptest.NewRecorder()
	s.handleSessionRoutingTurnResolution(w, httptest.NewRequest("POST", "/session-routing/turn-resolution", bytes.NewReader(body)))
	var out sessionRoutingTurnResolutionResponse
	if w.Code != 200 || json.Unmarshal(w.Body.Bytes(), &out) != nil || out.Status != "ok" {
		t.Fatalf("route: %s", w.Body.String())
	}
	return out
}
func waitRecoveryJob(t *testing.T, s *Server, job map[string]any) map[string]any {
	t.Helper()
	deadline := time.Now().Add(10 * time.Second)
	id := stringFromAny(job["job_id"])
	if id == "" {
		id = stringFromAny(job["id"])
	}
	for time.Now().Before(deadline) {
		v, ok := s.AdminJobs.get(id)
		if !ok {
			t.Fatalf("job missing: %+v", job)
		}
		if adminJobTerminal(stringFromAny(v["status"])) {
			return v
		}
		time.Sleep(time.Millisecond * 5)
	}
	t.Fatal("job timeout")
	return nil
}

func TestWorldlineSourceRecoveryProduction(t *testing.T) {
	s, f := newWorldlineRecoveryFixture()
	old := proxyHTTPClient
	defer func() { proxyHTTPClient = old }()
	var calls atomic.Int32
	proxyHTTPClient = &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		calls.Add(1)
		if r.URL.Host != "example.invalid" {
			return nil, fmt.Errorf("unexpected provider %s", r.URL)
		}
		body, _ := json.Marshal(map[string]any{"choices": []any{map[string]any{"message": map[string]any{"content": criticWireJSONForTest(map[string]any{"turn_summary": "", "importance_score": 0})}}}})
		return &http.Response{StatusCode: 200, Header: make(http.Header), Body: io.NopCloser(bytes.NewReader(body))}, nil
	})}
	parents := []risuWorldlineMessageObservation{{MessageIndex: 0, Role: "user", MessageChatID: "u"}, {MessageIndex: 1, Role: "char", MessageChatID: "a"}}
	req := originBranchRequest("parent", "child", parents, true)
	req.ChatSessionID = "B"
	req.Mode = "recover_inherited"
	first := recoveryRoute(t, s, req)
	if len(first.InheritedRecovery) != 1 {
		t.Fatalf("missing recovery plan: %+v", first)
	}
	plan := first.InheritedRecovery[0]
	if plan.SessionID != "A" || plan.TurnIndex != first.Worldline.InheritedThroughTurn {
		t.Fatalf("wrong owner: %+v", plan)
	}
	req.InheritedRecovery = []worldlineSourceRecoveryObservation{{MessageIndex: plan.MessageIndex, UserContent: "Open the gate.", AssistantContent: "The gate opened."}}
	req.RecoveryClientMeta = map[string]any{"critic_input_budget_observation": map[string]any{"contract_version": "critic_input_budget_observation.v1", "max_input_context_chars": 2468}}
	result := recoveryRoute(t, s, req)
	done := waitRecoveryJob(t, s, result.InheritedRecovery[0].Job)
	if done["status"] != "completed" {
		t.Fatalf("recovery failed: %+v", done)
	}
	if calls.Load() != 1 {
		t.Fatalf("critic calls=%d", calls.Load())
	}
	rows, _ := f.ListChatLogs(context.Background(), "A", plan.TurnIndex, plan.TurnIndex)
	if len(rows) != 2 {
		t.Fatalf("raw=%+v", rows)
	}
	childRows, _ := f.ListChatLogs(context.Background(), "B", 0, 0)
	if len(childRows) != 0 {
		t.Fatal("copied inherited raw into child")
	}
	repeat := recoveryRoute(t, s, req)
	if len(repeat.InheritedRecovery) != 0 {
		t.Fatalf("completed zero-memory result requested again: %+v", repeat.InheritedRecovery)
	}
	sibling := originBranchRequest("parent", "sibling", parents, false)
	sibling.ChatSessionID = "C"
	sibling.Mode = "recover_inherited"
	if got := recoveryRoute(t, s, sibling); len(got.InheritedRecovery) != 0 {
		t.Fatal("sibling repeated completed source")
	}
	sources, _ := f.ListActiveSourceRevisions(context.Background(), "A", 0, 0)
	if len(sources) != 1 || !strings.Contains(sources[0].CriticInputSnapshotJSON, "2468") {
		t.Fatalf("source/budget: %+v", sources)
	}
	// Late delivery from the original parent's next-input marker reuses the
	// recovered source; changed text and a new assistant row remain replacements.
	normal := completeTurnAnchoredAcceptanceTestRequest("A", plan.TurnIndex, "Open the gate.", "The gate opened.", time.Now().UnixMilli(), "g", "not_streaming", 0, 1, 2)
	obs, _ := completeTurnSourceObservationFromMeta(normal.ClientMeta)
	obs.MessageChatID = "a"
	obs.HostChatID, obs.HostChatIDState, obs.UserMessageChatID, obs.UserMessageChatIDState = "parent", "observed", "u", "observed"
	normal.ClientMeta["source_acceptance_observation"] = obs
	if !s.reuseWorldlineRecoveredCompletion(context.Background(), httptest.NewRecorder(), normal) {
		t.Fatal("late parent did not reuse")
	}
	changed := "The gate remained closed."
	normal.AssistantContent = &changed
	if s.reuseWorldlineRecoveredCompletion(context.Background(), httptest.NewRecorder(), normal) {
		t.Fatal("edit suppressed")
	}
	unchanged := "The gate opened."
	normal.AssistantContent = &unchanged
	obs.MessageChatID = "new-assistant"
	normal.ClientMeta["source_acceptance_observation"] = obs
	if s.reuseWorldlineRecoveredCompletion(context.Background(), httptest.NewRecorder(), normal) {
		t.Fatal("new assistant reroll suppressed")
	}
	obs.MessageChatID = ""
	obs.UserMessageChatID = "different-user-row"
	normal.ClientMeta["source_acceptance_observation"] = obs
	if s.reuseWorldlineRecoveredCompletion(context.Background(), httptest.NewRecorder(), normal) {
		t.Fatal("identical text in a new user row reused old turn")
	}
	if calls.Load() != 1 {
		t.Fatalf("duplicate calls=%d", calls.Load())
	}
}

func TestWorldlineSourceRecoveryUserForkExcludesPendingResponse(t *testing.T) {
	s, _ := newWorldlineRecoveryFixture()
	parents := []risuWorldlineMessageObservation{{MessageIndex: 0, Role: "user", MessageChatID: "u1"}, {MessageIndex: 1, Role: "char", MessageChatID: "a1"}, {MessageIndex: 2, Role: "user", MessageChatID: "u2"}}
	req := originBranchRequest("parent", "child", parents, false)
	req.ChatSessionID = "B"
	req.Mode = "recover_inherited"
	got := recoveryRoute(t, s, req)
	if len(got.InheritedRecovery) != 1 || got.InheritedRecovery[0].TurnIndex >= got.Worldline.ForkTurn {
		t.Fatalf("user-fork future response included: %+v", got)
	}
	// A real normal worker already owns the missing Critic work.
	p := got.InheritedRecovery[0]
	s.SourceAcceptances.workers[sourceAcceptanceStateKey("A", p.TurnIndex)] = completeTurnSourceAcceptanceWorker{done: make(chan struct{})}
	result, err := s.runWorldlineSourceRecovery(context.Background(), p, worldlineSourceRecoveryObservation{UserContent: "u", AssistantContent: "a"}, nil)
	if err != nil || result["status"] != "processing" {
		t.Fatalf("did not reuse worker: %v %+v", err, result)
	}
}

func TestWorldlineSourceRecoveryProductionJS(t *testing.T) {
	s, f := newWorldlineRecoveryFixture()
	// Missing Critic configuration is an external boundary outcome here. Raw
	// repair must still save the original; the full provider path is tested above.
	s.RuntimeConfig = RuntimeConfig{Synced: true, FailedQueueMaxAttempts: 3}
	local := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/session-routing/turn-resolution" {
			t.Errorf("unexpected route %s", r.URL.Path)
			http.NotFound(w, r)
			return
		}
		s.handleSessionRoutingTurnResolution(w, r)
	}))
	defer local.Close()
	root, _ := filepath.Abs(filepath.Join("..", "..", ".."))
	out, err := exec.Command("node", filepath.Join(root, "tests", "fixtures", "worldline-recovery-probe.cjs"), filepath.Join(root, "Archive Center.js"), local.URL).CombinedOutput()
	if err != nil {
		t.Fatalf("JS integration: %v\n%s", err, out)
	}
	var report struct {
		Items []worldlineSourceRecoveryPlan `json:"items"`
	}
	if err = json.Unmarshal(out, &report); err != nil {
		t.Fatal(err)
	}
	if len(report.Items) != 1 {
		t.Fatalf("report=%s", out)
	}
	waitRecoveryJob(t, s, report.Items[0].Job)
	rows, _ := f.ListChatLogs(context.Background(), "A", 0, 0)
	if len(rows) != 2 {
		t.Fatalf("rows=%+v", rows)
	}
	for _, row := range rows {
		if row.Role == "assistant" && row.Content != "The brass key opens the east gate." {
			t.Fatalf("wrong original %q", row.Content)
		}
	}
}

func TestWorldlineSourceRecoveryNestedOwnersAndOffset(t *testing.T) {
	s, f := newWorldlineRecoveryFixture()
	rootMessages := []risuWorldlineMessageObservation{{MessageIndex: 0, Role: "user", MessageChatID: "u78"}, {MessageIndex: 1, Role: "char", MessageChatID: "a78"}, {MessageIndex: 2, Role: "user", MessageChatID: "u79"}, {MessageIndex: 3, Role: "char", MessageChatID: "a79"}}
	anchor := activeSourceRevisionForUserAnchor("A", "parent", "u78", "a78", 78)
	anchor.ChatSessionID, anchor.SourceRevision = "A", "fixture-anchor-78"
	f.revisions[anchor.SourceRevision] = anchor
	rootReq := originBranchRequest("parent", "child", rootMessages, false)
	rootReq.ChatSessionID = "B"
	rootVM := s.resolveRisuWorldlineObservation(context.Background(), rootReq, "B")
	if rootVM.ForkTurn != anchor.TurnIndex+1 {
		t.Fatalf("offset lost: %+v", rootVM)
	}
	childMessages := append([]risuWorldlineMessageObservation(nil), rootMessages...)
	childMessages = append(childMessages, risuWorldlineMessageObservation{MessageIndex: 4, Role: "char", MessageChatID: "marker", Disabled: true}, risuWorldlineMessageObservation{MessageIndex: 5, Role: "user", MessageChatID: "u80"}, risuWorldlineMessageObservation{MessageIndex: 6, Role: "char", MessageChatID: "a80"})
	req := originBranchRequest("child", "sibling", childMessages, true)
	req.ChatSessionID = "C"
	req.Mode = "recover_inherited"
	req.WorldlineObservation.MessageOrigins.ParentObservation = rootReq.WorldlineObservation
	got := recoveryRoute(t, s, req)
	owners := map[int]string{}
	for _, p := range got.InheritedRecovery {
		owners[p.TurnIndex] = p.SessionID
	}
	if owners[rootVM.ForkTurn] != "A" || owners[rootVM.ForkTurn+1] != "B" {
		t.Fatalf("nested owners=%+v worldline=%+v", owners, got.Worldline)
	}
	for turn := range owners {
		if turn > got.Worldline.InheritedThroughTurn {
			t.Fatalf("future inherited: %d", turn)
		}
	}
}

func TestWorldlineSourceRecoveryConcurrentBranchesReuseJob(t *testing.T) {
	s, _ := newWorldlineRecoveryFixture()
	old := proxyHTTPClient
	defer func() { proxyHTTPClient = old }()
	entered, release := make(chan struct{}), make(chan struct{})
	var calls atomic.Int32
	proxyHTTPClient = &http.Client{Transport: roundTripFunc(func(*http.Request) (*http.Response, error) {
		if calls.Add(1) == 1 {
			close(entered)
		}
		<-release
		body, _ := json.Marshal(map[string]any{"choices": []any{map[string]any{"message": map[string]any{"content": criticWireJSONForTest(map[string]any{"turn_summary": "", "importance_score": 0})}}}})
		return &http.Response{StatusCode: 200, Header: make(http.Header), Body: io.NopCloser(bytes.NewReader(body))}, nil
	})}
	plan := worldlineSourceRecoveryPlan{SessionID: "A", TurnIndex: 79, hostID: "parent", userID: "u", assistantID: "a"}
	obs := worldlineSourceRecoveryObservation{UserContent: "Open.", AssistantContent: "It opened."}
	first := s.startWorldlineSourceRecovery(plan, obs, nil)
	select {
	case <-entered:
	case <-time.After(5 * time.Second):
		close(release)
		t.Fatal("critic not entered")
	}
	second := s.startWorldlineSourceRecovery(plan, obs, nil)
	if second["reused_running_job"] != true {
		close(release)
		t.Fatalf("running job not reused: %+v", second)
	}
	close(release)
	done := waitRecoveryJob(t, s, first)
	if done["status"] != "completed" || calls.Load() != 1 {
		t.Fatalf("done=%+v calls=%d", done, calls.Load())
	}
}

func TestWorldlineSourceRecoveryFailureUsesExistingRetryLimit(t *testing.T) {
	s, f := newWorldlineRecoveryFixture()
	old := proxyHTTPClient
	defer func() { proxyHTTPClient = old }()
	calls := 0
	proxyHTTPClient = &http.Client{Transport: roundTripFunc(func(*http.Request) (*http.Response, error) {
		calls++
		return &http.Response{StatusCode: 200, Header: make(http.Header), Body: io.NopCloser(strings.NewReader(`{"choices":[{"finish_reason":"length","message":{"content":"{"}}]}`))}, nil
	})}
	plan := worldlineSourceRecoveryPlan{SessionID: "A", TurnIndex: 79, hostID: "parent", userID: "u", assistantID: "a"}
	obs := worldlineSourceRecoveryObservation{UserContent: "Open.", AssistantContent: "It opened."}
	_, err := s.runWorldlineSourceRecovery(context.Background(), plan, obs, nil)
	if err == nil {
		t.Fatal("invalid critic incorrectly completed")
	}
	if calls != 1 || len(f.jobs) != 1 {
		t.Fatalf("first attempt calls=%d jobs=%d", calls, len(f.jobs))
	}
	result, err := s.runWorldlineSourceRecovery(context.Background(), plan, obs, nil)
	if err != nil || result["status"] != "queued_existing_reprocessing" || calls != 1 || len(f.jobs) != 1 {
		t.Fatalf("branch bypassed retry queue: %+v %v calls=%d jobs=%d", result, err, calls, len(f.jobs))
	}
	rows, _ := f.ListChatLogs(context.Background(), "A", 79, 79)
	if len(rows) != 2 {
		t.Fatal("critic failure lost raw")
	}
}
