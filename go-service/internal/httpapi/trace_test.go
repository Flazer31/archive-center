package httpapi

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestNewRequestTraceSetsFields(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "/test/path", nil)
	req.RemoteAddr = "192.168.1.1:1234"

	trace := NewRequestTrace(req)
	if trace == nil {
		t.Fatal("NewRequestTrace returned nil")
	}
	if trace.Endpoint != "/test/path" {
		t.Errorf("Endpoint = %q, want %q", trace.Endpoint, "/test/path")
	}
	if trace.Method != http.MethodPost {
		t.Errorf("Method = %q, want %q", trace.Method, http.MethodPost)
	}
	if trace.ClientIP != "192.168.1.1:1234" {
		t.Errorf("ClientIP = %q, want %q", trace.ClientIP, "192.168.1.1:1234")
	}
	if trace.RequestID == "" {
		t.Error("RequestID should not be empty")
	}
	if trace.StartedAt.IsZero() {
		t.Error("StartedAt should be non-zero")
	}
	if time.Since(trace.StartedAt) > time.Second {
		t.Error("StartedAt should be very recent")
	}
}

func TestContextWithTraceRoundTrip(t *testing.T) {
	trace := &RequestTrace{
		RequestID: "req-abc",
		Endpoint:  "/round",
		Method:    http.MethodGet,
		StartedAt: time.Now().UTC(),
		ClientIP:  "10.0.0.1",
	}

	ctx := ContextWithTrace(context.Background(), trace)
	got := TraceFromContext(ctx)
	if got != trace {
		t.Error("TraceFromContext did not return the same trace pointer")
	}
	if got.RequestID != "req-abc" {
		t.Errorf("RequestID = %q, want req-abc", got.RequestID)
	}
}

func TestTraceFromContextMissingReturnsNil(t *testing.T) {
	ctx := context.Background()
	if got := TraceFromContext(ctx); got != nil {
		t.Errorf("TraceFromContext = %v, want nil", got)
	}
}

func TestAuditEventJSONFieldNames(t *testing.T) {
	ev := AuditEvent{
		RequestID: "req-001",
		Action:    "create",
		Status:    "ok",
		Detail:    "detail text",
	}
	b, err := json.Marshal(ev)
	if err != nil {
		t.Fatalf("json.Marshal failed: %v", err)
	}
	var m map[string]any
	if err := json.Unmarshal(b, &m); err != nil {
		t.Fatalf("json.Unmarshal failed: %v", err)
	}

	expected := map[string]any{
		"request_id": "req-001",
		"action":     "create",
		"status":     "ok",
		"detail":     "detail text",
	}
	for k, want := range expected {
		got, ok := m[k]
		if !ok {
			t.Errorf("missing JSON key %q", k)
			continue
		}
		if got != want {
			t.Errorf("%s = %v, want %v", k, got, want)
		}
	}

	// Ensure no unexpected keys exist.
	if len(m) != len(expected) {
		t.Errorf("got %d keys, want %d", len(m), len(expected))
	}
}

func TestTraceFieldConstants(t *testing.T) {
	tests := []struct {
		constant string
		want     string
	}{
		{TraceFieldEndpoint, "endpoint"},
		{TraceFieldStatus, "status"},
		{TraceFieldCode, "code"},
		{TraceFieldDurationMS, "duration_ms"},
		{TraceFieldTimestamp, "timestamp"},
		{TraceFieldSource, "source"},
	}
	for _, tc := range tests {
		if tc.constant != tc.want {
			t.Errorf("constant = %q, want %q", tc.constant, tc.want)
		}
	}
}

func TestLogAuditEventNoOpDoesNotPanic(t *testing.T) {
	trace := &RequestTrace{RequestID: "req-nop"}
	ev := AuditEvent{Action: "test", Status: "ok"}
	// In R0/R1 this is a no-op placeholder; it must not panic.
	LogAuditEvent(trace, ev)
}
