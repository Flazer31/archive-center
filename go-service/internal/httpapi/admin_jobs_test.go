package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
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
