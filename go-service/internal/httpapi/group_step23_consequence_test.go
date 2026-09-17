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

type consequenceHTTPStore struct {
	store.Store
	records []store.ConsequenceRecord
	saved   store.ConsequenceRecord
}

func (f *consequenceHTTPStore) ListConsequenceRecords(ctx context.Context, chatSessionID string, limit int) ([]store.ConsequenceRecord, error) {
	out := make([]store.ConsequenceRecord, 0, len(f.records))
	for _, rec := range f.records {
		if rec.ChatSessionID == chatSessionID {
			out = append(out, rec)
		}
	}
	if limit > 0 && len(out) > limit {
		out = out[:limit]
	}
	return out, nil
}

func (f *consequenceHTTPStore) SaveConsequenceRecord(ctx context.Context, record store.ConsequenceRecord) (store.ConsequenceRecord, error) {
	record.ID = 99
	record.CreatedAt = time.Date(2026, 6, 23, 1, 0, 0, 0, time.UTC)
	record.UpdatedAt = record.CreatedAt
	f.saved = record
	f.records = append(f.records, record)
	return record, nil
}

func (f *consequenceHTTPStore) UpdateConsequenceRecordStatus(ctx context.Context, id int64, status string, paidTurn int) error {
	return nil
}

func TestStep23ConsequenceListReturnsSupportOnlyLedger(t *testing.T) {
	srv := NewServer(config.Default())
	srv.Store = &consequenceHTTPStore{
		Store: store.NewNoopStore(),
		records: []store.ConsequenceRecord{
			{
				ID:                 1,
				ChatSessionID:      "sess-23",
				SourceTurnStart:    3,
				SourceTurnEnd:      4,
				Decision:           "Rowan opened the sealed door.",
				ImmediateResult:    "Mina saw the hidden room.",
				DelayedEffect:      "The guard may investigate later.",
				Status:             "active",
				Importance:         0.7,
				Confidence:         0.8,
				ForegroundEligible: true,
				SourceHash:         "hash-1",
				EvidenceJSON:       `{"turns":[3,4]}`,
			},
			{
				ID:            2,
				ChatSessionID: "sess-23",
				Status:        "paid",
			},
		},
	}
	mux := http.NewServeMux()
	srv.RegisterRoutes(mux)

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/step23/consequences/sess-23?limit=10", nil)
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
	var resp step23ConsequenceListResponse
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if resp.ContractVersion != step23ConsequenceContractVersion {
		t.Fatalf("contract_version=%q", resp.ContractVersion)
	}
	if len(resp.Records) != 2 || resp.Counts.Total != 2 || resp.Counts.Active != 1 || resp.Counts.Paid != 1 {
		t.Fatalf("unexpected records/counts: %+v", resp)
	}
	if !resp.TruthBoundary.SupportOnly || resp.TruthBoundary.CanonicalTruthWriter || !resp.TruthBoundary.RequiresEvidence {
		t.Fatalf("truth boundary should stay support-only: %+v", resp.TruthBoundary)
	}
}

func TestStep23ConsequenceCreateRequiresEvidenceAndPersists(t *testing.T) {
	fake := &consequenceHTTPStore{Store: store.NewNoopStore()}
	srv := NewServer(config.Default())
	srv.Store = fake
	mux := http.NewServeMux()
	srv.RegisterRoutes(mux)

	body := `{
		"chat_session_id":"sess-23",
		"source_turn_start":5,
		"source_turn_end":5,
		"decision":"Mina keeps the letter.",
		"immediate_result":"Rowan notices the hesitation.",
		"delayed_effect":"The letter can resurface when trust drops.",
		"status":"pending",
		"importance":0.6,
		"confidence":2.0,
		"foreground_eligible":true,
		"source_hash":"hash-5",
		"evidence_json":"{\"turn\":5}"
	}`
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/step23/consequences", strings.NewReader(body))
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
	var resp step23ConsequenceCreateResponse
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if resp.Record.ID != 99 || fake.saved.ChatSessionID != "sess-23" {
		t.Fatalf("record was not saved as expected: resp=%+v saved=%+v", resp.Record, fake.saved)
	}
	if resp.Record.Confidence != 1 {
		t.Fatalf("confidence should be clamped to 1, got %v", resp.Record.Confidence)
	}
	if !resp.TruthBoundary.SupportOnly || resp.TruthBoundary.CanonicalTruthWriter || !resp.TruthBoundary.RequiresEvidence {
		t.Fatalf("truth boundary should stay support-only: %+v", resp.TruthBoundary)
	}
}

func TestStep23ConsequenceCreateRejectsMissingEvidence(t *testing.T) {
	srv := NewServer(config.Default())
	srv.Store = &consequenceHTTPStore{Store: store.NewNoopStore()}
	mux := http.NewServeMux()
	srv.RegisterRoutes(mux)

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/step23/consequences", strings.NewReader(`{"chat_session_id":"sess-23","source_turn_start":1,"source_turn_end":1,"decision":"Mina leaves."}`))
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
}
