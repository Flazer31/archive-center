package main

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/risulongmemory/archive-center-go/internal/bench"
)

func TestValidateFlags(t *testing.T) {
	if err := ValidateFlags(1, 1, 30, 500, 0, false); err != nil {
		t.Errorf("unexpected error for count=1, timeout=1: %v", err)
	}
	if err := ValidateFlags(5, 2, 30, 500, 0, false); err != nil {
		t.Errorf("unexpected error for count=5, timeout=2: %v", err)
	}

	cases := []struct {
		count             int
		timeout           int
		startupTimeout    int
		startupIntervalMs int
		pid               int
		waitReady         bool
		wantErr           string
	}{
		{0, 1, 30, 500, 0, false, "request count must be > 0"},
		{-1, 1, 30, 500, 0, false, "request count must be > 0"},
		{1, 0, 30, 500, 0, false, "timeout must be > 0"},
		{1, -1, 30, 500, 0, false, "timeout must be > 0"},
		{1, 1, 0, 500, 0, true, "startup-timeout must be > 0 when wait-ready is enabled"},
		{1, 1, 30, 0, 0, false, "startup-interval-ms must be > 0"},
		{1, 1, 30, 500, -1, false, "pid must be >= 0"},
	}

	for _, c := range cases {
		err := ValidateFlags(c.count, c.timeout, c.startupTimeout, c.startupIntervalMs, c.pid, c.waitReady)
		if err == nil {
			t.Errorf("expected error for count=%d, timeout=%d", c.count, c.timeout)
			continue
		}
		if !strings.Contains(err.Error(), c.wantErr) {
			t.Errorf("count=%d, timeout=%d: error = %q, want %q", c.count, c.timeout, err.Error(), c.wantErr)
		}
	}
}

func TestWaitForReadySuccess(t *testing.T) {
	attempt := 0
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		attempt++
		if attempt < 2 {
			w.WriteHeader(http.StatusServiceUnavailable)
			return
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer ts.Close()

	client := &http.Client{Timeout: 500 * time.Millisecond}
	attempts, elapsed, err := WaitForReady(context.Background(), client, ts.URL, 50*time.Millisecond)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if attempts != 2 {
		t.Errorf("attempts = %d, want 2", attempts)
	}
	if elapsed <= 0 {
		t.Error("elapsed should be > 0")
	}
}

func TestWaitForReadyTimeout(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusServiceUnavailable)
	}))
	defer ts.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()

	client := &http.Client{Timeout: 100 * time.Millisecond}
	attempts, elapsed, err := WaitForReady(ctx, client, ts.URL, 50*time.Millisecond)
	if err == nil {
		t.Fatal("expected timeout error")
	}
	if attempts < 1 {
		t.Errorf("attempts = %d, want >= 1", attempts)
	}
	if elapsed < 0 {
		t.Error("elapsed should be >= 0")
	}
}

func TestExecuteCaptureSkipsOnReadinessFailure(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer ts.Close()

	client := &http.Client{Timeout: 1 * time.Second}
	readiness := bench.ReadinessInfo{Enabled: true, Success: false}
	summaries := executeCapture(client, "GET", ts.URL, []string{"/health"}, 1, readiness)
	if len(summaries) != 0 {
		t.Errorf("expected 0 summaries on readiness failure, got %d", len(summaries))
	}
}

func TestExecuteCaptureRunsOnReadinessSuccess(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer ts.Close()

	client := &http.Client{Timeout: 1 * time.Second}
	readiness := bench.ReadinessInfo{Enabled: true, Success: true}
	summaries := executeCapture(client, "GET", ts.URL, []string{"/health"}, 2, readiness)
	if len(summaries) != 1 {
		t.Fatalf("expected 1 summary, got %d", len(summaries))
	}
	if summaries[0].Total != 2 {
		t.Errorf("total = %d, want 2", summaries[0].Total)
	}
}

func TestExecuteCaptureRunsWhenWaitReadyDisabled(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer ts.Close()

	client := &http.Client{Timeout: 1 * time.Second}
	readiness := bench.ReadinessInfo{Enabled: false}
	summaries := executeCapture(client, "GET", ts.URL, []string{"/health"}, 3, readiness)
	if len(summaries) != 1 {
		t.Fatalf("expected 1 summary, got %d", len(summaries))
	}
	if summaries[0].Total != 3 {
		t.Errorf("total = %d, want 3", summaries[0].Total)
	}
}
