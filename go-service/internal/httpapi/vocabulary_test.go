package httpapi

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestWriteErrorShape(t *testing.T) {
	rec := httptest.NewRecorder()
	writeError(rec, http.StatusBadRequest, CodeBadRequest, "test message")

	if rec.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusBadRequest)
	}
	ct := rec.Header().Get("Content-Type")
	if ct != "application/json; charset=utf-8" {
		t.Errorf("Content-Type = %q, want application/json; charset=utf-8", ct)
	}

	var resp ErrorResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to decode: %v", err)
	}
	if resp.Status != "error" {
		t.Errorf("status = %q, want error", resp.Status)
	}
	if resp.Code != CodeBadRequest {
		t.Errorf("code = %q, want %q", resp.Code, CodeBadRequest)
	}
	if resp.Error != "test message" {
		t.Errorf("error = %q, want test message", resp.Error)
	}
}

func TestWriteShadowGuard(t *testing.T) {
	rec := httptest.NewRecorder()
	writeShadowGuard(rec, "POST /complete-turn")

	if rec.Code != http.StatusServiceUnavailable {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusServiceUnavailable)
	}

	var resp ErrorResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to decode: %v", err)
	}
	if resp.Code != CodeShadowGuard {
		t.Errorf("code = %q, want %q", resp.Code, CodeShadowGuard)
	}
	if resp.Error == "" {
		t.Error("error message should not be empty")
	}
	if resp.Status != "error" {
		t.Errorf("status = %q, want error", resp.Status)
	}
}

func TestWriteBadRequestConvenience(t *testing.T) {
	rec := httptest.NewRecorder()
	writeBadRequest(rec, "missing chat_session_id")
	if rec.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusBadRequest)
	}
	var resp ErrorResponse
	_ = json.Unmarshal(rec.Body.Bytes(), &resp)
	if resp.Code != CodeBadRequest {
		t.Errorf("code = %q, want %q", resp.Code, CodeBadRequest)
	}
}

func TestWriteNotFoundConvenience(t *testing.T) {
	rec := httptest.NewRecorder()
	writeNotFound(rec, "session not found")
	if rec.Code != http.StatusNotFound {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusNotFound)
	}
	var resp ErrorResponse
	_ = json.Unmarshal(rec.Body.Bytes(), &resp)
	if resp.Code != CodeNotFound {
		t.Errorf("code = %q, want %q", resp.Code, CodeNotFound)
	}
}

func TestWriteForbiddenConvenience(t *testing.T) {
	rec := httptest.NewRecorder()
	writeForbidden(rec, "operator access required")
	if rec.Code != http.StatusForbidden {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusForbidden)
	}
	var resp ErrorResponse
	_ = json.Unmarshal(rec.Body.Bytes(), &resp)
	if resp.Code != CodeForbidden {
		t.Errorf("code = %q, want %q", resp.Code, CodeForbidden)
	}
}

func TestWriteInternalErrorConvenience(t *testing.T) {
	rec := httptest.NewRecorder()
	writeInternalError(rec, "database unavailable")
	if rec.Code != http.StatusInternalServerError {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusInternalServerError)
	}
	var resp ErrorResponse
	_ = json.Unmarshal(rec.Body.Bytes(), &resp)
	if resp.Code != CodeInternalError {
		t.Errorf("code = %q, want %q", resp.Code, CodeInternalError)
	}
}

func TestStatusFromCode(t *testing.T) {
	tests := []struct {
		code   string
		status int
	}{
		{CodeBadRequest, http.StatusBadRequest},
		{CodeMissingParam, http.StatusBadRequest},
		{CodeNotFound, http.StatusNotFound},
		{CodeForbidden, http.StatusForbidden},
		{CodeUnauthorized, http.StatusUnauthorized},
		{CodeRateLimit, http.StatusTooManyRequests},
		{CodeInternalError, http.StatusInternalServerError},
		{CodeBadGateway, http.StatusBadGateway},
		{CodeGatewayTimeout, http.StatusGatewayTimeout},
		{CodeShadowGuard, http.StatusServiceUnavailable},
		{"unknown_code", http.StatusInternalServerError},
	}
	for _, tc := range tests {
		got := statusFromCode(tc.code)
		if got != tc.status {
			t.Errorf("statusFromCode(%q) = %d, want %d", tc.code, got, tc.status)
		}
	}
}

func TestCodeFromStatus(t *testing.T) {
	tests := []struct {
		status int
		code   string
	}{
		{http.StatusBadRequest, CodeBadRequest},
		{http.StatusNotFound, CodeNotFound},
		{http.StatusForbidden, CodeForbidden},
		{http.StatusUnauthorized, CodeUnauthorized},
		{http.StatusTooManyRequests, CodeRateLimit},
		{http.StatusInternalServerError, CodeInternalError},
		{http.StatusBadGateway, CodeBadGateway},
		{http.StatusGatewayTimeout, CodeGatewayTimeout},
		{http.StatusServiceUnavailable, CodeShadowGuard},
		{418, CodeInternalError},
	}
	for _, tc := range tests {
		got := codeFromStatus(tc.status)
		if got != tc.code {
			t.Errorf("codeFromStatus(%d) = %q, want %q", tc.status, got, tc.code)
		}
	}
}

func TestStatusCodeRoundTrip(t *testing.T) {
	codes := []string{
		CodeBadRequest, CodeMissingParam, CodeNotFound, CodeForbidden,
		CodeUnauthorized, CodeRateLimit, CodeInternalError,
		CodeBadGateway, CodeGatewayTimeout, CodeShadowGuard,
	}
	for _, c := range codes {
		status := statusFromCode(c)
		back := codeFromStatus(status)
		if statusFromCode(back) != status {
			t.Errorf("round-trip failed for %q: got status %d", c, status)
		}
	}
}
