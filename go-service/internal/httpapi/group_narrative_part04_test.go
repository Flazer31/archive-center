package httpapi

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/risulongmemory/archive-center-go/internal/config"

	"github.com/risulongmemory/archive-center-go/internal/store"
)

func TestMetricsLC1DIntegrityReplayRetainsImportantLongMemory(t *testing.T) {
	fake := &narrativeFakeStore{
		chatLogs: []store.ChatLog{
			{ID: 1, ChatSessionID: "sess-lc1d", TurnIndex: 700, Role: "assistant", Content: "latest turn"},
		},
		memories: []store.Memory{
			{ID: 1, ChatSessionID: "sess-lc1d", TurnIndex: 50, Importance: 0.95, SummaryJSON: `{"summary":"old promise stays important"}`},
			{ID: 2, ChatSessionID: "sess-lc1d", TurnIndex: 350, NarrativeSignificance: 0.82, SummaryJSON: `{"summary":"middle arc still matters"}`},
			{ID: 3, ChatSessionID: "sess-lc1d", TurnIndex: 650, Importance: 0.99, SummaryJSON: `{"summary":"recent memory is not long-range"}`},
			{ID: 4, ChatSessionID: "sess-lc1d", TurnIndex: 20, Importance: 0.2, SummaryJSON: `{"summary":"old low-priority detail"}`},
		},
		evidence: []store.DirectEvidence{
			{ID: 1, ChatSessionID: "sess-lc1d", EvidenceText: "The old oath is verified.", TurnAnchor: 80, ArchiveState: "verified_direct", CaptureVerification: "verified"},
			{ID: 2, ChatSessionID: "sess-lc1d", EvidenceText: "Repair item should not count.", TurnAnchor: 90, ArchiveState: "repair_queue", RepairNeeded: true},
		},
		canonicalStateLayers: []store.CanonicalStateLayer{
			{ID: 1, ChatSessionID: "sess-lc1d", LayerType: "scene_state", Content: `{"mood":"charged"}`, SourceTurn: 60},
		},
		episodeSummaries: []store.EpisodeSummary{
			{ID: 1, ChatSessionID: "sess-lc1d", FromTurn: 1, ToTurn: 100, SummaryText: "dense early episode"},
		},
		resumePack: &store.ResumePack{
			PackStatus: "ok",
			Trigger:    "resume",
			Chapter:    &store.ChapterSummary{SummaryText: "chapter memory"},
			Arc:        &store.ArcSummary{CoreConflict: "arc memory"},
			Saga:       &store.SagaDigest{SagaSummary: "saga memory"},
		},
		storylines: []store.Storyline{
			{ID: 1, ChatSessionID: "sess-lc1d", Name: "Old oath", Status: "active", CurrentContext: "The oath remains unpaid.", LastEvidenceTurn: 100, EvidenceCount: 2},
		},
		pendingThreads: []store.PendingThread{
			{ID: 1, ChatSessionID: "sess-lc1d", ThreadKey: "oath", Status: "open", Description: "Resolve the old oath", CreatedTurn: 40, SourceTurn: 40},
		},
	}
	mux := http.NewServeMux()
	srv := setupTestServer()
	srv.Store = fake
	srv.RegisterRoutes(mux)

	req := httptest.NewRequest(http.MethodGet, "/metrics/lc1d/sess-lc1d", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200: %s", rec.Code, rec.Body.String())
	}
	var resp map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	replay, ok := resp["integrity_replay"].(map[string]any)
	if !ok {
		t.Fatalf("integrity_replay missing or wrong type: %#v", resp["integrity_replay"])
	}
	if replay["policy_version"] != "lc1d.v1" || replay["replay_query_source"] != "query_independent_store_replay" {
		t.Fatalf("replay policy/source mismatch: %#v", replay)
	}
	if replay["latest_turn_index"] != float64(700) || replay["candidates_total"] != float64(3) || replay["retained_total"] != float64(3) || replay["gaps_total"] != float64(0) {
		t.Fatalf("candidate retention mismatch: %#v", replay)
	}
	if replay["retention_rate"] != float64(1) || replay["scanned_direct_evidence_rows"] != float64(2) {
		t.Fatalf("retention/scanned mismatch: %#v", replay)
	}
	scopeCounts, ok := replay["scope_counts"].(map[string]any)
	if !ok {
		t.Fatalf("scope_counts missing or wrong type: %#v", replay["scope_counts"])
	}
	if scopeCounts["long"] != float64(3) || scopeCounts["ultra_long"] != float64(2) {
		t.Fatalf("scope_counts mismatch: %#v", scopeCounts)
	}
	retained, ok := replay["retained_by_layer"].(map[string]any)
	if !ok {
		t.Fatalf("retained_by_layer missing or wrong type: %#v", replay["retained_by_layer"])
	}
	wantLayers := map[string]float64{
		"memory":          2,
		"direct_evidence": 1,
		"canonical":       1,
		"dense_summary":   4,
		"live_ledger":     2,
	}
	for key, want := range wantLayers {
		if retained[key] != want {
			t.Fatalf("retained_by_layer[%s] = %v, want %v in %#v", key, retained[key], want, retained)
		}
	}
	examples, ok := replay["candidate_examples"].([]any)
	if !ok || len(examples) != 3 {
		t.Fatalf("candidate_examples = %#v, want 3 retained examples", replay["candidate_examples"])
	}
}

func TestMetricsLC1EComparesHypaMemoryAlwaysOnBudget(t *testing.T) {
	largeHypaSummary := strings.Repeat("HypaMemory imported summary. ", 500)
	fake := &narrativeFakeStore{
		memories: []store.Memory{
			{ID: 1, ChatSessionID: "sess-lc1e", TurnIndex: 1, SummaryJSON: largeHypaSummary, Evidence: largeHypaSummary, PlaceWing: "hypamemory"},
		},
		evidence: []store.DirectEvidence{
			{ID: 1, ChatSessionID: "sess-lc1e", EvidenceText: "Hypa imported evidence", LineageJSON: `{"source":"HypaMemory import"}`, CaptureStage: "hypamemory_import"},
		},
		kgTriples: []store.KGTriple{
			{ID: 1, ChatSessionID: "sess-lc1e", Subject: "Mina", Predicate: "remembers", Object: "oath"},
		},
		canonicalStateLayers: []store.CanonicalStateLayer{
			{ID: 1, ChatSessionID: "sess-lc1e", LayerType: "scene_state", Content: `{"mood":"focused"}`, SourceTurn: 1},
		},
		episodeSummaries: []store.EpisodeSummary{
			{ID: 1, ChatSessionID: "sess-lc1e", FromTurn: 1, ToTurn: 10, SummaryText: "short dense summary"},
		},
		resumePack: &store.ResumePack{PackStatus: "ok", Trigger: "resume"},
	}
	mux := http.NewServeMux()
	srv := setupTestServer()
	srv.Store = fake
	srv.RegisterRoutes(mux)

	req := httptest.NewRequest(http.MethodGet, "/metrics/lc1e/sess-lc1e", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200: %s", rec.Code, rec.Body.String())
	}
	var resp map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	compare, ok := resp["budget_compare"].(map[string]any)
	if !ok {
		t.Fatalf("budget_compare missing or wrong type: %#v", resp["budget_compare"])
	}
	if compare["policy_version"] != "lc1e.v1" || compare["hypamemory_always_on_mode"] != "discouraged_after_import" {
		t.Fatalf("budget policy mismatch: %#v", compare)
	}
	hypaChars := compare["hypamemory_always_on_chars"].(float64)
	layeredChars := compare["archive_center_layered_chars"].(float64)
	if hypaChars <= layeredChars {
		t.Fatalf("expected layered budget to be smaller than always-on HypaMemory, hypa=%v layered=%v", hypaChars, layeredChars)
	}
	if compare["recommended_mode"] != "archive_center_layered" {
		t.Fatalf("recommended_mode = %v, want archive_center_layered", compare["recommended_mode"])
	}
	if compare["saved_chars_vs_hypamemory"].(float64) <= 0 || compare["savings_ratio"].(float64) <= 0 {
		t.Fatalf("budget savings not positive: %#v", compare)
	}
	counts, ok := resp["counts"].(map[string]any)
	if !ok {
		t.Fatalf("counts missing or wrong type: %#v", resp["counts"])
	}
	if counts["memory_count"] != float64(1) || counts["kg_triple_count"] != float64(1) || counts["evidence_count"] != float64(1) {
		t.Fatalf("counts mismatch: %#v", counts)
	}
	trace, ok := resp["trace_summary"].([]any)
	if !ok || len(trace) == 0 {
		t.Fatalf("trace_summary missing: %#v", resp["trace_summary"])
	}
}

func TestMetricsLC1FConfirmsShortMidRegressionLayers(t *testing.T) {
	fake := &narrativeFakeStore{
		chatLogs: []store.ChatLog{
			{ID: 1, ChatSessionID: "sess-lc1f", TurnIndex: 12, Role: "assistant", Content: "latest"},
		},
		evidence: []store.DirectEvidence{
			{ID: 1, ChatSessionID: "sess-lc1f", EvidenceText: "Verified fact", TurnAnchor: 12, ArchiveState: "verified_direct", CaptureVerification: "verified"},
		},
		kgTriples: []store.KGTriple{
			{ID: 1, ChatSessionID: "sess-lc1f", Subject: "Mina", Predicate: "protects", Object: "ledger"},
		},
		activeStates: []store.ActiveState{
			{ID: 1, ChatSessionID: "sess-lc1f", StateType: "scene_state", Content: `{"mood":"tense"}`, TurnIndex: 12},
		},
		canonicalStateLayers: []store.CanonicalStateLayer{
			{ID: 1, ChatSessionID: "sess-lc1f", LayerType: "scene_state", Content: `{"mood":"tense"}`, SourceTurn: 12},
		},
		characterStates: []store.CharacterState{
			{ID: 1, ChatSessionID: "sess-lc1f", CharacterName: "Mina", StatusJSON: `{"intent":"protect ledger"}`, TurnIndex: 12},
		},
		storylines: []store.Storyline{
			{ID: 1, ChatSessionID: "sess-lc1f", Name: "Ledger oath", Status: "active", CurrentContext: "Oath remains active.", LastTurn: 12},
		},
		episodeSummaries: []store.EpisodeSummary{
			{ID: 1, ChatSessionID: "sess-lc1f", FromTurn: 1, ToTurn: 12, SummaryText: "Episode keeps the oath."},
		},
		worldRules: []store.WorldRule{
			{ID: 1, ChatSessionID: "sess-lc1f", Scope: "session", Category: "world", Key: "ledger_oath", ValueJSON: `{"rule":"Oaths require payoff."}`},
		},
		pendingThreads: []store.PendingThread{
			{ID: 1, ChatSessionID: "sess-lc1f", ThreadKey: "oath", Status: "open", Description: "Pay off the oath", SourceTurn: 12},
		},
		resumePack: &store.ResumePack{PackStatus: "ok", Trigger: "resume"},
	}
	mux := http.NewServeMux()
	srv := setupTestServer()
	srv.Store = fake
	srv.RegisterRoutes(mux)

	req := httptest.NewRequest(http.MethodGet, "/metrics/lc1f/sess-lc1f", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200: %s", rec.Code, rec.Body.String())
	}
	var resp map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	confirm, ok := resp["regression_confirm"].(map[string]any)
	if !ok {
		t.Fatalf("regression_confirm missing or wrong type: %#v", resp["regression_confirm"])
	}
	if confirm["policy_version"] != "lc1f.v1" || confirm["status"] != "pass" {
		t.Fatalf("regression confirm mismatch: %#v", confirm)
	}
	failed, ok := confirm["failed_checks"].([]any)
	if !ok || len(failed) != 0 {
		t.Fatalf("failed_checks = %#v, want empty", confirm["failed_checks"])
	}
	shortTerm, ok := confirm["short_term"].(map[string]any)
	if !ok {
		t.Fatalf("short_term missing: %#v", confirm["short_term"])
	}
	for _, key := range []string{"chat_logs_present", "direct_evidence_present", "kg_present", "current_state_present"} {
		if shortTerm[key] != true {
			t.Fatalf("short_term[%s] = %v, want true in %#v", key, shortTerm[key], shortTerm)
		}
	}
	midTerm, ok := confirm["mid_term"].(map[string]any)
	if !ok {
		t.Fatalf("mid_term missing: %#v", confirm["mid_term"])
	}
	for _, key := range []string{"storyline_present", "episode_summary_present", "world_rule_present", "pending_thread_present", "resume_pack_present"} {
		if midTerm[key] != true {
			t.Fatalf("mid_term[%s] = %v, want true in %#v", key, midTerm[key], midTerm)
		}
	}
}

func TestMetricsLC1GThroughLC1OExposeReplayAndGateEvidence(t *testing.T) {
	fake := &narrativeFakeStore{
		chatLogs: []store.ChatLog{
			{ID: 1, ChatSessionID: "sess-lc-tail", TurnIndex: 720, Role: "assistant", Content: "latest"},
		},
		memories: []store.Memory{
			{ID: 1, ChatSessionID: "sess-lc-tail", TurnIndex: 50, Importance: 0.9, SummaryJSON: `{"source":"hypamemory","summary":"imported idea"}`, PlaceWing: "hypamemory"},
		},
		evidence: []store.DirectEvidence{
			{ID: 1, ChatSessionID: "sess-lc-tail", EvidenceText: "Verified old promise.", TurnAnchor: 600, ArchiveState: "verified_direct", CaptureVerification: "verified", LineageJSON: `{"source":"HypaMemory import"}`, CaptureStage: "hypamemory_import"},
		},
		kgTriples: []store.KGTriple{
			{ID: 1, ChatSessionID: "sess-lc-tail", Subject: "Mina", Predicate: "guards", Object: "ledger"},
		},
		canonicalStateLayers: []store.CanonicalStateLayer{
			{ID: 1, ChatSessionID: "sess-lc-tail", LayerType: "scene_state", Content: `{"mood":"tense"}`, SourceTurn: 700, Confidence: 0.9},
			{ID: 2, ChatSessionID: "sess-lc-tail", LayerType: "relationship_state", Content: `{"trust":"rising"}`, SourceTurn: 700, Confidence: 0.88},
		},
		activeStates: []store.ActiveState{
			{ID: 1, ChatSessionID: "sess-lc-tail", StateType: "scene_state", Content: `{"pressure":"high"}`, TurnIndex: 720},
		},
		characterStates: []store.CharacterState{
			{ID: 1, ChatSessionID: "sess-lc-tail", CharacterName: "Mina", RelationshipsJSON: `{"Rowan":"trusted"}`, TurnIndex: 720},
		},
		storylines: []store.Storyline{
			{ID: 1, ChatSessionID: "sess-lc-tail", Name: "Ledger oath", Status: "active", CurrentContext: "The oath is still active.", LastEvidenceTurn: 650, EvidenceCount: 2, Confidence: 0.9},
		},
		worldRules: []store.WorldRule{
			{ID: 1, ChatSessionID: "sess-lc-tail", Scope: "session", Category: "world", Key: "ledger_oath", ValueJSON: `{"rule":"Oaths need payoff."}`, SourceTurn: 650},
		},
		pendingThreads: []store.PendingThread{
			{ID: 1, ChatSessionID: "sess-lc-tail", ThreadKey: "oath", Status: "open", Description: "Pay off the oath", CreatedTurn: 650, SourceTurn: 650},
		},
		episodeSummaries: []store.EpisodeSummary{
			{ID: 1, ChatSessionID: "sess-lc-tail", FromTurn: 600, ToTurn: 720, SummaryText: "Episode holds the oath."},
		},
		resumePack: &store.ResumePack{PackStatus: "ok", Trigger: "resume"},
		auditLogs: []store.AuditLog{
			{ID: 1, ChatSessionID: "sess-lc-tail", EventType: "critic_pipeline_trace", Summary: "split pipeline ok", DetailsJSON: `{"status":"ok"}`},
		},
	}
	mux := http.NewServeMux()
	srv := setupTestServer()
	srv.Store = fake
	srv.RegisterRoutes(mux)

	cases := []struct {
		path       string
		payloadKey string
		policy     string
	}{
		{"/metrics/lc1g/sess-lc-tail", "promotion_replay", "lc1g.v1"},
		{"/metrics/lc1h/sess-lc-tail", "false_negative_positive_replay", "lc1h.v1"},
		{"/metrics/lc1i/sess-lc-tail", "recall_ablation_compare", "lc1i.v1"},
		{"/metrics/lc1j/sess-lc-tail", "verification_gate", "lc1j.v1"},
		{"/metrics/lc1k/sess-lc-tail", "priority_budget_trace", "lc1k.v1"},
		{"/metrics/lc1l/sess-lc-tail", "imported_idea_contract_gate", "lc1l.v1"},
		{"/metrics/lc1m/sess-lc-tail", "split_pipeline_compare", "lc1m.v1"},
		{"/metrics/lc1n/sess-lc-tail", "rebuild_backfill_replay", "lc1n.v1"},
		{"/metrics/lc1o/sess-lc-tail", "deterministic_preview_ledger", "lc1o.v1"},
	}
	for _, tc := range cases {
		t.Run(tc.path, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, tc.path, nil)
			rec := httptest.NewRecorder()
			mux.ServeHTTP(rec, req)
			if rec.Code != http.StatusOK {
				t.Fatalf("status = %d, want 200: %s", rec.Code, rec.Body.String())
			}
			var resp map[string]any
			if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
				t.Fatalf("decode: %v", err)
			}
			payload, ok := resp[tc.payloadKey].(map[string]any)
			if !ok {
				t.Fatalf("%s missing or wrong type: %#v", tc.payloadKey, resp[tc.payloadKey])
			}
			if payload["policy_version"] != tc.policy {
				t.Fatalf("%s policy_version = %v, want %s: %#v", tc.payloadKey, payload["policy_version"], tc.policy, payload)
			}
			switch tc.payloadKey {
			case "promotion_replay":
				if payload["status"] != "pass" || payload["verified_promotion_count"].(float64) < 2 {
					t.Fatalf("promotion replay mismatch: %#v", payload)
				}
			case "false_negative_positive_replay":
				if payload["status"] != "pass" || payload["false_negative_risk_count"] != float64(0) || payload["false_positive_risk_count"] != float64(0) {
					t.Fatalf("false negative/positive replay mismatch: %#v", payload)
				}
			case "recall_ablation_compare":
				if payload["relationship_v2_signal_count"].(float64) <= 0 || payload["ledger_signal_count"].(float64) <= 0 || payload["world_pressure_signal_count"].(float64) <= 0 {
					t.Fatalf("ablation signal counts missing: %#v", payload)
				}
			case "verification_gate":
				if payload["status"] != "pass" || payload["release_gate_ready"] != true || payload["default_runtime_takeover"] != false {
					t.Fatalf("verification gate mismatch: %#v", payload)
				}
			case "priority_budget_trace":
				if payload["lower_tier_support_preserved"] != true || payload["high_priority_layer_count"].(float64) <= 0 {
					t.Fatalf("priority budget trace mismatch: %#v", payload)
				}
			case "imported_idea_contract_gate":
				if payload["default_takeover_blocked"] != true || payload["imported_signal_count"].(float64) <= 0 {
					t.Fatalf("imported idea gate mismatch: %#v", payload)
				}
			case "split_pipeline_compare":
				if payload["split_pipeline_enabled"] != true || payload["single_call_mode"] != false || payload["critic_pipeline_trace_count"].(float64) != 1 {
					t.Fatalf("split pipeline compare mismatch: %#v", payload)
				}
			case "rebuild_backfill_replay":
				if payload["status"] != "pass" || payload["drift_detected"] != false {
					t.Fatalf("backfill replay mismatch: %#v", payload)
				}
			case "deterministic_preview_ledger":
				if payload["llm_call_required"] != false || payload["preview_path"] != "deterministic" || payload["world_pressure_ready"] != true {
					t.Fatalf("deterministic preview ledger mismatch: %#v", payload)
				}
			}
		})
	}
}

func TestMetricsRoutesErrNotEnabledFallback(t *testing.T) {
	fake := &narrativeFakeStore{errNotEnabled: true}
	mux := http.NewServeMux()
	srv := setupTestServer()
	srv.Store = fake
	srv.RegisterRoutes(mux)

	req := httptest.NewRequest(http.MethodGet, "/metrics/lc1d/sess-1", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200: %s", rec.Code, rec.Body.String())
	}
	var resp map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if _, ok := resp["integrity_replay"].(map[string]any); !ok {
		t.Fatalf("integrity_replay missing or wrong type: %#v", resp["integrity_replay"])
	}
	if _, ok := resp["counts"]; ok {
		t.Fatalf("unexpected counts in Python-compatible metric shape: %#v", resp)
	}
}

func TestRemainingReadPlaceholdersStoreBackedEvidence(t *testing.T) {
	now := time.Now().UTC()
	fake := &narrativeFakeStore{
		sessions: []store.SessionSummary{
			{ChatSessionID: "sess-1"},
		},
		chatLogs: []store.ChatLog{
			{ID: 1, ChatSessionID: "sess-1", TurnIndex: 1, Role: "user"},
		},
		memories: []store.Memory{
			{ID: 1, ChatSessionID: "sess-1", TurnIndex: 1},
		},
		evidence: []store.DirectEvidence{
			{ID: 1, ChatSessionID: "sess-1", EvidenceText: "Alice entered the archive."},
		},
		kgTriples: []store.KGTriple{
			{ID: 1, ChatSessionID: "sess-1", Subject: "Alice", Predicate: "entered", Object: "archive"},
		},
		storylines: []store.Storyline{
			{ID: 1, ChatSessionID: "sess-1", Name: "Archive arrival", Status: "active", CurrentContext: "Archive arrival", KeyPointsJSON: `["arc"]`, OngoingTensionsJSON: `["answer pending"]`, Confidence: 0.8, EvidenceCount: 2, LastEvidenceTurn: 8, FirstTurn: 4, LastTurn: 8, UpdatedAt: now},
		},
		worldRules: []store.WorldRule{
			{ID: 1, ChatSessionID: "sess-1", Key: "archive_rules"},
		},
		characterStates: []store.CharacterState{
			{ID: 1, ChatSessionID: "sess-1", CharacterName: "Alice"},
		},
		pendingThreads: []store.PendingThread{
			{ID: 1, ChatSessionID: "sess-1", ThreadKey: "door", Status: "open"},
		},
		activeStates: []store.ActiveState{
			{ID: 1, ChatSessionID: "sess-1", StateType: "scene_state"},
		},
		canonicalStateLayers: []store.CanonicalStateLayer{
			{ID: 1, ChatSessionID: "sess-1", LayerType: "scene_state"},
		},
		episodeSummaries: []store.EpisodeSummary{
			{ID: 1, ChatSessionID: "sess-1", FromTurn: 1, ToTurn: 8, SummaryText: "Alice explores the archive.", KeyEntities: "Alice"},
			{ID: 2, ChatSessionID: "sess-1", FromTurn: 9, ToTurn: 12, SummaryText: "Bob waits outside.", KeyEntities: "Bob"},
		},
		resumePack: &store.ResumePack{
			PackStatus: "ok",
			Chapter: &store.ChapterSummary{
				ID:           10,
				FromTurn:     1,
				ToTurn:       12,
				ChapterTitle: "Archive Gate",
				SummaryText:  "Alice explores the archive.",
			},
		},
	}
	mux := http.NewServeMux()
	srv := setupTestServer()
	srv.Store = fake
	srv.RegisterRoutes(mux)

	getTests := []string{
		"/sessions/compare?session_ids=sess-1,sess-2&preview_limit=5",
		"/metrics/lc1r/regression-corpus?limit=5",
	}
	for _, path := range getTests {
		t.Run(path, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, path, nil)
			rec := httptest.NewRecorder()
			mux.ServeHTTP(rec, req)
			if rec.Code != http.StatusOK {
				t.Fatalf("status = %d, want 200: %s", rec.Code, rec.Body.String())
			}
			var resp map[string]any
			if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
				t.Fatalf("decode: %v", err)
			}
			if path == "/metrics/lc1r/regression-corpus?limit=5" {
				if _, ok := resp["regression_corpus_manifest"].(map[string]any); !ok {
					t.Fatalf("regression_corpus_manifest missing: %#v", resp)
				}
				return
			}
			if resp["status"] != "ok" {
				t.Fatalf("status = %v, want ok", resp["status"])
			}
			if _, ok := resp["store_status"]; ok {
				t.Fatalf("store_status should be omitted on Python-compatible compare response: %#v", resp)
			}
		})
	}

	postTests := []struct {
		path      string
		body      string
		wantCount float64
	}{
		{path: "/chapters/dry-run", body: `{"chat_session_id":"sess-1","turn_index":60,"interval":60,"limit":5}`, wantCount: -1},
		{path: "/chapters/search", body: `{"chat_session_id":"sess-1","query":"archive","limit":5}`, wantCount: 2},
		{path: "/episodes/search", body: `{"chat_session_id":"sess-1","query":"Alice","limit":5}`, wantCount: 1},
	}
	for _, tt := range postTests {
		t.Run(tt.path, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodPost, tt.path, strings.NewReader(tt.body))
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
			if _, ok := resp["store_status"]; ok {
				t.Fatalf("store_status should be omitted on Python-compatible response: %#v", resp)
			}
			if tt.wantCount >= 0 && resp["count"] != tt.wantCount {
				t.Fatalf("count = %v, want %v", resp["count"], tt.wantCount)
			}
		})
	}
}

func TestRemainingReadPlaceholdersErrNotEnabledFallback(t *testing.T) {
	fake := &narrativeFakeStore{errNotEnabled: true}
	mux := http.NewServeMux()
	srv := setupTestServer()
	srv.Store = fake
	srv.RegisterRoutes(mux)

	req := httptest.NewRequest(http.MethodGet, "/sessions/compare", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400: %s", rec.Code, rec.Body.String())
	}
	var resp map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if resp["status"] != "error" || resp["detail"] != "At least 2 session_ids are required." {
		t.Fatalf("unexpected compare fallback response: %#v", resp)
	}
}

func TestSessionResumePackIncludesGuidanceSnapshot(t *testing.T) {
	spJSON, _ := json.Marshal(map[string]any{"current_arc": "Resume Arc"})
	dirJSON, _ := json.Marshal(map[string]any{"scene_mandate": "Resume carefully"})
	mux := http.NewServeMux()
	srv := setupTestServer()
	srv.Store = &narrativeFakeStore{
		resumePack: &store.ResumePack{
			PackStatus:    "ready",
			Trigger:       "resume",
			SourcesUsed:   []string{"chapter", "arc"},
			LayerCount:    2,
			AssembledText: "Resume from the rooftop arc.",
			AssemblyNote:  "store-backed resume",
		},
		guidancePlanState: &store.GuidancePlanState{
			ChatSessionID: "sess-1",
			StoryPlanJSON: string(spJSON),
			DirectorJSON:  string(dirJSON),
			StateStatus:   "ready",
			LastTurn:      8,
		},
	}
	srv.RegisterRoutes(mux)

	req := httptest.NewRequest(http.MethodGet, "/sessions/sess-1/resume-pack?continuity_trigger_mode=resume", nil)
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
	pack, ok := resp["resume_pack"].(map[string]any)
	if !ok {
		t.Fatalf("resume_pack is not an object: %#v", resp)
	}
	if pack["pack_status"] != "ready" || pack["assembled_text"] != "Resume from the rooftop arc." {
		t.Fatalf("resume_pack mismatch: %#v", pack)
	}
	gs, ok := resp["guidance_snapshot"].(map[string]any)
	if !ok {
		t.Fatalf("guidance_snapshot missing: %#v", resp)
	}
	if gs["state_status"] != "active" || gs["last_turn"] != float64(8) {
		t.Fatalf("guidance_snapshot mismatch: %#v", gs)
	}
}

func TestChapterGenerateWritesDeterministicChapterSummary(t *testing.T) {
	mux := http.NewServeMux()
	srv := setupTestServer()
	srv.Cfg.StoreMode = config.StoreModeMariaDBAuthority
	fake := &narrativeFakeStore{
		episodeSummaries: []store.EpisodeSummary{
			{ChatSessionID: "sess-chg", FromTurn: 1, ToTurn: 15, SummaryText: "Alice enters the archive.", KeyEntities: "Alice"},
			{ChatSessionID: "sess-chg", FromTurn: 16, ToTurn: 30, SummaryText: "Bob finds a sealed ledger.", KeyEntities: "Bob"},
			{ChatSessionID: "sess-chg", FromTurn: 31, ToTurn: 45, SummaryText: "The tower rule changes.", KeyEntities: "rule"},
			{ChatSessionID: "sess-chg", FromTurn: 46, ToTurn: 60, SummaryText: "Alice chooses to stay.", KeyEntities: "choice"},
		},
	}
	srv.Store = fake
	srv.RegisterRoutes(mux)

	req := httptest.NewRequest(http.MethodPost, "/chapters/generate", strings.NewReader(`{"chat_session_id":"sess-chg","turn_index":60,"interval":60}`))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200: %s", rec.Code, rec.Body.String())
	}
	if len(fake.savedChapterSummaries) != 1 {
		t.Fatalf("saved chapters = %d, want 1", len(fake.savedChapterSummaries))
	}
	saved := fake.savedChapterSummaries[0]
	if saved.ChatSessionID != "sess-chg" || saved.FromTurn != 1 || saved.ToTurn != 60 {
		t.Fatalf("saved chapter range/session mismatch: %+v", saved)
	}
	if !strings.Contains(saved.SummaryText, "Alice enters the archive") || !strings.Contains(saved.ResumeText, "Turns 1-60") {
		t.Fatalf("saved chapter text incomplete: %+v", saved)
	}

	var resp map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if resp["generation_source"] != "deterministic_migration_stub" {
		t.Fatalf("generation_source = %v", resp["generation_source"])
	}
	if resp["llm_attempted"] != false || resp["saved"] != true {
		t.Fatalf("unexpected generation flags: %#v", resp)
	}
}

func TestChapterGenerateUsesConfiguredLLMWhenAvailable(t *testing.T) {
	calls := 0
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		if r.URL.Path != "/v1/chat/completions" {
			t.Fatalf("upstream path = %s, want /v1/chat/completions", r.URL.Path)
		}
		var body map[string]any
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Fatalf("upstream body decode: %v", err)
		}
		if body["model"] != "chapter-model" {
			t.Fatalf("upstream model = %v", body["model"])
		}
		writeJSON(w, http.StatusOK, map[string]any{
			"model": "chapter-model",
			"choices": []any{
				map[string]any{
					"message": map[string]any{
						"content": `{"chapter_title":"LLM Gate","summary_text":"LLM summary keeps the archive gate callback.","open_loops":["gate"],"relationship_changes":["Alice trusts Bob"],"world_changes":["archive gate opens"],"callback_candidates":["sealed ledger"],"resume_text":"LLM resume for turns 1-60."}`,
					},
				},
			},
			"usage": map[string]any{"total_tokens": 77},
		})
	}))
	defer upstream.Close()

	mux := http.NewServeMux()
	srv := setupTestServer()
	srv.Cfg.StoreMode = config.StoreModeMariaDBAuthority
	srv.updateRuntimeConfig(map[string]any{
		"mainProvider": "openai",
		"mainApiKey":   "sk-chapter-test",
		"mainEndpoint": upstream.URL,
		"mainModel":    "chapter-model",
		"mainTimeout":  5,
	})
	fake := &narrativeFakeStore{
		episodeSummaries: []store.EpisodeSummary{
			{ChatSessionID: "sess-chg-llm", FromTurn: 1, ToTurn: 15, SummaryText: "Alice enters the archive.", KeyEntities: "Alice"},
			{ChatSessionID: "sess-chg-llm", FromTurn: 16, ToTurn: 30, SummaryText: "Bob finds a sealed ledger.", KeyEntities: "Bob"},
			{ChatSessionID: "sess-chg-llm", FromTurn: 31, ToTurn: 45, SummaryText: "The tower rule changes.", KeyEntities: "rule"},
			{ChatSessionID: "sess-chg-llm", FromTurn: 46, ToTurn: 60, SummaryText: "Alice chooses to stay.", KeyEntities: "choice"},
		},
	}
	srv.Store = fake
	srv.RegisterRoutes(mux)

	req := httptest.NewRequest(http.MethodPost, "/chapters/generate", strings.NewReader(`{"chat_session_id":"sess-chg-llm","turn_index":60,"interval":60}`))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200: %s", rec.Code, rec.Body.String())
	}
	if calls != 1 {
		t.Fatalf("upstream calls = %d, want 1", calls)
	}
	if len(fake.savedChapterSummaries) != 1 {
		t.Fatalf("saved chapters = %d, want 1", len(fake.savedChapterSummaries))
	}
	saved := fake.savedChapterSummaries[0]
	if saved.ChapterTitle != "LLM Gate" || saved.SummaryText != "LLM summary keeps the archive gate callback." || saved.ResumeText != "LLM resume for turns 1-60." {
		t.Fatalf("saved chapter did not use LLM JSON: %+v", saved)
	}
	if !strings.Contains(saved.OpenLoopsJSON, "gate") || !strings.Contains(saved.WorldChangesJSON, "archive gate opens") {
		t.Fatalf("saved chapter JSON fields incomplete: %+v", saved)
	}

	var resp map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if resp["generation_source"] != "configured_llm" || resp["llm_attempted"] != true {
		t.Fatalf("unexpected LLM generation flags: %#v", resp)
	}
	shadow, ok := resp["chapter_shadow_compare"].(map[string]any)
	if !ok || shadow["enabled"] != true || shadow["summary_diverged"] != true {
		t.Fatalf("chapter_shadow_compare missing/divergence not recorded: %#v", resp)
	}
}

func TestChapterSearchUsesChapterSummaryStoreBeforeFallback(t *testing.T) {
	mux := http.NewServeMux()
	srv := setupTestServer()
	srv.Store = &narrativeFakeStore{
		chapterSummaries: []store.ChapterSummary{
			{
				ID:            7,
				ChatSessionID: "sess-search",
				FromTurn:      1,
				ToTurn:        60,
				ChapterIndex:  1,
				ChapterTitle:  "Archive Gate",
				SummaryText:   "Alice studies the sealed archive gate.",
				ResumeText:    "Archive gate callback is active.",
			},
		},
		episodeSummaries: []store.EpisodeSummary{
			{ID: 99, ChatSessionID: "sess-search", FromTurn: 1, ToTurn: 10, SummaryText: "Archive fallback episode."},
		},
	}
	srv.RegisterRoutes(mux)

	req := httptest.NewRequest(http.MethodPost, "/chapters/search", strings.NewReader(`{"chat_session_id":"sess-search","query":"gate","limit":5}`))
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
	if !ok || len(items) == 0 {
		t.Fatalf("chapters missing: %#v", resp)
	}
	first, ok := items[0].(map[string]any)
	if !ok {
		t.Fatalf("first chapter item shape = %#v", items[0])
	}
	if first["source"] != "chapter_summary" {
		t.Fatalf("source = %v, want chapter_summary", first["source"])
	}
	if first["chapter_title"] != "Archive Gate" {
		t.Fatalf("chapter_title = %v, want Archive Gate", first["chapter_title"])
	}
}

func TestChapterSearchIncludesDS1fSourceAnchors(t *testing.T) {
	mux := http.NewServeMux()
	srv := setupTestServer()
	srv.Store = &narrativeFakeStore{
		chapterSummaries: []store.ChapterSummary{
			{ID: 7, ChatSessionID: "sess-ds1f", FromTurn: 1, ToTurn: 60, ChapterIndex: 1, ChapterTitle: "Gate", SummaryText: "Alice opens the gate.", ResumeText: "Gate opened.", OpenLoopsJSON: `["loop1"]`, RelationshipChangesJSON: `["rel1"]`, WorldChangesJSON: `["world1"]`, CallbackCandidatesJSON: `["cb1"]`},
		},
	}
	srv.RegisterRoutes(mux)
	req := httptest.NewRequest(http.MethodPost, "/chapters/search", strings.NewReader(`{"chat_session_id":"sess-ds1f","query":"gate"}`))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	var resp map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	items := resp["chapters"].([]any)
	first := items[0].(map[string]any)
	if first["source_record_id"] != float64(7) {
		t.Fatalf("source_record_id = %v, want 7", first["source_record_id"])
	}
	if first["source_record_type"] != "chapter" {
		t.Fatalf("source_record_type = %v, want chapter", first["source_record_type"])
	}
	if first["dense_source_anchor_policy_version"] != "ds1f.v1" {
		t.Fatalf("dense_source_anchor_policy_version = %v", first["dense_source_anchor_policy_version"])
	}
}

func TestEpisodeSearchIncludesDS1gRetentionFields(t *testing.T) {
	mux := http.NewServeMux()
	srv := setupTestServer()
	srv.Store = &narrativeFakeStore{
		episodeSummaries: []store.EpisodeSummary{
			{ID: 1, ChatSessionID: "sess-ds1g", FromTurn: 1, ToTurn: 10, SummaryText: "Alice trusts Bob.", RelationshipChangesJSON: `["Alice trusts Bob"]`},
		},
	}
	srv.RegisterRoutes(mux)
	req := httptest.NewRequest(http.MethodPost, "/episodes/search", strings.NewReader(`{"chat_session_id":"sess-ds1g","query":"alice"}`))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	var resp map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	items := resp["episodes"].([]any)
	first := items[0].(map[string]any)
	if first["dense_retention_policy_version"] != "ds1g.v1" {
		t.Fatalf("dense_retention_policy_version = %v", first["dense_retention_policy_version"])
	}
	if first["dense_retention_applied"] != true {
		t.Fatalf("dense_retention_applied = %v, want true", first["dense_retention_applied"])
	}
	if first["dense_retention_reason"] != "important_fact_retention" {
		t.Fatalf("dense_retention_reason = %v", first["dense_retention_reason"])
	}
}

func TestChapterSearchIncludesDS1hRoleSplitFields(t *testing.T) {
	mux := http.NewServeMux()
	srv := setupTestServer()
	srv.Store = &narrativeFakeStore{
		chapterSummaries: []store.ChapterSummary{
			{ID: 3, ChatSessionID: "sess-ds1h", FromTurn: 1, ToTurn: 60, ChapterTitle: "Gate", SummaryText: "Summary.", OpenLoopsJSON: `["loop"]`, RelationshipChangesJSON: `["rel"]`, WorldChangesJSON: `["world"]`, CallbackCandidatesJSON: `["cb"]`},
		},
	}
	srv.RegisterRoutes(mux)
	req := httptest.NewRequest(http.MethodPost, "/chapters/search", strings.NewReader(`{"chat_session_id":"sess-ds1h","query":"gate"}`))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	var resp map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	items := resp["chapters"].([]any)
	first := items[0].(map[string]any)
	if first["dense_role_split_policy_version"] != "ds1h.v1" {
		t.Fatalf("dense_role_split_policy_version = %v", first["dense_role_split_policy_version"])
	}
	if first["dense_narrative_usage"] != "read_only" {
		t.Fatalf("dense_narrative_usage = %v", first["dense_narrative_usage"])
	}
	if first["dense_structured_usage"] != "adjudication_retrieval" {
		t.Fatalf("dense_structured_usage = %v", first["dense_structured_usage"])
	}
	payload, ok := first["dense_structured_payload"].(map[string]any)
	if !ok {
		t.Fatalf("dense_structured_payload missing or wrong type")
	}
	wc, ok := payload["world_changes"].([]any)
	if !ok || len(wc) == 0 {
		t.Fatalf("dense_structured_payload world_changes empty or wrong type: %v", payload["world_changes"])
	}
	if wc[0] != "world" {
		t.Fatalf("dense_structured_payload world_changes[0] = %v, want world", wc[0])
	}
}

func TestEpisodeSearchIncludesDS1iDirectEvidencePromotionFields(t *testing.T) {
	mux := http.NewServeMux()
	srv := setupTestServer()
	srv.Store = &narrativeFakeStore{
		episodeSummaries: []store.EpisodeSummary{
			{ID: 1, ChatSessionID: "sess-ds1i", FromTurn: 1, ToTurn: 10, SummaryText: "Gate opened.", KeyEvents: `["world gate opened"]`, OpenLoopsJSON: `["loop1"]`, RelationshipChangesJSON: `["Alice trusts Bob"]`},
		},
	}
	srv.RegisterRoutes(mux)
	req := httptest.NewRequest(http.MethodPost, "/episodes/search", strings.NewReader(`{"chat_session_id":"sess-ds1i","query":"gate"}`))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	var resp map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	items := resp["episodes"].([]any)
	first := items[0].(map[string]any)
	if first["dense_direct_evidence_promotion_policy_version"] != "ds1i.v1" {
		t.Fatalf("dense_direct_evidence_promotion_policy_version = %v", first["dense_direct_evidence_promotion_policy_version"])
	}
	if first["dense_structured_precedence_applied"] != true {
		t.Fatalf("dense_structured_precedence_applied = %v, want true", first["dense_structured_precedence_applied"])
	}
	if first["dense_direct_evidence_promoted_relationship_count"] != float64(1) {
		t.Fatalf("dense_direct_evidence_promoted_relationship_count = %v", first["dense_direct_evidence_promoted_relationship_count"])
	}
	if first["dense_direct_evidence_promoted_world_count"] != float64(1) {
		t.Fatalf("dense_direct_evidence_promoted_world_count = %v", first["dense_direct_evidence_promoted_world_count"])
	}
	if first["dense_direct_evidence_promoted_promise_count"] != float64(0) {
		t.Fatalf("dense_direct_evidence_promoted_promise_count = %v, want 0", first["dense_direct_evidence_promoted_promise_count"])
	}
	score, ok := first["dense_direct_evidence_promotion_score"].(float64)
	if !ok || score != 2 {
		t.Fatalf("dense_direct_evidence_promotion_score = %v, want 2", first["dense_direct_evidence_promotion_score"])
	}
}

func TestChapterSearchResumePackIncludesDS1fThroughDS1iFields(t *testing.T) {
	mux := http.NewServeMux()
	srv := setupTestServer()
	srv.Store = &narrativeFakeStore{
		resumePack: &store.ResumePack{
			Chapter: &store.ChapterSummary{
				ID: 99, ChatSessionID: "sess-resume", FromTurn: 1, ToTurn: 60, ChapterIndex: 1, ChapterTitle: "Gate",
				SummaryText: "Summary.", ResumeText: "Resume.",
				OpenLoopsJSON: `["loop"]`, RelationshipChangesJSON: `["rel"]`, WorldChangesJSON: `["world"]`, CallbackCandidatesJSON: `["cb"]`},
		},
	}
	srv.RegisterRoutes(mux)
	req := httptest.NewRequest(http.MethodPost, "/chapters/search", strings.NewReader(`{"chat_session_id":"sess-resume","query":"gate"}`))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	var resp map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	items := resp["chapters"].([]any)
	first := items[0].(map[string]any)
	if first["source_record_type"] != "chapter" {
		t.Fatalf("source_record_type = %v, want chapter", first["source_record_type"])
	}
	if first["dense_source_anchor_policy_version"] != "ds1f.v1" {
		t.Fatalf("dense_source_anchor_policy_version = %v", first["dense_source_anchor_policy_version"])
	}
	if first["dense_retention_applied"] != true {
		t.Fatalf("dense_retention_applied = %v", first["dense_retention_applied"])
	}
	if first["dense_role_split_policy_version"] != "ds1h.v1" {
		t.Fatalf("dense_role_split_policy_version = %v", first["dense_role_split_policy_version"])
	}
	if first["dense_direct_evidence_promotion_policy_version"] != "ds1i.v1" {
		t.Fatalf("dense_direct_evidence_promotion_policy_version = %v", first["dense_direct_evidence_promotion_policy_version"])
	}
}

func TestArcGenerateWritesArcSummaryFromChapters(t *testing.T) {
	mux := http.NewServeMux()
	srv := setupTestServer()
	srv.Cfg.StoreMode = config.StoreModeMariaDBAuthority
	fake := &narrativeFakeStore{
		chapterSummaries: []store.ChapterSummary{
			{ChatSessionID: "sess-arc", FromTurn: 1, ToTurn: 60, ChapterIndex: 1, ChapterTitle: "Gate", SummaryText: "Alice opens the gate.", ResumeText: "Gate opened."},
			{ChatSessionID: "sess-arc", FromTurn: 61, ToTurn: 120, ChapterIndex: 2, ChapterTitle: "Ledger", SummaryText: "Bob keeps the ledger.", ResumeText: "Ledger protected."},
			{ChatSessionID: "sess-arc", FromTurn: 121, ToTurn: 180, ChapterIndex: 3, ChapterTitle: "Tower", SummaryText: "The tower rule changes.", ResumeText: "Rule changed."},
		},
	}
	srv.Store = fake
	srv.RegisterRoutes(mux)

	req := httptest.NewRequest(http.MethodPost, "/arcs/generate", strings.NewReader(`{"chat_session_id":"sess-arc","from_turn":1,"to_turn":180}`))
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
	if saved.ChatSessionID != "sess-arc" || saved.FromTurn != 1 || saved.ToTurn != 180 || saved.ArcStatus != "active" {
		t.Fatalf("saved arc mismatch: %+v", saved)
	}
	if !strings.Contains(saved.ArcResumeText, "Gate opened") {
		t.Fatalf("saved arc resume missing chapter material: %+v", saved)
	}

	var resp map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if resp["generation_source"] != "deterministic_migration_stub" || resp["saved"] != true {
		t.Fatalf("unexpected arc generation response: %#v", resp)
	}
}

func TestChapterGeneratePrioritizesEpisodeDenseAnchorsOverSummaryText(t *testing.T) {
	mux := http.NewServeMux()
	srv := setupTestServer()
	srv.Cfg.StoreMode = config.StoreModeMariaDBAuthority
	fake := &narrativeFakeStore{
		episodeSummaries: []store.EpisodeSummary{
			{
				ChatSessionID:           "sess-ds1b-chapter",
				FromTurn:                1,
				ToTurn:                  20,
				SummaryText:             "Generic school day summary.",
				KeyEvents:               `["tower gate rule changes at midnight"]`,
				OpenLoopsJSON:           `["sealed ledger callback remains unresolved"]`,
				RelationshipChangesJSON: `["Alice starts trusting Bob"]`,
			},
			{
				ChatSessionID:           "sess-ds1b-chapter",
				FromTurn:                21,
				ToTurn:                  60,
				SummaryText:             "Another generic summary.",
				KeyEvents:               `["archive city pressure rises"]`,
				OpenLoopsJSON:           `["ask why the gate opened"]`,
				RelationshipChangesJSON: `["Bob promises not to hide the ledger"]`,
			},
		},
	}
	srv.Store = fake
	srv.RegisterRoutes(mux)

	req := httptest.NewRequest(http.MethodPost, "/chapters/generate", strings.NewReader(`{"chat_session_id":"sess-ds1b-chapter","turn_index":60,"interval":60}`))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200: %s", rec.Code, rec.Body.String())
	}
	if len(fake.savedChapterSummaries) != 1 {
		t.Fatalf("saved chapters = %d, want 1", len(fake.savedChapterSummaries))
	}
	saved := fake.savedChapterSummaries[0]
	if !strings.Contains(saved.OpenLoopsJSON, "sealed ledger callback") || !strings.Contains(saved.RelationshipChangesJSON, "Alice starts trusting Bob") {
		t.Fatalf("saved chapter anchors incomplete: %+v", saved)
	}
	if !strings.Contains(saved.WorldChangesJSON, "tower gate rule changes") || !strings.Contains(saved.CallbackCandidatesJSON, "ask why the gate opened") {
		t.Fatalf("saved chapter world/callback anchors incomplete: %+v", saved)
	}
	openIdx := strings.Index(saved.SummaryText, "open_loop:")
	summaryIdx := strings.Index(saved.SummaryText, "summary:")
	if openIdx < 0 || summaryIdx < 0 || openIdx > summaryIdx {
		t.Fatalf("summary did not prioritize anchors before summary text: %q", saved.SummaryText)
	}

	var resp map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	stats, ok := resp["input_stats"].(map[string]any)
	if !ok || stats["chapter_dense_summary_injection_policy_version"] != chapterDenseSummaryPolicyVersion {
		t.Fatalf("chapter dense stats missing: %#v", resp)
	}
}
