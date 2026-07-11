package httpapi

import (
	"context"
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
