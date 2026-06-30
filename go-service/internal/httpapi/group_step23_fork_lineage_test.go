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

type forkLineageHTTPStore struct {
	store.Store
	records []store.ForkLineageRecord
	saved   store.ForkLineageRecord
}

func (f *forkLineageHTTPStore) ListForkLineageRecords(ctx context.Context, chatSessionID, scopeID string, limit int) ([]store.ForkLineageRecord, error) {
	out := make([]store.ForkLineageRecord, 0, len(f.records))
	for _, record := range f.records {
		if record.ChatSessionID != chatSessionID {
			continue
		}
		if scopeID != "" && record.ScopeID != scopeID {
			continue
		}
		out = append(out, record)
	}
	if limit > 0 && len(out) > limit {
		out = out[:limit]
	}
	return out, nil
}

func (f *forkLineageHTTPStore) SaveForkLineageRecord(ctx context.Context, record store.ForkLineageRecord) (store.ForkLineageRecord, error) {
	record.ID = 88
	if record.ImportedAt.IsZero() {
		record.ImportedAt = time.Date(2026, 6, 23, 3, 0, 0, 0, time.UTC)
	}
	record.CreatedAt = record.ImportedAt
	record.UpdatedAt = record.ImportedAt
	f.saved = record
	f.records = append(f.records, record)
	return record, nil
}

func TestStep23ForkLineageListIsSupportOnlyManualMode(t *testing.T) {
	srv := NewServer(config.Default())
	srv.Store = &forkLineageHTTPStore{
		Store: store.NewNoopStore(),
		records: []store.ForkLineageRecord{
			{
				ID:                  1,
				ChatSessionID:       "sess-fork",
				ScopeID:             "scope-child",
				ParentScopeID:       "scope-parent",
				CopiedFromSessionID: "sess-parent",
				ProvenanceSource:    "manual",
				InheritanceMode:     "conservative_import",
				InheritedItemsJSON:  `["consequence_records"]`,
				ImportedAt:          time.Date(2026, 6, 23, 3, 0, 0, 0, time.UTC),
			},
		},
	}
	mux := http.NewServeMux()
	srv.RegisterRoutes(mux)

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/step23/fork-lineage/sess-fork?scope_id=scope-child", nil)
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
	var resp step23ForkLineageListResponse
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if resp.ContractVersion != step23ForkLineageContractVersion || len(resp.Records) != 1 {
		t.Fatalf("unexpected response: %+v", resp)
	}
	if !resp.TruthBoundary.SupportOnly || resp.TruthBoundary.CanonicalTruthWriter || resp.TruthBoundary.SilentMergeBackAllowed || resp.TruthBoundary.HiddenOverwriteAllowed {
		t.Fatalf("truth boundary should prevent silent merge/overwrite: %+v", resp.TruthBoundary)
	}
	if resp.TruthBoundary.AutomaticHookAvailable || resp.TruthBoundary.CloseoutMode != "manual_provenance_declaration" {
		t.Fatalf("manual closeout mode expected: %+v", resp.TruthBoundary)
	}
}

func TestStep23ForkLineageDeclareManualProvenance(t *testing.T) {
	fake := &forkLineageHTTPStore{Store: store.NewNoopStore()}
	srv := NewServer(config.Default())
	srv.Store = fake
	mux := http.NewServeMux()
	srv.RegisterRoutes(mux)

	body := `{
		"chat_session_id":"sess-fork",
		"scope_id":"scope-child",
		"parent_scope_id":"scope-parent",
		"copied_from_session_id":"sess-parent",
		"imported_at":"2026-06-23T03:00:00Z",
		"divergence_marker":"{\"turn\":12}",
		"provenance_source":"manual",
		"inheritance_mode":"conservative_import",
		"inherited_items_json":"[\"consequence_records\",\"psychology_branches\"]"
	}`
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/step23/fork-lineage", strings.NewReader(body))
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
	var resp step23ForkLineageDeclareResponse
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if resp.Record.ID != 88 || fake.saved.ScopeID != "scope-child" || fake.saved.InheritanceMode != "conservative_import" {
		t.Fatalf("record was not saved as expected: resp=%+v saved=%+v", resp.Record, fake.saved)
	}
	if resp.TruthBoundary.CanonicalTruthWriter || resp.TruthBoundary.SilentMergeBackAllowed {
		t.Fatalf("fork lineage must not become canonical/merge writer: %+v", resp.TruthBoundary)
	}
}

func TestStep23ForkLineageDeclareRejectsAutomaticHookAndSelfParent(t *testing.T) {
	srv := NewServer(config.Default())
	srv.Store = &forkLineageHTTPStore{Store: store.NewNoopStore()}
	mux := http.NewServeMux()
	srv.RegisterRoutes(mux)

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/step23/fork-lineage", strings.NewReader(`{"chat_session_id":"sess-fork","scope_id":"scope-child","parent_scope_id":"scope-parent","provenance_source":"automatic_hook","inherited_items_json":"[]"}`))
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("automatic hook status=%d body=%s", rec.Code, rec.Body.String())
	}

	rec = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodPost, "/step23/fork-lineage", strings.NewReader(`{"chat_session_id":"sess-fork","scope_id":"same","parent_scope_id":"same","provenance_source":"manual","inherited_items_json":"[]"}`))
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("self parent status=%d body=%s", rec.Code, rec.Body.String())
	}
}
