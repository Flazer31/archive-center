package httpapi

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/risulongmemory/archive-center-go/internal/config"
	"github.com/risulongmemory/archive-center-go/internal/store"
)

func TestCriticArchiveLedgerPreviewBuildsReadOnlyLedgerFromFixtureStore(t *testing.T) {
	dir := t.TempDir()
	writeLedgerFixture(t, dir, "direct_evidence_records", map[string]any{
		"id":                   1,
		"chat_session_id":      "sess-ledger",
		"evidence_kind":        "user_correction",
		"evidence_text":        "<thinking>private draft</thinking>User corrected the bridge name.",
		"source_turn_start":    9,
		"source_turn_end":      9,
		"archive_state":        "committed",
		"capture_stage":        "critic_extract",
		"capture_verification": "verified",
		"created_at":           "2026-06-21T00:00:00Z",
	})
	writeLedgerFixture(t, dir, "memories", map[string]any{
		"id":              2,
		"chat_session_id": "sess-ledger",
		"turn_index":      8,
		"summary_json":    `{"summary":"Mina trusts Rowan after the bridge scene."}`,
		"importance":      8.5,
		"created_at":      "2026-06-21T00:00:01Z",
	})
	writeLedgerFixture(t, dir, "active_states", map[string]any{
		"id":              3,
		"chat_session_id": "sess-ledger",
		"state_type":      "relationship",
		"content":         "Mina currently trusts Rowan.",
		"turn_index":      10,
		"created_at":      "2026-06-21T00:00:02Z",
	})
	writeLedgerFixture(t, dir, "canonical_state_layers", map[string]any{
		"id":              4,
		"chat_session_id": "sess-ledger",
		"layer_type":      "relationship",
		"content":         "Rowan protected Mina at the bridge.",
		"turn_index":      10,
		"confidence":      0.9,
		"created_at":      "2026-06-21T00:00:03Z",
	})
	writeLedgerFixture(t, dir, "pending_threads", map[string]any{
		"id":              5,
		"chat_session_id": "sess-ledger",
		"title":           "Ask Mina why she hesitated.",
		"thread_type":     "open_question",
		"status":          "open",
		"source_turn":     10,
		"confidence":      0.8,
		"created_at":      "2026-06-21T00:00:04Z",
		"updated_at":      "2026-06-21T00:00:05Z",
	})
	writeLedgerFixture(t, dir, "audit_logs", map[string]any{
		"id":              6,
		"chat_session_id": "sess-ledger",
		"event_type":      "memory_semantic_dedup",
		"summary":         "Merged duplicate memory about the bridge scene.",
		"source":          "critic",
		"created_at":      "2026-06-21T00:00:06Z",
	})
	writeLedgerFixture(t, dir, "critic_feedback", map[string]any{
		"id":              7,
		"chat_session_id": "sess-ledger",
		"target_type":     "memory",
		"target_id":       2,
		"feedback_value":  "review",
		"feedback_note":   "Reviewed stale bridge wording.",
		"source":          "manual_ui",
		"created_at":      "2026-06-21T00:00:07Z",
	})

	fixture, err := store.NewFixtureStoreFromExportDir(dir)
	if err != nil {
		t.Fatalf("fixture store: %v", err)
	}
	cfg := config.Default()
	srv := NewServer(cfg)
	srv.Store = fixture
	mux := http.NewServeMux()
	srv.RegisterRoutes(mux)

	body := `{"chat_session_id":"sess-ledger","turn_index":10,"assistant_final_text":"좋아, 이어서 진행할게.","streaming_mismatch":"none"}`
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/critic/archive-ledger/preview", strings.NewReader(body))
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
	var resp map[string]any
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if resp["contract_version"] != criticArchiveLedgerContractVersion {
		t.Fatalf("contract_version=%v", resp["contract_version"])
	}
	if resp["status"] != "ok" {
		t.Fatalf("status=%v body=%+v", resp["status"], resp)
	}
	if resp["write_attempted"] != false || resp["llm_call_attempted"] != false || resp["vector_write_attempted"] != false {
		t.Fatalf("preview must remain read-only: %+v", resp)
	}
	if resp["vector_status"] != "not_required" {
		t.Fatalf("vector_status=%v", resp["vector_status"])
	}
	items, ok := resp["items"].([]any)
	if !ok || len(items) < 5 {
		t.Fatalf("items missing or too short: %#v", resp["items"])
	}
	lanes := map[string]bool{}
	allSummaries := ""
	for _, raw := range items {
		item := raw.(map[string]any)
		lanes[item["lane"].(string)] = true
		allSummaries += " " + item["summary"].(string)
	}
	for _, lane := range []string{"direct_evidence", "recent_accepted_memory", "active_state_snapshot", "unresolved_pending_thread", "recent_resolution_event"} {
		if !lanes[lane] {
			t.Fatalf("lane %s missing in %#v", lane, lanes)
		}
	}
	if strings.Contains(allSummaries, "private draft") || strings.Contains(allSummaries, "<thinking>") {
		t.Fatalf("reasoning text leaked into ledger summaries: %s", allSummaries)
	}
}

func TestCriticArchiveLedgerPreviewEmptyNoopStoreIsReadOnly(t *testing.T) {
	srv := NewServer(config.Default())
	mux := http.NewServeMux()
	srv.RegisterRoutes(mux)

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/critic/archive-ledger/preview", strings.NewReader(`{"chat_session_id":"empty-session"}`))
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
	var resp map[string]any
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if resp["status"] != "empty" {
		t.Fatalf("status=%v body=%+v", resp["status"], resp)
	}
	if resp["write_attempted"] != false || resp["llm_call_attempted"] != false {
		t.Fatalf("empty preview must remain read-only: %+v", resp)
	}
}

func TestCriticArchiveLedgerDebugSurfaceExposesDashboardTraceReadOnly(t *testing.T) {
	dir := t.TempDir()
	writeLedgerFixture(t, dir, "direct_evidence_records", map[string]any{
		"id":                   11,
		"chat_session_id":      "sess-debug",
		"evidence_text":        "<analysis>hidden scratch</analysis>Debug surface should show final archive evidence.",
		"source_turn_start":    12,
		"source_turn_end":      12,
		"archive_state":        "committed",
		"capture_verification": "verified",
		"created_at":           "2026-06-21T00:01:00Z",
	})
	writeLedgerFixture(t, dir, "memories", map[string]any{
		"id":              12,
		"chat_session_id": "sess-debug",
		"turn_index":      12,
		"summary_json":    `{"summary":"Debug memory lane is available."}`,
		"importance":      7.5,
		"created_at":      "2026-06-21T00:01:01Z",
	})

	fixture, err := store.NewFixtureStoreFromExportDir(dir)
	if err != nil {
		t.Fatalf("fixture store: %v", err)
	}
	cfg := config.Default()
	srv := NewServer(cfg)
	srv.Store = fixture
	mux := http.NewServeMux()
	srv.RegisterRoutes(mux)

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/critic/archive-ledger/debug?chat_session_id=sess-debug&turn_index=12&max_items_total=4", nil)
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
	var resp map[string]any
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if resp["contract_version"] != criticArchiveLedgerDebugContractVersion {
		t.Fatalf("contract_version=%v", resp["contract_version"])
	}
	if resp["preview_contract_version"] != criticArchiveLedgerContractVersion {
		t.Fatalf("preview_contract_version=%v", resp["preview_contract_version"])
	}
	if resp["read_only"] != true || resp["debug_only"] != true || resp["write_attempted"] != false || resp["llm_call_attempted"] != false || resp["vector_write_attempted"] != false {
		t.Fatalf("debug surface must remain read-only: %+v", resp)
	}
	dashboard := resp["dashboard"].(map[string]any)
	if dashboard["badge_status"] != "ready" {
		t.Fatalf("badge_status=%v dashboard=%+v", dashboard["badge_status"], dashboard)
	}
	if int(dashboard["item_count"].(float64)) < 2 {
		t.Fatalf("item_count too small: %+v", dashboard)
	}
	items := dashboard["items_preview"].([]any)
	if len(items) == 0 {
		t.Fatalf("items_preview missing: %+v", dashboard)
	}
	previewText := ""
	for _, raw := range items {
		previewText += " " + raw.(map[string]any)["summary_preview"].(string)
	}
	if strings.Contains(previewText, "hidden scratch") || strings.Contains(previewText, "<analysis>") {
		t.Fatalf("reasoning text leaked into debug preview: %s", previewText)
	}
	trace := resp["trace"].(map[string]any)
	if trace["route"] != criticArchiveLedgerDebugRoute || trace["preview_route"] != criticArchiveLedgerPreviewRoute {
		t.Fatalf("unexpected trace routes: %+v", trace)
	}
	if trace["preview_trace"] == nil {
		t.Fatalf("preview trace missing: %+v", trace)
	}
}

func TestCriticArchiveLedgerDebugRejectsBadQuery(t *testing.T) {
	srv := NewServer(config.Default())
	mux := http.NewServeMux()
	srv.RegisterRoutes(mux)

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/critic/archive-ledger/debug?chat_session_id=sess&turn_index=oops", nil)
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
}

func TestSupersessionResolutionDebugReadsResolutionAudit(t *testing.T) {
	dir := t.TempDir()
	writeLedgerFixture(t, dir, "audit_logs", map[string]any{
		"id":              31,
		"chat_session_id": "sess-resolution",
		"event_type":      "supersession_resolution",
		"target_type":     "memory",
		"target_id":       11,
		"summary":         "Resolution stale_demote: memory #11",
		"details_json":    `{"contract_version":"supersession_resolution.v1","resolution_class":"stale_demote","source_turn":6,"relationship_key":"obsolete clue","hard_delete":false}`,
		"source":          "critic",
		"created_at":      "2026-06-21T00:02:00Z",
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
	req := httptest.NewRequest(http.MethodGet, "/critic/supersession-resolution/debug?chat_session_id=sess-resolution&limit=10", nil)
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
	var resp map[string]any
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if resp["contract_version"] != store.SupersessionResolutionContractVersion || resp["status"] != "ok" {
		t.Fatalf("unexpected response: %+v", resp)
	}
	if resp["read_only"] != true || resp["write_attempted"] != false || resp["llm_call_attempted"] != false {
		t.Fatalf("debug route must remain read-only: %+v", resp)
	}
	records := resp["records"].([]any)
	if len(records) != 1 {
		t.Fatalf("records=%#v", records)
	}
	record := records[0].(map[string]any)
	if record["resolution_class"] != "stale_demote" || record["target_type"] != "memory" {
		t.Fatalf("unexpected record: %+v", record)
	}
}

func TestCriticArchiveLedgerPreviewCanBeDisabled(t *testing.T) {
	cfg := config.Default()
	cfg.CriticLedgerPreviewEnabled = false
	srv := NewServer(cfg)
	mux := http.NewServeMux()
	srv.RegisterRoutes(mux)

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/critic/archive-ledger/preview", strings.NewReader(`{"chat_session_id":"sess"}`))
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
	var resp map[string]any
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if resp["status"] != "disabled" || resp["write_attempted"] != false || resp["llm_call_attempted"] != false {
		t.Fatalf("unexpected disabled response: %+v", resp)
	}
}

func TestCriticArchiveLedgerScrubsGenericReasoningMarkers(t *testing.T) {
	raw := strings.Join([]string{
		"Visible archive line.",
		"<thinking>private thinking</thinking>",
		"<analysis>private analysis</analysis>",
		"<reasoning>private reasoning</reasoning>",
		"<scratchpad>private scratchpad</scratchpad>",
		"Chain of thought: private chain",
		"Reasoning: private line",
		"Scratchpad: private scratch line",
		"Visible final line.",
	}, "\n")
	got, scrubbed := criticLedgerScrubText(raw)
	if !scrubbed {
		t.Fatalf("expected scrubbed=true, got false with %q", got)
	}
	for _, blocked := range []string{"private thinking", "private analysis", "private reasoning", "private scratchpad", "private chain", "private line", "private scratch line", "<thinking", "<analysis", "<reasoning", "<scratchpad"} {
		if strings.Contains(strings.ToLower(got), strings.ToLower(blocked)) {
			t.Fatalf("criticLedgerScrubText leaked %q in %q", blocked, got)
		}
	}
	if !strings.Contains(got, "Visible archive line.") || !strings.Contains(got, "Visible final line.") {
		t.Fatalf("criticLedgerScrubText removed visible text: %q", got)
	}
}

func TestCriticArchiveLedgerLanguageParityUsesFinalOutputLanguage(t *testing.T) {
	explicit := criticArchiveLedgerLanguageFromRequest(criticArchiveLedgerPreviewRequest{
		AssistantFinalText:     "This final response is English, but the caller knows the visible output language.",
		AssistantFinalLanguage: "ko",
	})
	if explicit.AssistantFinalLanguage != "ko" || explicit.Source != "request_assistant_final_language" || explicit.OverrideApplied {
		t.Fatalf("explicit language mismatch: %+v", explicit)
	}

	inferredKo := criticArchiveLedgerLanguageFromRequest(criticArchiveLedgerPreviewRequest{
		AssistantFinalText: "\uc548\ub155. \ucd5c\uc885 \uc751\ub2f5\uc740 \ud55c\uad6d\uc5b4\uc785\ub2c8\ub2e4.",
	})
	if inferredKo.AssistantFinalLanguage != "ko" || inferredKo.Source != "assistant_final_text" {
		t.Fatalf("inferred Korean language mismatch: %+v", inferredKo)
	}

	inferredEn := criticArchiveLedgerLanguageFromRequest(criticArchiveLedgerPreviewRequest{
		AssistantFinalText: "The final visible response is English.",
	})
	if inferredEn.AssistantFinalLanguage != "en" || inferredEn.Source != "assistant_final_text" {
		t.Fatalf("inferred English language mismatch: %+v", inferredEn)
	}
}

func TestCriticArchiveLedgerSafetyReportsStreamingMismatch(t *testing.T) {
	srv := NewServer(config.Default())
	mux := http.NewServeMux()
	srv.RegisterRoutes(mux)

	body := `{"chat_session_id":"sess-safety","assistant_final_language":"ko","streaming_mismatch":"confirmed"}`
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/critic/archive-ledger/preview", strings.NewReader(body))
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
	var resp map[string]any
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	language := resp["language"].(map[string]any)
	if language["assistant_final_language"] != "ko" || language["source"] != "request_assistant_final_language" {
		t.Fatalf("language parity missing from response: %+v", language)
	}
	safety := resp["safety"].(map[string]any)
	if safety["streaming_mismatch"] != "confirmed" || safety["reasoning_scrub_applied"] != true || safety["raw_archive_dump_blocked"] != true {
		t.Fatalf("safety mismatch: %+v", safety)
	}
}

func writeLedgerFixture(t *testing.T, dir, table string, row map[string]any) {
	t.Helper()
	data, err := json.Marshal(row)
	if err != nil {
		t.Fatalf("marshal fixture: %v", err)
	}
	path := filepath.Join(dir, table+".ndjson")
	f, err := os.OpenFile(path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0644)
	if err != nil {
		t.Fatalf("open fixture %s: %v", table, err)
	}
	defer f.Close()
	if _, err := f.Write(append(data, '\n')); err != nil {
		t.Fatalf("write fixture %s: %v", table, err)
	}
}
