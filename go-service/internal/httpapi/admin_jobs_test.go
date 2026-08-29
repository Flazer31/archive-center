package httpapi

import (
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
)

func TestAdminJobManagerReusesRunningJobForSameKindAndSession(t *testing.T) {
	manager := newAdminJobManager()
	started := make(chan struct{})
	release := make(chan struct{})

	first := manager.start("rescan", "session-one", map[string]any{"background": true}, func(context.Context, adminJobProgressFunc) (map[string]any, error) {
		close(started)
		<-release
		return map[string]any{"status": "ok"}, nil
	})
	select {
	case <-started:
	case <-time.After(time.Second):
		t.Fatal("first background job did not start")
	}

	secondWorkCalled := false
	second := manager.start("rescan", "session-one", map[string]any{"background": true}, func(context.Context, adminJobProgressFunc) (map[string]any, error) {
		secondWorkCalled = true
		return map[string]any{"status": "unexpected"}, nil
	})
	if first["job_id"] != second["job_id"] {
		t.Fatalf("same-session rescan started twice: first=%v second=%v", first["job_id"], second["job_id"])
	}
	if second["reused_running_job"] != true {
		t.Fatalf("reused_running_job = %v, want true", second["reused_running_job"])
	}
	if secondWorkCalled {
		t.Fatal("duplicate background work was invoked")
	}
	close(release)
}

func TestAdminJobManagerSessionNormalizeDeferredIsTerminalButNotCompleted(t *testing.T) {
	manager := newAdminJobManager()
	job := manager.start("session_normalize", "session-deferred", map[string]any{"background": true}, func(_ context.Context, progress adminJobProgressFunc) (map[string]any, error) {
		progress(map[string]any{
			"status":           "completed",
			"progress_percent": 100,
			"pending_count":    1,
			"pending_turns":    []int{64},
			"failed_count":     1,
			"failed_turns":     []map[string]any{{"turn_index": 64, "reason": "critic_retry_queued"}},
			"outcome_status":   "partial_deferred",
		})
		return map[string]any{
			"status": "partial_deferred",
			"rescan": map[string]any{
				"status":         "deferred",
				"deferred":       1,
				"deferred_turns": []map[string]any{{"turn_index": 64, "reason": "critic_retry_queued"}},
			},
		}, nil
	})

	snapshot := waitForAdminJobTerminal(t, manager, job["job_id"].(string))
	if snapshot["status"] != "deferred" || snapshot["terminal"] != true {
		t.Fatalf("deferred normalize snapshot = %#v", snapshot)
	}
	progress := snapshot["progress"].(map[string]any)
	if boolFromAny(progress["done"]) {
		t.Fatalf("deferred normalize reported done: %#v", progress)
	}
	if got := intFromAny(progress["progress_percent"], -1); got >= 100 {
		t.Fatalf("deferred normalize progress_percent=%d, want below 100", got)
	}
	if got := intFromAny(progress["pending_count"], 0); got != 1 {
		t.Fatalf("pending_count=%d, want 1: %#v", got, progress)
	}
}

func TestAdminJobManagerSessionNormalizePartialErrorPreservesSuccessfulProgress(t *testing.T) {
	manager := newAdminJobManager()
	job := manager.start("session_normalize", "session-partial", nil, func(_ context.Context, progress adminJobProgressFunc) (map[string]any, error) {
		progress(map[string]any{
			"processed":        3,
			"succeeded":        2,
			"failed_count":     1,
			"processed_turns":  []int{1, 2},
			"failed_turns":     []map[string]any{{"turn_index": 3, "reason": "critic_terminal_failure"}},
			"progress_percent": 100,
		})
		return map[string]any{"status": "partial_error"}, nil
	})

	snapshot := waitForAdminJobTerminal(t, manager, job["job_id"].(string))
	if snapshot["status"] != "partial_error" || snapshot["terminal"] != true {
		t.Fatalf("partial normalize snapshot = %#v", snapshot)
	}
	progress := snapshot["progress"].(map[string]any)
	if got := intFromAny(progress["succeeded"], 0); got != 2 {
		t.Fatalf("succeeded=%d, want 2: %#v", got, progress)
	}
	if got := intFromAny(progress["failed_count"], 0); got != 1 {
		t.Fatalf("failed_count=%d, want 1: %#v", got, progress)
	}
	if boolFromAny(progress["done"]) {
		t.Fatalf("partial normalize reported done: %#v", progress)
	}
}

func TestAdminJobManagerSessionNormalizeRerunCompletesAfterDeferredOutcome(t *testing.T) {
	manager := newAdminJobManager()
	first := manager.start("session_normalize", "session-rerun", nil, func(_ context.Context, _ adminJobProgressFunc) (map[string]any, error) {
		return map[string]any{"status": "partial_deferred"}, nil
	})
	firstSnapshot := waitForAdminJobTerminal(t, manager, first["job_id"].(string))
	if firstSnapshot["status"] != "deferred" {
		t.Fatalf("first normalize status=%v, want deferred", firstSnapshot["status"])
	}

	second := manager.start("session_normalize", "session-rerun", nil, func(_ context.Context, _ adminJobProgressFunc) (map[string]any, error) {
		return map[string]any{"status": "ok"}, nil
	})
	if second["job_id"] == first["job_id"] {
		t.Fatalf("terminal deferred job was reused: first=%v second=%v", first["job_id"], second["job_id"])
	}
	secondSnapshot := waitForAdminJobTerminal(t, manager, second["job_id"].(string))
	if secondSnapshot["status"] != "completed" || secondSnapshot["terminal"] != true {
		t.Fatalf("rerun normalize snapshot = %#v", secondSnapshot)
	}
	progress := secondSnapshot["progress"].(map[string]any)
	if progress["done"] != true || intFromAny(progress["progress_percent"], 0) != 100 {
		t.Fatalf("successful rerun progress = %#v", progress)
	}
}

func TestAdminJobManagerOtherKindsKeepCompletedTerminalStatus(t *testing.T) {
	manager := newAdminJobManager()
	job := manager.start("rescan", "session-generic", nil, func(_ context.Context, _ adminJobProgressFunc) (map[string]any, error) {
		return map[string]any{"status": "partial_error", "failed": 1}, nil
	})
	snapshot := waitForAdminJobTerminal(t, manager, job["job_id"].(string))
	if snapshot["status"] != "completed" {
		t.Fatalf("generic admin job status=%v, want existing completed behavior", snapshot["status"])
	}
}

func TestAdminSessionNormalizeHandlerReportsQueuedDerivedRetryAsDeferred(t *testing.T) {
	st := newAdminDuplicateReprocessingStore()
	st.source.DerivedAdmissionState = "pending"
	st.source.DerivedAdmissionVersion = ""
	st.source.DerivedExtractorVersion = ""
	st.source.DerivedIndexVersion = ""
	st.source.DerivedResultHash = ""
	st.source.DerivedResultJSON = ""
	st.enqueueNew = true

	cfg := config.Default()
	cfg.StoreMode = config.StoreModeMariaDBShadow
	server := NewServer(cfg)
	server.Store = st
	server.StoreOpenError = nil
	mux := http.NewServeMux()
	server.RegisterRoutes(mux)

	req := httptest.NewRequest(http.MethodPost, "/admin/session-normalize", strings.NewReader(`{
		"chat_session_id":"session",
		"turn_indices":[4],
		"skip_repair":true
	}`))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusAccepted {
		t.Fatalf("normalize start status=%d body=%s", rec.Code, rec.Body.String())
	}
	var started map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &started); err != nil {
		t.Fatalf("decode normalize start: %v", err)
	}
	jobID := strings.TrimSpace(stringFromAny(started["job_id"]))
	if jobID == "" {
		t.Fatalf("normalize job id missing: %#v", started)
	}

	job := waitForAdminJobTerminal(t, server.AdminJobs, jobID)
	if job["status"] != "deferred" || job["terminal"] != true {
		t.Fatalf("normalize job terminal state=%#v", job)
	}
	progress := job["progress"].(map[string]any)
	if got := intFromAny(progress["progress_percent"], -1); got >= 100 {
		t.Fatalf("queued retry progress_percent=%d, want below 100: %#v", got, progress)
	}
	if got := intFromAny(progress["pending_count"], 0); got != 1 {
		t.Fatalf("pending_count=%d, want 1: %#v", got, progress)
	}
	if boolFromAny(progress["pending_turns_available"]) {
		t.Fatalf("aggregate-only queue claimed an exact pending turn list: %#v", progress)
	}
	if progress["pending_turns_unavailable_reason"] != "durable_reprocessing_result_has_aggregate_count_only" {
		t.Fatalf("pending turn reason=%v", progress["pending_turns_unavailable_reason"])
	}
	if progress["reason"] != "derived_reprocessing_pending" {
		t.Fatalf("progress reason=%v, want derived_reprocessing_pending", progress["reason"])
	}
	result := job["result"].(map[string]any)
	if result["status"] != "partial_deferred" ||
		mapFromAny(result["reindex"])["status"] != "deferred" ||
		len(st.enqueuedJobs) != 1 {
		t.Fatalf("normalize result=%#v enqueued=%d", result, len(st.enqueuedJobs))
	}
}

func waitForAdminJobTerminal(t *testing.T, manager *adminJobManager, jobID string) map[string]any {
	t.Helper()
	deadline := time.Now().Add(time.Second)
	for time.Now().Before(deadline) {
		snapshot, ok := manager.get(jobID)
		if !ok {
			t.Fatalf("admin job %s disappeared", jobID)
		}
		if boolFromAny(snapshot["terminal"]) {
			return snapshot
		}
		time.Sleep(time.Millisecond)
	}
	t.Fatalf("admin job %s did not become terminal", jobID)
	return nil
}

func TestAdminJobDeleteCancelsManagerOwnedContextAndStaysTerminal(t *testing.T) {
	manager := newAdminJobManager()
	started := make(chan struct{})
	exited := make(chan struct{})
	job := manager.start("rescan", "session-cancel", map[string]any{"background": true}, func(ctx context.Context, _ adminJobProgressFunc) (map[string]any, error) {
		close(started)
		<-ctx.Done()
		close(exited)
		return nil, ctx.Err()
	})
	select {
	case <-started:
	case <-time.After(time.Second):
		t.Fatal("background job did not start")
	}

	server := &Server{AdminJobs: manager}
	mux := http.NewServeMux()
	server.registerAdminRoutes(mux)
	jobID := job["job_id"].(string)
	req := httptest.NewRequest(http.MethodDelete, "/admin/jobs/"+jobID, nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("cancel status=%d body=%s", rec.Code, rec.Body.String())
	}
	select {
	case <-exited:
	case <-time.After(time.Second):
		t.Fatal("manager cancellation did not reach job context")
	}
	snapshot, ok := manager.get(jobID)
	if !ok || snapshot["status"] != "cancelled" || snapshot["finished_at"] == nil {
		t.Fatalf("cancelled snapshot=%#v found=%t", snapshot, ok)
	}
}

func TestAdminJobProgressPublishesStageScopedDisplayTotal(t *testing.T) {
	manager := newAdminJobManager()
	manager.jobs["job-display-total"] = &adminBackgroundJob{
		ID:       "job-display-total",
		Status:   "running",
		Progress: map[string]any{"stage": "queued", "candidate_count": 0, "display_total": 0},
		changed:  make(chan struct{}),
	}
	manager.order = append(manager.order, "job-display-total")

	manager.update("job-display-total", "running", map[string]any{
		"stage": "raw_repair_replay", "candidate_count": 3,
	})
	first, ok := manager.get("job-display-total")
	if !ok {
		t.Fatal("job disappeared")
	}
	firstProgress := first["progress"].(map[string]any)
	if got := intFromAny(firstProgress["display_total"], -1); got != 3 {
		t.Fatalf("raw repair display_total=%d, want 3", got)
	}

	manager.update("job-display-total", "running", map[string]any{
		"stage": "inspect_after", "progress_percent": 90,
	})
	second, _ := manager.get("job-display-total")
	secondProgress := second["progress"].(map[string]any)
	if got := intFromAny(secondProgress["display_total"], -1); got != 0 {
		t.Fatalf("stage transition retained stale display_total=%d", got)
	}
}

func TestAdminJobEventsStreamPublishesChangedRevisionsUntilTerminal(t *testing.T) {
	manager := newAdminJobManager()
	started := make(chan struct{})
	release := make(chan struct{})
	job := manager.start("session_normalize", "session-stream", map[string]any{"background": true}, func(_ context.Context, progress adminJobProgressFunc) (map[string]any, error) {
		progress(map[string]any{"stage": "raw_repair_replay", "processed": 0, "candidate_count": 2})
		close(started)
		<-release
		progress(map[string]any{"stage": "raw_repair_replay", "processed": 2, "candidate_count": 2})
		return map[string]any{"status": "ok"}, nil
	})
	select {
	case <-started:
	case <-time.After(time.Second):
		t.Fatal("background job did not reach its first progress revision")
	}

	server := &Server{AdminJobs: manager}
	mux := http.NewServeMux()
	server.registerAdminRoutes(mux)
	httpServer := httptest.NewServer(mux)
	defer httpServer.Close()

	jobID := job["job_id"].(string)
	response, err := http.Get(httpServer.URL + "/admin/jobs/" + jobID + "/events?after_revision=0")
	if err != nil {
		t.Fatalf("open admin job event stream: %v", err)
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		t.Fatalf("event stream status=%d", response.StatusCode)
	}
	close(release)

	decoder := json.NewDecoder(response.Body)
	revisions := []uint64{}
	var terminal map[string]any
	for {
		var snapshot map[string]any
		if err := decoder.Decode(&snapshot); err != nil {
			if errors.Is(err, io.EOF) {
				break
			}
			t.Fatalf("decode event stream snapshot: %v", err)
		}
		revision := uint64FromAny(snapshot["revision"])
		if len(revisions) > 0 && revision <= revisions[len(revisions)-1] {
			t.Fatalf("revisions are not strictly increasing: %v then %v", revisions, revision)
		}
		revisions = append(revisions, revision)
		if boolFromAny(snapshot["terminal"]) {
			terminal = snapshot
		}
	}
	if len(revisions) < 2 {
		t.Fatalf("event stream emitted %d revisions, want running and terminal snapshots", len(revisions))
	}
	if terminal == nil || terminal["status"] != "completed" {
		t.Fatalf("terminal snapshot=%#v, want completed", terminal)
	}
}

func TestAdminJobEventsStreamEndsForDeferredSessionNormalize(t *testing.T) {
	manager := newAdminJobManager()
	job := manager.start("session_normalize", "session-stream-deferred", nil, func(_ context.Context, _ adminJobProgressFunc) (map[string]any, error) {
		return map[string]any{"status": "partial_deferred", "pending_count": 1}, nil
	})
	jobID := job["job_id"].(string)
	waitForAdminJobTerminal(t, manager, jobID)

	server := &Server{AdminJobs: manager}
	mux := http.NewServeMux()
	server.registerAdminRoutes(mux)
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/admin/jobs/"+jobID+"/events?after_revision=0", nil)
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("deferred event stream status=%d body=%s", rec.Code, rec.Body.String())
	}
	decoder := json.NewDecoder(rec.Body)
	var terminal map[string]any
	if err := decoder.Decode(&terminal); err != nil {
		t.Fatalf("decode deferred terminal snapshot: %v", err)
	}
	if terminal["status"] != "deferred" || terminal["terminal"] != true {
		t.Fatalf("deferred terminal snapshot=%#v", terminal)
	}
	var unexpected map[string]any
	if err := decoder.Decode(&unexpected); !errors.Is(err, io.EOF) {
		t.Fatalf("deferred event stream did not end after terminal snapshot: err=%v snapshot=%#v", err, unexpected)
	}
}
