package httpapi

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/risulongmemory/archive-center-go/internal/config"

	"github.com/risulongmemory/archive-center-go/internal/store"
)

func TestSessionStep7HealthSeededLongSession(t *testing.T) {

	chatLogs := make([]store.ChatLog, 12)
	for i := 0; i < 12; i++ {
		chatLogs[i] = store.ChatLog{ID: int64(i + 1), ChatSessionID: "sess-l5", TurnIndex: i + 1, Role: "user", Content: "msg"}
	}

	auditLogs := make([]store.AuditLog, 8)
	for i := 0; i < 8; i++ {
		auditLogs[i] = store.AuditLog{
			ID:            int64(i + 1),
			ChatSessionID: "sess-l5",
			EventType:     "maintenance_enqueued",
			DetailsJSON:   `{"suggestion":"ok"}`,
		}
	}

	spJSON, _ := json.Marshal(map[string]any{
		"next_beats":      []any{"approach the rooftop", "check the alleyway"},
		"active_tensions": []any{"tensionA"},
	})
	dirJSON, _ := json.Marshal(map[string]any{
		"scene_mandate":     "Rooftop confrontation",
		"required_outcomes": []any{"keep rooftop promise", "preserve hesitation"},
		"forbidden_moves":   []any{"do not jump scene", "do not erase tension", "do not resolve offscreen"},
		"resolved_outcomes": []any{"old staircase beat resolved"},
	})
	warnJSON, _ := json.Marshal([]any{})

	fake := &narrativeFakeStore{
		chatLogs: chatLogs,
		storylines: []store.Storyline{
			{ChatSessionID: "sess-l5", Name: "Arc A", Status: "resolved", FirstTurn: 1, LastTurn: 5, Confidence: 0.9, EvidenceCount: 2},
		},
		pendingThreads: []store.PendingThread{
			{ChatSessionID: "sess-l5", ThreadKey: "hook-1", Status: "resolved", ResolvedTurn: 4, Pinned: true},
		},
		guidancePlanState: &store.GuidancePlanState{
			ChatSessionID: "sess-l5",
			StoryPlanJSON: string(spJSON),
			DirectorJSON:  string(dirJSON),
			StateStatus:   "active",
			LastTurn:      7,
			WarningsJSON:  string(warnJSON),
		},
		auditLogs: auditLogs,
	}

	mux := http.NewServeMux()
	srv := setupTestServer()
	srv.Store = fake
	srv.RegisterRoutes(mux)

	req := httptest.NewRequest(http.MethodGet, "/sessions/sess-l5/step7-health", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200: %s", rec.Code, rec.Body.String())
	}
	var resp map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if resp["status"] != "ok" {
		t.Fatalf("status = %v, want ok", resp["status"])
	}
	if resp["total_turns"] != float64(12) {
		t.Fatalf("total_turns = %v, want 12", resp["total_turns"])
	}

	gs, ok := resp["guidance_state"].(map[string]any)
	if !ok {
		t.Fatal("guidance_state missing")
	}
	if gs["status"] != "active" {
		t.Fatalf("guidance_state.status = %v, want active", gs["status"])
	}
	if gs["last_built_turn"] != float64(7) {
		t.Fatalf("last_built_turn = %v, want 7", gs["last_built_turn"])
	}
	if gs["arc_age_turns"] != float64(5) {
		t.Fatalf("arc_age_turns = %v, want 5", gs["arc_age_turns"])
	}
	if gs["active_tensions"] != float64(1) {
		t.Fatalf("active_tensions = %v, want 1", gs["active_tensions"])
	}
	if gs["next_beats"] != float64(2) {
		t.Fatalf("next_beats = %v, want 2", gs["next_beats"])
	}
	if gs["open_required"] != float64(2) {
		t.Fatalf("open_required = %v, want 2", gs["open_required"])
	}
	if gs["forbidden_count"] != float64(3) {
		t.Fatalf("forbidden_count = %v, want 3", gs["forbidden_count"])
	}

	cs, ok := resp["compaction_summary"].(map[string]any)
	if !ok {
		t.Fatal("compaction_summary missing")
	}
	if cs["total_records"] == float64(0) {
		t.Fatalf("compaction_summary.total_records = 0, want >0")
	}

	ms, ok := resp["maintenance_summary"].(map[string]any)
	if !ok {
		t.Fatal("maintenance_summary missing")
	}
	if ms["total_passes"] != float64(8) {
		t.Fatalf("maintenance_summary.total_passes = %v, want 8", ms["total_passes"])
	}
	if ms["ok_count"] != float64(8) {
		t.Fatalf("maintenance_summary.ok_count = %v, want 8", ms["ok_count"])
	}
	if ms["ok_rate"] != float64(1.0) {
		t.Fatalf("maintenance_summary.ok_rate = %v, want 1.0", ms["ok_rate"])
	}
	ds, ok := resp["drift_summary"].(map[string]any)
	if !ok {
		t.Fatal("drift_summary missing")
	}
	if ds["passes_analyzed"] != float64(8) || ds["high_severity"] != float64(0) {
		t.Fatalf("drift_summary mismatch: %#v", ds)
	}

	rc, ok := resp["regression_checks"].(map[string]any)
	if !ok {
		t.Fatal("regression_checks missing")
	}
	if rc["guidance_persistence"] != "pass" {
		t.Fatalf("guidance_persistence = %v, want pass", rc["guidance_persistence"])
	}
	if rc["arc_stability"] != "pass" {
		t.Fatalf("arc_stability = %v, want pass", rc["arc_stability"])
	}
	if rc["compaction_health"] != "pass" {
		t.Fatalf("compaction_health = %v, want pass", rc["compaction_health"])
	}
	if rc["maintenance_effect"] != "pass" {
		t.Fatalf("maintenance_effect = %v, want pass", rc["maintenance_effect"])
	}

	warnings, _ := resp["warnings"].([]any)
	if len(warnings) != 0 {
		t.Fatalf("expected empty warnings, got %v", warnings)
	}
}

func TestChapterSummaryStoreAndExportContract(t *testing.T) {
	cs := store.ChapterSummary{
		ID:                      1,
		ChatSessionID:           "sess-p174",
		FromTurn:                1,
		ToTurn:                  60,
		ChapterIndex:            1,
		ChapterTitle:            "Gate",
		SummaryText:             "Alice opens the gate.",
		OpenLoopsJSON:           `["gate"]`,
		RelationshipChangesJSON: `["Alice trusts Bob"]`,
		WorldChangesJSON:        `["gate opens"]`,
		CallbackCandidatesJSON:  `["sealed ledger"]`,
		ResumeText:              "Resume for turns 1-60.",
		EmbeddingVector:         "vec",
		EmbeddingModel:          "model",
	}
	if cs.ChatSessionID != "sess-p174" || cs.FromTurn != 1 || cs.ToTurn != 60 {
		t.Fatalf("ChapterSummary contract fields mismatch: %+v", cs)
	}
	rp := store.ResumePack{Chapter: &cs}
	if rp.Chapter == nil || rp.Chapter.ChapterTitle != "Gate" {
		t.Fatalf("ResumePack.Chapter link broken")
	}
}

func TestEpisodeGeneratePersistsDS1aStructuredAnchors(t *testing.T) {
	mux := http.NewServeMux()
	srv := setupTestServer()
	srv.Cfg.StoreMode = config.StoreModeMariaDBAuthority
	fake := &narrativeFakeStore{
		chatLogs: []store.ChatLog{
			{ID: 1, ChatSessionID: "sess-ds1a", TurnIndex: 1, Role: "user", Content: "Alice asks Bob to open the sealed gate.", CreatedAt: time.Unix(10, 0)},
			{ID: 2, ChatSessionID: "sess-ds1a", TurnIndex: 2, Role: "assistant", Content: "Bob keeps his promise, Alice trusts Bob, but the sealed gate remains unresolved.", CreatedAt: time.Unix(11, 0)},
		},
	}
	srv.Store = fake
	srv.RegisterRoutes(mux)

	req := httptest.NewRequest(http.MethodPost, "/episodes/generate", strings.NewReader(`{"chat_session_id":"sess-ds1a","from_turn":1,"to_turn":2}`))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200: %s", rec.Code, rec.Body.String())
	}
	var resp map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if resp["status"] != "ok" || resp["saved"] != true {
		t.Fatalf("expected saved ok episode response, got %#v", resp)
	}
	if len(fake.savedEpisodeSummaries) != 1 {
		t.Fatalf("savedEpisodeSummaries = %d, want 1", len(fake.savedEpisodeSummaries))
	}
	saved := fake.savedEpisodeSummaries[0]
	if saved.SummaryText == "" || saved.KeyEvents == "" {
		t.Fatalf("episode summary/key events missing: %+v", saved)
	}
	if !strings.Contains(saved.KeyEvents, "Alice asks Bob") {
		t.Fatalf("key_events did not preserve event anchor: %s", saved.KeyEvents)
	}
	if !strings.Contains(saved.RelationshipChangesJSON, "Alice trusts Bob") {
		t.Fatalf("relationship_changes_json did not preserve relationship anchor: %s", saved.RelationshipChangesJSON)
	}
	if !strings.Contains(saved.OpenLoopsJSON, "sealed gate") {
		t.Fatalf("open_loops_json did not preserve open-loop anchor: %s", saved.OpenLoopsJSON)
	}
	trace, _ := resp["generation_trace"].(map[string]any)
	if trace["dense_summary_contract"] != "ds1a.v1" {
		t.Fatalf("generation_trace missing ds1a contract: %+v", trace)
	}
}

func TestEpisodeRegenerateReplacesExistingRange(t *testing.T) {
	mux := http.NewServeMux()
	srv := setupTestServer()
	srv.Cfg.StoreMode = config.StoreModeMariaDBAuthority
	fake := &narrativeFakeStore{
		chatLogs: []store.ChatLog{
			{ID: 1, ChatSessionID: "sess-regen", TurnIndex: 1, Role: "user", Content: "Alice reaches the broken bridge.", CreatedAt: time.Unix(10, 0)},
			{ID: 2, ChatSessionID: "sess-regen", TurnIndex: 2, Role: "assistant", Content: "Bob repairs a cable while the bridge remains unsafe.", CreatedAt: time.Unix(11, 0)},
		},
		episodeSummaries: []store.EpisodeSummary{
			{ID: 7, ChatSessionID: "sess-regen", FromTurn: 1, ToTurn: 2, SummaryText: "old truncated episode"},
		},
	}
	srv.Store = fake
	srv.RegisterRoutes(mux)

	req := httptest.NewRequest(http.MethodPost, "/episodes/regenerate", strings.NewReader(`{"chat_session_id":"sess-regen","from_turn":1,"to_turn":2}`))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200: %s", rec.Code, rec.Body.String())
	}
	var resp map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if resp["status"] != "ok" || resp["code"] != "episode_regenerated" || resp["saved"] != true {
		t.Fatalf("expected regenerated ok response, got %#v", resp)
	}
	if got := fmt.Sprint(fake.deletedEpisodeRanges); !strings.Contains(got, "sess-regen:1:2") {
		t.Fatalf("range delete not recorded: %s", got)
	}
	if len(fake.savedEpisodeSummaries) != 1 {
		t.Fatalf("savedEpisodeSummaries = %d, want 1", len(fake.savedEpisodeSummaries))
	}
	if strings.Contains(fake.savedEpisodeSummaries[0].SummaryText, "old truncated") {
		t.Fatalf("regenerated summary kept old text: %+v", fake.savedEpisodeSummaries[0])
	}
}

func TestEpisodeDeleteRemovesByID(t *testing.T) {
	mux := http.NewServeMux()
	srv := setupTestServer()
	srv.Cfg.StoreMode = config.StoreModeMariaDBAuthority
	fake := &narrativeFakeStore{
		episodeSummaries: []store.EpisodeSummary{
			{ID: 42, ChatSessionID: "sess-del", FromTurn: 1, ToTurn: 2, SummaryText: "delete me"},
		},
	}
	srv.Store = fake
	srv.RegisterRoutes(mux)

	req := httptest.NewRequest(http.MethodDelete, "/episodes/42", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200: %s", rec.Code, rec.Body.String())
	}
	var resp map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if resp["status"] != "ok" || resp["deleted"] != true || fake.deletedEpisodeID != 42 {
		t.Fatalf("delete response/store mismatch: resp=%#v deleted=%d", resp, fake.deletedEpisodeID)
	}
	if len(fake.episodeSummaries) != 0 {
		t.Fatalf("episode not removed: %+v", fake.episodeSummaries)
	}
}

func TestChapterGenerateDuplicateCheckAndRollbackInvalidation(t *testing.T) {
	mux := http.NewServeMux()
	srv := setupTestServer()
	srv.Cfg.StoreMode = config.StoreModeMariaDBAuthority
	fake := &narrativeFakeStore{
		chapterSummaries: []store.ChapterSummary{
			{ChatSessionID: "sess-dup", FromTurn: 1, ToTurn: 60, ChapterIndex: 1, ChapterTitle: "Existing", SummaryText: "Already here.", ResumeText: "Resume."},
		},
		episodeSummaries: []store.EpisodeSummary{
			{ChatSessionID: "sess-dup", FromTurn: 1, ToTurn: 15, SummaryText: "E1"},
			{ChatSessionID: "sess-dup", FromTurn: 16, ToTurn: 30, SummaryText: "E2"},
			{ChatSessionID: "sess-dup", FromTurn: 31, ToTurn: 45, SummaryText: "E3"},
			{ChatSessionID: "sess-dup", FromTurn: 46, ToTurn: 60, SummaryText: "E4"},
		},
	}
	srv.Store = fake
	srv.RegisterRoutes(mux)

	req := httptest.NewRequest(http.MethodPost, "/chapters/generate", strings.NewReader(`{"chat_session_id":"sess-dup","turn_index":60,"interval":60}`))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200: %s", rec.Code, rec.Body.String())
	}
	var resp map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if resp["status"] != "skipped" {
		t.Fatalf("expected skipped due to duplicate, got %#v", resp)
	}
	if resp["already_exists"] != true {
		t.Fatalf("expected already_exists=true, got %#v", resp)
	}
	if len(fake.savedChapterSummaries) != 0 {
		t.Fatalf("expected no new chapter saved, got %d", len(fake.savedChapterSummaries))
	}

	req2 := httptest.NewRequest(http.MethodPost, "/chapters/generate", strings.NewReader(`{"chat_session_id":"sess-dup","turn_index":60,"interval":60,"force":true}`))
	req2.Header.Set("Content-Type", "application/json")
	rec2 := httptest.NewRecorder()
	mux.ServeHTTP(rec2, req2)
	if rec2.Code != http.StatusOK {
		t.Fatalf("force status = %d, want 200: %s", rec2.Code, rec2.Body.String())
	}
	var resp2 map[string]any
	if err := json.Unmarshal(rec2.Body.Bytes(), &resp2); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if resp2["status"] != "ok" {
		t.Fatalf("expected ok with force, got %#v", resp2)
	}
	if len(fake.savedChapterSummaries) != 1 {
		t.Fatalf("expected 1 new chapter saved with force, got %d", len(fake.savedChapterSummaries))
	}
}

func TestChapterExportAndSnapshotSurfacesIncludeChapters(t *testing.T) {
	mux := http.NewServeMux()
	srv := setupTestServer()
	fake := &narrativeFakeStore{
		chapterSummaries: []store.ChapterSummary{
			{ChatSessionID: "sess-exp", FromTurn: 1, ToTurn: 60, ChapterIndex: 1, ChapterTitle: "Gate", SummaryText: "Open.", ResumeText: "Resume."},
		},
	}
	srv.Store = fake
	srv.RegisterRoutes(mux)

	req := httptest.NewRequest(http.MethodGet, "/sessions/sess-exp/export", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("export status = %d, want 200: %s", rec.Code, rec.Body.String())
	}
	var exp map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &exp); err != nil {
		t.Fatalf("decode export: %v", err)
	}
	contract, ok := exp["portability_contract"].(map[string]any)
	if !ok {
		t.Fatalf("portability_contract missing")
	}
	portable, ok := contract["portable_units"].([]any)
	if !ok {
		t.Fatalf("portable_units missing")
	}
	found := false
	for _, u := range portable {
		if u == "chapter_summaries" {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("chapter_summaries not in portable_units: %v", portable)
	}
	if _, ok := exp["chapter_summaries"]; !ok {
		t.Fatalf("chapter_summaries missing in export response")
	}

	req2 := httptest.NewRequest(http.MethodGet, "/session-state/sess-exp", nil)
	rec2 := httptest.NewRecorder()
	mux.ServeHTTP(rec2, req2)
	if rec2.Code != http.StatusOK {
		t.Fatalf("state status = %d, want 200: %s", rec2.Code, rec2.Body.String())
	}
	var st map[string]any
	if err := json.Unmarshal(rec2.Body.Bytes(), &st); err != nil {
		t.Fatalf("decode state: %v", err)
	}
	if _, ok := st["chapter_summaries"]; !ok {
		t.Fatalf("chapter_summaries missing in session state")
	}
}

func TestChapterDryRunReturnsIntervalCheckAndInputStats(t *testing.T) {
	mux := http.NewServeMux()
	srv := setupTestServer()
	fake := &narrativeFakeStore{
		episodeSummaries: []store.EpisodeSummary{
			{ChatSessionID: "sess-dry", FromTurn: 1, ToTurn: 15, SummaryText: "E1"},
			{ChatSessionID: "sess-dry", FromTurn: 16, ToTurn: 30, SummaryText: "E2"},
			{ChatSessionID: "sess-dry", FromTurn: 31, ToTurn: 45, SummaryText: "E3"},
			{ChatSessionID: "sess-dry", FromTurn: 46, ToTurn: 60, SummaryText: "E4"},
		},
	}
	srv.Store = fake
	srv.RegisterRoutes(mux)

	req := httptest.NewRequest(http.MethodPost, "/chapters/dry-run", strings.NewReader(`{"chat_session_id":"sess-dry","turn_index":60,"interval":60}`))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200: %s", rec.Code, rec.Body.String())
	}
	var resp map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if resp["status"] != "ok" {
		t.Fatalf("status = %v, want ok", resp["status"])
	}
	if resp["mode"] != "dry_run" {
		t.Fatalf("mode = %v, want dry_run", resp["mode"])
	}
	if resp["triggered"] != true {
		t.Fatalf("triggered = %v, want true", resp["triggered"])
	}
	ic, ok := resp["interval_check"].(map[string]any)
	if !ok {
		t.Fatalf("interval_check missing")
	}
	if _, ok := ic["range"]; !ok {
		t.Fatalf("interval_check.range missing")
	}
	stats, ok := resp["input_stats"].(map[string]any)
	if !ok {
		t.Fatalf("input_stats missing")
	}
	if stats["episode_count"] != float64(4) {
		t.Fatalf("episode_count = %v, want 4", stats["episode_count"])
	}
	if stats["episode_count_recommended"] != true {
		t.Fatalf("episode_count_recommended = %v, want true", stats["episode_count_recommended"])
	}
	ready, _ := resp["ready"].(bool)
	if !ready {
		t.Fatalf("ready = %v, want true", ready)
	}
	br, _ := resp["blocking_reasons"].([]any)
	if len(br) != 0 {
		t.Fatalf("expected empty blocking_reasons, got %v", br)
	}
}
