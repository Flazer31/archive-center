package httpapi

import (
	"bytes"
	"fmt"
	"log/slog"
	"net/http"
	"time"

	"github.com/risulongmemory/archive-center-go/internal/diagnostics"
)

func (s *Server) handleDiagnosticReport(w http.ResponseWriter, r *http.Request) {
	dir := diagnostics.Directory()
	if s.DiagnosticWriter != nil {
		dir = s.DiagnosticWriter.Dir
	}
	report := diagnostics.Collect(dir, s.Cfg.BuildVersion, s.Cfg.MariaDBDSN)
	if s.DiagnosticWriter != nil && s.DiagnosticWriter.Error() != "" {
		report.LogError = s.DiagnosticWriter.Error()
	}
	w.Header().Set("Cache-Control", "no-store")
	writeJSON(w, http.StatusOK, report)
}

type diagnosticResponseWriter struct {
	http.ResponseWriter
	status int
	body   bytes.Buffer
}

func (w *diagnosticResponseWriter) Unwrap() http.ResponseWriter { return w.ResponseWriter }
func (w *diagnosticResponseWriter) WriteHeader(code int) {
	if code >= 200 && w.status == 0 {
		w.status = code
	}
	w.ResponseWriter.WriteHeader(code)
}
func (w *diagnosticResponseWriter) Write(p []byte) (int, error) {
	if w.status == 0 {
		w.status = http.StatusOK
	}
	if w.status >= 400 && w.body.Len() < 4096 {
		n := min(4096-w.body.Len(), len(p))
		_, _ = w.body.Write(p[:n])
	}
	n, err := w.ResponseWriter.Write(p)
	if err != nil {
		slog.Error("http response write failed", "error", diagnostics.Redact(err.Error()))
	}
	return n, err
}

// Preserve HUD/SSE flushing and net/http ResponseController unwrapping.
func (w *diagnosticResponseWriter) Flush() {
	if w.status == 0 {
		w.status = http.StatusOK
	}
	if err := http.NewResponseController(w.ResponseWriter).Flush(); err != nil {
		slog.Error("http response flush failed", "error", err)
	}
}

func (s *Server) diagnosticMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		started := time.Now()
		captured := &diagnosticResponseWriter{ResponseWriter: w}
		defer func() {
			if cause := recover(); cause != nil {
				slog.ErrorContext(r.Context(), "http handler panic", "path", r.URL.Path, "error", diagnostics.Redact(fmt.Sprint(cause), s.Cfg.MariaDBDSN))
				panic(cause) // net/http keeps its existing connection recovery behavior.
			}
			if captured.status >= 400 {
				details := s.completeTurnPersistenceDiagnostics([]string{"http: " + captured.body.String()})
				slog.ErrorContext(r.Context(), "http request failed", "method", r.Method, "path", r.URL.Path,
					"status", captured.status, "duration_ms", time.Since(started).Milliseconds(), "error", details)
			}
		}()
		next.ServeHTTP(captured, r)
	})
}
