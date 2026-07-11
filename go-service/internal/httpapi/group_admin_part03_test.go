package httpapi

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/risulongmemory/archive-center-go/internal/config"
	"github.com/risulongmemory/archive-center-go/internal/store"
	"github.com/risulongmemory/archive-center-go/internal/vector"
)

func TestAdminReindexBlocksChromaDimensionMismatchAtFirstVectorError(t *testing.T) {
	cfg := config.Default()
	cfg.StoreMode = config.StoreModeMariaDBShadow
	cfg.ChromaEndpoint = "http://127.0.0.1:8000"
	srv := NewServer(cfg)
	srv.Store = &turnRecordingStore{
		returnMemories: []store.Memory{
			{ID: 1, ChatSessionID: "sess-dim-mismatch", TurnIndex: 1, SummaryJSON: `{"summary":"Already embedded memory."}`, Embedding: `[0.1,0.2]`, EmbeddingModel: "old-model"},
		},
	}
	srv.StoreOpenError = nil
	srv.Vector = &turnRecordingVectorStore{upsertErr: fmt.Errorf("chroma collection dimension mismatch: current embedding dimension=1024; existing collection was created with a different embedding dimension")}

	resp, err := srv.runAdminReindexJob(context.Background(), "sess-dim-mismatch", map[string]any{}, nil)
	if err != nil {
		t.Fatalf("runAdminReindexJob: %v", err)
	}
	if resp["status"] != "blocked" || resp["reason"] != "chroma_collection_dimension_mismatch" {
		t.Fatalf("response = %#v, want blocked chroma_collection_dimension_mismatch", resp)
	}
	if resp["stage"] != "collection_recreate_required" || resp["ui_action"] != "recreate_chromadb_collection_then_reindex" {
		t.Fatalf("dimension mismatch guidance missing: %#v", resp)
	}
	if resp["blocked_tier"] != "memory" || resp["blocked_row_id"] != int64(1) {
		t.Fatalf("blocked target mismatch: %#v", resp)
	}
}

func TestAdminReindexDerivedArtifactsEmitsTierProgress(t *testing.T) {
	srv := NewServer(config.Default())
	events := []map[string]any{}
	progress := adminReindexDerivedArtifactProgress{
		Total: 2,
		Progress: func(item map[string]any) {
			events = append(events, cloneMapAny(item))
		},
	}
	result := srv.adminReindexDerivedArtifacts(
		context.Background(),
		"sess-derived-progress",
		completeTurnExtractionConfig{},
		false,
		100,
		[]store.DirectEvidence{{ID: 10, ChatSessionID: "sess-derived-progress", EvidenceText: "The brass key opens the cellar.", SourceTurnEnd: 1}},
		[]store.WorldRule{{ID: 20, ChatSessionID: "sess-derived-progress", Scope: "location", ScopeName: "cellar", Category: "access", Key: "brass_key", ValueJSON: `{"value":"The cellar opens with a brass key."}`, SourceTurn: 1}},
		progress,
	)
	if result.Processed != 2 || result.Skipped != 2 {
		t.Fatalf("result = %+v, want processed/skipped 2", result)
	}
	if len(events) == 0 {
		t.Fatal("expected progress events")
	}
	seenEvidence := false
	seenWorldRule := false
	for _, event := range events {
		if event["stage"] != "derived_artifact_reindex" {
			continue
		}
		if event["tier"] == "evidence" && event["phase"] == "item_done" {
			seenEvidence = true
		}
		if event["tier"] == "world_rule" && event["phase"] == "item_done" {
			seenWorldRule = true
		}
	}
	if !seenEvidence || !seenWorldRule {
		t.Fatalf("missing tier progress: evidence=%v world_rule=%v events=%#v", seenEvidence, seenWorldRule, events)
	}
}

func TestAdminVectorOrphanAuditDeletesFullListingOrphans(t *testing.T) {
	cfg := config.Default()
	cfg.StoreMode = config.StoreModeMariaDBAuthority
	cfg.ChromaEndpoint = "http://127.0.0.1:8000"
	srv := NewServer(cfg)
	fake := &turnRecordingStore{
		returnMemories: []store.Memory{{ID: 1, ChatSessionID: "sess-orphan", TurnIndex: 1, SummaryJSON: `{"summary":"Kept memory"}`}},
	}
	vec := &turnRecordingVectorStore{docs: []vector.VectorDocument{
		{ID: "memory:sess-orphan:1", Tier: "memory", ChatSessionID: "sess-orphan", SourceTable: "memories", SourceRowID: "1"},
		{ID: "evidence:sess-orphan:999", Tier: "evidence", ChatSessionID: "sess-orphan", SourceTable: "direct_evidence_records", SourceRowID: "999"},
	}}
	srv.Store = fake
	srv.StoreOpenError = nil
	srv.Vector = vec

	mux := http.NewServeMux()
	srv.RegisterRoutes(mux)

	req := httptest.NewRequest(http.MethodPost, "/admin/vector-orphan-audit", strings.NewReader(`{"chat_session_id":"sess-orphan","delete_orphans":true}`))
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
	if resp["full_listing_available"] != true || resp["orphan_count"] != float64(1) || resp["deleted_orphan_count"] != float64(1) {
		t.Fatalf("orphan audit response = %#v", resp)
	}
	if len(vec.docs) != 1 || vec.docs[0].ID != "memory:sess-orphan:1" {
		t.Fatalf("remaining vector docs = %#v", vec.docs)
	}
}

func TestAdminDedupeCleanupDryRunAndApply(t *testing.T) {
	cfg := config.Default()
	cfg.StoreMode = config.StoreModeMariaDBAuthority
	cfg.ChromaEndpoint = "http://127.0.0.1:8000"
	srv := NewServer(cfg)
	fake := &turnRecordingStore{
		returnMemories: []store.Memory{
			{ID: 1, ChatSessionID: "sess-dedupe", TurnIndex: 1, SummaryJSON: `{"summary":"Shared vow persists."}`, Importance: 3},
			{ID: 2, ChatSessionID: "sess-dedupe", TurnIndex: 2, SummaryJSON: `{"summary":"Shared vow persists."}`, Importance: 5},
		},
		returnStorylines: []store.Storyline{
			{ID: 10, ChatSessionID: "sess-dedupe", Name: "Bridge promise", CurrentContext: "The bridge promise remains open.", LastTurn: 1},
			{ID: 11, ChatSessionID: "sess-dedupe", Name: "Bridge promise", CurrentContext: "The bridge promise remains open.", LastTurn: 2},
		},
		returnWorldRules: []store.WorldRule{
			{ID: 20, ChatSessionID: "sess-dedupe", Scope: "location", ScopeName: "bridge", Category: "access", Key: "guarded_gate", ValueJSON: `{"value":"The gate is guarded."}`, SourceTurn: 1},
			{ID: 21, ChatSessionID: "sess-dedupe", Scope: "location", ScopeName: "bridge", Category: "access", Key: "guarded_gate", ValueJSON: `{"value":"The gate is guarded."}`, SourceTurn: 2},
		},
	}
	vec := &turnRecordingVectorStore{docs: []vector.VectorDocument{
		{ID: "memory:sess-dedupe:1", Tier: "memory", ChatSessionID: "sess-dedupe", SourceTable: "memories", SourceRowID: "1"},
		{ID: "memory:sess-dedupe:2", Tier: "memory", ChatSessionID: "sess-dedupe", SourceTable: "memories", SourceRowID: "2"},
		{ID: "world_rule:sess-dedupe:20", Tier: "world_rule", ChatSessionID: "sess-dedupe", SourceTable: "world_rules", SourceRowID: "20"},
		{ID: "world_rule:sess-dedupe:21", Tier: "world_rule", ChatSessionID: "sess-dedupe", SourceTable: "world_rules", SourceRowID: "21"},
	}}
	srv.Store = fake
	srv.StoreOpenError = nil
	srv.Vector = vec
	mux := http.NewServeMux()
	srv.RegisterRoutes(mux)

	req := httptest.NewRequest(http.MethodPost, "/admin/dedupe-cleanup", strings.NewReader(`{"chat_session_id":"sess-dedupe"}`))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("dry-run status = %d: %s", rec.Code, rec.Body.String())
	}
	var preview map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &preview); err != nil {
		t.Fatalf("decode preview: %v", err)
	}
	counts, ok := preview["candidate_counts"].(map[string]any)
	if !ok || counts["memories"] != float64(1) || counts["storylines"] != float64(1) || counts["world_rules"] != float64(1) {
		t.Fatalf("candidate_counts = %#v", preview["candidate_counts"])
	}

	req = httptest.NewRequest(http.MethodPost, "/admin/dedupe-cleanup", strings.NewReader(`{"chat_session_id":"sess-dedupe","apply":true}`))
	req.Header.Set("Content-Type", "application/json")
	rec = httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("apply status = %d: %s", rec.Code, rec.Body.String())
	}
	var applied map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &applied); err != nil {
		t.Fatalf("decode applied: %v", err)
	}
	deleted, ok := applied["deleted_counts"].(map[string]any)
	if !ok || deleted["memories"] != float64(1) || deleted["storylines"] != float64(1) || deleted["world_rules"] != float64(1) {
		t.Fatalf("deleted_counts = %#v", applied["deleted_counts"])
	}
	if len(fake.returnMemories) != 1 || fake.returnMemories[0].ID != 2 {
		t.Fatalf("remaining memories = %#v", fake.returnMemories)
	}
	if len(fake.returnStorylines) != 1 || fake.returnStorylines[0].ID != 11 {
		t.Fatalf("remaining storylines = %#v", fake.returnStorylines)
	}
	if len(fake.returnWorldRules) != 1 || fake.returnWorldRules[0].ID != 21 {
		t.Fatalf("remaining world rules = %#v", fake.returnWorldRules)
	}
}

func TestAdminReindexBackgroundJobReportsProgress(t *testing.T) {
	cfg := config.Default()
	cfg.StoreMode = config.StoreModeMariaDBShadow
	cfg.ChromaEndpoint = "http://127.0.0.1:8000"
	srv := NewServer(cfg)
	fake := &memoryFakeStore{
		memories: []store.Memory{
			{
				ID:             42,
				ChatSessionID:  "sess-reindex-bg",
				TurnIndex:      7,
				SummaryJSON:    `{"summary":"Blue lantern oath persists."}`,
				Embedding:      `[0.1,0.2,0.3]`,
				EmbeddingModel: "test-embedding",
			},
		},
	}
	vec := &turnRecordingVectorStore{}
	srv.Store = fake
	srv.StoreOpenError = nil
	srv.Vector = vec

	mux := http.NewServeMux()
	srv.RegisterRoutes(mux)

	req := httptest.NewRequest(http.MethodPost, "/admin/reindex", strings.NewReader(`{"chat_session_id":"sess-reindex-bg","dry_run":false,"background":true}`))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusAccepted {
		t.Fatalf("status = %d, want 202: %s", rec.Code, rec.Body.String())
	}
	var start map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &start); err != nil {
		t.Fatalf("decode start: %v", err)
	}
	jobID, _ := start["job_id"].(string)
	if jobID == "" {
		t.Fatalf("job_id missing: %#v", start)
	}

	var job map[string]any
	for i := 0; i < 50; i++ {
		rec = httptest.NewRecorder()
		req = httptest.NewRequest(http.MethodGet, "/admin/jobs/"+jobID, nil)
		mux.ServeHTTP(rec, req)
		if rec.Code != http.StatusOK {
			t.Fatalf("job status = %d: %s", rec.Code, rec.Body.String())
		}
		job = map[string]any{}
		if err := json.Unmarshal(rec.Body.Bytes(), &job); err != nil {
			t.Fatalf("decode job: %v", err)
		}
		if job["status"] == "completed" {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	if job["status"] != "completed" {
		t.Fatalf("job did not complete: %#v", job)
	}
	progress, ok := job["progress"].(map[string]any)
	if !ok {
		t.Fatalf("progress missing: %#v", job)
	}
	if progress["processed"] != float64(1) || progress["upserted"] != float64(1) {
		t.Fatalf("progress mismatch: %#v", progress)
	}
	if progress["progress_percent"] != float64(100) {
		t.Fatalf("progress_percent = %v, want 100", progress["progress_percent"])
	}
	result, ok := job["result"].(map[string]any)
	if !ok || result["reindex_executed"] != true {
		t.Fatalf("result mismatch: %#v", job["result"])
	}
}

func TestAdminRescanBackgroundJobReportsFailedTurns(t *testing.T) {
	cfg := config.Default()
	cfg.StoreMode = config.StoreModeMariaDBShadow
	srv := NewServer(cfg)
	srv.Store = &memoryFakeStore{
		chatLogs: []store.ChatLog{
			{ID: 1, ChatSessionID: "sess-rescan-bg", TurnIndex: 1, Role: "user", Content: "turn one user"},
			{ID: 2, ChatSessionID: "sess-rescan-bg", TurnIndex: 1, Role: "assistant", Content: "turn one assistant"},
			{ID: 3, ChatSessionID: "sess-rescan-bg", TurnIndex: 2, Role: "user", Content: "turn two user"},
			{ID: 4, ChatSessionID: "sess-rescan-bg", TurnIndex: 2, Role: "assistant", Content: "turn two assistant"},
		},
	}
	srv.StoreOpenError = nil

	mux := http.NewServeMux()
	srv.RegisterRoutes(mux)

	req := httptest.NewRequest(http.MethodPost, "/admin/rescan", strings.NewReader(`{"chat_session_id":"sess-rescan-bg","max_items":222,"background":true}`))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusAccepted {
		t.Fatalf("status = %d, want 202: %s", rec.Code, rec.Body.String())
	}
	var start map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &start); err != nil {
		t.Fatalf("decode start: %v", err)
	}
	jobID, _ := start["job_id"].(string)
	if jobID == "" {
		t.Fatalf("job_id missing: %#v", start)
	}

	var job map[string]any
	for i := 0; i < 50; i++ {
		rec = httptest.NewRecorder()
		req = httptest.NewRequest(http.MethodGet, "/admin/jobs/"+jobID, nil)
		mux.ServeHTTP(rec, req)
		if rec.Code != http.StatusOK {
			t.Fatalf("job status = %d: %s", rec.Code, rec.Body.String())
		}
		job = map[string]any{}
		if err := json.Unmarshal(rec.Body.Bytes(), &job); err != nil {
			t.Fatalf("decode job: %v", err)
		}
		if job["status"] == "completed" {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	if job["status"] != "completed" {
		t.Fatalf("job did not complete: %#v", job)
	}
	progress, ok := job["progress"].(map[string]any)
	if !ok {
		t.Fatalf("progress missing: %#v", job)
	}
	if progress["candidate_count"] != float64(2) || progress["failed_count"] != float64(2) {
		t.Fatalf("progress mismatch: %#v", progress)
	}
	failedTurns, ok := progress["failed_turns"].([]any)
	if !ok || len(failedTurns) != 2 {
		t.Fatalf("failed_turns mismatch: %#v", progress["failed_turns"])
	}
	result, ok := job["result"].(map[string]any)
	if !ok || result["failed"] != float64(2) {
		t.Fatalf("result mismatch: %#v", job["result"])
	}
}

func TestAdminSessionNormalizeQueuesRedactedBackgroundJob(t *testing.T) {
	cfg := config.Default()
	cfg.StoreMode = config.StoreModeMariaDBShadow
	srv := NewServer(cfg)
	srv.Store = &memoryFakeStore{}
	srv.StoreOpenError = nil

	mux := http.NewServeMux()
	srv.RegisterRoutes(mux)

	body := `{"chat_session_id":"sess-normalize-bg","max_items":25,"repair_entries":[{"turn_index":1,"user_content":"private user text","assistant_content":"private assistant text"}]}`
	req := httptest.NewRequest(http.MethodPost, "/admin/session-normalize", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusAccepted {
		t.Fatalf("status = %d, want 202: %s", rec.Code, rec.Body.String())
	}
	var start map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &start); err != nil {
		t.Fatalf("decode start: %v", err)
	}
	if start["kind"] != "session_normalize" {
		t.Fatalf("kind = %v, want session_normalize", start["kind"])
	}
	if start["job_id"] == "" {
		t.Fatalf("job_id missing: %#v", start)
	}
	request, ok := start["request"].(map[string]any)
	if !ok {
		t.Fatalf("request missing: %#v", start)
	}
	if request["repair_entry_count"] != float64(1) {
		t.Fatalf("repair_entry_count = %v, want 1", request["repair_entry_count"])
	}
	raw, _ := json.Marshal(request)
	if strings.Contains(string(raw), "private user text") || strings.Contains(string(raw), "private assistant text") {
		t.Fatalf("job request leaked raw content: %s", string(raw))
	}
	if request["destructive"] != false {
		t.Fatalf("destructive flag = %v, want false", request["destructive"])
	}
}
