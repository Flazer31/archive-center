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

type themeOffscreenHTTPStore struct {
	store.Store
	records []store.ThemeOffscreenCarryRecord
	saved   store.ThemeOffscreenCarryRecord
	updated struct {
		id         int64
		status     string
		quietTurns int
	}
}

func (f *themeOffscreenHTTPStore) ListThemeOffscreenCarries(ctx context.Context, chatSessionID, surfaceType string, limit int) ([]store.ThemeOffscreenCarryRecord, error) {
	out := make([]store.ThemeOffscreenCarryRecord, 0, len(f.records))
	for _, record := range f.records {
		if record.ChatSessionID != chatSessionID {
			continue
		}
		if surfaceType != "" && record.SurfaceType != surfaceType {
			continue
		}
		out = append(out, record)
	}
	if limit > 0 && len(out) > limit {
		out = out[:limit]
	}
	return out, nil
}

func (f *themeOffscreenHTTPStore) SaveThemeOffscreenCarry(ctx context.Context, record store.ThemeOffscreenCarryRecord) (store.ThemeOffscreenCarryRecord, error) {
	record.ID = 66
	record.CreatedAt = time.Date(2026, 6, 23, 4, 0, 0, 0, time.UTC)
	record.UpdatedAt = record.CreatedAt
	f.saved = record
	f.records = append(f.records, record)
	return record, nil
}

func (f *themeOffscreenHTTPStore) UpdateThemeOffscreenCarryStatus(ctx context.Context, id int64, status string, quietTurns int) error {
	f.updated.id = id
	f.updated.status = status
	f.updated.quietTurns = quietTurns
	return nil
}

func TestStep23ThemeOffscreenListSupportOnlyBoundary(t *testing.T) {
	srv := NewServer(config.Default())
	srv.Store = &themeOffscreenHTTPStore{
		Store: store.NewNoopStore(),
		records: []store.ThemeOffscreenCarryRecord{
			{
				ID:                     1,
				ChatSessionID:          "sess-theme",
				SurfaceType:            "theme_trace",
				Label:                  "winter promises",
				Summary:                "Recurring image of promises made during snowstorms.",
				Status:                 "active",
				Confidence:             0.8,
				ConfidenceLabel:        "high",
				SourceTurnStart:        3,
				SourceTurnEnd:          5,
				SourceHash:             "hash-theme",
				EvidenceJSON:           `{"turns":[3,5]}`,
				DormantAfterQuietTurns: 15,
				ForegroundEligible:     true,
				ForegroundReasonJSON:   `{"reason":"current scene repeats motif"}`,
			},
			{ID: 2, ChatSessionID: "sess-theme", SurfaceType: "offscreen_thread", Status: "dormant"},
		},
	}
	mux := http.NewServeMux()
	srv.RegisterRoutes(mux)

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/step23/theme-offscreen/sess-theme", nil)
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
	var resp step23ThemeOffscreenListResponse
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if resp.ContractVersion != step23ThemeOffscreenContractVersion || len(resp.Records) != 2 {
		t.Fatalf("unexpected response: %+v", resp)
	}
	if resp.Counts.ThemeTrace != 1 || resp.Counts.OffscreenThread != 1 || resp.Counts.ForegroundEligible != 1 {
		t.Fatalf("unexpected counts: %+v", resp.Counts)
	}
	if !resp.TruthBoundary.SupportOnly || resp.TruthBoundary.CanonicalWorldFactWriter || resp.TruthBoundary.AlwaysInjected {
		t.Fatalf("truth boundary should be support-only and not always injected: %+v", resp.TruthBoundary)
	}
	if resp.TruthBoundary.MayOverrideCurrentUserInput || resp.TruthBoundary.MayOverrideDirectEvidence {
		t.Fatalf("theme/offscreen must not override higher authority: %+v", resp.TruthBoundary)
	}
}

func TestStep23ThemeOffscreenCreateAppliesDormancyDefault(t *testing.T) {
	fake := &themeOffscreenHTTPStore{Store: store.NewNoopStore()}
	srv := NewServer(config.Default())
	srv.Store = fake
	mux := http.NewServeMux()
	srv.RegisterRoutes(mux)

	body := `{
		"chat_session_id":"sess-theme",
		"surface_type":"offscreen_thread",
		"label":"north gate unrest",
		"summary":"The north gate faction is pressuring the council offscreen.",
		"confidence":0.6,
		"source_turn_start":7,
		"source_turn_end":8,
		"source_hash":"hash-offscreen",
		"quiet_turns":15
	}`
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/step23/theme-offscreen", strings.NewReader(body))
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
	var resp step23ThemeOffscreenCreateResponse
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if resp.Record.ID != 66 || fake.saved.SurfaceType != "offscreen_thread" {
		t.Fatalf("record not saved as expected: resp=%+v saved=%+v", resp.Record, fake.saved)
	}
	if resp.Record.Status != "dormant" || resp.Record.DormantAfterQuietTurns != 15 {
		t.Fatalf("default dormancy not applied: %+v", resp.Record)
	}
	if resp.Record.ConfidenceLabel != "medium" {
		t.Fatalf("confidence label = %q, want medium", resp.Record.ConfidenceLabel)
	}
}

func TestStep23ThemeOffscreenCreateRejectsMissingEvidenceAndForegroundReason(t *testing.T) {
	srv := NewServer(config.Default())
	srv.Store = &themeOffscreenHTTPStore{Store: store.NewNoopStore()}
	mux := http.NewServeMux()
	srv.RegisterRoutes(mux)

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/step23/theme-offscreen", strings.NewReader(`{"chat_session_id":"sess-theme","surface_type":"theme_trace","label":"winter","summary":"snow motif","source_turn_start":1,"source_turn_end":1}`))
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("missing evidence status=%d body=%s", rec.Code, rec.Body.String())
	}

	rec = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodPost, "/step23/theme-offscreen", strings.NewReader(`{"chat_session_id":"sess-theme","surface_type":"theme_trace","label":"winter","summary":"snow motif","source_turn_start":1,"source_turn_end":1,"source_hash":"hash","foreground_eligible":true}`))
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("foreground reason status=%d body=%s", rec.Code, rec.Body.String())
	}
}

func TestStep23ThemeOffscreenStatusUpdatesLifecycleOnly(t *testing.T) {
	fake := &themeOffscreenHTTPStore{Store: store.NewNoopStore()}
	srv := NewServer(config.Default())
	srv.Store = fake
	mux := http.NewServeMux()
	srv.RegisterRoutes(mux)

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPatch, "/step23/theme-offscreen/66/status", strings.NewReader(`{"status":"dormant","quiet_turns":16}`))
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
	if fake.updated.id != 66 || fake.updated.status != "dormant" || fake.updated.quietTurns != 16 {
		t.Fatalf("unexpected update: %+v", fake.updated)
	}
}
