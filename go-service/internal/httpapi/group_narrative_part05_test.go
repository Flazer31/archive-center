package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/risulongmemory/archive-center-go/internal/config"
	"github.com/risulongmemory/archive-center-go/internal/vector"

	"github.com/risulongmemory/archive-center-go/internal/store"
)

func TestArcGeneratePrioritizesChapterDenseAnchorsAndPersistsDS1cFields(t *testing.T) {
	mux := http.NewServeMux()
	srv := setupTestServer()
	srv.Cfg.StoreMode = config.StoreModeMariaDBAuthority
	fake := &narrativeFakeStore{
		chapterSummaries: []store.ChapterSummary{
			{
				ChatSessionID:           "sess-ds1c-arc",
				FromTurn:                1,
				ToTurn:                  60,
				ChapterIndex:            1,
				ChapterTitle:            "Gate",
				SummaryText:             "Generic gate summary.",
				OpenLoopsJSON:           `["sealed gate callback debt"]`,
				RelationshipChangesJSON: `["Alice pivots from suspicion to trust"]`,
				WorldChangesJSON:        `["tower gate law changes permanently"]`,
				CallbackCandidatesJSON:  `["return to the sealed gate"]`,
				ResumeText:              "Resume should be below dense anchors.",
			},
			{
				ChatSessionID:           "sess-ds1c-arc",
				FromTurn:                61,
				ToTurn:                  120,
				ChapterIndex:            2,
				ChapterTitle:            "Ledger",
				SummaryText:             "Generic ledger summary.",
				OpenLoopsJSON:           `["ledger promise remains unpaid"]`,
				RelationshipChangesJSON: `["Bob becomes a guarded ally"]`,
				WorldChangesJSON:        `["archive faction pressure rises"]`,
				CallbackCandidatesJSON:  `["ledger oath callback"]`,
				ResumeText:              "Ledger resume should be below dense anchors.",
			},
		},
	}
	srv.Store = fake
	srv.RegisterRoutes(mux)

	req := httptest.NewRequest(http.MethodPost, "/arcs/generate", strings.NewReader(`{"chat_session_id":"sess-ds1c-arc","from_turn":1,"to_turn":120}`))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200: %s", rec.Code, rec.Body.String())
	}
	if len(fake.savedArcSummaries) != 1 {
		t.Fatalf("saved arcs = %d, want 1", len(fake.savedArcSummaries))
	}
	saved := fake.savedArcSummaries[0]
	if !strings.Contains(saved.IrreversibleTurnsJSON, "tower gate law") || !strings.Contains(saved.CallbackDebtsJSON, "sealed gate callback") || !strings.Contains(saved.RelationshipPivotsJSON, "suspicion to trust") {
		t.Fatalf("saved arc DS-1c fields incomplete: %+v", saved)
	}
	openIdx := strings.Index(saved.CoreConflict, "open_loop:")
	summaryIdx := strings.Index(saved.CoreConflict, "summary:")
	if openIdx < 0 || summaryIdx < 0 || openIdx > summaryIdx {
		t.Fatalf("arc core did not prioritize chapter anchors before summary text: %q", saved.CoreConflict)
	}

	var resp map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	stats, ok := resp["input_stats"].(map[string]any)
	if !ok || stats["chapter_dense_summary_injection_policy_version"] != chapterDenseSummaryPolicyVersion || stats["arc_dense_summary_policy_version"] != arcDenseSummaryPolicyVersion {
		t.Fatalf("arc dense stats missing: %#v", resp)
	}
}

func TestSagaGenerateConsumesArcDS1cAnchors(t *testing.T) {
	mux := http.NewServeMux()
	srv := setupTestServer()
	srv.Cfg.StoreMode = config.StoreModeMariaDBAuthority
	fake := &narrativeFakeStore{
		arcSummaries: []store.ArcSummary{
			{
				ChatSessionID:          "sess-ds1c-saga",
				FromTurn:               1,
				ToTurn:                 180,
				ArcIndex:               1,
				ArcName:                "Gate Arc",
				ArcStatus:              "active",
				CoreConflict:           "Generic core conflict.",
				IrreversibleTurnsJSON:  `["tower gate law cannot be reversed"]`,
				CallbackDebtsJSON:      `["repay the sealed gate callback"]`,
				RelationshipPivotsJSON: `["Alice and Bob become allies"]`,
				ArcResumeText:          "Resume should be lower priority.",
			},
		},
	}
	srv.Store = fake
	srv.RegisterRoutes(mux)

	req := httptest.NewRequest(http.MethodPost, "/sagas/generate", strings.NewReader(`{"chat_session_id":"sess-ds1c-saga","from_turn":1,"to_turn":180}`))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200: %s", rec.Code, rec.Body.String())
	}
	if len(fake.savedSagaDigests) != 1 {
		t.Fatalf("saved sagas = %d, want 1", len(fake.savedSagaDigests))
	}
	saved := fake.savedSagaDigests[0]
	if !strings.Contains(saved.SagaSummary, "irreversible: tower gate law") || !strings.Contains(saved.SagaSummary, "callback_debt: repay") || !strings.Contains(saved.SagaSummary, "relationship_pivot: Alice") {
		t.Fatalf("saved saga did not consume DS-1c anchors first: %+v", saved)
	}
	if !strings.Contains(saved.NeverDropCandidatesJSON, "sealed gate callback") || !strings.Contains(saved.NeverDropCandidatesJSON, "become allies") {
		t.Fatalf("never drop candidates missing DS-1c anchors: %+v", saved)
	}
}

func TestChapterSearchDensePriorityPromotesAnchorsOverRecency(t *testing.T) {
	mux := http.NewServeMux()
	srv := setupTestServer()
	srv.Store = &narrativeFakeStore{
		chapterSummaries: []store.ChapterSummary{
			{
				ID:            2,
				ChatSessionID: "sess-ds1d-search",
				FromTurn:      61,
				ToTurn:        120,
				ChapterIndex:  2,
				ChapterTitle:  "Recent Gate Mention",
				SummaryText:   "gate appears in a plain recent recap",
				ResumeText:    "recent but anchor thin",
			},
			{
				ID:                      1,
				ChatSessionID:           "sess-ds1d-search",
				FromTurn:                1,
				ToTurn:                  60,
				ChapterIndex:            1,
				ChapterTitle:            "Older Dense Gate",
				SummaryText:             "brief recap",
				ResumeText:              "gate anchor still matters",
				OpenLoopsJSON:           `["gate promise remains unpaid"]`,
				RelationshipChangesJSON: `["Alice trusts Bob because of the gate"]`,
				WorldChangesJSON:        `["gate law changes the archive"]`,
				CallbackCandidatesJSON:  `["return to the gate promise"]`,
			},
		},
	}
	srv.RegisterRoutes(mux)

	req := httptest.NewRequest(http.MethodPost, "/chapters/search", strings.NewReader(`{"chat_session_id":"sess-ds1d-search","query":"gate","limit":1}`))
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
	items, ok := resp["chapters"].([]any)
	if !ok || len(items) != 1 {
		t.Fatalf("chapters = %#v", resp["chapters"])
	}
	first, ok := items[0].(map[string]any)
	if !ok {
		t.Fatalf("first chapter shape = %#v", items[0])
	}
	if first["chapter_title"] != "Older Dense Gate" {
		t.Fatalf("dense chapter was not promoted: %#v", first)
	}
	if first["dense_summary_policy_version"] != denseSummaryPriorityPolicyVersion {
		t.Fatalf("dense policy version missing: %#v", first)
	}
	if score, ok := first["dense_priority_score"].(float64); !ok || score <= 0 {
		t.Fatalf("dense priority score missing: %#v", first)
	}
}

func TestDenseSummarySearchResultsExposeSourceRoleRetentionAndEvidencePromotion(t *testing.T) {
	mux := http.NewServeMux()
	srv := setupTestServer()
	srv.Store = &narrativeFakeStore{
		evidence: []store.DirectEvidence{
			{
				ID:              31,
				ChatSessionID:   "sess-ds1f-search",
				EvidenceKind:    "relationship_world_promise",
				EvidenceText:    "Alice and Bob promise to obey the gate law together.",
				SourceTurnStart: 40,
				SourceTurnEnd:   44,
				TurnAnchor:      42,
			},
		},
		chapterSummaries: []store.ChapterSummary{
			{
				ID:                      7,
				ChatSessionID:           "sess-ds1f-search",
				FromTurn:                40,
				ToTurn:                  60,
				ChapterIndex:            2,
				ChapterTitle:            "Gate Promise",
				SummaryText:             "A plain summary of the gate promise.",
				ResumeText:              "Gate promise remains important.",
				OpenLoopsJSON:           `["gate promise remains unpaid"]`,
				RelationshipChangesJSON: `["Alice and Bob form a durable alliance"]`,
				WorldChangesJSON:        `["gate law changes the archive city"]`,
				CallbackCandidatesJSON:  `["repay the gate promise later"]`,
			},
		},
	}
	srv.RegisterRoutes(mux)

	req := httptest.NewRequest(http.MethodPost, "/chapters/search", strings.NewReader(`{"chat_session_id":"sess-ds1f-search","query":"gate","limit":1}`))
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
	items, ok := resp["chapters"].([]any)
	if !ok || len(items) != 1 {
		t.Fatalf("chapters = %#v", resp["chapters"])
	}
	item := items[0].(map[string]any)
	if item["dense_source_anchor_policy_version"] != denseSourceAnchorPolicyVersion {
		t.Fatalf("DS-1f source anchor policy missing: %#v", item)
	}
	if item["source_record_id"] != float64(7) || item["source_record_type"] != "chapter" {
		t.Fatalf("DS-1f source record mismatch: %#v", item)
	}
	if item["dense_role_split_policy_version"] != denseRoleSplitPolicyVersion || item["dense_narrative_usage"] != "read_only" || item["dense_structured_usage"] != "adjudication_retrieval" {
		t.Fatalf("DS-1h role split missing: %#v", item)
	}
	payload, ok := item["dense_structured_payload"].(map[string]any)
	if !ok || payload["relationship_changes"] == nil || payload["world_changes"] == nil || payload["callback_candidates"] == nil {
		t.Fatalf("DS-1h structured payload missing: %#v", item["dense_structured_payload"])
	}
	if item["dense_retention_policy_version"] != denseRetentionPolicyVersion || item["dense_retention_applied"] != true {
		t.Fatalf("DS-1g retention fields missing: %#v", item)
	}
	if item["dense_direct_evidence_promotion_policy_version"] != denseEvidencePromotionPolicy || item["dense_structured_precedence_applied"] != true {
		t.Fatalf("DS-1i evidence promotion missing: %#v", item)
	}
	if item["dense_direct_evidence_promoted_relationship_count"].(float64) < 1 || item["dense_direct_evidence_promoted_world_count"].(float64) < 1 || item["dense_direct_evidence_promoted_promise_count"].(float64) < 1 {
		t.Fatalf("DS-1i evidence promotion counts missing: %#v", item)
	}
}

func TestSagaGenerateWritesSagaDigestFromArcs(t *testing.T) {
	mux := http.NewServeMux()
	srv := setupTestServer()
	srv.Cfg.StoreMode = config.StoreModeMariaDBAuthority
	fake := &narrativeFakeStore{
		arcSummaries: []store.ArcSummary{
			{ChatSessionID: "sess-saga", FromTurn: 1, ToTurn: 180, ArcIndex: 1, ArcName: "Gate Arc", ArcStatus: "active", CoreConflict: "Gate opens.", ArcResumeText: "The gate arc is active."},
			{ChatSessionID: "sess-saga", FromTurn: 181, ToTurn: 360, ArcIndex: 2, ArcName: "Ledger Arc", ArcStatus: "active", CoreConflict: "Ledger returns.", ArcResumeText: "The ledger arc returns."},
		},
	}
	srv.Store = fake
	srv.RegisterRoutes(mux)

	req := httptest.NewRequest(http.MethodPost, "/sagas/generate", strings.NewReader(`{"chat_session_id":"sess-saga","from_turn":1,"to_turn":360}`))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200: %s", rec.Code, rec.Body.String())
	}
	if len(fake.savedSagaDigests) != 1 {
		t.Fatalf("saved sagas = %d, want 1", len(fake.savedSagaDigests))
	}
	saved := fake.savedSagaDigests[0]
	if saved.ChatSessionID != "sess-saga" || saved.FromTurn != 1 || saved.ToTurn != 360 {
		t.Fatalf("saved saga mismatch: %+v", saved)
	}
	if !strings.Contains(saved.ResumePackText, "gate arc") && !strings.Contains(saved.ResumePackText, "Gate") {
		t.Fatalf("saved saga resume missing arc material: %+v", saved)
	}

	var resp map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if resp["generation_source"] != "deterministic_migration_stub" || resp["saved"] != true {
		t.Fatalf("unexpected saga generation response: %#v", resp)
	}
}

type narrativeFakeVectorStore struct {
	deleteCalled    bool
	deleteSessionID string
	deleteErr       error
}

func (f *narrativeFakeVectorStore) Search(ctx context.Context, sessionID string, vec []float32, limit int, filter string) ([]vector.VectorDocument, error) {
	return nil, vector.ErrNotEnabled
}

func (f *narrativeFakeVectorStore) Upsert(ctx context.Context, sessionID string, docs []vector.VectorDocument) error {
	return vector.ErrNotEnabled
}

func (f *narrativeFakeVectorStore) DeleteSession(ctx context.Context, sessionID string) error {
	f.deleteCalled = true
	f.deleteSessionID = sessionID
	return f.deleteErr
}

func (f *narrativeFakeVectorStore) Rebuild(ctx context.Context, sessionID string) error {
	return vector.ErrNotEnabled
}

func (f *narrativeFakeVectorStore) Health(ctx context.Context) (vector.HealthSnapshot, error) {
	return vector.HealthSnapshot{Status: "ok"}, nil
}

func (f *narrativeFakeVectorStore) Count(ctx context.Context, sessionID string) (int, error) {
	return 0, vector.ErrNotEnabled
}

func (f *narrativeFakeVectorStore) Close(ctx context.Context) error { return nil }

func TestSessionDeleteShadowNoMutation(t *testing.T) {
	mux := http.NewServeMux()
	srv := setupTestServer()
	srv.Store = &narrativeFakeStore{}
	srv.RegisterRoutes(mux)

	req := httptest.NewRequest(http.MethodDelete, "/sessions/sess-shadow", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	var resp map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if resp["deleted"] != false {
		t.Errorf("deleted = %v, want false", resp["deleted"])
	}
	if resp["mutation_enabled"] != false {
		t.Errorf("mutation_enabled = %v, want false", resp["mutation_enabled"])
	}
}

func TestSessionDeleteLiveExecutes(t *testing.T) {
	cfg := config.Default()
	cfg.StoreMode = config.StoreModeMariaDBAuthority

	fake := &narrativeFakeStore{}
	srv := NewServer(cfg)
	srv.Store = fake
	srv.StoreOpenError = nil
	vs := &narrativeFakeVectorStore{}
	srv.Vector = vs

	mux := http.NewServeMux()
	srv.RegisterRoutes(mux)

	req := httptest.NewRequest(http.MethodDelete, "/sessions/sess-live", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200: %s", rec.Code, rec.Body.String())
	}
	if !fake.deleteSessionCalled {
		t.Error("DeleteSession was not called")
	}
	if !vs.deleteCalled {
		t.Error("Vector DeleteSession was not called")
	}
	if vs.deleteSessionID != "sess-live" {
		t.Errorf("vector deleteSessionID = %s, want sess-live", vs.deleteSessionID)
	}

	var resp map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if resp["deleted"] != true {
		t.Errorf("deleted = %v, want true", resp["deleted"])
	}
	if resp["mutation_enabled"] != true {
		t.Errorf("mutation_enabled = %v, want true", resp["mutation_enabled"])
	}
	vc, ok := resp["vector_cleanup"].(map[string]any)
	if !ok {
		t.Fatalf("vector_cleanup is not an object, got %T", resp["vector_cleanup"])
	}
	if vc["attempted"] != true {
		t.Errorf("vector_cleanup.attempted = %v, want true", vc["attempted"])
	}
	if vc["ok"] != true {
		t.Errorf("vector_cleanup.ok = %v, want true", vc["ok"])
	}
	if len(fake.auditLogs) == 0 {
		t.Fatal("expected session_delete audit log")
	}
	audit := fake.auditLogs[0]
	if audit.EventType != "session_delete" {
		t.Fatalf("event_type = %q, want session_delete", audit.EventType)
	}
	if audit.ChatSessionID != "sess-live" {
		t.Fatalf("chat_session_id = %q, want sess-live", audit.ChatSessionID)
	}
	if audit.TargetType != "session" {
		t.Fatalf("target_type = %q, want session", audit.TargetType)
	}
}

func TestSessionDeleteLiveVectorWarning(t *testing.T) {
	cfg := config.Default()
	cfg.StoreMode = config.StoreModeMariaDBAuthority

	fake := &narrativeFakeStore{}
	srv := NewServer(cfg)
	srv.Store = fake
	srv.StoreOpenError = nil
	vs := &narrativeFakeVectorStore{deleteErr: vector.ErrNotEnabled}
	srv.Vector = vs

	mux := http.NewServeMux()
	srv.RegisterRoutes(mux)

	req := httptest.NewRequest(http.MethodDelete, "/sessions/sess-vec-warn", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200: %s", rec.Code, rec.Body.String())
	}

	var resp map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if resp["status"] != "ok" {
		t.Errorf("status = %v, want ok", resp["status"])
	}
	vc, ok := resp["vector_cleanup"].(map[string]any)
	if !ok {
		t.Fatalf("vector_cleanup is not an object, got %T", resp["vector_cleanup"])
	}
	if vc["ok"] != true {
		t.Errorf("vector_cleanup.ok = %v, want true", vc["ok"])
	}
	if vc["warning"] != "vector store is not enabled" {
		t.Errorf("vector_cleanup.warning = %v, want 'vector store is not enabled'", vc["warning"])
	}
}

func TestSessionDeleteLiveStoreError(t *testing.T) {
	cfg := config.Default()
	cfg.StoreMode = config.StoreModeMariaDBAuthority

	fake := &narrativeFakeStore{deleteSessionErr: errors.New("db failure")}
	srv := NewServer(cfg)
	srv.Store = fake
	srv.StoreOpenError = nil

	mux := http.NewServeMux()
	srv.RegisterRoutes(mux)

	req := httptest.NewRequest(http.MethodDelete, "/sessions/sess-err", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want 500", rec.Code)
	}
}

func TestNarrativeControlGetCachedFresh(t *testing.T) {
	cachedPlan := map[string]any{
		"current_arc":        "Arc A",
		"narrative_goal":     "goal A",
		"active_tensions":    []any{"t1"},
		"next_beats":         []any{"b1"},
		"continuity_anchors": []any{"a1"},
		"guardrails":         []any{},
		"persona_priorities": []any{},
		"execution_notes":    []any{},
		"focus_characters":   []any{},
		"last_plan_turn":     float64(10),
		"state_status":       "ready",
	}
	cachedDir := map[string]any{
		"scene_mandate":       "Continue arc: Arc A",
		"required_outcomes":   []any{},
		"forbidden_moves":     []any{},
		"pressure_level":      "steady",
		"execution_checklist": []any{},
		"persona_guardrails":  []any{},
		"world_guardrails":    []any{},
		"focus_characters":    []any{},
		"last_turn":           float64(10),
		"state_status":        "ready",
		"resolved_outcomes":   []any{},
		"expired_forbidden":   []any{},
	}
	spJSON, _ := json.Marshal(cachedPlan)
	dirJSON, _ := json.Marshal(cachedDir)
	warnJSON, _ := json.Marshal([]any{})

	fake := &narrativeFakeStore{
		guidancePlanState: &store.GuidancePlanState{
			ChatSessionID: "sess-fresh",
			StoryPlanJSON: string(spJSON),
			DirectorJSON:  string(dirJSON),
			StateStatus:   "ready",
			LastTurn:      10,
			WarningsJSON:  string(warnJSON),
		},
		storylines: []store.Storyline{
			{ChatSessionID: "sess-fresh", Name: "Arc A", LastTurn: 9, FirstTurn: 1},
		},
	}

	cfg := config.Default()
	srv := NewServer(cfg)
	srv.Store = fake
	srv.StoreOpenError = nil

	mux := http.NewServeMux()
	srv.RegisterRoutes(mux)

	req := httptest.NewRequest(http.MethodGet, "/narrative-control/sess-fresh", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200: %s", rec.Code, rec.Body.String())
	}
	var resp map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if resp["state_status"] != "ready" {
		t.Fatalf("state_status = %v, want ready", resp["state_status"])
	}
	if fake.savedGuidancePlanState != nil {
		t.Fatal("expected no upsert when cache is fresh")
	}
	plan, ok := resp["story_plan"].(map[string]any)
	if !ok {
		t.Fatal("story_plan is not an object")
	}
	if plan["current_arc"] != "Arc A" {
		t.Fatalf("current_arc = %v, want Arc A", plan["current_arc"])
	}
}

func TestNarrativeControlGetStaleRebuildsAndUpserts(t *testing.T) {
	cachedPlan := map[string]any{
		"current_arc":        "Old Arc",
		"narrative_goal":     "old goal",
		"active_tensions":    []any{},
		"next_beats":         []any{"old-beat"},
		"continuity_anchors": []any{"old-anchor"},
		"guardrails":         []any{},
		"persona_priorities": []any{},
		"execution_notes":    []any{},
		"focus_characters":   []any{},
		"last_plan_turn":     float64(5),
		"state_status":       "ready",
	}
	cachedDir := map[string]any{
		"scene_mandate":       "Continue arc: Old Arc",
		"required_outcomes":   []any{},
		"forbidden_moves":     []any{},
		"pressure_level":      "light",
		"execution_checklist": []any{},
		"persona_guardrails":  []any{},
		"world_guardrails":    []any{},
		"focus_characters":    []any{},
		"last_turn":           float64(5),
		"state_status":        "ready",
		"resolved_outcomes":   []any{},
		"expired_forbidden":   []any{},
	}
	spJSON, _ := json.Marshal(cachedPlan)
	dirJSON, _ := json.Marshal(cachedDir)
	warnJSON, _ := json.Marshal([]any{})

	fake := &narrativeFakeStore{
		guidancePlanState: &store.GuidancePlanState{
			ChatSessionID: "sess-stale",
			StoryPlanJSON: string(spJSON),
			DirectorJSON:  string(dirJSON),
			StateStatus:   "ready",
			LastTurn:      5,
			WarningsJSON:  string(warnJSON),
		},
		storylines: []store.Storyline{
			{ChatSessionID: "sess-stale", Name: "New Arc", LastTurn: 12, FirstTurn: 6, CurrentContext: "new ctx"},
		},
	}

	cfg := config.Default()
	srv := NewServer(cfg)
	srv.Store = fake
	srv.StoreOpenError = nil

	mux := http.NewServeMux()
	srv.RegisterRoutes(mux)

	req := httptest.NewRequest(http.MethodGet, "/narrative-control/sess-stale", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200: %s", rec.Code, rec.Body.String())
	}
	var resp map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if fake.savedGuidancePlanState == nil {
		t.Fatal("expected upsert after stale rebuild")
	}
	if fake.savedGuidancePlanState.LastTurn != 12 {
		t.Fatalf("upsert last_turn = %d, want 12", fake.savedGuidancePlanState.LastTurn)
	}
	plan, _ := resp["story_plan"].(map[string]any)
	if plan["current_arc"] != "New Arc" {
		t.Fatalf("current_arc = %v, want New Arc", plan["current_arc"])
	}
}

func TestNarrativeControlGetSameArcConservativeMerge(t *testing.T) {
	cachedPlan := map[string]any{
		"current_arc":        "Arc A",
		"narrative_goal":     "old goal",
		"active_tensions":    []any{},
		"next_beats":         []any{"cached-beat-1", "cached-beat-2"},
		"continuity_anchors": []any{"cached-anchor"},
		"guardrails":         []any{},
		"persona_priorities": []any{},
		"execution_notes":    []any{},
		"focus_characters":   []any{},
		"last_plan_turn":     float64(8),
		"state_status":       "ready",
	}
	cachedDir := map[string]any{
		"scene_mandate":       "Continue arc: Arc A",
		"required_outcomes":   []any{},
		"forbidden_moves":     []any{},
		"pressure_level":      "light",
		"execution_checklist": []any{},
		"persona_guardrails":  []any{},
		"world_guardrails":    []any{},
		"focus_characters":    []any{},
		"last_turn":           float64(8),
		"state_status":        "ready",
		"resolved_outcomes":   []any{},
		"expired_forbidden":   []any{},
	}
	spJSON, _ := json.Marshal(cachedPlan)
	dirJSON, _ := json.Marshal(cachedDir)
	warnJSON, _ := json.Marshal([]any{})

	fake := &narrativeFakeStore{
		guidancePlanState: &store.GuidancePlanState{
			ChatSessionID: "sess-merge",
			StoryPlanJSON: string(spJSON),
			DirectorJSON:  string(dirJSON),
			StateStatus:   "ready",
			LastTurn:      6,
			WarningsJSON:  string(warnJSON),
		},
		storylines: []store.Storyline{
			{ChatSessionID: "sess-merge", Name: "Arc A", LastTurn: 10, FirstTurn: 1, CurrentContext: "new goal"},
		},
	}

	cfg := config.Default()
	srv := NewServer(cfg)
	srv.Store = fake
	srv.StoreOpenError = nil

	mux := http.NewServeMux()
	srv.RegisterRoutes(mux)

	req := httptest.NewRequest(http.MethodGet, "/narrative-control/sess-merge", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200: %s", rec.Code, rec.Body.String())
	}
	var resp map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	plan, _ := resp["story_plan"].(map[string]any)
	if plan["current_arc"] != "Arc A" {
		t.Fatalf("current_arc = %v, want Arc A", plan["current_arc"])
	}
	if plan["narrative_goal"] != "new goal" {
		t.Fatalf("narrative_goal = %v, want new goal", plan["narrative_goal"])
	}
	beats := plan["next_beats"].([]any)
	foundCached := false
	for _, b := range beats {
		if b == "cached-beat-1" || b == "cached-beat-2" {
			foundCached = true
		}
	}
	if !foundCached {
		t.Fatalf("expected merged next_beats to include cached beats")
	}
	anchors := plan["continuity_anchors"].([]any)
	foundAnchor := false
	for _, a := range anchors {
		if a == "cached-anchor" {
			foundAnchor = true
		}
	}
	if !foundAnchor {
		t.Fatalf("expected merged continuity_anchors to include cached anchor")
	}
}

func TestNarrativeControlGetNoStoreSupportNonFatal(t *testing.T) {
	cfg := config.Default()
	srv := NewServer(cfg)
	srv.Store = store.NewNoopStore()
	srv.StoreOpenError = nil

	mux := http.NewServeMux()
	srv.RegisterRoutes(mux)

	req := httptest.NewRequest(http.MethodGet, "/narrative-control/sess-nogps", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200: %s", rec.Code, rec.Body.String())
	}
	var resp map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if resp["status"] != "ok" {
		t.Fatalf("status = %v, want ok", resp["status"])
	}
	if resp["state_status"] != "skeleton" {
		t.Fatalf("state_status = %v, want skeleton", resp["state_status"])
	}
}

func TestNarrativeControlGetUpsertFailureNonFatal(t *testing.T) {
	fake := &narrativeFakeStore{
		storylines: []store.Storyline{
			{ChatSessionID: "sess-upsert-err", Name: "Arc C", LastTurn: 4, FirstTurn: 1},
		},
		guidancePlanUpsertErr: errors.New("upsert failure"),
	}

	cfg := config.Default()
	srv := NewServer(cfg)
	srv.Store = fake
	srv.StoreOpenError = nil

	mux := http.NewServeMux()
	srv.RegisterRoutes(mux)

	req := httptest.NewRequest(http.MethodGet, "/narrative-control/sess-upsert-err", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200: %s", rec.Code, rec.Body.String())
	}
	var resp map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if resp["status"] != "ok" {
		t.Fatalf("status = %v, want ok", resp["status"])
	}
}

func TestNarrativeControlGetInvalidCacheRebuildsNonFatal(t *testing.T) {
	fake := &narrativeFakeStore{
		guidancePlanState: &store.GuidancePlanState{
			ChatSessionID: "sess-invalid-cache",
			StoryPlanJSON: "{not-json",
			DirectorJSON:  `{"scene_mandate":"old"}`,
			StateStatus:   "ready",
			LastTurn:      7,
			WarningsJSON:  "[]",
		},
		storylines: []store.Storyline{
			{ChatSessionID: "sess-invalid-cache", Name: "Arc D", LastTurn: 7, FirstTurn: 1, CurrentContext: "rebuilt"},
		},
	}

	cfg := config.Default()
	srv := NewServer(cfg)
	srv.Store = fake
	srv.StoreOpenError = nil

	mux := http.NewServeMux()
	srv.RegisterRoutes(mux)

	req := httptest.NewRequest(http.MethodGet, "/narrative-control/sess-invalid-cache", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200: %s", rec.Code, rec.Body.String())
	}
	if fake.savedGuidancePlanState == nil {
		t.Fatal("expected invalid cache to rebuild and upsert")
	}
	var resp map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	plan, _ := resp["story_plan"].(map[string]any)
	if plan["current_arc"] != "Arc D" {
		t.Fatalf("current_arc = %v, want Arc D", plan["current_arc"])
	}
}

func TestNarrativeControlGetBackwardFreshnessRebuildsAfterRollback(t *testing.T) {
	cachedPlan := map[string]any{
		"current_arc":        "Future Arc",
		"narrative_goal":     "future goal",
		"active_tensions":    []any{},
		"next_beats":         []any{"future-beat"},
		"continuity_anchors": []any{"future-anchor"},
		"guardrails":         []any{},
		"persona_priorities": []any{},
		"execution_notes":    []any{},
		"focus_characters":   []any{},
		"last_plan_turn":     float64(10),
		"state_status":       "ready",
	}
	cachedDir := map[string]any{
		"scene_mandate":       "Continue arc: Future Arc",
		"required_outcomes":   []any{},
		"forbidden_moves":     []any{},
		"pressure_level":      "steady",
		"execution_checklist": []any{},
		"persona_guardrails":  []any{},
		"world_guardrails":    []any{},
		"focus_characters":    []any{},
		"last_turn":           float64(10),
		"state_status":        "ready",
		"resolved_outcomes":   []any{},
		"expired_forbidden":   []any{},
	}
	spJSON, _ := json.Marshal(cachedPlan)
	dirJSON, _ := json.Marshal(cachedDir)
	warnJSON, _ := json.Marshal([]any{})
	fake := &narrativeFakeStore{
		guidancePlanState: &store.GuidancePlanState{
			ChatSessionID: "sess-backward-freshness",
			StoryPlanJSON: string(spJSON),
			DirectorJSON:  string(dirJSON),
			StateStatus:   "ready",
			LastTurn:      10,
			WarningsJSON:  string(warnJSON),
		},
		storylines: []store.Storyline{
			{ChatSessionID: "sess-backward-freshness", Name: "Rolled Back Arc", LastTurn: 5, FirstTurn: 1, CurrentContext: "after rollback"},
		},
	}

	cfg := config.Default()
	srv := NewServer(cfg)
	srv.Store = fake
	srv.StoreOpenError = nil

	mux := http.NewServeMux()
	srv.RegisterRoutes(mux)

	req := httptest.NewRequest(http.MethodGet, "/narrative-control/sess-backward-freshness", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200: %s", rec.Code, rec.Body.String())
	}
	if fake.savedGuidancePlanState == nil {
		t.Fatal("expected future cache to be rejected and rebuilt")
	}
	if fake.savedGuidancePlanState.LastTurn != 5 {
		t.Fatalf("upsert last_turn = %d, want 5", fake.savedGuidancePlanState.LastTurn)
	}
	var resp map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	plan, _ := resp["story_plan"].(map[string]any)
	if plan["current_arc"] != "Rolled Back Arc" {
		t.Fatalf("current_arc = %v, want Rolled Back Arc", plan["current_arc"])
	}
}

func TestNarrativeControlGetStoryGuidanceYieldsToExplicitUserInput(t *testing.T) {
	cachedPlan := map[string]any{
		"current_arc":        "Old Arc",
		"narrative_goal":     "force the confession scene",
		"active_tensions":    []any{"confession pressure"},
		"next_beats":         []any{"force confession now"},
		"continuity_anchors": []any{"old promise"},
		"guardrails":         []any{},
		"persona_priorities": []any{},
		"execution_notes":    []any{},
		"focus_characters":   []any{"Chloe"},
		"last_plan_turn":     float64(8),
		"state_status":       "ready",
	}
	cachedDir := map[string]any{
		"scene_mandate":       "Continue arc: Old Arc",
		"required_outcomes":   []any{"force confession now"},
		"forbidden_moves":     []any{"ignore the old arc"},
		"pressure_level":      "strong",
		"execution_checklist": []any{"advance the confession"},
		"persona_guardrails":  []any{},
		"world_guardrails":    []any{},
		"focus_characters":    []any{"Chloe"},
		"last_turn":           float64(8),
		"state_status":        "ready",
		"resolved_outcomes":   []any{},
		"expired_forbidden":   []any{},
	}
	spJSON, _ := json.Marshal(cachedPlan)
	dirJSON, _ := json.Marshal(cachedDir)
	warnJSON, _ := json.Marshal([]any{})
	fake := &narrativeFakeStore{
		guidancePlanState: &store.GuidancePlanState{
			ChatSessionID: "sess-user-conflict",
			StoryPlanJSON: string(spJSON),
			DirectorJSON:  string(dirJSON),
			StateStatus:   "ready",
			LastTurn:      8,
			WarningsJSON:  string(warnJSON),
		},
		storylines: []store.Storyline{
			{ChatSessionID: "sess-user-conflict", Name: "Old Arc", LastTurn: 8, FirstTurn: 1, CurrentContext: "old arc pressure"},
		},
	}

	cfg := config.Default()
	srv := NewServer(cfg)
	srv.Store = fake
	srv.StoreOpenError = nil

	mux := http.NewServeMux()
	srv.RegisterRoutes(mux)

	req := httptest.NewRequest(http.MethodGet, "/narrative-control/sess-user-conflict?current_user_input=leave+the+scene", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200: %s", rec.Code, rec.Body.String())
	}
	var resp map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	guidance, ok := resp["story_guidance"].(map[string]any)
	if !ok {
		t.Fatalf("story_guidance missing: %+v", resp)
	}
	conflictPolicy, ok := guidance["conflict_policy"].(map[string]any)
	if !ok {
		t.Fatalf("conflict_policy missing: %+v", guidance)
	}
	if conflictPolicy["current_user_input_wins"] != true || conflictPolicy["guidance_may_override_user_input"] != false {
		t.Fatalf("conflict_policy does not yield to user input: %+v", conflictPolicy)
	}
	if conflictPolicy["on_conflict"] != "yield_to_current_user_input" {
		t.Fatalf("on_conflict = %v, want yield_to_current_user_input", conflictPolicy["on_conflict"])
	}
	precedence, ok := guidance["precedence"].(map[string]any)
	if !ok {
		t.Fatalf("precedence missing: %+v", guidance)
	}
	if precedence["guidance_authority"] != "subordinate" {
		t.Fatalf("guidance_authority = %v, want subordinate", precedence["guidance_authority"])
	}
	higherPriority, _ := precedence["higher_priority_sources"].([]any)
	if len(higherPriority) == 0 || higherPriority[0] != "current_user_input" {
		t.Fatalf("higher_priority_sources = %+v, want current_user_input first", higherPriority)
	}
	disallowed, _ := precedence["disallowed_usage"].([]any)
	if !containsAnyStringValue(disallowed, "current_user_input_override") {
		t.Fatalf("disallowed_usage = %+v, want current_user_input_override", disallowed)
	}
	turnDirectives, ok := guidance["turn_directives"].(map[string]any)
	if !ok {
		t.Fatalf("turn_directives missing: %+v", guidance)
	}
	failMode, ok := turnDirectives["fail_mode"].(map[string]any)
	if !ok {
		t.Fatalf("fail_mode missing: %+v", turnDirectives)
	}
	if failMode["respect_explicit_user_correction"] != true {
		t.Fatalf("fail_mode does not respect explicit user correction: %+v", failMode)
	}
	if fake.savedGuidancePlanState != nil {
		t.Fatal("fresh conflicting cache should not be rewritten by a read-only precedence proof")
	}
}

func containsAnyStringValue(items []any, needle string) bool {
	for _, item := range items {
		if item == needle {
			return true
		}
	}
	return false
}

func TestNarrativeControlGetResolvedOutcomesAccumulate(t *testing.T) {
	prevDirector := map[string]any{
		"scene_mandate":       "Continue arc: Arc A",
		"required_outcomes":   []string{"Carry forward: Hook A"},
		"forbidden_moves":     []string{},
		"pressure_level":      "light",
		"execution_checklist": []any{},
		"persona_guardrails":  []any{},
		"world_guardrails":    []any{},
		"focus_characters":    []any{},
		"last_turn":           float64(5),
		"state_status":        "ready",
		"resolved_outcomes":   []string{},
		"expired_forbidden":   []string{},
	}
	prevPlan := map[string]any{
		"current_arc":        "Arc A",
		"narrative_goal":     "goal",
		"active_tensions":    []any{},
		"next_beats":         []any{},
		"continuity_anchors": []any{},
		"guardrails":         []any{},
		"persona_priorities": []any{},
		"execution_notes":    []any{},
		"focus_characters":   []any{},
		"last_plan_turn":     float64(5),
		"state_status":       "heuristic",
	}
	dirJSON, _ := json.Marshal(prevDirector)
	planJSON, _ := json.Marshal(prevPlan)
	warnJSON, _ := json.Marshal([]any{})

	fake := &narrativeFakeStore{
		guidancePlanState: &store.GuidancePlanState{
			ChatSessionID: "sess-resolved",
			StoryPlanJSON: string(planJSON),
			DirectorJSON:  string(dirJSON),
			StateStatus:   "ready",
			LastTurn:      5,
			WarningsJSON:  string(warnJSON),
		},
		storylines: []store.Storyline{
			{ChatSessionID: "sess-resolved", Name: "Arc A", LastTurn: 10, FirstTurn: 1},
		},
		pendingThreads: []store.PendingThread{},
	}

	cfg := config.Default()
	srv := NewServer(cfg)
	srv.Store = fake
	srv.StoreOpenError = nil

	mux := http.NewServeMux()
	srv.RegisterRoutes(mux)

	req := httptest.NewRequest(http.MethodGet, "/narrative-control/sess-resolved", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200: %s", rec.Code, rec.Body.String())
	}
	var resp map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	director, _ := resp["director"].(map[string]any)
	resolved, _ := director["resolved_outcomes"].([]any)
	found := false
	for _, r := range resolved {
		if r == "Carry forward: Hook A" {
			found = true
		}
	}
	if !found {
		t.Fatalf("expected resolved_outcomes to include previously required hook, got %v", resolved)
	}
	required, _ := director["required_outcomes"].([]any)
	for _, r := range required {
		if r == "Carry forward: Hook A" {
			t.Fatal("expected required_outcomes to NOT include resolved hook")
		}
	}
}
