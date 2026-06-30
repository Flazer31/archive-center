package httpapi

import (
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

// ErrorResponse is the common error shape returned by all handlers.
type ErrorResponse struct {
	Status  string `json:"status"`
	Error   string `json:"error,omitempty"`
	Code    string `json:"code,omitempty"`
	Request string `json:"request_id,omitempty"`
}

// SuccessResponse is the common success envelope used when a handler
// returns no domain-specific body.
type SuccessResponse struct {
	Status string `json:"status"`
}

// writeJSON writes v as JSON with the given HTTP status.
func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(v); err != nil {
		// Fallback if JSON encoding fails.
		fmt.Fprintf(w, `{"error":"%s"}`, err.Error())
	}
}

func formatKSTTime(t time.Time) any {
	if t.IsZero() {
		return nil
	}
	return t.UTC().In(time.FixedZone("KST", 9*60*60)).Format("2006-01-02 15:04:05")
}

func nullableInt64(v int64) any {
	if v == 0 {
		return nil
	}
	return v
}

func nullableString(v string) any {
	if v == "" {
		return nil
	}
	return v
}

func nullableInt(v int) any {
	if v == 0 {
		return nil
	}
	return v
}

// writeError writes a standard ErrorResponse with the given HTTP status.
func writeError(w http.ResponseWriter, status int, code, message string) {
	writeJSON(w, status, ErrorResponse{
		Status: "error",
		Error:  message,
		Code:   code,
	})
}

// RouteTier classifies endpoints by release-phase risk.
type RouteTier string

const (
	TierR0 RouteTier = "R0" // no side effect (health, probe, static)
	TierR1 RouteTier = "R1" // read-only (search, list, get, audit)
	TierR2 RouteTier = "R2" // write / state mutation
)

// Common error codes used across handlers.
const (
	CodeBadRequest     = "bad_request"
	CodeMissingParam   = "missing_param"
	CodeNotFound       = "not_found"
	CodeForbidden      = "forbidden"
	CodeUnauthorized   = "unauthorized"
	CodeRateLimit      = "rate_limit_exceeded"
	CodeInternalError  = "internal_error"
	CodeBadGateway     = "bad_gateway"
	CodeGatewayTimeout = "gateway_timeout"
	CodeShadowGuard    = "shadow_guard"
)

// TraceField is a common field name for trace/audit metadata.
const (
	TraceFieldEndpoint   = "endpoint"
	TraceFieldStatus     = "status"
	TraceFieldCode       = "code"
	TraceFieldDurationMS = "duration_ms"
	TraceFieldTimestamp  = "timestamp"
	TraceFieldSource     = "source"
)

// writeShadowGuard blocks the request because the endpoint is not
// available in the current R-phase.
func writeShadowGuard(w http.ResponseWriter, endpoint string) {
	writeError(w, http.StatusServiceUnavailable, CodeShadowGuard,
		fmt.Sprintf("%s is not available in R0/R1 shadow mode", endpoint))
}

// writeBadRequest is a convenience wrapper for 400 with CodeBadRequest.
func writeBadRequest(w http.ResponseWriter, message string) {
	writeError(w, http.StatusBadRequest, CodeBadRequest, message)
}

// writeNotFound is a convenience wrapper for 404 with CodeNotFound.
func writeNotFound(w http.ResponseWriter, message string) {
	writeError(w, http.StatusNotFound, CodeNotFound, message)
}

// writeForbidden is a convenience wrapper for 403 with CodeForbidden.
func writeForbidden(w http.ResponseWriter, message string) {
	writeError(w, http.StatusForbidden, CodeForbidden, message)
}

// writeInternalError is a convenience wrapper for 500 with CodeInternalError.
func writeInternalError(w http.ResponseWriter, message string) {
	writeError(w, http.StatusInternalServerError, CodeInternalError, message)
}

// statusFromCode returns the canonical HTTP status for an error code.
func statusFromCode(code string) int {
	switch code {
	case CodeBadRequest, CodeMissingParam:
		return http.StatusBadRequest
	case CodeNotFound:
		return http.StatusNotFound
	case CodeForbidden:
		return http.StatusForbidden
	case CodeUnauthorized:
		return http.StatusUnauthorized
	case CodeRateLimit:
		return http.StatusTooManyRequests
	case CodeInternalError:
		return http.StatusInternalServerError
	case CodeBadGateway:
		return http.StatusBadGateway
	case CodeGatewayTimeout:
		return http.StatusGatewayTimeout
	case CodeShadowGuard:
		return http.StatusServiceUnavailable
	default:
		return http.StatusInternalServerError
	}
}

// codeFromStatus returns the canonical error code for an HTTP status.
func codeFromStatus(status int) string {
	switch status {
	case http.StatusBadRequest:
		return CodeBadRequest
	case http.StatusNotFound:
		return CodeNotFound
	case http.StatusForbidden:
		return CodeForbidden
	case http.StatusUnauthorized:
		return CodeUnauthorized
	case http.StatusTooManyRequests:
		return CodeRateLimit
	case http.StatusInternalServerError:
		return CodeInternalError
	case http.StatusBadGateway:
		return CodeBadGateway
	case http.StatusGatewayTimeout:
		return CodeGatewayTimeout
	case http.StatusServiceUnavailable:
		return CodeShadowGuard
	default:
		return CodeInternalError
	}
}
