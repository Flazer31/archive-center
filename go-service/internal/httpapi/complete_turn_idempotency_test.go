package httpapi

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/risulongmemory/archive-center-go/internal/dto"
)

func TestCompleteTurnRequestLedgerSharesCompletedResponse(t *testing.T) {
	ledger := newCompleteTurnRequestLedger()
	entry, owner := ledger.begin("req-1")
	if !owner || entry == nil {
		t.Fatal("first request must own the idempotency key")
	}
	duplicate, duplicateOwner := ledger.begin("req-1")
	if duplicateOwner || duplicate != entry {
		t.Fatal("duplicate request must join the in-flight entry")
	}
	ledger.finish("req-1", completeTurnRecordedResponse{status: http.StatusOK, body: []byte(`{"status":"ok"}`)})
	select {
	case <-duplicate.done:
	case <-time.After(time.Second):
		t.Fatal("duplicate request did not observe completion")
	}
	status, response, found := ledger.status("req-1")
	if !found || status != "completed" || response.status != http.StatusOK {
		t.Fatalf("unexpected completed status: found=%v status=%q response=%+v", found, status, response)
	}
}

func TestCompleteTurnRequestLedgerBoundsUnfinishedOwners(t *testing.T) {
	ledger := newCompleteTurnRequestLedger()
	for index := 0; index < completeTurnRequestMax; index++ {
		entry, owner, conflict := ledger.beginWithFingerprint(
			fmt.Sprintf("pending-%d", index),
			fmt.Sprintf("fingerprint-%d", index),
		)
		if entry == nil || !owner || conflict {
			t.Fatalf("entry %d was not admitted", index)
		}
	}
	entry, owner, conflict := ledger.beginWithFingerprint("over-capacity", "fingerprint")
	if entry != nil || owner || conflict {
		t.Fatalf("over-capacity request was admitted: entry=%v owner=%v conflict=%v", entry, owner, conflict)
	}
}

func TestCompleteTurnRequestLedgerEvictsOldestFinishedEntryAtCapacity(t *testing.T) {
	ledger := newCompleteTurnRequestLedger()
	for index := 0; index < completeTurnRequestMax; index++ {
		key := fmt.Sprintf("finished-%d", index)
		if _, owner := ledger.begin(key); !owner {
			t.Fatalf("entry %d was not admitted", index)
		}
		ledger.finish(key, completeTurnRecordedResponse{
			status: http.StatusOK,
			body:   []byte(`{"status":"ok","save_ok":true}`),
		})
	}
	if _, owner := ledger.begin("capacity-replacement"); !owner {
		t.Fatal("finished capacity did not admit a replacement")
	}
	if _, _, found := ledger.status("finished-0"); found {
		t.Fatal("oldest finished entry was not evicted at capacity")
	}
}

func TestCompleteTurnIdempotentExecutionRunsWriterOnce(t *testing.T) {
	server := &Server{CompleteTurns: newCompleteTurnRequestLedger()}
	started := make(chan struct{})
	release := make(chan struct{})
	var calls atomic.Int32
	run := func(w http.ResponseWriter) {
		if calls.Add(1) == 1 {
			close(started)
		}
		<-release
		writeJSON(w, http.StatusOK, map[string]any{"status": "ok", "save_ok": true})
	}

	first := httptest.NewRecorder()
	firstDone := make(chan struct{})
	go func() {
		server.executeCompleteTurnIdempotent(context.Background(), first, "same-key", "same-fingerprint", run)
		close(firstDone)
	}()
	<-started

	second := httptest.NewRecorder()
	secondDone := make(chan struct{})
	go func() {
		server.executeCompleteTurnIdempotent(context.Background(), second, "same-key", "same-fingerprint", run)
		close(secondDone)
	}()
	close(release)
	<-firstDone
	<-secondDone

	if calls.Load() != 1 {
		t.Fatalf("writer calls=%d, want 1", calls.Load())
	}
	if first.Body.String() != second.Body.String() {
		t.Fatalf("duplicate response mismatch: first=%q second=%q", first.Body.String(), second.Body.String())
	}
}

func TestCompleteTurnCancelledOwnerIsUnknownUntilOwnerResolves(t *testing.T) {
	server := &Server{CompleteTurns: newCompleteTurnRequestLedger()}
	ctx, cancel := context.WithCancel(context.Background())
	started := make(chan struct{})
	release := make(chan struct{})
	done := make(chan struct{})
	go func() {
		defer close(done)
		server.executeCompleteTurnIdempotent(ctx, httptest.NewRecorder(), "cancel-key", "fingerprint", func(w http.ResponseWriter) {
			close(started)
			<-release
			writeJSON(w, http.StatusOK, map[string]any{"status": "ok", "save_ok": true, "raw_committed": true})
		})
	}()
	<-started
	cancel()

	deadline := time.Now().Add(time.Second)
	for {
		status, response, found := server.CompleteTurns.status("cancel-key")
		if found && status == "completed" {
			if got := string(response.body); !strings.Contains(got, `"commit_state":"unknown"`) {
				t.Fatalf("cancelled owner did not publish unknown outcome: %s", got)
			}
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("cancelled owner did not leave processing state")
		}
		time.Sleep(time.Millisecond)
	}

	close(release)
	<-done
	status, response, found := server.CompleteTurns.status("cancel-key")
	if !found || status != "completed" || !strings.Contains(string(response.body), `"save_ok":true`) {
		t.Fatalf("resolved owner did not replace provisional outcome: found=%v status=%q body=%s", found, status, response.body)
	}
}

func TestCompleteTurnIdempotentExecutionRejectsFingerprintConflict(t *testing.T) {
	server := &Server{CompleteTurns: newCompleteTurnRequestLedger()}
	var calls atomic.Int32
	run := func(w http.ResponseWriter) {
		calls.Add(1)
		writeJSON(w, http.StatusOK, map[string]any{"status": "ok", "save_ok": true})
	}

	server.executeCompleteTurnIdempotent(
		context.Background(), httptest.NewRecorder(), "same-key", "fingerprint-a", run,
	)
	conflict := httptest.NewRecorder()
	server.executeCompleteTurnIdempotent(
		context.Background(), conflict, "same-key", "fingerprint-b", run,
	)

	if calls.Load() != 1 {
		t.Fatalf("writer calls=%d, want 1 after fingerprint conflict", calls.Load())
	}
	for _, want := range []string{`"code":"idempotency_key_conflict"`, `"retryable":false`, `"queue_action":"discard"`} {
		if !strings.Contains(conflict.Body.String(), want) {
			t.Fatalf("conflict response missing %s: %s", want, conflict.Body.String())
		}
	}
}

func TestCompleteTurnRequestFingerprintCoversDerivedProcessingInputs(t *testing.T) {
	user, assistant, requestType := "user", "assistant", "model"
	improvement := map[string]any{"verdict": "keep"}
	base := dto.M4CompleteTurnRequest{
		ChatSessionID:    "session-1",
		TurnIndex:        3,
		UserInput:        &user,
		AssistantContent: &assistant,
		RequestType:      &requestType,
		ContextMessages:  []map[string]any{{"role": "user", "content": "context-a"}},
		ImprovementTrace: &improvement,
		ClientMeta:       map[string]any{"idempotency_key": "same-key", "source_acceptance_required": true},
	}
	changed := base
	changed.ContextMessages = []map[string]any{{"role": "user", "content": "context-b"}}
	if completeTurnRequestFingerprint(base) == completeTurnRequestFingerprint(changed) {
		t.Fatal("semantic payload change did not change complete-turn fingerprint")
	}
}

func TestCompleteTurnServerErrorAllowsLaterRetry(t *testing.T) {
	ledger := newCompleteTurnRequestLedger()
	if _, owner := ledger.begin("retry-key"); !owner {
		t.Fatal("first request did not acquire key")
	}
	ledger.finish("retry-key", completeTurnRecordedResponse{status: http.StatusInternalServerError})
	if _, owner := ledger.begin("retry-key"); !owner {
		t.Fatal("server-error request key must be available for retry")
	}
}

func TestCompleteTurnFailedSaveAllowsLaterRetry(t *testing.T) {
	server := &Server{CompleteTurns: newCompleteTurnRequestLedger()}
	var calls atomic.Int32
	run := func(w http.ResponseWriter) {
		saveOK := calls.Add(1) > 1
		writeJSON(w, http.StatusOK, map[string]any{"status": "ok", "save_ok": saveOK})
	}

	server.executeCompleteTurnIdempotent(context.Background(), httptest.NewRecorder(), "retry-save-key", "same-fingerprint", run)
	second := httptest.NewRecorder()
	server.executeCompleteTurnIdempotent(context.Background(), second, "retry-save-key", "same-fingerprint", run)

	if calls.Load() != 2 {
		t.Fatalf("writer calls=%d, want 2 after save_ok=false", calls.Load())
	}
	if !strings.Contains(second.Body.String(), `"save_ok":true`) {
		t.Fatalf("retry response did not report successful save: %s", second.Body.String())
	}
}

func TestCompleteTurnUnknownCommitOutcomeStaysTerminalInLedger(t *testing.T) {
	ledger := newCompleteTurnRequestLedger()
	entry, owner := ledger.begin("unknown-commit-key")
	if !owner || entry == nil {
		t.Fatal("first request did not acquire key")
	}
	ledger.finish("unknown-commit-key", completeTurnRecordedResponse{
		status: http.StatusOK,
		body: []byte(`{"status":"error","code":"logical_turn_commit_outcome_unknown",` +
			`"save_ok":false,"raw_committed":false,"commit_state":"unknown",` +
			`"reconciliation_required":true,"retryable":false,"queue_action":"discard"}`),
	})
	duplicate, duplicateOwner := ledger.begin("unknown-commit-key")
	if duplicateOwner || duplicate != entry {
		t.Fatal("unknown commit outcome must not allow raw replacement replay")
	}
	status, _, found := ledger.status("unknown-commit-key")
	if !found || status != "completed" {
		t.Fatalf("unknown commit outcome was not retained: found=%v status=%q", found, status)
	}
}

func TestCompleteTurnTerminalFailedSaveStaysInLedger(t *testing.T) {
	ledger := newCompleteTurnRequestLedger()
	entry, owner := ledger.begin("terminal-save-key")
	if !owner || entry == nil {
		t.Fatal("first request did not acquire key")
	}
	ledger.finish("terminal-save-key", completeTurnRecordedResponse{
		status: http.StatusOK,
		body:   []byte(`{"status":"error","save_ok":false,"retryable":false,"queue_action":"discard"}`),
	})
	duplicate, duplicateOwner := ledger.begin("terminal-save-key")
	if duplicateOwner || duplicate != entry {
		t.Fatal("terminal failed save must return the recorded response without re-running")
	}
}

func TestCompleteTurnSemanticSuccessChecksSaveOK(t *testing.T) {
	response := completeTurnRecordedResponse{status: http.StatusOK, body: []byte(`{"status":"error","save_ok":false}`)}
	if completeTurnRecordedResponseSuccessful(response) {
		t.Fatal("HTTP 200 with save_ok=false must not be a successful completion")
	}
}

func TestCompleteTurnDerivedRetryRequirementKeepsRawSaveCompletion(t *testing.T) {
	ledger := newCompleteTurnRequestLedger()
	if _, owner := ledger.begin("retry-derived-key"); !owner {
		t.Fatal("first request did not acquire key")
	}
	ledger.finish("retry-derived-key", completeTurnRecordedResponse{
		status: http.StatusOK,
		body:   []byte(`{"status":"ok","save_ok":true,"derived_retry_required":true}`),
	})
	if _, owner := ledger.begin("retry-derived-key"); owner {
		t.Fatal("raw-saved request must remain completed while derived retry is handled separately")
	}
}

func TestCompleteTurnResponseBufferKeepsFirstStatus(t *testing.T) {
	buffer := newCompleteTurnResponseBuffer()
	buffer.WriteHeader(http.StatusBadRequest)
	buffer.WriteHeader(http.StatusOK)
	_, _ = buffer.Write([]byte(`{"code":"bad_request"}`))

	response := buffer.recorded()
	if response.status != http.StatusBadRequest {
		t.Fatalf("response status=%d, want %d", response.status, http.StatusBadRequest)
	}
}

func TestCompleteTurnRequestStatusDistinguishesFailedCompletion(t *testing.T) {
	server := &Server{CompleteTurns: newCompleteTurnRequestLedger()}
	if _, owner := server.CompleteTurns.begin("failed-key"); !owner {
		t.Fatal("failed to acquire test key")
	}
	server.CompleteTurns.finish("failed-key", completeTurnRecordedResponse{status: http.StatusBadRequest})

	req := httptest.NewRequest(http.MethodGet, "/complete-turn/request-status?idempotency_key=failed-key", nil)
	rec := httptest.NewRecorder()
	server.handleCompleteTurnRequestStatus(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status endpoint code=%d, want 200", rec.Code)
	}
	if got := rec.Body.String(); got == "" || !strings.Contains(got, `"success":false`) {
		t.Fatalf("status endpoint did not report failed completion: %s", got)
	}
}

func TestCompleteTurnRequestStatusSeparatesRawSaveFromDerivedRetry(t *testing.T) {
	server := &Server{CompleteTurns: newCompleteTurnRequestLedger()}
	if _, owner := server.CompleteTurns.begin("derived-key"); !owner {
		t.Fatal("failed to acquire test key")
	}
	server.CompleteTurns.finish("derived-key", completeTurnRecordedResponse{
		status: http.StatusOK,
		body:   []byte(`{"status":"ok","save_ok":true,"derived_retry_required":true}`),
	})

	req := httptest.NewRequest(http.MethodGet, "/complete-turn/request-status?idempotency_key=derived-key", nil)
	rec := httptest.NewRecorder()
	server.handleCompleteTurnRequestStatus(rec, req)
	got := rec.Body.String()
	for _, want := range []string{`"success":true`, `"raw_saved":true`, `"save_ok":true`, `"derived_retry_required":true`} {
		if !strings.Contains(got, want) {
			t.Fatalf("status endpoint missing %s: %s", want, got)
		}
	}
}

func TestCompleteTurnRequestStatusPreservesReconciliationTruth(t *testing.T) {
	server := &Server{CompleteTurns: newCompleteTurnRequestLedger()}
	if _, owner := server.CompleteTurns.begin("reconcile-key"); !owner {
		t.Fatal("failed to acquire test key")
	}
	server.CompleteTurns.finish("reconcile-key", completeTurnRecordedResponse{
		status: http.StatusOK,
		body: []byte(`{"status":"partial","code":"logical_turn_reference_cleanup_failed","save_ok":true,` +
			`"raw_committed":true,"commit_state":"committed","reconciliation_required":true,` +
			`"reconciliation_retry_idempotency_key":"reconcile:backend-owned","queue_action":"retry"}`),
	})

	req := httptest.NewRequest(http.MethodGet, "/complete-turn/request-status?idempotency_key=reconcile-key", nil)
	rec := httptest.NewRecorder()
	server.handleCompleteTurnRequestStatus(rec, req)
	got := rec.Body.String()
	for _, want := range []string{
		`"raw_saved":true`,
		`"raw_committed":true`,
		`"commit_state":"committed"`,
		`"reconciliation_required":true`,
		`"reconciliation_retry_idempotency_key":"reconcile:backend-owned"`,
		`"result_status":"partial"`,
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("status endpoint missing %s: %s", want, got)
		}
	}
}

func TestCompleteTurnReconciliationRetryIdempotencyKeyIsBackendOwnedAndStable(t *testing.T) {
	meta := map[string]any{"idempotency_key": "original-request"}
	first := completeTurnReconciliationRetryIdempotencyKey(meta)
	second := completeTurnReconciliationRetryIdempotencyKey(meta)
	if first == "" || first == "original-request" || first != second {
		t.Fatalf("retry key must be fresh and stable: first=%q second=%q", first, second)
	}
	if got := completeTurnReconciliationRetryIdempotencyKey(map[string]any{}); got != "" {
		t.Fatalf("missing original idempotency must not synthesize a key: %q", got)
	}
}

func TestCompleteTurnRejectedRawSavedIsNotSemanticSuccess(t *testing.T) {
	response := completeTurnRecordedResponse{
		status: http.StatusOK,
		body:   []byte(`{"status":"rejected","save_ok":true,"queue_action":"discard"}`),
	}
	if completeTurnRecordedResponseSuccessful(response) {
		t.Fatal("rejected response must not be reported as semantic success")
	}
	if completeTurnRecordedResponseHasFailedSave(response) {
		t.Fatal("save_ok=true must preserve raw-save truth even when semantic result is rejected")
	}
}
