package httpapi

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

func TestCompleteTurnRequestLedgerSharesCompletedResponse(t *testing.T) {
	ledger := newCompleteTurnRequestLedger()
	entry, owner := ledger.begin("req-1", time.Now().UTC())
	if !owner || entry == nil {
		t.Fatal("first request must own the idempotency key")
	}
	duplicate, duplicateOwner := ledger.begin("req-1", time.Now().UTC())
	if duplicateOwner || duplicate != entry {
		t.Fatal("duplicate request must join the in-flight entry")
	}
	ledger.finish("req-1", completeTurnRecordedResponse{status: http.StatusOK, body: []byte(`{"status":"ok"}`)})
	select {
	case <-duplicate.done:
	case <-time.After(time.Second):
		t.Fatal("duplicate request did not observe completion")
	}
	status, response, found := ledger.status("req-1", time.Now().UTC())
	if !found || status != "completed" || response.status != http.StatusOK {
		t.Fatalf("unexpected completed status: found=%v status=%q response=%+v", found, status, response)
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
		server.executeCompleteTurnIdempotent(context.Background(), first, "same-key", run)
		close(firstDone)
	}()
	<-started

	second := httptest.NewRecorder()
	secondDone := make(chan struct{})
	go func() {
		server.executeCompleteTurnIdempotent(context.Background(), second, "same-key", run)
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

func TestCompleteTurnServerErrorAllowsLaterRetry(t *testing.T) {
	ledger := newCompleteTurnRequestLedger()
	if _, owner := ledger.begin("retry-key", time.Now().UTC()); !owner {
		t.Fatal("first request did not acquire key")
	}
	ledger.finish("retry-key", completeTurnRecordedResponse{status: http.StatusInternalServerError})
	if _, owner := ledger.begin("retry-key", time.Now().UTC()); !owner {
		t.Fatal("server-error request key must be available for retry")
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
	if _, owner := server.CompleteTurns.begin("failed-key", time.Now().UTC()); !owner {
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
