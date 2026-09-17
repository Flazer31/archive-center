package httpapi

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"net/http"
	"time"
)

// TraceKey is the context key type for request tracing.
type TraceKey struct{}

// RequestTrace holds per-request metadata for observability and audit.
type RequestTrace struct {
	RequestID string    `json:"request_id"`
	Endpoint  string    `json:"endpoint"`
	Method    string    `json:"method"`
	StartedAt time.Time `json:"started_at"`
	ClientIP  string    `json:"client_ip,omitempty"`
}

// NewRequestTrace creates a trace for the incoming request.
func NewRequestTrace(r *http.Request) *RequestTrace {
	return &RequestTrace{
		RequestID: generateRequestID(),
		Endpoint:  r.URL.Path,
		Method:    r.Method,
		StartedAt: time.Now().UTC(),
		ClientIP:  r.RemoteAddr,
	}
}

// ContextWithTrace returns a context carrying the request trace.
func ContextWithTrace(ctx context.Context, trace *RequestTrace) context.Context {
	return context.WithValue(ctx, TraceKey{}, trace)
}

// TraceFromContext extracts the request trace, or returns nil.
func TraceFromContext(ctx context.Context) *RequestTrace {
	if v, ok := ctx.Value(TraceKey{}).(*RequestTrace); ok {
		return v
	}
	return nil
}

// AuditEvent is a placeholder for the future audit logging surface.
// In R0/R1 it is a no-op; it will be wired to a real sink in R2.
type AuditEvent struct {
	RequestID string `json:"request_id"`
	Action    string `json:"action"`
	Status    string `json:"status"`
	Detail    string `json:"detail,omitempty"`
}

// LogAuditEvent records an audit event.  In R0/R1 this is a no-op placeholder.
func LogAuditEvent(_ *RequestTrace, _ AuditEvent) {
	// Placeholder: will be wired to a persistent audit sink in R2.
}

func generateRequestID() string {
	b := make([]byte, 8)
	if _, err := rand.Read(b); err != nil {
		return "req-unknown"
	}
	return hex.EncodeToString(b)
}
