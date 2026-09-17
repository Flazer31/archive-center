package httpapi

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"sync"

	"github.com/risulongmemory/archive-center-go/internal/dto"
)

const completeTurnRequestMax = 1024

type completeTurnRecordedResponse struct {
	status int
	header http.Header
	body   []byte
}

type completeTurnRequestEntry struct {
	sequence    uint64
	fingerprint string
	done        chan struct{}
	response    completeTurnRecordedResponse
	finished    bool
}

type completeTurnRequestLedger struct {
	mu           sync.Mutex
	entries      map[string]*completeTurnRequestEntry
	nextSequence uint64
}

func newCompleteTurnRequestLedger() *completeTurnRequestLedger {
	return &completeTurnRequestLedger{entries: map[string]*completeTurnRequestEntry{}}
}

func (l *completeTurnRequestLedger) begin(key string) (*completeTurnRequestEntry, bool) {
	entry, owner, _ := l.beginWithFingerprint(key, "")
	return entry, owner
}

func (l *completeTurnRequestLedger) beginWithFingerprint(key, fingerprint string) (*completeTurnRequestEntry, bool, bool) {
	l.mu.Lock()
	defer l.mu.Unlock()
	if entry := l.entries[key]; entry != nil {
		conflict := entry.fingerprint != "" && fingerprint != "" && entry.fingerprint != fingerprint
		return entry, false, conflict
	}
	if len(l.entries) >= completeTurnRequestMax {
		l.evictFinishedForCapacityLocked()
		if len(l.entries) >= completeTurnRequestMax {
			return nil, false, false
		}
	}
	l.nextSequence++
	entry := &completeTurnRequestEntry{sequence: l.nextSequence, fingerprint: fingerprint, done: make(chan struct{})}
	l.entries[key] = entry
	return entry, true, false
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
	if completeTurnRecordedResponseAllowsWholeRequestRetry(response) {
		delete(l.entries, key)
	}
}

func (l *completeTurnRequestLedger) finishIfCurrent(key string, expected *completeTurnRequestEntry, response completeTurnRecordedResponse) {
	l.mu.Lock()
	defer l.mu.Unlock()
	entry := l.entries[key]
	if entry == nil || entry != expected || entry.finished {
		return
	}
	entry.response = response
	entry.finished = true
	close(entry.done)
	if completeTurnRecordedResponseAllowsWholeRequestRetry(response) {
		delete(l.entries, key)
	}
}

func (l *completeTurnRequestLedger) finishOwner(key string, expected *completeTurnRequestEntry, response completeTurnRecordedResponse) {
	l.mu.Lock()
	defer l.mu.Unlock()
	entry := l.entries[key]
	if entry == nil || entry != expected {
		return
	}
	entry.response = response
	if !entry.finished {
		entry.finished = true
		close(entry.done)
	}
	if completeTurnRecordedResponseAllowsWholeRequestRetry(response) {
		delete(l.entries, key)
	}
}

func (l *completeTurnRequestLedger) status(key string) (string, completeTurnRecordedResponse, bool) {
	l.mu.Lock()
	defer l.mu.Unlock()
	entry := l.entries[key]
	if entry == nil {
		return "unknown", completeTurnRecordedResponse{}, false
	}
	if !entry.finished {
		return "processing", completeTurnRecordedResponse{}, true
	}
	return "completed", entry.response, true
}

func (l *completeTurnRequestLedger) responseForEntry(key string, expected *completeTurnRequestEntry) (completeTurnRecordedResponse, bool) {
	l.mu.Lock()
	defer l.mu.Unlock()
	entry := l.entries[key]
	if entry == nil || entry != expected || !entry.finished {
		return completeTurnRecordedResponse{}, false
	}
	return entry.response, true
}

func (l *completeTurnRequestLedger) evictFinishedForCapacityLocked() {
	oldestKey := ""
	var oldestSequence uint64
	for key, entry := range l.entries {
		if !entry.finished || completeTurnRecordedResponsePinsOutcome(entry.response) {
			continue
		}
		if oldestKey == "" || entry.sequence < oldestSequence {
			oldestKey = key
			oldestSequence = entry.sequence
		}
	}
	if oldestKey != "" {
		delete(l.entries, oldestKey)
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

func completeTurnRecordedResponseHasFailedSave(response completeTurnRecordedResponse) bool {
	if response.status < http.StatusOK || response.status >= http.StatusMultipleChoices {
		return false
	}
	payload, ok := parseCompleteTurnRecordedResponseState(response)
	if !ok {
		return false
	}
	return payload.SaveOK != nil && !*payload.SaveOK
}

func completeTurnRecordedResponseAllowsWholeRequestRetry(response completeTurnRecordedResponse) bool {
	payload, ok := parseCompleteTurnRecordedResponseState(response)
	if ok {
		if payload.RawCommitted || strings.EqualFold(strings.TrimSpace(payload.CommitState), "unknown") {
			return false
		}
		if payload.SaveOK != nil && *payload.SaveOK {
			return false
		}
		if payload.Retryable != nil && !*payload.Retryable {
			return false
		}
		if strings.EqualFold(strings.TrimSpace(payload.QueueAction), "discard") {
			return false
		}
		if payload.SaveOK != nil && !*payload.SaveOK {
			return true
		}
	}
	return response.status >= http.StatusInternalServerError
}

func completeTurnRecordedResponsePinsOutcome(response completeTurnRecordedResponse) bool {
	payload, ok := parseCompleteTurnRecordedResponseState(response)
	return ok && strings.EqualFold(strings.TrimSpace(payload.CommitState), "unknown")
}

func completeTurnRecordedResponseSuccessful(response completeTurnRecordedResponse) bool {
	if response.status < http.StatusOK || response.status >= http.StatusMultipleChoices {
		return false
	}
	payload, ok := parseCompleteTurnRecordedResponseState(response)
	if !ok || (payload.SaveOK != nil && !*payload.SaveOK) {
		return false
	}
	switch strings.ToLower(strings.TrimSpace(payload.Status)) {
	case "error", "rejected", "processing", "skeleton":
		return false
	default:
		return true
	}
}

type completeTurnRecordedResponseState struct {
	Status                            string `json:"status"`
	Code                              string `json:"code"`
	SaveOK                            *bool  `json:"save_ok"`
	RawCommitted                      bool   `json:"raw_committed"`
	CommitState                       string `json:"commit_state"`
	DerivedRetryRequired              bool   `json:"derived_retry_required"`
	ReconciliationRequired            bool   `json:"reconciliation_required"`
	ReconciliationRetryIdempotencyKey string `json:"reconciliation_retry_idempotency_key"`
	Retryable                         *bool  `json:"retryable"`
	QueueAction                       string `json:"queue_action"`
}

func parseCompleteTurnRecordedResponseState(response completeTurnRecordedResponse) (completeTurnRecordedResponseState, bool) {
	var payload completeTurnRecordedResponseState
	if json.Unmarshal(response.body, &payload) != nil {
		return completeTurnRecordedResponseState{}, false
	}
	return payload, true
}

func completeTurnIdempotencyKey(clientMeta map[string]any) string {
	key := strings.TrimSpace(stringFromAny(clientMeta["idempotency_key"]))
	if len(key) > 240 {
		key = key[:240]
	}
	return key
}

func completeTurnReconciliationRetryIdempotencyKey(clientMeta map[string]any) string {
	current := completeTurnIdempotencyKey(clientMeta)
	if current == "" {
		return ""
	}
	sum := sha256.Sum256([]byte("complete_turn_reconciliation_retry.v1|" + current))
	return "reconcile:" + fmt.Sprintf("%x", sum[:])
}

func completeTurnRequestFingerprint(req dto.M4CompleteTurnRequest) string {
	// The idempotency key protects the complete semantic request, not only the
	// raw pair. Context, improvement trace, language override, request type and
	// client source lineage can all change derived processing.
	encoded, _ := json.Marshal(req)
	return fmt.Sprintf("%x", sha256.Sum256(encoded))
}

func (s *Server) executeCompleteTurnIdempotent(ctx context.Context, w http.ResponseWriter, key, fingerprint string, run func(http.ResponseWriter)) {
	if key == "" || s.CompleteTurns == nil {
		run(w)
		return
	}
	entry, owner, conflict := s.CompleteTurns.beginWithFingerprint(key, fingerprint)
	if conflict {
		writeJSON(w, http.StatusOK, map[string]any{
			"status":          "error",
			"code":            "idempotency_key_conflict",
			"idempotency_key": key,
			"retryable":       false,
			"queue_action":    "discard",
			"save_ok":         false,
		})
		return
	}
	if entry == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]any{
			"status":          "error",
			"code":            "complete_turn_processing_capacity_reached",
			"idempotency_key": key,
			"retryable":       true,
			"queue_action":    "retry",
			"save_ok":         false,
		})
		return
	}
	if !owner {
		select {
		case <-entry.done:
			if response, ok := s.CompleteTurns.responseForEntry(key, entry); ok {
				writeCompleteTurnRecordedResponse(w, response)
			} else {
				writeJSON(w, http.StatusServiceUnavailable, map[string]any{
					"status":          "error",
					"code":            "complete_turn_recorded_response_unavailable",
					"idempotency_key": key,
					"retryable":       true,
					"queue_action":    "retry",
					"save_ok":         false,
				})
			}
		case <-ctx.Done():
			writeJSON(w, http.StatusAccepted, map[string]any{
				"status":          "processing",
				"code":            "idempotent_request_processing",
				"idempotency_key": key,
			})
		}
		return
	}
	if done := ctx.Done(); done != nil {
		go func() {
			<-done
			cancelled := newCompleteTurnResponseBuffer()
			writeJSON(cancelled, http.StatusRequestTimeout, map[string]any{
				"status":                  "error",
				"code":                    "complete_turn_request_outcome_unknown",
				"idempotency_key":         key,
				"commit_state":            "unknown",
				"reconciliation_required": true,
				"retryable":               false,
				"queue_action":            "discard",
				"save_ok":                 false,
			})
			s.CompleteTurns.finishIfCurrent(key, entry, cancelled.recorded())
		}()
	}

	buffer := newCompleteTurnResponseBuffer()
	run(buffer)
	response := buffer.recorded()
	s.CompleteTurns.finishOwner(key, entry, response)
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
	status, response, found := s.CompleteTurns.status(key)
	payload := map[string]any{
		"status":          status,
		"idempotency_key": key,
	}
	if found && status == "completed" {
		payload["http_status"] = response.status
		payload["success"] = completeTurnRecordedResponseSuccessful(response)
		if recorded, ok := parseCompleteTurnRecordedResponseState(response); ok {
			rawSaved := recorded.SaveOK != nil && *recorded.SaveOK
			payload["raw_saved"] = rawSaved
			payload["save_ok"] = rawSaved
			payload["result_status"] = recorded.Status
			payload["raw_committed"] = recorded.RawCommitted
			if recorded.CommitState != "" {
				payload["commit_state"] = recorded.CommitState
			}
			payload["derived_retry_required"] = recorded.DerivedRetryRequired
			payload["reconciliation_required"] = recorded.ReconciliationRequired
			if recorded.ReconciliationRetryIdempotencyKey != "" {
				payload["reconciliation_retry_idempotency_key"] = recorded.ReconciliationRetryIdempotencyKey
			}
			if recorded.Code != "" {
				payload["code"] = recorded.Code
			}
			if recorded.Retryable != nil {
				payload["retryable"] = *recorded.Retryable
			}
			if recorded.QueueAction != "" {
				payload["queue_action"] = recorded.QueueAction
			}
		}
	}
	writeJSON(w, http.StatusOK, payload)
}
