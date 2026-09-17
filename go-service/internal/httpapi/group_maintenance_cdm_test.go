package httpapi

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/risulongmemory/archive-center-go/internal/config"
	"github.com/risulongmemory/archive-center-go/internal/store"
)

func TestMaintenanceContradictionDuplicatePreviewFindsEvidenceBoundCandidates(t *testing.T) {
	dir := t.TempDir()
	writeLedgerFixture(t, dir, "memories", map[string]any{
		"id":              11,
		"chat_session_id": "sess-cdm",
		"turn_index":      3,
		"summary_json":    `{"summary":"Mina promised Rowan she would return with the brass key."}`,
		"importance":      0.8,
		"created_at":      "2026-06-22T00:00:00Z",
	})
	writeLedgerFixture(t, dir, "memories", map[string]any{
		"id":              12,
		"chat_session_id": "sess-cdm",
		"turn_index":      4,
		"summary_json":    `{"summary":"Mina promised Rowan to return with the brass key."}`,
		"importance":      0.7,
		"created_at":      "2026-06-22T00:00:01Z",
	})
	writeLedgerFixture(t, dir, "direct_evidence_records", map[string]any{
		"id":                   21,
		"chat_session_id":      "sess-cdm",
		"evidence_text":        "Alice trusts Bob.",
		"source_turn_start":    5,
		"source_turn_end":      5,
		"archive_state":        "committed",
		"capture_verification": "verified",
		"created_at":           "2026-06-22T00:00:02Z",
	})
	writeLedgerFixture(t, dir, "direct_evidence_records", map[string]any{
		"id":                   22,
		"chat_session_id":      "sess-cdm",
		"evidence_text":        "Alice no longer trusts Bob.",
		"source_turn_start":    6,
		"source_turn_end":      6,
		"archive_state":        "committed",
		"capture_verification": "verified",
		"created_at":           "2026-06-22T00:00:03Z",
	})
	writeLedgerFixture(t, dir, "pending_threads", map[string]any{
		"id":              31,
		"chat_session_id": "sess-cdm",
		"title":           "Ask Mina why she hid the brass key.",
		"thread_type":     "open_question",
		"status":          "open",
		"source_turn":     4,
		"last_seen_turn":  8,
		"confidence":      0.8,
		"created_at":      "2026-06-22T00:00:04Z",
		"updated_at":      "2026-06-22T00:00:05Z",
	})
	writeLedgerFixture(t, dir, "audit_logs", map[string]any{
		"id":              41,
		"chat_session_id": "sess-cdm",
		"event_type":      "supersession_resolution",
		"target_type":     "pending_thread",
		"target_id":       31,
		"summary":         "Resolution close: pending_thread #31",
		"details_json":    `{"contract_version":"supersession_resolution.v1","resolution_class":"close","target":{"type":"pending_thread","id":31},"hard_delete":false}`,
		"source":          "critic",
		"created_at":      "2026-06-22T00:00:06Z",
	})

	fixture, err := store.NewFixtureStoreFromExportDir(dir)
	if err != nil {
		t.Fatalf("fixture store: %v", err)
	}
	srv := NewServer(config.Default())
	srv.Store = fixture
	mux := http.NewServeMux()
	srv.RegisterRoutes(mux)

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/maintenance/contradiction-duplicates/preview?chat_session_id=sess-cdm&limit=20", nil)
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
	var resp map[string]any
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if resp["contract_version"] != maintenanceCDMContractVersion {
		t.Fatalf("contract_version=%v", resp["contract_version"])
	}
	if resp["write_attempted"] != false || resp["llm_call_attempted"] != false || resp["auto_apply"] != false {
		t.Fatalf("preview must be inspect-only: %+v", resp)
	}
	candidates := resp["candidates"].([]any)
	if len(candidates) < 3 {
		t.Fatalf("expected at least 3 candidates, got %d: %s", len(candidates), rec.Body.String())
	}
	types := map[string]bool{}
	actions := map[string]bool{}
	for _, raw := range candidates {
		item := raw.(map[string]any)
		if item["evidence_bound"] != true {
			t.Fatalf("candidate is not evidence bound: %+v", item)
		}
		types[item["candidate_type"].(string)] = true
		actions[item["proposed_action"].(string)] = true
	}
	for _, want := range []string{"near_duplicate_memory", "direct_evidence_conflict", "thread_open_closed_contradiction"} {
		if !types[want] {
			t.Fatalf("candidate type %s missing in %#v; body=%s", want, types, rec.Body.String())
		}
	}
	if !actions["merge"] || !actions["review"] {
		t.Fatalf("expected merge and review action surface, got %#v", actions)
	}
	if strings.Contains(rec.Body.String(), `"write_attempted":true`) {
		t.Fatalf("route attempted a write: %s", rec.Body.String())
	}
}

func TestMaintenanceContradictionDuplicatePreviewRequiresSession(t *testing.T) {
	srv := NewServer(config.Default())
	mux := http.NewServeMux()
	srv.RegisterRoutes(mux)

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/maintenance/contradiction-duplicates/preview", nil)
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
}
