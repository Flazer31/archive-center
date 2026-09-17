package httpapi

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestChromaPreflightValues(t *testing.T) {
	mux := http.NewServeMux()
	srv := setupTestServer()
	srv.Cfg.ChromaShadowPersistDir = t.TempDir()
	srv.RegisterRoutes(mux)

	req := httptest.NewRequest(http.MethodGet, "/chroma-shadow/preflight", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, rec.Code)
	}

	var resp map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to decode: %v", err)
	}

	if resp["step"] != "17-C1" {
		t.Errorf("step = %v, want 17-C1", resp["step"])
	}
	if resp["enabled"] != true {
		t.Errorf("enabled = %v, want true", resp["enabled"])
	}
	if resp["collection_name"] != "archive_center_shadow" {
		t.Errorf("collection_name = %v, want archive_center_shadow", resp["collection_name"])
	}
	issues, ok := resp["issues"].([]any)
	if !ok {
		t.Fatalf("issues is not an array: %T", resp["issues"])
	}
	if len(issues) != 1 {
		t.Errorf("issues length = %d, want 1", len(issues))
	}
	if resp["ready"] != false {
		t.Errorf("ready = %v, want false", resp["ready"])
	}
	sp, ok := resp["session_partitioning"].(map[string]any)
	if !ok {
		t.Fatalf("session_partitioning is not an object: %T", resp["session_partitioning"])
	}
	if sp["mode"] != "session_partitioned" {
		t.Errorf("session_partitioning.mode = %v, want session_partitioned", sp["mode"])
	}
	if sp["session_partitioned"] != true {
		t.Errorf("session_partitioned = %v, want true", sp["session_partitioned"])
	}
	if sp["shadow_runtime_mode"] != "shadow" {
		t.Errorf("shadow_runtime_mode = %v, want shadow", sp["shadow_runtime_mode"])
	}
	if sp["shadow_write_enabled"] != true {
		t.Errorf("shadow_write_enabled = %v, want true", sp["shadow_write_enabled"])
	}
	if sp["active_session_count"] != float64(0) {
		t.Errorf("active_session_count = %v, want 0", sp["active_session_count"])
	}
	pd, ok := resp["persist_directory"].(map[string]any)
	if !ok {
		t.Fatalf("persist_directory is not an object: %T", resp["persist_directory"])
	}
	if pd["path"] != srv.Cfg.ChromaShadowPersistDir {
		t.Errorf("persist_directory.path = %v, want actual path", pd["path"])
	}
	if pd["exists"] != true {
		t.Errorf("persist_directory.exists = %v, want true", pd["exists"])
	}
	if pd["writable"] != true {
		t.Errorf("persist_directory.writable = %v, want true", pd["writable"])
	}
	db, ok := resp["disk_budget"].(map[string]any)
	if !ok {
		t.Fatalf("disk_budget is not an object: %T", resp["disk_budget"])
	}
	if db["budget_mb"] != float64(2048) {
		t.Errorf("disk_budget.budget_mb = %v, want 2048", db["budget_mb"])
	}
	if _, ok := db["free_mb"].(float64); !ok {
		t.Errorf("disk_budget.free_mb = %T, want number", db["free_mb"])
	}
	if _, ok := db["total_mb"].(float64); !ok {
		t.Errorf("disk_budget.total_mb = %T, want number", db["total_mb"])
	}
	if db["target_size_mb"] != 0.16 {
		t.Errorf("disk_budget.target_size_mb = %v, want 0.16", db["target_size_mb"])
	}
	dep, ok := resp["dependency"].(map[string]any)
	if !ok {
		t.Fatalf("dependency is not an object: %T", resp["dependency"])
	}
	if dep["available"] != false {
		t.Errorf("dependency.available = %v, want false", dep["available"])
	}
	if dep["package"] != "chromadb" {
		t.Errorf("dependency.package = %v, want chromadb", dep["package"])
	}
	if dep["detail"] != "ModuleNotFoundError" {
		t.Errorf("dependency.detail = %v, want ModuleNotFoundError", dep["detail"])
	}
	ei, ok := resp["embedder_identity"].(map[string]any)
	if !ok {
		t.Fatalf("embedder_identity is not an object: %T", resp["embedder_identity"])
	}
	if ei["provider"] != "voyageai" {
		t.Errorf("embedder_identity.provider = %v, want voyageai", ei["provider"])
	}
	if ei["model"] != "voyage-4-large" {
		t.Errorf("embedder_identity.model = %v, want voyage-4-large", ei["model"])
	}
	if ei["endpoint"] != "https://api.voyageai.com/v1/embeddings" {
		t.Errorf("embedder_identity.endpoint = %v, want voyage endpoint", ei["endpoint"])
	}
	rds, ok := resp["retrieval_document_schema"].(map[string]any)
	if !ok {
		t.Fatalf("retrieval_document_schema is not an object: %T", resp["retrieval_document_schema"])
	}
	if rds["version"] != "q1a.v1" {
		t.Errorf("retrieval_document_schema.version = %v, want q1a.v1", rds["version"])
	}
	if rds["index_version"] != "q1e.v1" {
		t.Errorf("retrieval_document_schema.index_version = %v, want q1e.v1", rds["index_version"])
	}
}
func TestChromaPreflightKeyShape(t *testing.T) {
	mux := http.NewServeMux()
	srv := setupTestServer()
	srv.Cfg.ChromaShadowPersistDir = t.TempDir()
	srv.RegisterRoutes(mux)

	req := httptest.NewRequest(http.MethodGet, "/chroma-shadow/preflight", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, rec.Code)
	}

	var resp map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to decode: %v", err)
	}

	expected := []string{
		"collection_name", "dependency", "disk_budget", "embedder_identity",
		"enabled", "issues", "persist_directory", "ready",
		"retrieval_document_schema", "session_partitioning", "status", "step",
	}
	for _, k := range expected {
		if _, ok := resp[k]; !ok {
			t.Errorf("missing key %q", k)
		}
	}
	if _, ok := resp["note"]; ok {
		t.Error("unexpected note key")
	}
}

func TestRetrievalIndexSnapshotKeyShape(t *testing.T) {
	mux := http.NewServeMux()
	srv := setupTestServer()
	srv.RegisterRoutes(mux)

	req := httptest.NewRequest(http.MethodGet, "/retrieval-index/sess-123", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, rec.Code)
	}

	var resp map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to decode: %v", err)
	}

	expected := []string{
		"chat_session_id", "dirty", "dirty_reason", "dirty_turn", "discard_turn",
		"document_count", "index_version", "last_dirty_at", "last_discarded_at",
		"last_event", "last_event_reason", "partition_count", "runtime_mode",
		"runtime_reason", "runtime_updated_at", "session_partitioned",
		"shadow_write_enabled", "source_type_counts", "status", "tier_counts", "updated_at",
	}
	for _, k := range expected {
		if _, ok := resp[k]; !ok {
			t.Errorf("missing key %q", k)
		}
	}
}

func TestRetrievalIndexSnapshotValues(t *testing.T) {
	mux := http.NewServeMux()
	srv := setupTestServer()
	srv.RegisterRoutes(mux)

	req := httptest.NewRequest(http.MethodGet, "/retrieval-index/sess-123", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, rec.Code)
	}

	var resp map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to decode: %v", err)
	}

	if resp["chat_session_id"] != "sess-123" {
		t.Errorf("chat_session_id = %v, want sess-123", resp["chat_session_id"])
	}
	if resp["status"] != "empty" {
		t.Errorf("status = %v, want empty", resp["status"])
	}
	if resp["index_version"] != "q1e.v1" {
		t.Errorf("index_version = %v, want q1e.v1", resp["index_version"])
	}
	if resp["session_partitioned"] != true {
		t.Errorf("session_partitioned = %v, want true", resp["session_partitioned"])
	}
	if resp["shadow_write_enabled"] != true {
		t.Errorf("shadow_write_enabled = %v, want true", resp["shadow_write_enabled"])
	}
	if resp["runtime_reason"] != "default" {
		t.Errorf("runtime_reason = %v, want default", resp["runtime_reason"])
	}
	if resp["dirty_turn"] != nil {
		t.Errorf("dirty_turn = %v, want nil", resp["dirty_turn"])
	}
	if resp["discard_turn"] != nil {
		t.Errorf("discard_turn = %v, want nil", resp["discard_turn"])
	}
	tc, ok := resp["tier_counts"].(map[string]any)
	if !ok {
		t.Fatalf("tier_counts is not an object: %T", resp["tier_counts"])
	}
	for _, tier := range []string{"memory", "episode", "chapter", "arc", "saga"} {
		if tc[tier] != float64(0) {
			t.Errorf("tier_counts[%s] = %v, want 0", tier, tc[tier])
		}
	}
}

func TestMetricsLC1CKeyShape(t *testing.T) {
	mux := http.NewServeMux()
	srv := setupTestServer()
	srv.RegisterRoutes(mux)

	req := httptest.NewRequest(http.MethodGet, "/metrics/lc1c/sess-123", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, rec.Code)
	}

	var resp map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to decode: %v", err)
	}

	expected := []string{"chat_session_id", "memory_footprint", "status"}
	for _, k := range expected {
		if _, ok := resp[k]; !ok {
			t.Errorf("missing key %q", k)
		}
	}
	if _, ok := resp["note"]; ok {
		t.Error("unexpected note key")
	}
	// Verify memory_footprint placeholder shape
	mfp, ok := resp["memory_footprint"].(map[string]any)
	if !ok {
		t.Fatalf("memory_footprint is not an object: %T", resp["memory_footprint"])
	}
	mfpExpected := []string{"policy_version", "turn_window", "status", "latest_turn_index", "window_start_turn", "canonical_state_chars", "dense_summary_chars", "live_ledger_chars", "total_chars", "counts"}
	for _, k := range mfpExpected {
		if _, ok := mfp[k]; !ok {
			t.Errorf("memory_footprint missing key %q", k)
		}
	}
	if mfp["policy_version"] != "lc1c.v1" {
		t.Errorf("policy_version = %v, want lc1c.v1", mfp["policy_version"])
	}
	if mfp["status"] != "ok" {
		t.Errorf("status = %v, want ok", mfp["status"])
	}
	counts, ok := mfp["counts"].(map[string]any)
	if !ok {
		t.Fatalf("counts is not an object: %T", mfp["counts"])
	}
	countKeys := []string{"canonical_layers", "episodes", "chapters", "arcs", "sagas"}
	for _, k := range countKeys {
		if _, ok := counts[k]; !ok {
			t.Errorf("counts missing key %q", k)
		}
	}
}

func TestNarrativeControlKeyShape(t *testing.T) {
	mux := http.NewServeMux()
	srv := setupTestServer()
	srv.RegisterRoutes(mux)

	req := httptest.NewRequest(http.MethodGet, "/narrative-control/sess-123", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, rec.Code)
	}

	var resp map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to decode: %v", err)
	}

	expected := []string{
		"chat_session_id", "compact_history", "director", "generated_at",
		"last_advanced_turn", "last_validated_turn", "progression_ledger",
		"skeleton_only", "state_status", "status", "story_guidance", "story_plan", "warnings",
	}
	for _, k := range expected {
		if _, ok := resp[k]; !ok {
			t.Errorf("missing key %q", k)
		}
	}
	if _, ok := resp["note"]; ok {
		t.Error("unexpected note key")
	}
}

func TestSessionStateKeyShape(t *testing.T) {
	mux := http.NewServeMux()
	srv := setupTestServer()
	srv.RegisterRoutes(mux)

	req := httptest.NewRequest(http.MethodGet, "/session-state/sess-123", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, rec.Code)
	}

	var resp map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to decode: %v", err)
	}

	expected := []string{
		"active_states", "canonical_state_layer", "chapter_summaries", "characters",
		"chat_session_id", "generated_at", "pending_threads", "section_meta",
		"snapshot_status", "status", "storylines", "warnings", "world_rules",
	}
	for _, k := range expected {
		if _, ok := resp[k]; !ok {
			t.Errorf("missing key %q", k)
		}
	}
	if _, ok := resp["note"]; ok {
		t.Error("unexpected note key")
	}
}

func TestContinuityPackKeyShape(t *testing.T) {
	mux := http.NewServeMux()
	srv := setupTestServer()
	srv.RegisterRoutes(mux)

	req := httptest.NewRequest(http.MethodGet, "/continuity-pack/sess-123", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, rec.Code)
	}

	var resp map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to decode: %v", err)
	}

	expected := []string{
		"active_storylines", "chat_session_id", "generated_at", "latest_episode",
		"pack_status", "pending_threads", "relationship_shifts", "section_status",
		"skeleton_only", "status", "warnings", "world_constraints",
	}
	for _, k := range expected {
		if _, ok := resp[k]; !ok {
			t.Errorf("missing key %q", k)
		}
	}
	if _, ok := resp["note"]; ok {
		t.Error("unexpected note key")
	}
}

func TestMomentumPacketKeyShape(t *testing.T) {
	mux := http.NewServeMux()
	srv := setupTestServer()
	srv.RegisterRoutes(mux)

	req := httptest.NewRequest(http.MethodGet, "/momentum-packet/sess-123", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, rec.Code)
	}

	var resp map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to decode: %v", err)
	}

	expected := []string{
		"beats_to_avoid", "chat_session_id", "generated_at", "next_pressure",
		"packet_status", "payoff_candidates", "status", "tension_to_reuse", "warnings",
	}
	for _, k := range expected {
		if _, ok := resp[k]; !ok {
			t.Errorf("missing key %q", k)
		}
	}
	if _, ok := resp["note"]; ok {
		t.Error("unexpected note key")
	}
}

func TestLongSessionHealthKeyShape(t *testing.T) {
	mux := http.NewServeMux()
	srv := setupTestServer()
	srv.RegisterRoutes(mux)

	req := httptest.NewRequest(http.MethodGet, "/long-session-health/sess-123", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, rec.Code)
	}

	var resp map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to decode: %v", err)
	}

	expected := []string{"status", "session_id", "chat_session_id", "snapshot", "maintenance_pipeline", "benchmarks", "surface_version", "warnings"}
	for _, k := range expected {
		if _, ok := resp[k]; !ok {
			t.Errorf("missing key %q", k)
		}
	}
	if _, ok := resp["note"]; ok {
		t.Error("unexpected note key")
	}
	if resp["surface_version"] != "r3d.v1" {
		t.Errorf("surface_version = %v, want r3d.v1", resp["surface_version"])
	}
	pipeline, ok := resp["maintenance_pipeline"].(map[string]any)
	if !ok {
		t.Fatalf("maintenance_pipeline is not an object")
	}
	for _, key := range []string{"chapter_summary_generation_enqueue", "stale_arc_refresh", "saga_digest_rebuild", "index_sync_dirty_row_flush", "rollback_stale_summary_discard"} {
		if _, ok := pipeline[key]; !ok {
			t.Errorf("maintenance_pipeline missing key %q", key)
		}
	}
}

func TestFakeID404Routes(t *testing.T) {
	mux := http.NewServeMux()
	srv := setupTestServer()
	srv.RegisterRoutes(mux)

	cases := []string{
		"/sessions/fake-id",
		"/session/fake-id",
		"/explorer/fake-id",
	}

	for _, path := range cases {
		req := httptest.NewRequest(http.MethodGet, path, nil)
		rec := httptest.NewRecorder()
		mux.ServeHTTP(rec, req)

		if rec.Code != http.StatusNotFound {
			t.Errorf("%s: expected status %d, got %d", path, http.StatusNotFound, rec.Code)
			continue
		}

		var resp map[string]any
		if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
			t.Fatalf("%s: failed to decode: %v", path, err)
		}

		if _, ok := resp["detail"]; !ok {
			t.Errorf("%s: missing detail key", path)
		}
		if resp["detail"] != "Not Found" {
			t.Errorf("%s: detail = %v, want Not Found", path, resp["detail"])
		}
		for k := range resp {
			if k != "detail" {
				t.Errorf("%s: unexpected key %q", path, k)
			}
		}
	}
}

func TestSpecificRoutesNotShadowed(t *testing.T) {
	mux := http.NewServeMux()
	srv := setupTestServer()
	srv.RegisterRoutes(mux)

	cases := []struct {
		path   string
		method string
		want   int
	}{
		{"/explorer/chat_logs", http.MethodGet, http.StatusOK},
		{"/explorer/memories", http.MethodGet, http.StatusOK},
		{"/sessions", http.MethodGet, http.StatusOK},
		{"/sessions/sess-123/export", http.MethodGet, http.StatusOK},
		{"/session/sess-123/active-scope", http.MethodGet, http.StatusOK},
	}

	for _, tc := range cases {
		req := httptest.NewRequest(tc.method, tc.path, nil)
		rec := httptest.NewRecorder()
		mux.ServeHTTP(rec, req)

		if rec.Code != tc.want {
			t.Errorf("%s %s: expected status %d, got %d", tc.method, tc.path, tc.want, rec.Code)
		}
	}
}

func TestStorylinesReferenceTurnNil(t *testing.T) {
	mux := http.NewServeMux()
	srv := setupTestServer()
	srv.RegisterRoutes(mux)

	req := httptest.NewRequest(http.MethodGet, "/storylines/sess-123", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, rec.Code)
	}

	var resp map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to decode: %v", err)
	}

	if resp["reference_turn"] != nil {
		t.Errorf("reference_turn = %v, want nil", resp["reference_turn"])
	}
}

func TestPendingThreadsStatusFilter(t *testing.T) {
	mux := http.NewServeMux()
	srv := setupTestServer()
	srv.RegisterRoutes(mux)

	req := httptest.NewRequest(http.MethodGet, "/pending-threads/sess-123", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, rec.Code)
	}

	var resp map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to decode: %v", err)
	}

	if resp["status_filter"] != "open+paused" {
		t.Errorf("status_filter = %v, want open+paused", resp["status_filter"])
	}
}
