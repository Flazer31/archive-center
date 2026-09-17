package httpapi

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/risulongmemory/archive-center-go/internal/config"
	"github.com/risulongmemory/archive-center-go/internal/diagnostics"
)

func installDiagnosticTestLogger(t *testing.T, w *diagnostics.Writer) {
	t.Helper()
	old := slog.Default()
	slog.SetDefault(slog.New(slog.NewJSONHandler(w, &slog.HandlerOptions{Level: w})))
	t.Cleanup(func() { slog.SetDefault(old) })
}

func TestDiagnosticLoggingControlAuthProxyAndInvalidValue(t *testing.T) {
	cfg := config.Default()
	cfg.Auth.Enforce, cfg.Auth.BearerToken = true, "fixture-auth"
	s := NewServer(cfg)
	w := &diagnostics.Writer{Dir: t.TempDir()}
	s.DiagnosticWriter = w
	installDiagnosticTestLogger(t, w)
	mux := http.NewServeMux()
	s.RegisterRoutes(mux)
	call := func(method, path, body, token string) *httptest.ResponseRecorder {
		r := httptest.NewRequest(method, path, strings.NewReader(body))
		if token != "" {
			r.Header.Set("Authorization", "Bearer "+token)
		}
		rec := httptest.NewRecorder()
		mux.ServeHTTP(rec, r)
		return rec
	}
	if r := call("POST", "/diagnostics/logging", `{"level":"debug"}`, ""); r.Code != 401 {
		t.Fatal(r.Code)
	}
	if w.Level() != slog.LevelInfo {
		t.Fatal("unauthenticated mode change")
	}
	if r := call("POST", "/ac/diagnostics/logging", `{"level":"debug"}`, "fixture-auth"); r.Code != 200 {
		t.Fatal(r.Code, r.Body)
	}
	state := w.LoggingState()
	if state.Level != "debug" {
		t.Fatal(state)
	}
	if r := call("POST", "/diagnostics/logging", `{"level":"trace"}`, "fixture-auth"); r.Code != 400 {
		t.Fatal(r.Code)
	}
	if w.LoggingState() != state {
		t.Fatal("invalid mode mutated state")
	}
	r := call("GET", "/diagnostics/report", "", "fixture-auth")
	var report diagnostics.Report
	if err := json.Unmarshal(r.Body.Bytes(), &report); err != nil {
		t.Fatal(err)
	}
	if report.Logging == nil || *report.Logging != state || r.Header().Get("Cache-Control") != "no-store" {
		t.Fatal(report.Logging)
	}
	if !reflect.DeepEqual(s.Cfg, cfg) {
		t.Fatal("logging changed application settings")
	}
}

func TestDiagnosticHTTPTracePreservesStreamAndOmitsPayload(t *testing.T) {
	for _, diskFailure := range []bool{false, true} {
		t.Run(fmt.Sprint(diskFailure), func(t *testing.T) {
			var console bytes.Buffer
			dir := t.TempDir()
			if diskFailure {
				dir = filepath.Join(dir, "file")
				if err := os.WriteFile(dir, []byte("file"), 0600); err != nil {
					t.Fatal(err)
				}
			}
			w := &diagnostics.Writer{Dir: dir, Console: &console}
			_ = w.SetLevel("debug")
			installDiagnosticTestLogger(t, w)
			s := NewServer(config.Default())
			h := s.diagnosticMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				// The start is already durable even if this operation has not finished.
				if !strings.Contains(console.String(), "http request started") {
					t.Error("missing in-flight start")
				}
				w.(http.Flusher).Flush()
				_, _ = w.Write([]byte("private-response"))
			}))
			r := httptest.NewRecorder()
			h.ServeHTTP(r, httptest.NewRequest("POST", "/fixture?secret=private-query", strings.NewReader("private-input")))
			if r.Code != 200 || r.Body.String() != "private-response" || !r.Flushed {
				t.Fatal("HTTP/SSE changed")
			}
			if !strings.Contains(console.String(), "http request finished") || strings.Contains(console.String(), "private-") {
				t.Fatal(console.String())
			}
			if diskFailure && w.Error() == "" {
				t.Fatal("disk failure was hidden")
			}
		})
	}
}

func TestDiagnosticModePreservesInputGroupDecisionWithoutReads(t *testing.T) {
	for _, count := range []int{1, 2, 3} {
		t.Run(fmt.Sprint(count), func(t *testing.T) {
			s := NewServer(config.Default())
			spy := &prepareReadRecordingStore{}
			s.Store = spy
			vector := &prepareReadRecordingVectorStore{}
			s.Vector = vector
			var console bytes.Buffer
			w := &diagnostics.Writer{Dir: t.TempDir(), Console: &console}
			s.DiagnosticWriter = w
			installDiagnosticTestLogger(t, w)
			mux := http.NewServeMux()
			s.RegisterRoutes(mux)
			active := []map[string]any{}
			for i := 0; i < count; i++ {
				text := fmt.Sprint("PRIVATE-DIALOGUE-", i)
				active = append(active, map[string]any{"observation_ref": fmt.Sprint("active:", i), "source_kind": "active_chat", "observation_stage": "active_chat_stored_message", "message_index": i, "message_id": fmt.Sprint("user-", i), "role": "user", "raw_content": text, "content_hash": prepareOR1CHash(text), "hash_algorithm": "or1c_utf16_djb2.v1", "evidence_state": "observed"})
			}
			body, _ := json.Marshal(map[string]any{"chat_session_id": "diagnostic-fixture", "request_type": "model", "source_decision_only": true, "host_observations": map[string]any{"contract_version": prepareHostObservationsVersion, "session_id": "diagnostic-fixture", "request_id": "diagnostic-request", "request_type": "model", "payload_writable": true, "active_chat": active, "payload": active}})
			var baseline any
			for _, level := range []string{"info", "debug"} {
				_ = w.SetLevel(level)
				r := httptest.NewRecorder()
				mux.ServeHTTP(r, httptest.NewRequest("POST", "/prepare-turn", bytes.NewReader(body)))
				if r.Code != 200 {
					t.Fatal(r.Code, r.Body)
				}
				var result map[string]any
				if err := json.Unmarshal(r.Body.Bytes(), &result); err != nil {
					t.Fatal(err)
				}
				decision := result["current_input_decision"].(map[string]any)
				if decision["status"] != "eligible" {
					t.Fatal(decision)
				}
				if level == "info" {
					baseline = decision
				} else if !reflect.DeepEqual(baseline, decision) {
					t.Fatal("mode changed input decision")
				}
			}
			if spy.readCalls != 0 || vector.searchCalls != 0 || vector.upsertCalls != 0 {
				t.Fatal("diagnostic mode caused memory reads/writes")
			}
			if strings.Contains(console.String(), "PRIVATE-DIALOGUE") {
				t.Fatal("conversation logged")
			}
			found := false
			for _, line := range strings.Split(console.String(), "\n") {
				var event map[string]any
				if json.Unmarshal([]byte(line), &event) == nil && event["msg"] == "prepare input decision" {
					found = true
					if event["input_count"] != float64(count) || event["request_id"] != "diagnostic-request" {
						t.Fatal(event)
					}
				}
			}
			if !found {
				t.Fatal("input count trace absent")
			}
		})
	}
}

type diagnosticLockCheckingWriter struct {
	ledger *turnWorkflowHUDLedger
	locked bool
	text   strings.Builder
}

func (w *diagnosticLockCheckingWriter) Write(p []byte) (int, error) {
	if !w.ledger.mu.TryLock() {
		w.locked = true
	} else {
		w.ledger.mu.Unlock()
	}
	return w.text.Write(p)
}

func TestDiagnosticWorkflowLoggingDoesNotHoldLedgerLock(t *testing.T) {
	l := newTurnWorkflowHUDLedger()
	w := &diagnosticLockCheckingWriter{ledger: l}
	old := slog.Default()
	slog.SetDefault(slog.New(slog.NewJSONHandler(w, &slog.HandlerOptions{Level: slog.LevelDebug})))
	defer slog.SetDefault(old)
	l.begin("fixture-complete", "session", 1)
	l.startStage("fixture-complete", turnWorkflowStageRecall)
	l.finishStage("fixture-complete", turnWorkflowStageRecall, "succeeded", "")
	l.complete("fixture-complete")
	l.begin("fixture-failed", "session", 2)
	l.fail("fixture-failed", "fixture_failure", "fixture", turnWorkflowStageContext, false)
	if w.locked {
		t.Fatal("log I/O held workflow lock")
	}
	for _, event := range []string{"turn stage started", "turn stage finished", "turn workflow completed", "turn workflow stopped"} {
		if !strings.Contains(w.text.String(), event) {
			t.Fatal(event)
		}
	}
}

func TestDiagnosticDebugKeepsLifecycleRegressions(t *testing.T) {
	w := &diagnostics.Writer{Dir: t.TempDir()}
	_ = w.SetLevel("debug")
	installDiagnosticTestLogger(t, w)
	ctx, finish := diagnosticTurnRequest(context.Background(), "fixture", "fixture-request")
	defer finish()
	if ctx.Value(diagnosticRequestIDKey{}) != "fixture-request" {
		t.Fatal("correlation lost")
	}
	t.Run("input_groups_retry_edit_delete_restart_new_row", Test44InputGroupAcceptanceRetryEditDeleteRestartAndNewRow)
	t.Run("source_decision_no_reads", TestPrepareTurnSourceDecisionOnlyIsReadFree)
	t.Run("actual_provider_failure", TestProviderFailureLoggedWithoutChangingResult)
}
