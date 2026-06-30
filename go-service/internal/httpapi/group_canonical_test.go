package httpapi

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/risulongmemory/archive-center-go/internal/config"
	"github.com/risulongmemory/archive-center-go/internal/store"
)

// TestSeq13P86EmbeddingProvenanceInExport verifies EM-1a: embedding model provenance
// is exposed in session export, including current project model and per-memory status counts.
func TestSeq13P86EmbeddingProvenanceInExport(t *testing.T) {
	fake := &canonicalFakeStore{
		memories: []store.Memory{
			{ID: 1, ChatSessionID: "sess-p86", TurnIndex: 1, Embedding: "[0.1]", EmbeddingModel: "text-embedding-3-small"},
			{ID: 2, ChatSessionID: "sess-p86", TurnIndex: 2, Embedding: "", EmbeddingModel: ""},
			{ID: 3, ChatSessionID: "sess-p86", TurnIndex: 3, Embedding: "[0.3]", EmbeddingModel: "old-model"},
			{ID: 4, ChatSessionID: "sess-p86", TurnIndex: 4, Embedding: "", EmbeddingModel: "text-embedding-3-small"},
			{ID: 5, ChatSessionID: "sess-p86", TurnIndex: 5, Embedding: "[0.5]", EmbeddingModel: ""},
		},
	}
	mux := http.NewServeMux()
	srv := setupTestServer()
	srv.RuntimeConfig.EmbeddingModel = "text-embedding-3-small"
	srv.Store = fake
	srv.RegisterRoutes(mux)

	req := httptest.NewRequest(http.MethodGet, "/sessions/sess-p86/export", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200: %s", rec.Code, rec.Body.String())
	}
	var resp map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}

	prov, ok := resp["embedding_provenance"].(map[string]any)
	if !ok {
		t.Fatalf("embedding_provenance missing or not object: %T", resp["embedding_provenance"])
	}
	if prov["policy_version"] != "em1a.v1" {
		t.Errorf("policy_version = %v, want em1a.v1", prov["policy_version"])
	}
	if prov["needs_reembed_policy_version"] != "em1b.v1" {
		t.Errorf("needs_reembed_policy_version = %v, want em1b.v1", prov["needs_reembed_policy_version"])
	}
	cm, ok := prov["current_project_embedding_model"].(string)
	if !ok || cm == "" {
		t.Errorf("current_project_embedding_model missing or empty: %v", prov["current_project_embedding_model"])
	}

	counts, ok := prov["memory_status_counts"].(map[string]any)
	if !ok {
		t.Fatalf("memory_status_counts missing or not object: %T", prov["memory_status_counts"])
	}
	// With current model "text-embedding-3-small":
	// memory 1 => current_model_match, memory 2 => missing_embedding_and_model,
	// memory 3 => model_mismatch, memory 4 => missing_embedding_vector,
	// memory 5 => missing_embedding_model.
	if counts["current_model_match"] == nil || counts["current_model_match"].(float64) != 1 {
		t.Errorf("current_model_match count = %v, want 1", counts["current_model_match"])
	}
	if counts["missing_embedding_and_model"] == nil || counts["missing_embedding_and_model"].(float64) != 1 {
		t.Errorf("missing_embedding_and_model count = %v, want 1", counts["missing_embedding_and_model"])
	}
	if counts["missing_embedding_vector"] == nil || counts["missing_embedding_vector"].(float64) != 1 {
		t.Errorf("missing_embedding_vector count = %v, want 1", counts["missing_embedding_vector"])
	}
	if counts["missing_embedding_model"] == nil || counts["missing_embedding_model"].(float64) != 1 {
		t.Errorf("missing_embedding_model count = %v, want 1", counts["missing_embedding_model"])
	}
	if counts["model_mismatch"] == nil || counts["model_mismatch"].(float64) != 1 {
		t.Errorf("model_mismatch count = %v, want 1", counts["model_mismatch"])
	}
	if prov["needs_reembed_count"] == nil || prov["needs_reembed_count"].(float64) != 4 {
		t.Errorf("needs_reembed_count = %v, want 4", prov["needs_reembed_count"])
	}
}

// TestSeq13P89NeedsReembedLifecyclePolicy verifies EM-1b: needs_reembed lifecycle
// states (missing_embedding, model_mismatch, current_model_match) are classified correctly.
func TestSeq13P89NeedsReembedLifecyclePolicy(t *testing.T) {
	tests := []struct {
		name      string
		embedding string
		model     string
		current   any
		want      string
	}{
		{"missing both", "", "", "text-embedding-3-small", "missing_embedding_and_model"},
		{"missing vector", "", "text-embedding-3-small", "text-embedding-3-small", "missing_embedding_vector"},
		{"missing model", "[0.1]", "", "text-embedding-3-small", "missing_embedding_model"},
		{"current match", "[0.1]", "text-embedding-3-small", "text-embedding-3-small", "current_model_match"},
		{"model mismatch", "[0.1]", "old-model", "text-embedding-3-small", "model_mismatch"},
		{"project unset with model", "[0.1]", "some-model", "", "project_model_unset"},
		{"project unset missing", "", "", "", "project_model_unset"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := store.Memory{Embedding: tt.embedding, EmbeddingModel: tt.model}
			got := classifyMemoryEmbeddingStatus(m, tt.current)
			if got != tt.want {
				t.Errorf("classifyMemoryEmbeddingStatus() = %q, want %q", got, tt.want)
			}
		})
	}
}

// TestSeq13P92ModelMismatchRetrievalFallback verifies EM-1c: search results include
// embedding_provenance so callers can detect model mismatch, and the search path remains
// Store-backed (canonical truth) with vector as accelerator only.
func TestSeq13P92ModelMismatchRetrievalFallback(t *testing.T) {
	fake := &canonicalFakeStore{
		memories: []store.Memory{
			{ID: 1, ChatSessionID: "sess-p92", TurnIndex: 1, SummaryJSON: `{"summary":"alpha"}`, Embedding: "[0.1]", EmbeddingModel: "text-embedding-3-small", Importance: 0.8},
			{ID: 2, ChatSessionID: "sess-p92", TurnIndex: 2, SummaryJSON: `{"summary":"beta"}`, Embedding: "[0.2]", EmbeddingModel: "old-model", Importance: 0.7},
			{ID: 3, ChatSessionID: "sess-p92", TurnIndex: 3, SummaryJSON: `{"summary":"gamma"}`, Embedding: "", EmbeddingModel: "text-embedding-3-small", Importance: 0.9},
		},
	}
	mux := http.NewServeMux()
	srv := setupTestServer()
	srv.RuntimeConfig.EmbeddingModel = "text-embedding-3-small"
	srv.Store = fake
	srv.RegisterRoutes(mux)

	body := `{"chat_session_id":"sess-p92","user_input":"alpha beta gamma","top_k":5}`
	req := httptest.NewRequest(http.MethodPost, "/search", strings.NewReader(body))
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
	items := resp["items"].([]any)
	if len(items) == 0 {
		t.Fatalf("search returned no items")
	}
	byID := map[int64]map[string]any{}
	for _, it := range items {
		item := it.(map[string]any)
		byID[int64(item["id"].(float64))] = item
		prov, ok := item["embedding_provenance"].(map[string]any)
		if !ok {
			t.Fatalf("item missing embedding_provenance: %#v", item)
		}
		if _, ok := prov["embedding_model"]; !ok {
			t.Errorf("embedding_provenance missing embedding_model")
		}
		if _, ok := prov["has_embedding"]; !ok {
			t.Errorf("embedding_provenance missing has_embedding")
		}
		if prov["truth_authority"] != "store_canonical" {
			t.Errorf("truth_authority = %v, want store_canonical", prov["truth_authority"])
		}
		if prov["vector_role"] != "accelerator_only" {
			t.Errorf("vector_role = %v, want accelerator_only", prov["vector_role"])
		}
	}
	assertProv := func(id int64, wantStatus, wantFallback string, wantNeedsReembed bool) {
		t.Helper()
		item, ok := byID[id]
		if !ok {
			t.Fatalf("missing search item id %d in %#v", id, byID)
		}
		prov := item["embedding_provenance"].(map[string]any)
		if prov["status"] != wantStatus {
			t.Errorf("id %d status = %v, want %s", id, prov["status"], wantStatus)
		}
		if prov["retrieval_fallback"] != wantFallback {
			t.Errorf("id %d retrieval_fallback = %v, want %s", id, prov["retrieval_fallback"], wantFallback)
		}
		if prov["needs_reembed"] != wantNeedsReembed {
			t.Errorf("id %d needs_reembed = %v, want %v", id, prov["needs_reembed"], wantNeedsReembed)
		}
	}
	assertProv(1, "current_model_match", "embedding_current", false)
	assertProv(2, "model_mismatch", "hybrid_degrade", true)
	assertProv(3, "missing_embedding_vector", "importance_only", true)
	// Assert Store-backed canonical truth: source must be "memory" (not vector)
	first := items[0].(map[string]any)
	if first["source"] != "memory" {
		t.Errorf("search source = %v, want memory (Store canonical truth)", first["source"])
	}
}

func TestSearchEmbeddingProvenanceUsesConfiguredLargeModel(t *testing.T) {
	fake := &canonicalFakeStore{
		memories: []store.Memory{
			{ID: 1, ChatSessionID: "sess-large", TurnIndex: 1, SummaryJSON: `{"summary":"large vector memory"}`, Embedding: "[0.1]", EmbeddingModel: "text-embedding-3-large", Importance: 0.8},
		},
	}
	cfg := config.Default()
	cfg.EmbedderModel = "text-embedding-3-large"
	srv := NewServer(cfg)
	srv.Store = fake

	mux := http.NewServeMux()
	srv.RegisterRoutes(mux)

	body := `{"chat_session_id":"sess-large","user_input":"large vector memory","top_k":5}`
	req := httptest.NewRequest(http.MethodPost, "/search", strings.NewReader(body))
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
	items := resp["items"].([]any)
	if len(items) == 0 {
		t.Fatalf("search returned no items")
	}
	prov := items[0].(map[string]any)["embedding_provenance"].(map[string]any)
	if prov["current_model"] != "text-embedding-3-large" {
		t.Fatalf("current_model = %v, want text-embedding-3-large", prov["current_model"])
	}
	if prov["current_model_source"] != "config.AC_EMBEDDER_MODEL" {
		t.Fatalf("current_model_source = %v, want config.AC_EMBEDDER_MODEL", prov["current_model_source"])
	}
	if prov["status"] != "current_model_match" {
		t.Fatalf("status = %v, want current_model_match", prov["status"])
	}
	if prov["retrieval_fallback"] != "embedding_current" {
		t.Fatalf("retrieval_fallback = %v, want embedding_current", prov["retrieval_fallback"])
	}
}

func TestSearchEmbeddingProvenanceUsesConfigUpdateRuntimeLargeModel(t *testing.T) {
	fake := &canonicalFakeStore{
		memories: []store.Memory{
			{ID: 1, ChatSessionID: "sess-ui-large", TurnIndex: 1, SummaryJSON: `{"summary":"large vector memory"}`, Embedding: "[0.1]", EmbeddingModel: "text-embedding-3-large", Importance: 0.8},
		},
	}
	cfg := config.Default()
	cfg.EmbedderModel = "text-embedding-3-small"
	srv := NewServer(cfg)
	srv.Store = fake

	mux := http.NewServeMux()
	srv.RegisterRoutes(mux)

	updateReq := httptest.NewRequest(http.MethodPost, "/config/update", strings.NewReader(`{
		"embeddingProvider":"openai",
		"embeddingApiKey":"ui-key",
		"embeddingEndpoint":"https://example.test/v1/embeddings",
		"embeddingModel":"text-embedding-3-large"
	}`))
	updateReq.Header.Set("Content-Type", "application/json")
	updateRec := httptest.NewRecorder()
	mux.ServeHTTP(updateRec, updateReq)
	if updateRec.Code != http.StatusOK {
		t.Fatalf("config update status = %d, want 200: %s", updateRec.Code, updateRec.Body.String())
	}

	searchReq := httptest.NewRequest(http.MethodPost, "/search", strings.NewReader(`{"chat_session_id":"sess-ui-large","user_input":"large vector memory","top_k":5}`))
	searchReq.Header.Set("Content-Type", "application/json")
	searchRec := httptest.NewRecorder()
	mux.ServeHTTP(searchRec, searchReq)
	if searchRec.Code != http.StatusOK {
		t.Fatalf("search status = %d, want 200: %s", searchRec.Code, searchRec.Body.String())
	}
	var resp map[string]any
	if err := json.Unmarshal(searchRec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	items := resp["items"].([]any)
	if len(items) == 0 {
		t.Fatalf("search returned no items")
	}
	prov := items[0].(map[string]any)["embedding_provenance"].(map[string]any)
	if prov["current_model"] != "text-embedding-3-large" {
		t.Fatalf("current_model = %v, want text-embedding-3-large", prov["current_model"])
	}
	if prov["current_model_source"] != "runtime.embeddingModel" {
		t.Fatalf("current_model_source = %v, want runtime.embeddingModel", prov["current_model_source"])
	}
	if prov["status"] != "current_model_match" {
		t.Fatalf("status = %v, want current_model_match", prov["status"])
	}
	if prov["retrieval_fallback"] != "embedding_current" {
		t.Fatalf("retrieval_fallback = %v, want embedding_current", prov["retrieval_fallback"])
	}
}

func TestSessionExportEmbeddingProvenanceUsesRuntimeModel(t *testing.T) {
	fake := &canonicalFakeStore{
		memories: []store.Memory{
			{ID: 1, ChatSessionID: "sess-runtime-large", TurnIndex: 1, Embedding: "[0.1]", EmbeddingModel: "text-embedding-3-large"},
		},
	}
	srv := setupTestServer()
	srv.RuntimeConfig.EmbeddingModel = "text-embedding-3-large"
	srv.Store = fake

	mux := http.NewServeMux()
	srv.RegisterRoutes(mux)

	req := httptest.NewRequest(http.MethodGet, "/sessions/sess-runtime-large/export", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200: %s", rec.Code, rec.Body.String())
	}
	var resp map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	prov := resp["embedding_provenance"].(map[string]any)
	if prov["current_project_embedding_model"] != "text-embedding-3-large" {
		t.Fatalf("current_project_embedding_model = %v, want text-embedding-3-large", prov["current_project_embedding_model"])
	}
	if prov["current_embedding_model_source"] != "runtime.embeddingModel" {
		t.Fatalf("current_embedding_model_source = %v, want runtime.embeddingModel", prov["current_embedding_model_source"])
	}
	counts := prov["memory_status_counts"].(map[string]any)
	if counts["current_model_match"] != float64(1) {
		t.Fatalf("current_model_match count = %v, want 1", counts["current_model_match"])
	}
	if counts["model_mismatch"] != nil {
		t.Fatalf("model_mismatch count = %v, want nil", counts["model_mismatch"])
	}
}

func TestEmbeddingModelIdentityAcceptsLegacyLiteAlias(t *testing.T) {
	t.Setenv("AC_EMBEDDER_MODEL", "")
	t.Setenv("AC_LT_EMBEDDING_MODEL", "text-embedding-3-large")
	srv := setupTestServer()

	identity := srv.currentEmbeddingModelIdentity()
	if identity.Model != "text-embedding-3-large" {
		t.Fatalf("model = %q, want text-embedding-3-large", identity.Model)
	}
	if identity.Source != "env.AC_LT_EMBEDDING_MODEL" {
		t.Fatalf("source = %q, want env.AC_LT_EMBEDDING_MODEL", identity.Source)
	}
}

func TestEmbeddingModelIdentitySyncedEmptyDoesNotFallback(t *testing.T) {
	t.Setenv("AC_LT_EMBEDDING_MODEL", "text-embedding-3-large")
	cfg := config.Default()
	cfg.EmbedderModel = "text-embedding-3-small"
	srv := NewServer(cfg)
	srv.RuntimeConfig.Synced = true

	identity := srv.currentEmbeddingModelIdentity()
	if identity.Model != "" {
		t.Fatalf("model = %q, want empty after synced runtime setting is empty", identity.Model)
	}
	if identity.Source != "unset.runtime.embeddingModel" {
		t.Fatalf("source = %q, want unset.runtime.embeddingModel", identity.Source)
	}
}

func TestCompleteTurnEmbeddingConfigSyncedMissingProviderDoesNotFallback(t *testing.T) {
	t.Setenv("AC_LT_EMBEDDING_PROVIDER", "openai")
	cfg := config.Default()
	cfg.EmbedderProvider = "openai"
	srv := NewServer(cfg)
	srv.RuntimeConfig.Synced = true
	srv.RuntimeConfig.EmbeddingAPIKey = "runtime-key"
	srv.RuntimeConfig.EmbeddingEndpoint = "https://example.test/v1/embeddings"
	srv.RuntimeConfig.EmbeddingModel = "text-embedding-3-large"

	extractionCfg := srv.completeTurnExtractionConfig(map[string]any{})
	if extractionCfg.Embedder.Provider != "" {
		t.Fatalf("provider = %q, want empty after synced runtime setting is empty", extractionCfg.Embedder.Provider)
	}
	if extractionCfg.Embedder.hasConfig() {
		t.Fatalf("embedder hasConfig=true, want false when runtime provider is empty")
	}
	missing := extractionCfg.Embedder.missingFields()
	foundProvider := false
	for _, field := range missing {
		if field == "provider" {
			foundProvider = true
		}
	}
	if !foundProvider {
		t.Fatalf("missing fields = %v, want provider", missing)
	}
}

func TestCompleteTurnEmbeddingConfigUsesLegacyLiteAliases(t *testing.T) {
	t.Setenv("AC_EMBEDDER_PROVIDER", "")
	t.Setenv("AC_EMBEDDER_API_KEY", "")
	t.Setenv("AC_EMBEDDER_ENDPOINT", "")
	t.Setenv("AC_EMBEDDER_MODEL", "")
	t.Setenv("AC_LT_EMBEDDING_PROVIDER", "openai")
	t.Setenv("AC_LT_EMBEDDING_API_KEY", "lite-key")
	t.Setenv("AC_LT_EMBEDDING_ENDPOINT", "https://example.test/v1/embeddings")
	t.Setenv("AC_LT_EMBEDDING_MODEL", "text-embedding-3-large")

	srv := setupTestServer()
	cfg := srv.completeTurnExtractionConfig(map[string]any{})

	if cfg.Embedder.Provider != "openai" {
		t.Fatalf("provider = %q, want openai", cfg.Embedder.Provider)
	}
	if cfg.Embedder.APIKey != "lite-key" {
		t.Fatalf("api key = %q, want lite-key", cfg.Embedder.APIKey)
	}
	if cfg.Embedder.Endpoint != "https://example.test/v1/embeddings" {
		t.Fatalf("endpoint = %q, want https://example.test/v1/embeddings", cfg.Embedder.Endpoint)
	}
	if cfg.Embedder.Model != "text-embedding-3-large" {
		t.Fatalf("model = %q, want text-embedding-3-large", cfg.Embedder.Model)
	}
}

// TestSeq13P96ReembedScheduleDryRun verifies EM-1d: session-level reembed schedule
// returns a dry-run contract with candidate rows, without executing live reembed.
func TestSeq13P96ReembedScheduleDryRun(t *testing.T) {
	fake := &canonicalFakeStore{
		memories: []store.Memory{
			{ID: 1, ChatSessionID: "sess-p96", TurnIndex: 1, Embedding: "[0.1]", EmbeddingModel: "text-embedding-3-small"},
			{ID: 2, ChatSessionID: "sess-p96", TurnIndex: 2, Embedding: "", EmbeddingModel: ""},
			{ID: 3, ChatSessionID: "sess-p96", TurnIndex: 3, Embedding: "[0.3]", EmbeddingModel: "old-model"},
		},
	}
	mux := http.NewServeMux()
	srv := setupTestServer()
	srv.RuntimeConfig.EmbeddingModel = "text-embedding-3-small"
	srv.Store = fake
	srv.RegisterRoutes(mux)

	body := `{"chat_session_id":"sess-p96"}`
	req := httptest.NewRequest(http.MethodPost, "/chroma-shadow/reembed-schedule", strings.NewReader(body))
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
	ev, ok := resp["evidence"].(map[string]any)
	if !ok {
		t.Fatalf("evidence missing")
	}
	if ev["live_execution_allowed"] != false {
		t.Errorf("live_execution_allowed = %v, want false", ev["live_execution_allowed"])
	}
	if ev["truth_authority"] != "store_canonical" {
		t.Errorf("truth_authority = %v, want store_canonical", ev["truth_authority"])
	}
	if ev["vector_role"] != "accelerator_only" {
		t.Errorf("vector_role = %v, want accelerator_only", ev["vector_role"])
	}
	if ev["candidate_count"] == nil || ev["candidate_count"].(float64) != 2 {
		t.Errorf("candidate_count = %v, want 2", ev["candidate_count"])
	}
	schedule, ok := ev["schedule"].([]any)
	if !ok || len(schedule) != 2 {
		t.Fatalf("schedule length = %v, want 2", len(schedule))
	}
	for _, row := range schedule {
		r := row.(map[string]any)
		status, ok := r["status"].(string)
		if !ok || status == "" {
			t.Fatalf("schedule status missing: %#v", r)
		}
		if r["needs_reembed"] != true {
			t.Errorf("schedule needs_reembed = %v, want true", r["needs_reembed"])
		}
		switch status {
		case "missing_embedding_and_model":
			if r["retrieval_fallback"] != "importance_only" {
				t.Errorf("missing row fallback = %v, want importance_only", r["retrieval_fallback"])
			}
		case "model_mismatch":
			if r["retrieval_fallback"] != "hybrid_degrade" {
				t.Errorf("mismatch row fallback = %v, want hybrid_degrade", r["retrieval_fallback"])
			}
		default:
			t.Errorf("unexpected schedule status = %v", status)
		}
		if r["action"] != "dry_run_reembed" {
			t.Errorf("schedule action = %v, want dry_run_reembed", r["action"])
		}
		if r["truth_authority"] != "store_canonical" {
			t.Errorf("schedule truth_authority = %v, want store_canonical", r["truth_authority"])
		}
	}
	trace, ok := resp["trace_summary"].(map[string]any)
	if !ok || trace["policy_version"] != "em1d.v1" {
		t.Errorf("trace_summary policy_version = %v, want em1d.v1", trace["policy_version"])
	}
}

// TestSeq13P100ModelSwitchReembedAudit verifies EM-1e: model-switch reembed replay audit
// contract guards stale vectors from becoming truth authority.
func TestSeq13P100ModelSwitchReembedAudit(t *testing.T) {
	fake := &canonicalFakeStore{
		memories: []store.Memory{
			{ID: 1, ChatSessionID: "sess-p100", TurnIndex: 1, Embedding: "[0.1]", EmbeddingModel: "text-embedding-3-small"},
			{ID: 2, ChatSessionID: "sess-p100", TurnIndex: 2, Embedding: "", EmbeddingModel: ""},
			{ID: 3, ChatSessionID: "sess-p100", TurnIndex: 3, Embedding: "[0.3]", EmbeddingModel: "stale-model"},
		},
	}
	mux := http.NewServeMux()
	srv := setupTestServer()
	srv.RuntimeConfig.EmbeddingModel = "text-embedding-3-small"
	srv.Store = fake
	srv.RegisterRoutes(mux)

	// Use reembed-audit endpoint to verify stale rows are surfaced but not promoted.
	body := `{"chat_session_id":"sess-p100"}`
	req := httptest.NewRequest(http.MethodPost, "/chroma-shadow/reembed-audit", strings.NewReader(body))
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
	ev, ok := resp["evidence"].(map[string]any)
	if !ok {
		t.Fatalf("evidence missing")
	}
	// memory_embedding_models must surface the stale model so operators can detect switch need.
	models, ok := ev["memory_embedding_models"].(map[string]any)
	if !ok {
		t.Fatalf("memory_embedding_models missing")
	}
	if models["stale-model"] == nil || models["stale-model"].(float64) != 1 {
		t.Errorf("stale-model count = %v, want 1", models["stale-model"])
	}
	if models["none"] == nil || models["none"].(float64) != 1 {
		t.Errorf("none count = %v, want 1", models["none"])
	}
	if models["text-embedding-3-small"] == nil || models["text-embedding-3-small"].(float64) != 1 {
		t.Errorf("current model count = %v, want 1", models["text-embedding-3-small"])
	}
	statusCounts, ok := ev["memory_status_counts"].(map[string]any)
	if !ok {
		t.Fatalf("memory_status_counts missing")
	}
	if statusCounts["current_model_match"] == nil || statusCounts["current_model_match"].(float64) != 1 {
		t.Errorf("current_model_match count = %v, want 1", statusCounts["current_model_match"])
	}
	if statusCounts["missing_embedding_and_model"] == nil || statusCounts["missing_embedding_and_model"].(float64) != 1 {
		t.Errorf("missing_embedding_and_model count = %v, want 1", statusCounts["missing_embedding_and_model"])
	}
	if statusCounts["model_mismatch"] == nil || statusCounts["model_mismatch"].(float64) != 1 {
		t.Errorf("model_mismatch count = %v, want 1", statusCounts["model_mismatch"])
	}
	if ev["needs_reembed_count"] == nil || ev["needs_reembed_count"].(float64) != 2 {
		t.Errorf("needs_reembed_count = %v, want 2", ev["needs_reembed_count"])
	}
	if ev["model_switch_replay_policy_version"] != "em1e.v1" {
		t.Errorf("model_switch_replay_policy_version = %v, want em1e.v1", ev["model_switch_replay_policy_version"])
	}
	if ev["retrieval_fallback_before_reembed"] != "hybrid_degrade_or_importance_only" {
		t.Errorf("retrieval_fallback_before_reembed = %v", ev["retrieval_fallback_before_reembed"])
	}
	if ev["retrieval_state_after_reembed"] != "embedding_current" {
		t.Errorf("retrieval_state_after_reembed = %v, want embedding_current", ev["retrieval_state_after_reembed"])
	}
	if ev["truth_authority"] != "store_canonical" {
		t.Errorf("truth_authority = %v, want store_canonical", ev["truth_authority"])
	}
	if ev["vector_role"] != "accelerator_only" {
		t.Errorf("vector_role = %v, want accelerator_only", ev["vector_role"])
	}
	trace, ok := resp["trace_summary"].(map[string]any)
	if !ok || trace["policy_version"] != "em1e.v1" {
		t.Errorf("trace_summary policy_version = %v, want em1e.v1", trace["policy_version"])
	}
	// Verify the note explicitly states Store/Vector-backed R1 evidence (truth authority guard).
	if !strings.Contains(resp["note"].(string), "Store/Vector-backed") {
		t.Errorf("note missing Store/Vector-backed truth guard: %v", resp["note"])
	}
}

func TestCanonicalListMemoriesDefaultNoopRouteRegistered(t *testing.T) {
	mux := http.NewServeMux()
	srv := setupTestServer()
	srv.RegisterRoutes(mux)

	req := httptest.NewRequest(http.MethodGet, "/canonical/sess-1/memories", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200: %s", rec.Code, rec.Body.String())
	}
	var resp map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if resp["source"] != "shadow" {
		t.Errorf("source = %v, want shadow", resp["source"])
	}
	if resp["count"] != float64(0) {
		t.Errorf("count = %v, want 0", resp["count"])
	}
}

func TestCanonicalListMemoriesUsesStore(t *testing.T) {
	fake := &canonicalFakeStore{
		memories: []store.Memory{{ID: 7, ChatSessionID: "sess-1", TurnIndex: 3, SummaryJSON: `{"summary":"ok"}`}},
	}
	mux := http.NewServeMux()
	srv := setupTestServer()
	srv.Store = fake
	srv.RegisterRoutes(mux)

	req := httptest.NewRequest(http.MethodGet, "/canonical/sess-1/memories?from_turn=2&to_turn=5", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200: %s", rec.Code, rec.Body.String())
	}
	if fake.listMemoriesSession != "sess-1" || fake.listMemoriesFrom != 2 || fake.listMemoriesTo != 5 {
		t.Fatalf("store args not captured: %+v", fake)
	}
	var resp map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if resp["count"] != float64(1) {
		t.Errorf("count = %v, want 1", resp["count"])
	}
}

func TestCanonicalReadCoreFamiliesUseStore(t *testing.T) {
	fake := &canonicalFakeStore{
		chatLogs:       []store.ChatLog{{ID: 1, ChatSessionID: "sess-1", TurnIndex: 4, Role: "user", Content: "hello"}},
		effectiveInput: &store.EffectiveInput{ID: 2, ChatSessionID: "sess-1", TurnIndex: 4, EffectiveInput: "hello refined"},
		feedbackItems:  []store.CriticFeedback{{ID: 3, ChatSessionID: "sess-1", TargetType: "turn", TargetID: 4, FeedbackValue: "keep"}},
	}
	mux := http.NewServeMux()
	srv := setupTestServer()
	srv.Store = fake
	srv.RegisterRoutes(mux)

	tests := []struct {
		path      string
		wantCode  int
		wantCount any
	}{
		{path: "/canonical/sess-1/chat-logs?from_turn=3&to_turn=5", wantCode: http.StatusOK, wantCount: float64(1)},
		{path: "/canonical/sess-1/effective-inputs?turn_index=4", wantCode: http.StatusOK, wantCount: nil},
		{path: "/canonical/sess-1/critic-feedback?target_type=turn&target_id=4", wantCode: http.StatusOK, wantCount: float64(1)},
	}

	for _, tt := range tests {
		t.Run(tt.path, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, tt.path, nil)
			rec := httptest.NewRecorder()
			mux.ServeHTTP(rec, req)

			if rec.Code != tt.wantCode {
				t.Fatalf("status = %d, want %d: %s", rec.Code, tt.wantCode, rec.Body.String())
			}
			var resp map[string]any
			if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
				t.Fatalf("decode: %v", err)
			}
			if resp["source"] != "shadow" {
				t.Errorf("source = %v, want shadow", resp["source"])
			}
			if tt.wantCount != nil && resp["count"] != tt.wantCount {
				t.Errorf("count = %v, want %v", resp["count"], tt.wantCount)
			}
		})
	}

	if fake.listChatLogsSession != "sess-1" || fake.listChatLogsFrom != 3 || fake.listChatLogsTo != 5 {
		t.Fatalf("chat log args not captured: %+v", fake)
	}
	if fake.getEffectiveInputSession != "sess-1" || fake.getEffectiveInputTurn != 4 {
		t.Fatalf("effective input args not captured: %+v", fake)
	}
	if fake.listFeedbackSession != "sess-1" || fake.listFeedbackTargetType != "turn" || fake.listFeedbackTargetID != 4 {
		t.Fatalf("critic feedback args not captured: %+v", fake)
	}
}

func TestSessionExportUsesCanonicalStore(t *testing.T) {
	fake := &canonicalFakeStore{
		chatLogs: []store.ChatLog{
			{ID: 1, ChatSessionID: "sess-1", TurnIndex: 4, Role: "user", Content: "hello"},
			{ID: 2, ChatSessionID: "sess-1", TurnIndex: 4, Role: "assistant", Content: "hi"},
		},
		effectiveInput:  &store.EffectiveInput{ID: 3, ChatSessionID: "sess-1", TurnIndex: 4, EffectiveInput: "hello refined"},
		memories:        []store.Memory{{ID: 4, ChatSessionID: "sess-1", TurnIndex: 4, SummaryJSON: `{"summary":"ok"}`}},
		evidenceItems:   []store.DirectEvidence{{ID: 5, ChatSessionID: "sess-1", EvidenceKind: "fact_event", EvidenceText: "evidence"}},
		kgItems:         []store.KGTriple{{ID: 6, ChatSessionID: "sess-1", Subject: "a", Predicate: "knows", Object: "b"}},
		auditItems:      []store.AuditLog{{ID: 7, ChatSessionID: "sess-1", EventType: "export"}},
		feedbackItems:   []store.CriticFeedback{{ID: 8, ChatSessionID: "sess-1", TargetType: "turn", TargetID: 4, FeedbackValue: "keep"}},
		characterEvents: []store.CharacterEvent{{ID: 9, ChatSessionID: "sess-1", CharacterName: "Chloe", TurnIndex: 4, EventType: "state"}},
	}
	mux := http.NewServeMux()
	srv := setupTestServer()
	srv.Store = fake
	srv.RegisterRoutes(mux)

	req := httptest.NewRequest(http.MethodGet, "/sessions/sess-1/export", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200: %s", rec.Code, rec.Body.String())
	}
	var resp map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if resp["status"] != "ok" || resp["chat_session_id"] != "sess-1" || resp["export_version"] != "1.1" {
		t.Fatalf("unexpected response header fields: %+v", resp)
	}
	counts := resp["summary"].(map[string]any)
	expectedCounts := map[string]float64{
		"chat_logs_count":               2,
		"effective_inputs_count":        1,
		"memories_count":                1,
		"direct_evidence_records_count": 1,
		"canonical_state_layers_count":  0,
		"kg_triples_count":              1,
		"chapter_summaries_count":       0,
		"arc_summaries_count":           0,
		"saga_digests_count":            0,
	}
	for key, want := range expectedCounts {
		if counts[key] != want {
			t.Fatalf("count[%s] = %v, want %v", key, counts[key], want)
		}
	}
	for _, key := range []string{"chat_logs", "effective_inputs", "memories", "direct_evidence_records", "canonical_state_layers", "kg_triples", "chapter_summaries", "arc_summaries", "saga_digests", "guidance_snapshot", "embedding_provenance", "lineage_summary", "portability_contract"} {
		if _, ok := resp[key]; !ok {
			t.Fatalf("missing Python 0.8 export key %q in response: %+v", key, resp)
		}
	}
	if fake.listChatLogsSession != "sess-1" || fake.getEffectiveInputSession != "sess-1" || fake.getEffectiveInputTurn != 4 {
		t.Fatalf("store calls were not made with session/turn: %+v", fake)
	}
}

func TestSeq13P42SessionExportPreservesLineagePackageFields(t *testing.T) {
	fake := &canonicalFakeStore{
		chatLogs: []store.ChatLog{
			{ID: 1, ChatSessionID: "sess-sp1a", TurnIndex: 12, Role: "user", Content: "preserve the lineage"},
			{ID: 2, ChatSessionID: "sess-sp1a", TurnIndex: 12, Role: "assistant", Content: "lineage preserved"},
		},
		effectiveInput: &store.EffectiveInput{ID: 3, ChatSessionID: "sess-sp1a", TurnIndex: 12, EffectiveInput: "preserve the lineage"},
		evidenceItems: []store.DirectEvidence{{
			ID:                   4,
			ChatSessionID:        "sess-sp1a",
			EvidenceKind:         "fact_event",
			EvidenceText:         "The vault source hash must survive import.",
			SourceTurnStart:      12,
			SourceTurnEnd:        13,
			TurnAnchor:           12,
			SourceMessageIDsJSON: mustCompactJSON([]string{"turn:12:user", "turn:12:assistant"}),
			SourceHash:           "sha256:sp1a-lineage",
			ArchiveState:         "superseded",
			CaptureStage:         "critic",
			CaptureVerification:  "verified",
			CommittedGate:        "ready",
			LineageJSON:          mustCompactJSON(map[string]any{"source": "session_export", "session_origin": "sess-sp1a"}),
			Tombstoned:           true,
			SupersededByID:       44,
		}},
		canonicalLayers: []store.CanonicalStateLayer{{
			ID:               5,
			ChatSessionID:    "sess-sp1a",
			LayerType:        "relationship_state",
			Content:          `{"state":"kept"}`,
			SourceTurn:       12,
			SourceRecord:     4,
			LastVerifiedTurn: 13,
			Confidence:       0.88,
		}},
	}
	mux := http.NewServeMux()
	srv := setupTestServer()
	srv.Store = fake
	srv.RegisterRoutes(mux)

	req := httptest.NewRequest(http.MethodGet, "/sessions/sess-sp1a/export", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200: %s", rec.Code, rec.Body.String())
	}
	var resp map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}

	inputs := resp["effective_inputs"].([]any)
	input := inputs[0].(map[string]any)
	if input["effective_input"] != "preserve the lineage" || input["turn_index"] != float64(12) {
		t.Fatalf("effective input did not export with Python-compatible fields: %+v", input)
	}

	evidence := resp["direct_evidence_records"].([]any)
	item := evidence[0].(map[string]any)
	for _, key := range []string{"source_hash", "source_turn_start", "source_turn_end", "turn_anchor", "lineage", "tombstoned", "superseded_by_id"} {
		if _, ok := item[key]; !ok {
			t.Fatalf("direct evidence export missing lineage key %q: %+v", key, item)
		}
	}
	if item["source_hash"] != "sha256:sp1a-lineage" || item["source_turn_start"] != float64(12) || item["source_turn_end"] != float64(13) {
		t.Fatalf("direct evidence source lineage not preserved: %+v", item)
	}
	if item["tombstoned"] != true || item["superseded_by_id"] != float64(44) {
		t.Fatalf("tombstone/supersede lineage not preserved: %+v", item)
	}
	lineage := item["lineage"].(map[string]any)
	if lineage["source"] != "session_export" || lineage["session_origin"] != "sess-sp1a" {
		t.Fatalf("lineage payload not preserved: %+v", lineage)
	}

	layers := resp["canonical_state_layers"].([]any)
	layer := layers[0].(map[string]any)
	if layer["source_turn"] != float64(12) || layer["source_record"] != float64(4) || layer["last_verified_turn"] != float64(13) {
		t.Fatalf("canonical layer source lineage not preserved: %+v", layer)
	}

	summary := resp["lineage_summary"].(map[string]any)
	if summary["direct_evidence_source_hash_count"] != float64(1) ||
		summary["direct_evidence_tombstoned_count"] != float64(1) ||
		summary["direct_evidence_superseded_count"] != float64(1) ||
		summary["canonical_layers_with_source_turn_count"] != float64(1) ||
		summary["canonical_layers_with_source_record_count"] != float64(1) {
		t.Fatalf("lineage summary did not count preserved fields: %+v", summary)
	}

	contract := resp["portability_contract"].(map[string]any)
	if contract["package_policy_version"] != "sp1a.v1" || contract["manual_first"] != true || contract["auto_copy_detection"] != "deferred" {
		t.Fatalf("portability contract header mismatch: %+v", contract)
	}
	rebuild := contract["rebuild_handoff"].(map[string]any)
	if rebuild["dirty_event_type"] != "backfill_import" || rebuild["rebuild_mode"] != "selective" || rebuild["start_point"] != "next_prepare_turn_fetch" {
		t.Fatalf("backfill rebuild handoff mismatch: %+v", rebuild)
	}
}

func TestAuditAndFeedbackLatestUseCanonicalStore(t *testing.T) {
	fake := &canonicalFakeStore{
		auditItems:    []store.AuditLog{{ID: 7, ChatSessionID: "sess-1", EventType: "export", Summary: "ok"}},
		feedbackItems: []store.CriticFeedback{{ID: 8, ChatSessionID: "sess-1", TargetType: "turn", TargetID: 4, FeedbackValue: "keep"}},
	}
	mux := http.NewServeMux()
	srv := setupTestServer()
	srv.Store = fake
	srv.RegisterRoutes(mux)

	auditReq := httptest.NewRequest(http.MethodGet, "/audit?chat_session_id=sess-1&event_type=export&limit=5", nil)
	auditRec := httptest.NewRecorder()
	mux.ServeHTTP(auditRec, auditReq)
	if auditRec.Code != http.StatusOK {
		t.Fatalf("audit status = %d, want 200: %s", auditRec.Code, auditRec.Body.String())
	}
	var auditResp map[string]any
	if err := json.Unmarshal(auditRec.Body.Bytes(), &auditResp); err != nil {
		t.Fatalf("decode audit: %v", err)
	}
	if auditResp["total"] != float64(1) {
		t.Fatalf("unexpected audit response: %+v", auditResp)
	}
	if _, ok := auditResp["source"]; ok {
		t.Fatalf("audit response should keep Python-compatible shape without source: %+v", auditResp)
	}
	if fake.listAuditSession != "sess-1" || fake.listAuditEventType != "export" || fake.listAuditLimit != 5 {
		t.Fatalf("audit store args not captured: %+v", fake)
	}

	feedbackReq := httptest.NewRequest(http.MethodGet, "/feedback/latest?chat_session_id=sess-1&target_type=turn&target_id=4", nil)
	feedbackRec := httptest.NewRecorder()
	mux.ServeHTTP(feedbackRec, feedbackReq)
	if feedbackRec.Code != http.StatusOK {
		t.Fatalf("feedback status = %d, want 200: %s", feedbackRec.Code, feedbackRec.Body.String())
	}
	var feedbackResp map[string]any
	if err := json.Unmarshal(feedbackRec.Body.Bytes(), &feedbackResp); err != nil {
		t.Fatalf("decode feedback: %v", err)
	}
	if feedbackResp["count"] != float64(1) || feedbackResp["source"] != "shadow" {
		t.Fatalf("unexpected feedback response: %+v", feedbackResp)
	}
	if fake.listFeedbackSession != "sess-1" || fake.listFeedbackTargetType != "turn" || fake.listFeedbackTargetID != 4 {
		t.Fatalf("feedback store args not captured: %+v", fake)
	}
}

func TestCanonicalWriteDefaultNoopIsGuarded(t *testing.T) {
	mux := http.NewServeMux()
	srv := setupTestServer()
	srv.RegisterRoutes(mux)

	req := httptest.NewRequest(http.MethodPost, "/canonical/sess-1/memories", strings.NewReader(`{"turn_index":1}`))
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200: %s", rec.Code, rec.Body.String())
	}
	var resp map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if resp["saved"] != false {
		t.Errorf("saved = %v, want false", resp["saved"])
	}
	if resp["error"] != "shadow_write_store_not_configured" {
		t.Errorf("error = %v, want shadow_write_store_not_configured", resp["error"])
	}
}

func TestCanonicalWriteDualShadowSavesMemory(t *testing.T) {
	cfg := config.Default()
	cfg.StoreMode = config.StoreModeDualShadow
	mux := http.NewServeMux()
	srv := NewServer(cfg)
	srv.RegisterRoutes(mux)

	req := httptest.NewRequest(http.MethodPost, "/canonical/sess-1/memories", strings.NewReader(`{"turn_index":1,"summary_json":"{\"summary\":\"ok\"}"}`))
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200: %s", rec.Code, rec.Body.String())
	}
	var resp map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if resp["saved"] != true {
		t.Errorf("saved = %v, want true", resp["saved"])
	}
	if resp["source"] != "shadow" {
		t.Errorf("source = %v, want shadow", resp["source"])
	}
}

func TestCanonicalWriteChatLogAcceptsStartupTurnZero(t *testing.T) {
	fake := &canonicalFakeStore{}
	cfg := config.Default()
	cfg.StoreMode = config.StoreModeDualShadow
	mux := http.NewServeMux()
	srv := &Server{Cfg: cfg, Store: fake}
	srv.RegisterRoutes(mux)

	req := httptest.NewRequest(http.MethodPost, "/canonical/sess-start/chat-logs", strings.NewReader(`{"turn_index":0,"role":"assistant","content":"Opening story."}`))
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200: %s", rec.Code, rec.Body.String())
	}
	if fake.chatLog == nil {
		t.Fatal("SaveChatLog was not called")
	}
	if fake.chatLog.ChatSessionID != "sess-start" || fake.chatLog.TurnIndex != 0 || fake.chatLog.Role != "assistant" || fake.chatLog.Content != "Opening story." {
		t.Fatalf("unexpected startup chat log payload: %+v", fake.chatLog)
	}
}

func TestCanonicalWriteChatLogSkipsExistingTurnRoleDuplicate(t *testing.T) {
	fake := &canonicalFakeStore{
		chatLogs: []store.ChatLog{
			{ID: 1, ChatSessionID: "sess-dup", TurnIndex: 1, Role: "assistant", Content: "old assistant text"},
		},
	}
	cfg := config.Default()
	cfg.StoreMode = config.StoreModeDualShadow
	mux := http.NewServeMux()
	srv := &Server{Cfg: cfg, Store: fake}
	srv.RegisterRoutes(mux)

	req := httptest.NewRequest(http.MethodPost, "/canonical/sess-dup/chat-logs", strings.NewReader(`{"turn_index":1,"role":"assistant","content":"new assistant text"}`))
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200: %s", rec.Code, rec.Body.String())
	}
	var resp map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if fake.chatLog != nil {
		t.Fatalf("SaveChatLog should not be called for duplicate turn role, got %+v", fake.chatLog)
	}
	if resp["deduped"] != true || resp["conflict"] != true || resp["saved"] != true {
		t.Fatalf("unexpected duplicate response: %+v", resp)
	}
}

func TestCanonicalWriteExtendedFamiliesUseStore(t *testing.T) {
	tests := []struct {
		name        string
		path        string
		body        string
		assertStore func(t *testing.T, fake *canonicalFakeStore)
	}{
		{
			name: "chat logs",
			path: "/canonical/sess-1/chat-logs",
			body: `{"turn_index":10,"role":"assistant","content":"ok"}`,
			assertStore: func(t *testing.T, fake *canonicalFakeStore) {
				t.Helper()
				if fake.chatLog == nil {
					t.Fatal("SaveChatLog was not called")
				}
				if fake.chatLog.ChatSessionID != "sess-1" || fake.chatLog.TurnIndex != 10 || fake.chatLog.Role != "assistant" {
					t.Fatalf("unexpected chat log payload: %+v", fake.chatLog)
				}
			},
		},
		{
			name: "effective inputs",
			path: "/canonical/sess-1/effective-inputs",
			body: `{"turn_index":10,"effective_input":"go forward"}`,
			assertStore: func(t *testing.T, fake *canonicalFakeStore) {
				t.Helper()
				if fake.savedEffectiveInput == nil {
					t.Fatal("SaveEffectiveInput was not called")
				}
				if fake.savedEffectiveInput.ChatSessionID != "sess-1" || fake.savedEffectiveInput.TurnIndex != 10 || fake.savedEffectiveInput.EffectiveInput != "go forward" {
					t.Fatalf("unexpected effective input payload: %+v", fake.savedEffectiveInput)
				}
			},
		},
		{
			name: "audit logs",
			path: "/canonical/sess-1/audit-logs",
			body: `{"event_type":"memory_commit","target_type":"memory","target_id":11,"summary":"ok","details_json":"{\"k\":1}","source":"test"}`,
			assertStore: func(t *testing.T, fake *canonicalFakeStore) {
				t.Helper()
				if fake.audit == nil {
					t.Fatal("SaveAuditLog was not called")
				}
				if fake.audit.ChatSessionID != "sess-1" || fake.audit.EventType != "memory_commit" || fake.audit.TargetID != 11 {
					t.Fatalf("unexpected audit payload: %+v", fake.audit)
				}
			},
		},
		{
			name: "critic feedback",
			path: "/canonical/sess-1/critic-feedback",
			body: `{"target_type":"turn","target_id":12,"feedback_value":"keep","feedback_note":"good","source":"test"}`,
			assertStore: func(t *testing.T, fake *canonicalFakeStore) {
				t.Helper()
				if fake.feedback == nil {
					t.Fatal("SaveCriticFeedback was not called")
				}
				if fake.feedback.ChatSessionID != "sess-1" || fake.feedback.TargetType != "turn" || fake.feedback.FeedbackValue != "keep" {
					t.Fatalf("unexpected feedback payload: %+v", fake.feedback)
				}
			},
		},
		{
			name: "character events",
			path: "/canonical/sess-1/character-events",
			body: `{"character_name":"Chloe","turn_index":13,"event_type":"state","details_json":"{\"mood\":\"steady\"}"}`,
			assertStore: func(t *testing.T, fake *canonicalFakeStore) {
				t.Helper()
				if fake.characterEvent == nil {
					t.Fatal("SaveCharacterEvent was not called")
				}
				if fake.characterEvent.ChatSessionID != "sess-1" || fake.characterEvent.CharacterName != "Chloe" || fake.characterEvent.TurnIndex != 13 {
					t.Fatalf("unexpected character event payload: %+v", fake.characterEvent)
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fake := &canonicalFakeStore{}
			cfg := config.Default()
			cfg.StoreMode = config.StoreModeDualShadow
			srv := NewServer(cfg)
			srv.Store = fake

			mux := http.NewServeMux()
			srv.RegisterRoutes(mux)

			req := httptest.NewRequest(http.MethodPost, tt.path, strings.NewReader(tt.body))
			rec := httptest.NewRecorder()
			mux.ServeHTTP(rec, req)

			if rec.Code != http.StatusOK {
				t.Fatalf("status = %d, want 200: %s", rec.Code, rec.Body.String())
			}
			var resp map[string]any
			if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
				t.Fatalf("decode: %v", err)
			}
			if resp["saved"] != true {
				t.Fatalf("saved = %v, want true", resp["saved"])
			}
			tt.assertStore(t, fake)
		})
	}
}

func TestCanonicalMissingSessionIDHandler(t *testing.T) {
	srv := setupTestServer()
	req := httptest.NewRequest(http.MethodGet, "/canonical//memories", nil)
	rec := httptest.NewRecorder()
	srv.handleCanonicalListMemories(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", rec.Code)
	}
}

type canonicalFakeStore struct {
	chatLogs        []store.ChatLog
	effectiveInput  *store.EffectiveInput
	feedbackItems   []store.CriticFeedback
	memories        []store.Memory
	evidenceItems   []store.DirectEvidence
	canonicalLayers []store.CanonicalStateLayer
	kgItems         []store.KGTriple
	auditItems      []store.AuditLog
	characterEvents []store.CharacterEvent

	listChatLogsSession string
	listChatLogsFrom    int
	listChatLogsTo      int

	getEffectiveInputSession string
	getEffectiveInputTurn    int

	listMemoriesSession string
	listMemoriesFrom    int
	listMemoriesTo      int

	listFeedbackSession    string
	listFeedbackTargetType string
	listFeedbackTargetID   int64
	listAuditSession       string
	listAuditEventType     string
	listAuditLimit         int

	chatLog             *store.ChatLog
	savedEffectiveInput *store.EffectiveInput
	audit               *store.AuditLog
	feedback            *store.CriticFeedback
	characterEvent      *store.CharacterEvent
}

func (f *canonicalFakeStore) SaveChatLog(ctx context.Context, log *store.ChatLog) error {
	f.chatLog = log
	return nil
}
func (f *canonicalFakeStore) ListChatLogs(ctx context.Context, chatSessionID string, fromTurn, toTurn int) ([]store.ChatLog, error) {
	f.listChatLogsSession = chatSessionID
	f.listChatLogsFrom = fromTurn
	f.listChatLogsTo = toTurn
	return f.chatLogs, nil
}
func (f *canonicalFakeStore) SaveEffectiveInput(ctx context.Context, in *store.EffectiveInput) error {
	f.savedEffectiveInput = in
	return nil
}
func (f *canonicalFakeStore) GetEffectiveInput(ctx context.Context, chatSessionID string, turnIndex int) (*store.EffectiveInput, error) {
	f.getEffectiveInputSession = chatSessionID
	f.getEffectiveInputTurn = turnIndex
	if f.effectiveInput == nil {
		return nil, store.ErrNotFound
	}
	return f.effectiveInput, nil
}
func (f *canonicalFakeStore) SaveMemory(ctx context.Context, m *store.Memory) error { return nil }
func (f *canonicalFakeStore) ListMemories(ctx context.Context, chatSessionID string, fromTurn, toTurn int) ([]store.Memory, error) {
	f.listMemoriesSession = chatSessionID
	f.listMemoriesFrom = fromTurn
	f.listMemoriesTo = toTurn
	return f.memories, nil
}
func (f *canonicalFakeStore) SaveEvidence(ctx context.Context, e *store.DirectEvidence) error {
	return nil
}
func (f *canonicalFakeStore) ListEvidence(ctx context.Context, chatSessionID string) ([]store.DirectEvidence, error) {
	return f.evidenceItems, nil
}
func (f *canonicalFakeStore) SaveKGTriple(ctx context.Context, t *store.KGTriple) error { return nil }
func (f *canonicalFakeStore) ListKGTriples(ctx context.Context, chatSessionID string) ([]store.KGTriple, error) {
	return f.kgItems, nil
}
func (f *canonicalFakeStore) SaveAuditLog(ctx context.Context, a *store.AuditLog) error {
	f.audit = a
	return nil
}
func (f *canonicalFakeStore) ListAuditLogs(ctx context.Context, chatSessionID string, eventType string, limit int) ([]store.AuditLog, error) {
	f.listAuditSession = chatSessionID
	f.listAuditEventType = eventType
	f.listAuditLimit = limit
	return f.auditItems, nil
}
func (f *canonicalFakeStore) SaveCriticFeedback(ctx context.Context, cf *store.CriticFeedback) error {
	f.feedback = cf
	return nil
}
func (f *canonicalFakeStore) ListCriticFeedback(ctx context.Context, chatSessionID string, targetType string, targetID int64) ([]store.CriticFeedback, error) {
	f.listFeedbackSession = chatSessionID
	f.listFeedbackTargetType = targetType
	f.listFeedbackTargetID = targetID
	return f.feedbackItems, nil
}
func (f *canonicalFakeStore) SaveCharacterEvent(ctx context.Context, e *store.CharacterEvent) error {
	f.characterEvent = e
	return nil
}
func (f *canonicalFakeStore) ListCharacterEvents(ctx context.Context, chatSessionID string, characterName string) ([]store.CharacterEvent, error) {
	return f.characterEvents, nil
}
func (f *canonicalFakeStore) Stats(ctx context.Context) (store.StatsResult, error) {
	return store.StatsResult{}, nil
}
func (f *canonicalFakeStore) ListSessions(ctx context.Context) ([]store.SessionSummary, error) {
	return nil, nil
}
func (f *canonicalFakeStore) GetResumePack(ctx context.Context, chatSessionID string, trigger string) (*store.ResumePack, error) {
	return nil, nil
}
func (f *canonicalFakeStore) ListStorylines(ctx context.Context, chatSessionID string) ([]store.Storyline, error) {
	return nil, nil
}
func (f *canonicalFakeStore) ListWorldRules(ctx context.Context, chatSessionID string) ([]store.WorldRule, error) {
	return nil, nil
}
func (f *canonicalFakeStore) ListInheritedWorldRules(ctx context.Context, chatSessionID string, activeScope, scopeName string) ([]store.WorldRule, error) {
	return nil, nil
}
func (f *canonicalFakeStore) ListCharacterStates(ctx context.Context, chatSessionID string) ([]store.CharacterState, error) {
	return nil, nil
}
func (f *canonicalFakeStore) GetCharacterState(ctx context.Context, chatSessionID, characterName string) (*store.CharacterState, error) {
	return nil, store.ErrNotFound
}
func (f *canonicalFakeStore) ListPendingThreads(ctx context.Context, chatSessionID, status string) ([]store.PendingThread, error) {
	return nil, nil
}
func (f *canonicalFakeStore) ListActiveStates(ctx context.Context, chatSessionID, stateType string) ([]store.ActiveState, error) {
	return nil, nil
}
func (f *canonicalFakeStore) ListCanonicalStateLayers(ctx context.Context, chatSessionID, layerType string) ([]store.CanonicalStateLayer, error) {
	return f.canonicalLayers, nil
}
func (f *canonicalFakeStore) ListEpisodeSummaries(ctx context.Context, chatSessionID string, limit, fromTurn, toTurn int) ([]store.EpisodeSummary, error) {
	return nil, nil
}
func (f *canonicalFakeStore) GetEpisodeSummary(ctx context.Context, episodeID int64) (*store.EpisodeSummary, error) {
	return nil, store.ErrNotFound
}

func TestSeq123P76CanonicalStoreAuthorityMarkers(t *testing.T) {
	authoritySrv := &Server{Cfg: config.Config{StoreMode: config.StoreModeMariaDBAuthority}}
	if authoritySrv.storeWriteSource() != "mariadb_authority" {
		t.Fatalf("store write source = %q, want mariadb_authority", authoritySrv.storeWriteSource())
	}

	fake := &canonicalFakeStore{
		memories: []store.Memory{{ID: 1, ChatSessionID: "sess-p76", TurnIndex: 1, SummaryJSON: `{"s":"ok"}`}},
	}
	srv := setupTestServer()
	srv.Store = fake
	mux := http.NewServeMux()
	srv.RegisterRoutes(mux)

	// Canonical read must be backed by Store (MariaDB truth authority).
	req := httptest.NewRequest(http.MethodGet, "/canonical/sess-p76/memories", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("canonical read status = %d, want 200", rec.Code)
	}
	var resp map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if resp["source"] != "shadow" {
		t.Errorf("source = %v, want shadow", resp["source"])
	}
	if resp["count"] != float64(1) {
		t.Errorf("count = %v, want 1", resp["count"])
	}
	if fake.listMemoriesSession != "sess-p76" {
		t.Errorf("store not consulted: session = %q", fake.listMemoriesSession)
	}

	// Canonical write must be shadow-guarded; vector lane must not hold canonical write authority.
	body := `{"chat_session_id":"sess-p76","turn_index":1,"summary_json":"{\"s\":\"ok\"}","importance":0.5}`
	req2 := httptest.NewRequest(http.MethodPost, "/canonical/sess-p76/memories", strings.NewReader(body))
	req2.Header.Set("Content-Type", "application/json")
	rec2 := httptest.NewRecorder()
	mux.ServeHTTP(rec2, req2)
	if rec2.Code != http.StatusOK {
		t.Errorf("canonical write status = %d, want 200 shadow guard", rec2.Code)
	}
	var resp2 map[string]any
	if err := json.Unmarshal(rec2.Body.Bytes(), &resp2); err != nil {
		t.Fatalf("decode write guard: %v", err)
	}
	if resp2["source"] != "shadow" {
		t.Errorf("write source = %v, want shadow", resp2["source"])
	}
	if resp2["saved"] != false {
		t.Errorf("saved = %v, want false", resp2["saved"])
	}
	if resp2["error"] != "shadow_write_store_not_configured" {
		t.Errorf("error = %v, want shadow_write_store_not_configured", resp2["error"])
	}
}
