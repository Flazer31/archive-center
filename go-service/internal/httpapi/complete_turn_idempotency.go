package httpapi

import (
	"bytes"
	"context"
	"net/http"
	"strings"
	"sync"
	"time"
)

const (
	completeTurnRequestTTL = 10 * time.Minute
	completeTurnRequestMax = 1024
)

type completeTurnRecordedResponse struct {
	status int
	header http.Header
	body   []byte
}

type completeTurnRequestEntry struct {
	createdAt time.Time
	done      chan struct{}
	response  completeTurnRecordedResponse
	finished  bool
}

type completeTurnRequestLedger struct {
	mu      sync.Mutex
	entries map[string]*completeTurnRequestEntry
}

func newCompleteTurnRequestLedger() *completeTurnRequestLedger {
	return &completeTurnRequestLedger{entries: map[string]*completeTurnRequestEntry{}}
}

func (l *completeTurnRequestLedger) begin(key string, now time.Time) (*completeTurnRequestEntry, bool) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.pruneLocked(now)
	if entry := l.entries[key]; entry != nil {
		return entry, false
	}
	entry := &completeTurnRequestEntry{createdAt: now, done: make(chan struct{})}
	l.entries[key] = entry
	return entry, true
}

func (l *completeTurnRequestLedger) finish(key string, response completeTurnRecordedResponse) {
	l.mu.Lock()
	defer l.mu.Unlock()
	entry := l.entries[key]
	if entry == nil || entry.finished {
		return
	}
	entry.response = response
	entry.finished = true
	close(entry.done)
	if response.status >= http.StatusInternalServerError {
		delete(l.entries, key)
	}
}

func (l *completeTurnRequestLedger) status(key string, now time.Time) (string, completeTurnRecordedResponse, bool) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.pruneLocked(now)
	entry := l.entries[key]
	if entry == nil {
		return "unknown", completeTurnRecordedResponse{}, false
	}
	if !entry.finished {
		return "processing", completeTurnRecordedResponse{}, true
	}
	return "completed", entry.response, true
}

func (l *completeTurnRequestLedger) pruneLocked(now time.Time) {
	for key, entry := range l.entries {
		if entry.finished && now.Sub(entry.createdAt) > completeTurnRequestTTL {
			delete(l.entries, key)
		}
	}
	if len(l.entries) <= completeTurnRequestMax {
		return
	}
	for key, entry := range l.entries {
		if entry.finished {
			delete(l.entries, key)
			if len(l.entries) <= completeTurnRequestMax {
				return
			}
		}
	}
}

type completeTurnResponseBuffer struct {
	header      http.Header
	status      int
	wroteHeader bool
	body        bytes.Buffer
}

func newCompleteTurnResponseBuffer() *completeTurnResponseBuffer {
	return &completeTurnResponseBuffer{header: make(http.Header)}
}

func (w *completeTurnResponseBuffer) Header() http.Header { return w.header }

func (w *completeTurnResponseBuffer) WriteHeader(status int) {
	if w.wroteHeader {
		return
	}
	w.status = status
	w.wroteHeader = true
}

func (w *completeTurnResponseBuffer) Write(data []byte) (int, error) {
	if !w.wroteHeader {
		w.WriteHeader(http.StatusOK)
	}
	return w.body.Write(data)
}

func (w *completeTurnResponseBuffer) recorded() completeTurnRecordedResponse {
	status := w.status
	if status == 0 {
		status = http.StatusOK
	}
	return completeTurnRecordedResponse{status: status, header: w.header.Clone(), body: append([]byte(nil), w.body.Bytes()...)}
}

func writeCompleteTurnRecordedResponse(w http.ResponseWriter, response completeTurnRecordedResponse) {
	for key, values := range response.header {
		for _, value := range values {
			w.Header().Add(key, value)
		}
	}
	status := response.status
	if status == 0 {
		status = http.StatusOK
	}
	w.WriteHeader(status)
	_, _ = w.Write(response.body)
}

func completeTurnIdempotencyKey(clientMeta map[string]any) string {
	key := strings.TrimSpace(stringFromAny(clientMeta["idempotency_key"]))
	if len(key) > 240 {
		key = key[:240]
	}
	return key
}

func (s *Server) executeCompleteTurnIdempotent(ctx context.Context, w http.ResponseWriter, key string, run func(http.ResponseWriter)) {
	if key == "" || s.CompleteTurns == nil {
		run(w)
		return
	}
	entry, owner := s.CompleteTurns.begin(key, time.Now().UTC())
	if !owner {
		select {
		case <-entry.done:
			writeCompleteTurnRecordedResponse(w, entry.response)
		case <-ctx.Done():
			writeJSON(w, http.StatusAccepted, map[string]any{
				"status":          "processing",
				"code":            "idempotent_request_processing",
				"idempotency_key": key,
			})
		}
		return
	}

	buffer := newCompleteTurnResponseBuffer()
	run(buffer)
	response := buffer.recorded()
	s.CompleteTurns.finish(key, response)
	writeCompleteTurnRecordedResponse(w, response)
}

func (s *Server) handleCompleteTurnRequestStatus(w http.ResponseWriter, r *http.Request) {
	key := strings.TrimSpace(r.URL.Query().Get("idempotency_key"))
	if key == "" {
		writeError(w, http.StatusBadRequest, "missing_param", "idempotency_key is required")
		return
	}
	if s.CompleteTurns == nil {
		writeJSON(w, http.StatusOK, map[string]any{"status": "unknown", "idempotency_key": key})
		return
	}
	status, response, found := s.CompleteTurns.status(key, time.Now().UTC())
	payload := map[string]any{
		"status":          status,
		"idempotency_key": key,
	}
	if found && status == "completed" {
		payload["http_status"] = response.status
		payload["success"] = response.status >= http.StatusOK && response.status < http.StatusMultipleChoices
	}
	writeJSON(w, http.StatusOK, payload)
}
