package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/risulongmemory/archive-center-go/internal/config"
	"github.com/risulongmemory/archive-center-go/internal/diagnostics"
	"github.com/risulongmemory/archive-center-go/internal/dto"
)

func TestDiagnosticsRecordsErrorsAndPreservesHTTP(t *testing.T) {
	w := &diagnostics.Writer{Dir: t.TempDir()}
	old := slog.Default()
	slog.SetDefault(slog.New(slog.NewJSONHandler(w, nil)))
	defer slog.SetDefault(old)
	s := NewServer(config.Load())
	s.DiagnosticWriter = w
	h := s.diagnosticMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/broken" {
			http.Error(w, "fixture upstream unavailable", 502)
			return
		}
		w.(http.Flusher).Flush()
		_, _ = w.Write([]byte("fixture SSE"))
	}))
	for _, test := range []struct {
		path   string
		status int
		body   string
	}{{"/broken", 502, "fixture upstream unavailable\n"}, {"/stream", 200, "fixture SSE"}} {
		r := httptest.NewRecorder()
		h.ServeHTTP(r, httptest.NewRequest("GET", test.path, nil))
		if r.Code != test.status || r.Body.String() != test.body {
			t.Fatalf("HTTP behavior changed: %d %s", r.Code, r.Body)
		}
		if test.path == "/stream" && !r.Flushed {
			t.Fatal("streaming stopped")
		}
	}
	mux := http.NewServeMux()
	s.RegisterRoutes(mux)
	r := httptest.NewRecorder()
	mux.ServeHTTP(r, httptest.NewRequest("GET", "/diagnostics/report", nil))
	if r.Code != 200 || !json.Valid(r.Body.Bytes()) || !strings.Contains(r.Body.String(), "fixture upstream unavailable") {
		t.Fatalf("report: %d %s", r.Code, r.Body)
	}
	if strings.Contains(r.Body.String(), "fixture SSE") {
		t.Fatal("successful payload was logged")
	}
	proxied := httptest.NewRecorder()
	mux.ServeHTTP(proxied, httptest.NewRequest("GET", "/ac/diagnostics/report", nil))
	if proxied.Code != 200 || !strings.Contains(proxied.Body.String(), "fixture upstream unavailable") {
		t.Fatal("reverse-proxy diagnostic route lost")
	}
}

func TestProviderFailureLoggedWithoutChangingResult(t *testing.T) {
	w := &diagnostics.Writer{Dir: t.TempDir()}
	old := slog.Default()
	slog.SetDefault(slog.New(slog.NewJSONHandler(w, nil)))
	defer slog.SetDefault(old)
	provider, model := "invalid-fixture-provider", "fixture-model"
	key := "fixture-key-not-for-report"
	_, status, err := callProxyProviderWithPolicy(context.Background(), dto.ProxyPluginMainRequest{Provider: &provider, Model: &model, APIKey: &key}, proxyRequestPolicy{Purpose: "critic", SessionID: "fixture-session"}, nil)
	var local *proxyLocalRequestError
	if status != 400 || !errors.As(err, &local) {
		t.Fatalf("failure changed: %d %v", status, err)
	}
	data, _ := json.Marshal(diagnostics.Collect(w.Dir, "fixture"))
	if !strings.Contains(string(data), "fixture-model") || !strings.Contains(string(data), "critic") || strings.Contains(string(data), key) {
		t.Fatalf("provider diagnostics missing or unsafe: %s", data)
	}
}
