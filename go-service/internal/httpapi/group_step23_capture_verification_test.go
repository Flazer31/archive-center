package httpapi

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/risulongmemory/archive-center-go/internal/config"
	"github.com/risulongmemory/archive-center-go/internal/store"
)

type captureVerificationHTTPStore struct {
	store.Store
	records []store.CaptureVerificationRecord
	saved   store.CaptureVerificationRecord
	updated struct {
		id                 int64
		state              string
		degradedReason     string
		repairEvidenceJSON string
		repairedByRecordID int64
		userInputPreserved bool
	}
}

func (f *captureVerificationHTTPStore) ListCaptureVerifications(ctx context.Context, chatSessionID string, limit int) ([]store.CaptureVerificationRecord, error) {
	out := make([]store.CaptureVerificationRecord, 0, len(f.records))
	for _, record := range f.records {
		if record.ChatSessionID == chatSessionID {
			out = append(out, record)
		}
	}
	if limit > 0 && len(out) > limit {
		out = out[:limit]
	}
	return out, nil
}

func (f *captureVerificationHTTPStore) SaveCaptureVerification(ctx context.Context, record store.CaptureVerificationRecord) (store.CaptureVerificationRecord, error) {
	record.ID = 77
	record.CreatedAt = time.Date(2026, 6, 23, 5, 0, 0, 0, time.UTC)
	record.UpdatedAt = record.CreatedAt
	f.saved = record
	f.records = append(f.records, record)
	return record, nil
}

func (f *captureVerificationHTTPStore) UpdateCaptureVerificationRepair(ctx context.Context, id int64, state, degradedReason, repairEvidenceJSON string, repairedByID int64, userInputPreserved bool) error {
	f.updated.id = id
	f.updated.state = state
	f.updated.degradedReason = degradedReason
	f.updated.repairEvidenceJSON = repairEvidenceJSON
	f.updated.repairedByRecordID = repairedByID
	f.updated.userInputPreserved = userInputPreserved
	return nil
}

func TestStep23CaptureVerificationListSupportOnlyBoundary(t *testing.T) {
	srv := NewServer(config.Default())
	srv.Store = &captureVerificationHTTPStore{
		Store: store.NewNoopStore(),
		records: []store.CaptureVerificationRecord{
			{ID: 1, ChatSessionID: "sess-cap", TurnIndex: 1, StageName: "beforeRequestResponse", VerificationState: "single-stage", ContentHash: "h1", UserInputPreserved: true},
			{ID: 2, ChatSessionID: "sess-cap", TurnIndex: 1, StageName: "afterRequest", VerificationState: "multi-stage", ContentHash: "h2", UserInputPreserved: true},
			{ID: 3, ChatSessionID: "sess-cap", TurnIndex: 1, StageName: "finalize", VerificationState: "verified", ContentHash: "h3", UserInputPreserved: true},
			{ID: 4, ChatSessionID: "sess-cap", TurnIndex: 1, StageName: "recovery", VerificationState: "verified-final", ContentHash: "h4", UserInputPreserved: true},
			{ID: 5, ChatSessionID: "sess-cap", TurnIndex: 2, StageName: "recovery", VerificationState: "degraded", DegradedReason: "thinking_only_fragment", ContentHash: "h5", UserInputPreserved: true},
		},
	}
	mux := http.NewServeMux()
	srv.RegisterRoutes(mux)

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/step23/capture-verification/sess-cap", nil)
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
	var resp step23CaptureVerificationListResponse
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if resp.ContractVersion != step23CaptureVerificationContractVersion || len(resp.Records) != 5 {
		t.Fatalf("unexpected response: %+v", resp)
	}
	if resp.Counts.SingleStage != 1 || resp.Counts.MultiStage != 1 || resp.Counts.Verified != 1 || resp.Counts.VerifiedFinal != 1 || resp.Counts.Degraded != 1 {
		t.Fatalf("unexpected counts: %+v", resp.Counts)
	}
	if !resp.TruthBoundary.SupportOnly || resp.TruthBoundary.CanonicalTruthWriter || resp.TruthBoundary.MayRewriteUserInput || resp.TruthBoundary.MayAutoRepair {
		t.Fatalf("unexpected truth boundary: %+v", resp.TruthBoundary)
	}
}

func TestStep23CaptureVerificationCreateNormalizesStageAndDefaultsUserInputPreserved(t *testing.T) {
	fake := &captureVerificationHTTPStore{Store: store.NewNoopStore()}
	srv := NewServer(config.Default())
	srv.Store = fake
	mux := http.NewServeMux()
	srv.RegisterRoutes(mux)

	body := `{
		"chat_session_id":"sess-cap",
		"turn_index":4,
		"stage_name":"afterRequest",
		"verification_state":"verified-final",
		"compact_metadata_json":"{\"source\":\"native_afterRequest\"}",
		"content_hash":"sha256:final"
	}`
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/step23/capture-verification", strings.NewReader(body))
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
	if fake.saved.StageName != "afterRequest" || fake.saved.VerificationState != "verified-final" {
		t.Fatalf("stage/state not normalized: %+v", fake.saved)
	}
	if !fake.saved.UserInputPreserved || fake.saved.PayloadRewrite {
		t.Fatalf("immutability flags wrong: %+v", fake.saved)
	}
}

func TestStep23CaptureVerificationCreateRejectsRewriteAndUnevidencedCapture(t *testing.T) {
	srv := NewServer(config.Default())
	srv.Store = &captureVerificationHTTPStore{Store: store.NewNoopStore()}
	mux := http.NewServeMux()
	srv.RegisterRoutes(mux)

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/step23/capture-verification", strings.NewReader(`{"chat_session_id":"sess-cap","turn_index":1,"stage_name":"afterRequest","verification_state":"single-stage"}`))
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("missing evidence status=%d body=%s", rec.Code, rec.Body.String())
	}

	rec = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodPost, "/step23/capture-verification", strings.NewReader(`{"chat_session_id":"sess-cap","turn_index":1,"stage_name":"afterRequest","verification_state":"degraded","degraded_reason":"payload_mismatch","content_hash":"sha256:x","payload_rewrite":true}`))
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("payload rewrite status=%d body=%s", rec.Code, rec.Body.String())
	}
}

func TestStep23CaptureVerificationCreateRejectsInvalidJSONFieldsBeforeStore(t *testing.T) {
	fake := &captureVerificationHTTPStore{Store: store.NewNoopStore()}
	srv := NewServer(config.Default())
	srv.Store = fake
	mux := http.NewServeMux()
	srv.RegisterRoutes(mux)

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/step23/capture-verification", strings.NewReader(`{
		"chat_session_id":"sess-cap",
		"turn_index":1,
		"stage_name":"afterRequest",
		"verification_state":"degraded",
		"degraded_reason":"manual_bad_json",
		"content_hash":"sha256:x",
		"compact_metadata_json":"{\"source\":\"manual\""
	}`))
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("invalid compact metadata status=%d body=%s", rec.Code, rec.Body.String())
	}
	if fake.saved.ID != 0 {
		t.Fatalf("invalid JSON should be rejected before store save: %+v", fake.saved)
	}
}

func TestStep23CaptureVerificationCreateWorksBehindReverseProxyBasePath(t *testing.T) {
	fake := &captureVerificationHTTPStore{Store: store.NewNoopStore()}
	srv := NewServer(config.Default())
	srv.Store = fake
	mux := http.NewServeMux()
	srv.RegisterRoutes(mux)

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/proxy2/step23/capture-verification", strings.NewReader(`{
		"chat_session_id":"sess-cap",
		"turn_index":1,
		"stage_name":"afterRequest",
		"verification_state":"degraded",
		"degraded_reason":"manual_debug",
		"content_hash":"sha256:x",
		"compact_metadata_json":"{\"source\":\"manual\"}"
	}`))
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusCreated {
		t.Fatalf("prefixed capture verification status=%d body=%s", rec.Code, rec.Body.String())
	}
	if fake.saved.ChatSessionID != "sess-cap" || fake.saved.ID == 0 {
		t.Fatalf("prefixed route did not save record: %+v", fake.saved)
	}
}

func TestStep23CaptureVerificationRepairRequiresEvidenceForSuccess(t *testing.T) {
	fake := &captureVerificationHTTPStore{Store: store.NewNoopStore()}
	srv := NewServer(config.Default())
	srv.Store = fake
	mux := http.NewServeMux()
	srv.RegisterRoutes(mux)

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPatch, "/step23/capture-verification/77/repair", strings.NewReader(`{"verification_state":"verified-final"}`))
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("missing repair evidence status=%d body=%s", rec.Code, rec.Body.String())
	}

	rec = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodPatch, "/step23/capture-verification/77/repair", strings.NewReader(`{"verification_state":"verified-final","repair_evidence_json":"{\"matched_stage\":\"finalize\"}","repaired_by_record_id":76}`))
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
	if fake.updated.id != 77 || fake.updated.state != "verified-final" || fake.updated.repairEvidenceJSON == "" || fake.updated.repairedByRecordID != 76 || !fake.updated.userInputPreserved {
		t.Fatalf("unexpected repair update: %+v", fake.updated)
	}
}

func TestStep23CaptureVerificationUserInputNotPreservedRequiresDegraded(t *testing.T) {
	srv := NewServer(config.Default())
	srv.Store = &captureVerificationHTTPStore{Store: store.NewNoopStore()}
	mux := http.NewServeMux()
	srv.RegisterRoutes(mux)

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/step23/capture-verification", strings.NewReader(`{"chat_session_id":"sess-cap","turn_index":1,"stage_name":"afterRequest","verification_state":"verified","content_hash":"sha256:x","user_input_preserved":false}`))
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("create status=%d body=%s", rec.Code, rec.Body.String())
	}

	rec = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodPatch, "/step23/capture-verification/77/repair", strings.NewReader(`{"verification_state":"verified","repair_evidence_json":"{\"ok\":true}","user_input_preserved":false}`))
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("repair status=%d body=%s", rec.Code, rec.Body.String())
	}
}
