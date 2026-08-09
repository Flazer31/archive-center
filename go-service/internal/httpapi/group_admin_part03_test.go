package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/risulongmemory/archive-center-go/internal/config"
	"github.com/risulongmemory/archive-center-go/internal/dto"
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

func TestAdminReindexSkipsAlreadyCurrentIndexWithoutForce(t *testing.T) {
	cfg := config.Default()
	cfg.StoreMode = config.StoreModeMariaDBAuthority
	cfg.ChromaEndpoint = "http://127.0.0.1:8000"
	srv := NewServer(cfg)
	srv.Store = &turnRecordingStore{returnMemories: []store.Memory{{
		ID:             7,
		ChatSessionID:  "sess-reindex-current",
		TurnIndex:      3,
		SummaryJSON:    `{"summary":"The gate is already indexed."}`,
		Embedding:      `[0.1,0.2,0.3]`,
		EmbeddingModel: "test-embedding",
	}}}
	srv.StoreOpenError = nil
	vec := &turnRecordingVectorStore{docs: []vector.VectorDocument{{
		ID:            "memory:sess-reindex-current:7",
		Tier:          "memory",
		ChatSessionID: "sess-reindex-current",
		SourceTable:   "memories",
		SourceRowID:   "7",
	}}}
	srv.Vector = vec

	result, err := srv.runAdminReindexJob(context.Background(), "sess-reindex-current", map[string]any{"force": false}, nil)
	if err != nil {
		t.Fatalf("runAdminReindexJob: %v", err)
	}
	if result["reason"] != "vector_index_already_current" || result["reindex_executed"] != false {
		t.Fatalf("already-current result mismatch: %#v", result)
	}
	if len(vec.docs) != 1 {
		t.Fatalf("already-current reindex must not upsert duplicates: %#v", vec.docs)
	}
}

func TestAdminReindexZeroLimitProcessesWholeCandidateSet(t *testing.T) {
	const candidateCount = 250
	memories := make([]store.Memory, 0, candidateCount)
	for index := 1; index <= candidateCount; index++ {
		memories = append(memories, store.Memory{
			ID:             int64(index),
			ChatSessionID:  "sess-reindex-unlimited",
			TurnIndex:      index,
			SummaryJSON:    fmt.Sprintf(`{"summary":"memory %d"}`, index),
			Embedding:      `[0.1,0.2,0.3]`,
			EmbeddingModel: "test-embedding",
		})
	}
	srv := NewServer(config.Default())
	srv.Store = &turnRecordingStore{returnMemories: memories}
	srv.StoreOpenError = nil

	result, err := srv.runAdminReindexJob(
		context.Background(),
		"sess-reindex-unlimited",
		map[string]any{"max_items": 0, "dry_run": true},
		nil,
	)
	if err != nil {
		t.Fatalf("runAdminReindexJob: %v", err)
	}
	if intFromAny(result["max_items"], -1) != 0 ||
		intFromAny(result["candidates"], 0) != candidateCount {
		t.Fatalf("result=%#v", result)
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

func TestAdminSessionNormalizeDefaultsToResumeExistingArtifacts(t *testing.T) {
	meta := adminSessionNormalizeClientMeta(map[string]any{"source": "explorer_session_normalize"})
	for _, key := range []string{
		"force_derived_rebuild",
		"force_world_rule_backfill",
		"force_raw_world_rule_audit",
		"force_episode_backfill",
		"force_hierarchy_backfill",
	} {
		if boolFromAny(meta[key]) {
			t.Fatalf("%s must be opt-in for resumable session normalization: %#v", key, meta)
		}
	}
	if !boolFromAny(meta["resume_existing_artifacts"]) || !boolFromAny(meta["full_session_backfill"]) {
		t.Fatalf("resume/full-session markers missing: %#v", meta)
	}
	if adminSessionNormalizeForceReindex(adminSessionNormalizeRequest{}) {
		t.Fatal("force_reindex must default to false so completed vectors are not rebuilt on every retry")
	}
}

func TestAdminSessionNormalizePreservesExplicitForceOptions(t *testing.T) {
	meta := adminSessionNormalizeClientMeta(map[string]any{
		"force_derived_rebuild":      true,
		"force_world_rule_backfill":  true,
		"force_raw_world_rule_audit": true,
		"force_episode_backfill":     true,
		"force_hierarchy_backfill":   true,
	})
	for _, key := range []string{
		"force_derived_rebuild",
		"force_world_rule_backfill",
		"force_raw_world_rule_audit",
		"force_episode_backfill",
		"force_hierarchy_backfill",
	} {
		if !boolFromAny(meta[key]) {
			t.Fatalf("explicit %s option was not preserved: %#v", key, meta)
		}
	}
	force := true
	resume := false
	if !adminSessionNormalizeForceReindex(adminSessionNormalizeRequest{ForceReindex: &force, ResumeExisting: &resume}) {
		t.Fatal("explicit force_reindex=true must remain available")
	}
}

func TestAdminSessionNormalizeHasNoHistoricalHostSourceSynthesisContract(t *testing.T) {
	meta := adminSessionNormalizeClientMeta(map[string]any{
		"source": "explorer_session_normalize",
	})
	if _, exists := meta["session_normalize_inline_reprocessing"]; exists {
		t.Fatalf("Session Normalize retained inline Critic control: %#v", meta)
	}
	request := adminSessionNormalizeJobRequest(
		"sess-normalize-canonical-only",
		adminSessionNormalizeRequest{},
		nil,
	)
	if _, exists := request["source_observation_count"]; exists {
		t.Fatalf("Session Normalize retained historical host-source metadata: %#v", request)
	}
}

type canonicalRawReplaySessionNormalizeStore struct {
	*memoryAdmissionWorkerStore
	sources map[string]*store.MemorySourceRevision
}

func (f *canonicalRawReplaySessionNormalizeStore) SaveCriticInputSnapshot(
	_ context.Context,
	_ string,
	revision string,
	snapshotJSON string,
	snapshotHash string,
	_ time.Time,
) error {
	source := f.sources[revision]
	if source == nil {
		return store.ErrNotFound
	}
	source.CriticInputSnapshotJSON = snapshotJSON
	source.CriticInputSnapshotHash = snapshotHash
	return nil
}

func (f *canonicalRawReplaySessionNormalizeStore) RegisterAcceptedSourceRevision(
	_ context.Context,
	source *store.MemorySourceRevision,
) (store.SourceRevisionRegistration, error) {
	if source == nil {
		return store.SourceRevisionRegistration{}, errors.New("source is required")
	}
	if f.sources == nil {
		f.sources = map[string]*store.MemorySourceRevision{}
	}
	if existing := f.sources[source.SourceRevision]; existing != nil {
		return store.SourceRevisionRegistration{Idempotent: true}, nil
	}
	copySource := *source
	f.sources[source.SourceRevision] = &copySource
	return store.SourceRevisionRegistration{Inserted: true}, nil
}

func (f *canonicalRawReplaySessionNormalizeStore) GetSourceRevision(
	_ context.Context,
	_ string,
	sourceRevision string,
) (*store.MemorySourceRevision, error) {
	source := f.sources[sourceRevision]
	if source == nil {
		return nil, store.ErrNotFound
	}
	copySource := *source
	return &copySource, nil
}

func (f *canonicalRawReplaySessionNormalizeStore) IsSourceRevisionActive(
	ctx context.Context,
	sid string,
	sourceRevision string,
) (bool, error) {
	source, err := f.GetSourceRevision(ctx, sid, sourceRevision)
	if err != nil {
		return false, err
	}
	return source.LifecycleState == "active", nil
}

func (f *canonicalRawReplaySessionNormalizeStore) ListActiveSourceRevisions(
	_ context.Context,
	sid string,
	fromTurn int,
	toTurn int,
) ([]store.MemorySourceRevision, error) {
	out := []store.MemorySourceRevision{}
	for _, source := range f.sources {
		if source == nil ||
			source.ChatSessionID != sid ||
			source.LifecycleState != "active" ||
			(fromTurn > 0 && source.TurnIndex < fromTurn) ||
			(toTurn > 0 && source.TurnIndex > toTurn) {
			continue
		}
		out = append(out, *source)
	}
	return out, nil
}

func (f *canonicalRawReplaySessionNormalizeStore) CommitMemoryAdmission(
	ctx context.Context,
	item *store.MemoryAdmission,
) (store.MemoryAdmissionResult, error) {
	result, err := f.memoryAdmissionWorkerStore.CommitMemoryAdmission(ctx, item)
	if err == nil && item != nil && item.Memory != nil {
		f.memories = append(f.memories, *item.Memory)
	}
	return result, err
}

func TestAdminSessionNormalizeReplaysCanonicalRawLogsThroughSharedDerivationOwner(t *testing.T) {
	const sid = "sess-normalize-canonical-raw-replay"
	logs := []store.ChatLog{
		{ChatSessionID: sid, TurnIndex: 1, Role: "user", Content: "The traveler reaches the gate."},
		{ChatSessionID: sid, TurnIndex: 1, Role: "assistant", Content: "The guard refuses entry until dawn."},
	}
	fake := &canonicalRawReplaySessionNormalizeStore{
		memoryAdmissionWorkerStore: &memoryAdmissionWorkerStore{
			Store: store.NewNoopStore(),
			logs:  logs,
			memories: []store.Memory{{
				ChatSessionID: sid,
				TurnIndex:     1,
				SummaryJSON:   `{"turn_summary":"preexisting partial memory"}`,
			}},
		},
		sources: map[string]*store.MemorySourceRevision{},
	}
	cfg := config.Default()
	cfg.StoreMode = config.StoreModeMariaDBAuthority
	srv := NewServer(cfg)
	srv.Store = fake
	srv.StoreOpenError = nil

	oldClient := proxyHTTPClient
	proxyHTTPClient = &http.Client{Transport: roundTripFunc(func(*http.Request) (*http.Response, error) {
		body, _ := json.Marshal(map[string]any{
			"choices": []any{map[string]any{
				"message": map[string]any{
					"content": `{"turn_summary":"The guard refused entry until dawn.","importance_score":6}`,
				},
			}},
		})
		return &http.Response{
			StatusCode: http.StatusOK,
			Header:     make(http.Header),
			Body:       io.NopCloser(strings.NewReader(string(body))),
		}, nil
	})}
	defer func() { proxyHTTPClient = oldClient }()

	request := adminSessionNormalizeRequest{
		SkipRepair:  true,
		SkipReindex: true,
		ClientMeta: map[string]any{
			"critic": map[string]any{
				"api_key":    "test-key",
				"endpoint":   "https://api.example.com/v1",
				"model":      "critic",
				"provider":   "openai",
				"timeout_ms": 45000,
			},
		},
	}
	result, err := srv.runAdminSessionNormalize(context.Background(), sid, request, nil)
	if err != nil {
		t.Fatalf("runAdminSessionNormalize: %v", err)
	}
	rescan := mapFromAny(result["rescan"])
	if result["status"] != "ok" ||
		intFromAny(rescan["candidate_count"], 0) != len(logs)/2 ||
		intFromAny(rescan["succeeded"], 0) != len(logs)/2 ||
		intFromAny(rescan["failed"], -1) != 0 ||
		len(fake.admissions) != len(logs)/2 ||
		len(fake.enqueuedJobs) != 0 {
		t.Fatalf(
			"status=%v rescan=%#v admissions=%d queued=%d",
			result["status"], rescan, len(fake.admissions), len(fake.enqueuedJobs),
		)
	}
	if _, exists := result["source_revisions"]; exists {
		t.Fatalf("removed historical source stage leaked into result: %#v", result)
	}
	if len(fake.sources) != len(logs)/2 {
		t.Fatalf("canonical replay sources=%d, want=%d", len(fake.sources), len(logs)/2)
	}
	for _, source := range fake.sources {
		if !strings.HasPrefix(source.SourceRevision, "sar_") ||
			!strings.HasPrefix(source.LogicalTurnID, "canonical_turn_") {
			t.Fatalf("canonical raw replay synthesized a live host source: %#v", source)
		}
	}
}

func TestAdminSessionNormalizeDefersReindexForQueuedCanonicalReplay(t *testing.T) {
	reasons := adminSessionNormalizeReindexDeferredReasons(map[string]any{
		"status":   "deferred",
		"deferred": 2,
		"queued":   2,
	})
	joined := strings.Join(reasons, ",")
	if !strings.Contains(joined, "rescan:deferred") ||
		!strings.Contains(joined, "rescan:pending_reprocessing") {
		t.Fatalf("queued canonical replay did not defer reindex: %#v", reasons)
	}
	if strings.Contains(joined, "source_revisions") {
		t.Fatalf("removed historical source stage still controls reindex: %#v", reasons)
	}
}

type blockingSessionNormalizeStore struct {
	*memoryFakeStore
	entered chan struct{}
}

func (f *blockingSessionNormalizeStore) ListChatLogs(ctx context.Context, sid string, from, to int) ([]store.ChatLog, error) {
	if from == 1 && to == 1 {
		close(f.entered)
		<-ctx.Done()
		return nil, ctx.Err()
	}
	return f.memoryFakeStore.ListChatLogs(ctx, sid, from, to)
}

func TestAdminSessionNormalizeRawRepairReportsRealCandidateCountAndCompletes(t *testing.T) {
	userText := "The traveler reaches the old gate."
	assistantText := "The guard refuses entry until dawn."
	srv := NewServer(config.Default())
	srv.Store = &memoryFakeStore{}
	srv.StoreOpenError = nil

	updates := []map[string]any{}
	result, err := srv.runAdminSessionNormalize(context.Background(), "sess-normalize-complete", adminSessionNormalizeRequest{
		RepairEntries: []dto.ChatLogRepairEntryRequest{{
			TurnIndex:        1,
			UserContent:      &userText,
			AssistantContent: &assistantText,
		}},
		SkipRescan:  true,
		SkipReindex: true,
	}, func(progress map[string]any) {
		updates = append(updates, cloneMapAny(progress))
	})
	if err != nil {
		t.Fatalf("runAdminSessionNormalize: %v", err)
	}
	if result["status"] != "ok" {
		t.Fatalf("result = %#v, want ok", result)
	}
	sawRawRepair := false
	for _, update := range updates {
		if update["stage"] != "raw_repair_replay" {
			continue
		}
		sawRawRepair = true
		if got := intFromAny(update["candidate_count"], 0); got != 1 {
			t.Fatalf("raw repair candidate_count = %d, want 1: %#v", got, update)
		}
	}
	if !sawRawRepair {
		t.Fatalf("raw_repair_replay progress missing: %#v", updates)
	}
}

func TestAdminSessionNormalizeCancellationReachesBlockedRawChatQuery(t *testing.T) {
	userText := "The traveler reaches the old gate."
	assistantText := "The guard refuses entry until dawn."
	fake := &blockingSessionNormalizeStore{
		memoryFakeStore: &memoryFakeStore{},
		entered:         make(chan struct{}),
	}
	srv := NewServer(config.Default())
	srv.Store = fake
	srv.StoreOpenError = nil

	ctx, cancel := context.WithCancel(context.Background())
	resultCh := make(chan error, 1)
	go func() {
		_, err := srv.runAdminSessionNormalize(ctx, "sess-normalize-cancel", adminSessionNormalizeRequest{
			RepairEntries: []dto.ChatLogRepairEntryRequest{{
				TurnIndex:        1,
				UserContent:      &userText,
				AssistantContent: &assistantText,
			}},
			SkipRescan:  true,
			SkipReindex: true,
		}, nil)
		resultCh <- err
	}()

	select {
	case <-fake.entered:
	case <-time.After(2 * time.Second):
		t.Fatal("session normalize did not reach raw chat query")
	}
	cancel()
	select {
	case err := <-resultCh:
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("runAdminSessionNormalize error = %v, want context canceled", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("session normalize did not stop after caller cancellation")
	}
}
