package httpapi

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/risulongmemory/archive-center-go/internal/config"
	"github.com/risulongmemory/archive-center-go/internal/store"
)

func setupTestServer() *Server {
	cfg := config.Default()
	return NewServer(cfg)
}
func TestHandleHealth(t *testing.T) {
	mux := http.NewServeMux()
	srv := setupTestServer()
	srv.RegisterRoutes(mux)

	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, rec.Code)
	}

	var resp healthResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if resp.Status != "ok" {
		t.Errorf("Status = %q, want %q", resp.Status, "ok")
	}
}

func TestReverseProxyBasePathRoutesArchiveEndpoints(t *testing.T) {
	mux := http.NewServeMux()
	srv := setupTestServer()
	srv.RegisterRoutes(mux)

	for _, path := range []string{"/proxy2/health", "/archive-center/wakeup"} {
		req := httptest.NewRequest(http.MethodGet, path, nil)
		rec := httptest.NewRecorder()
		mux.ServeHTTP(rec, req)
		if rec.Code != http.StatusOK {
			t.Fatalf("%s status = %d, want 200: %s", path, rec.Code, rec.Body.String())
		}
	}
}

func TestSeq13P60BridgeHealthFalseGreenContract(t *testing.T) {
	mux := http.NewServeMux()
	srv := setupTestServer()
	srv.RegisterRoutes(mux)

	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("health status = %d, want 200: %s", rec.Code, rec.Body.String())
	}
	var health map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &health); err != nil {
		t.Fatalf("health decode: %v", err)
	}
	if health["status"] != "ok" || health["scope"] != "liveness_only" {
		t.Fatalf("health should remain liveness-only ok: %+v", health)
	}
	if health["bridge_health_contract_version"] != "bf13b.v1" {
		t.Fatalf("health contract version = %v, want bf13b.v1", health["bridge_health_contract_version"])
	}
	if health["localhost_default_scope"] != "same_host_local_only" {
		t.Fatalf("localhost default scope mismatch: %+v", health)
	}
	if health["false_green_guard"] != "do_not_treat_health_as_route_readiness" || health["route_level_health_required"] != true {
		t.Fatalf("false-green guard missing: %+v", health)
	}
	routeHealth := health["route_health"].(map[string]any)
	if routeHealth["ready_route"] != "/ready" || routeHealth["prepare_turn_route"] != "/prepare-turn" {
		t.Fatalf("route-level health contract missing ready/prepare-turn routes: %+v", routeHealth)
	}
	if routeHealth["supervisor_probe"] != "wakeup_is_service_ping_only" {
		t.Fatalf("wakeup should not be treated as supervisor/LLM readiness: %+v", routeHealth)
	}
	notes := health["remote_bridge_notes"].([]any)
	if len(notes) < 2 {
		t.Fatalf("remote bridge notes should document Docker/localhost false-green risks: %+v", health)
	}

	req = httptest.NewRequest(http.MethodGet, "/wakeup", nil)
	rec = httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("wakeup status = %d, want 200: %s", rec.Code, rec.Body.String())
	}
	var wakeup map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &wakeup); err != nil {
		t.Fatalf("wakeup decode: %v", err)
	}
	if wakeup["scope"] != "service_ping_only" || wakeup["route_health_required_for_green"] != true {
		t.Fatalf("wakeup must remain service-ping-only with route probe required: %+v", wakeup)
	}
}

func TestCORSMiddlewareUsesConfiguredAllowedOrigins(t *testing.T) {
	cfg := config.Default()
	cfg.AllowedOrigins = []string{"https://risu.example"}
	srv := NewServer(cfg)
	mux := http.NewServeMux()
	srv.RegisterRoutes(mux)

	req := httptest.NewRequest(http.MethodOptions, "/health", nil)
	req.Header.Set("Origin", "https://risu.example")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusNoContent {
		t.Fatalf("status = %d, want 204", rec.Code)
	}
	if got := rec.Header().Get("Access-Control-Allow-Origin"); got != "https://risu.example" {
		t.Errorf("Access-Control-Allow-Origin = %q", got)
	}
}

func TestHandleReady(t *testing.T) {
	mux := http.NewServeMux()
	srv := setupTestServer()
	srv.RegisterRoutes(mux)

	req := httptest.NewRequest(http.MethodGet, "/ready", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, rec.Code)
	}

	var resp readyResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if !resp.Ready {
		t.Error("Ready should be true")
	}
	if resp.Mode != "shadow" {
		t.Errorf("Mode = %q, want %q", resp.Mode, "shadow")
	}
	if resp.Checks["shadow_mode"] != "active" {
		t.Errorf("shadow_mode check = %q, want %q", resp.Checks["shadow_mode"], "active")
	}
	if resp.Checks["mariadb"] != "not_configured" {
		t.Errorf("mariadb check = %q, want %q", resp.Checks["mariadb"], "not_configured")
	}
	if resp.Checks["chromadb"] != "not_configured" {
		t.Errorf("chromadb check = %q, want %q", resp.Checks["chromadb"], "not_configured")
	}
	if resp.Checks["chromadb_vector"] != "degraded_fallback" {
		t.Errorf("chromadb_vector check = %q, want %q", resp.Checks["chromadb_vector"], "degraded_fallback")
	}
	if resp.RuntimeProfile != "core_lite" {
		t.Errorf("RuntimeProfile = %q, want core_lite", resp.RuntimeProfile)
	}
	if resp.VectorMode != "fallback" {
		t.Errorf("VectorMode = %q, want fallback", resp.VectorMode)
	}
	if !resp.Degraded {
		t.Error("Degraded should be true when vector fallback is active")
	}
	if resp.Checks["live_cutover"] != "disabled" {
		t.Errorf("live_cutover check = %q, want %q", resp.Checks["live_cutover"], "disabled")
	}
	if resp.Timestamp == "" {
		t.Error("Timestamp should not be empty")
	}
}

func TestHandleReadyWithDependencies(t *testing.T) {
	cfg := config.Default()
	cfg.Readiness.MariaDBConfigured = true
	cfg.ChromaEndpoint = "http://127.0.0.1:8000"
	cfg.Readiness.ChromaConfigured = true
	cfg.RuntimeProfile = config.RuntimeProfileFullLocal
	cfg.VectorMode = config.VectorModeBundled
	cfg.ChromaEnabled = true

	mux := http.NewServeMux()
	srv := NewServer(cfg)
	srv.RegisterRoutes(mux)

	req := httptest.NewRequest(http.MethodGet, "/ready", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, rec.Code)
	}

	var resp readyResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if resp.Checks["mariadb"] != "configured" {
		t.Errorf("mariadb check = %q, want %q", resp.Checks["mariadb"], "configured")
	}
	if resp.Checks["chromadb"] != "configured" {
		t.Errorf("chromadb check = %q, want %q", resp.Checks["chromadb"], "configured")
	}
	if resp.Checks["chromadb_vector"] != "enabled" {
		t.Errorf("chromadb_vector check = %q, want %q", resp.Checks["chromadb_vector"], "enabled")
	}
	if resp.Checks["live_cutover"] != "disabled" {
		t.Errorf("live_cutover check = %q, want %q", resp.Checks["live_cutover"], "disabled")
	}
}

func TestHandleVersion(t *testing.T) {
	mux := http.NewServeMux()
	srv := setupTestServer()
	srv.RegisterRoutes(mux)

	req := httptest.NewRequest(http.MethodGet, "/version", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, rec.Code)
	}

	var resp versionResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if resp.Version != "2.0.0-dev" {
		t.Errorf("Version = %q, want %q", resp.Version, "2.0.0-dev")
	}
	if resp.Commit != "unknown" {
		t.Errorf("Commit = %q, want %q", resp.Commit, "unknown")
	}
	if resp.BuildTime == "" {
		t.Error("BuildTime should not be empty")
	}
}

func TestHandleHealthContentType(t *testing.T) {
	mux := http.NewServeMux()
	srv := setupTestServer()
	srv.RegisterRoutes(mux)

	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	ct := rec.Header().Get("Content-Type")
	if ct != "application/json; charset=utf-8" {
		t.Errorf("Content-Type = %q, want %q", ct, "application/json; charset=utf-8")
	}
}

func TestHandleReadyBlocksNonShadowModes(t *testing.T) {
	// Product modes are allowed only after the MariaDB authority contract is
	// present. Bare live/cutover configs still fail readiness.
	for _, mode := range []config.Mode{config.ModeLive, config.ModeCutover} {
		cfg := config.Default()
		cfg.Mode = mode
		mux := http.NewServeMux()
		srv := NewServer(cfg)
		srv.RegisterRoutes(mux)

		req := httptest.NewRequest(http.MethodGet, "/ready", nil)
		rec := httptest.NewRecorder()
		mux.ServeHTTP(rec, req)

		if rec.Code != http.StatusServiceUnavailable {
			t.Errorf("mode %s: expected status %d, got %d", mode, http.StatusServiceUnavailable, rec.Code)
		}

		var resp readyResponse
		if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
			t.Fatalf("mode %s: failed to decode response: %v", mode, err)
		}

		if resp.Ready {
			t.Errorf("mode %s: Ready should be false", mode)
		}
		if resp.Checks["shadow_mode"] != "inactive" {
			t.Errorf("mode %s: shadow_mode = %q, want %q", mode, resp.Checks["shadow_mode"], "inactive")
		}
		if resp.Checks["live_cutover"] != "disabled" {
			t.Errorf("mode %s: live_cutover = %q, want %q", mode, resp.Checks["live_cutover"], "disabled")
		}
		wantGuard := "mode \"" + string(mode) + "\" requires MariaDB authority and the selected vector policy to be satisfied"
		if resp.Checks["mode_guard"] != wantGuard {
			t.Errorf("mode %s: mode_guard = %q, want %q", mode, resp.Checks["mode_guard"], wantGuard)
		}
	}
}

func TestHandleReadyAllowsLiveMariaDBAuthorityWhenStoreOpen(t *testing.T) {
	cfg := config.Default()
	cfg.Mode = config.ModeLive
	cfg.StoreMode = config.StoreModeMariaDBAuthority
	cfg.MariaDBDSN = "ac_user:pw@tcp(127.0.0.1:3307)/archive_center?parseTime=true"
	cfg.Readiness.MariaDBConfigured = true
	cfg.ChromaEndpoint = "http://127.0.0.1:8000"
	cfg.Readiness.ChromaConfigured = true

	mux := http.NewServeMux()
	srv := &Server{
		Cfg:     cfg,
		Started: time.Now().UTC(),
		Store:   store.NewNoopStore(),
	}
	srv.RegisterRoutes(mux)

	req := httptest.NewRequest(http.MethodGet, "/ready", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d: %s", http.StatusOK, rec.Code, rec.Body.String())
	}

	var resp readyResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if !resp.Ready {
		t.Fatal("Ready should be true")
	}
	if resp.Checks["live_cutover"] != "enabled" {
		t.Errorf("live_cutover = %q, want enabled", resp.Checks["live_cutover"])
	}
	if resp.Checks["product_mode"] != "active" {
		t.Errorf("product_mode = %q, want active", resp.Checks["product_mode"])
	}
	if resp.Checks["mariadb_authority"] != "enabled" {
		t.Errorf("mariadb_authority = %q, want enabled", resp.Checks["mariadb_authority"])
	}
}

func TestServerStartedTimestamp(t *testing.T) {
	before := time.Now().UTC().Add(-time.Second)
	srv := setupTestServer()
	after := time.Now().UTC().Add(time.Second)

	if srv.Started.Before(before) || srv.Started.After(after) {
		t.Error("Started timestamp is not within expected range")
	}
}

// ---------------------------------------------------------------------------
// Low-risk read-only route shadow tests
// ---------------------------------------------------------------------------
func TestHandleStatsShadow(t *testing.T) {
	mux := http.NewServeMux()
	srv := setupTestServer()
	srv.RegisterRoutes(mux)

	req := httptest.NewRequest(http.MethodGet, "/stats", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, rec.Code)
	}

	var resp map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to decode: %v", err)
	}
	if resp["status"] != "ok" {
		t.Errorf("status = %q, want ok", resp["status"])
	}
	// 0.8 shape: chat_logs, memories, kg_triples must be present as numbers
	for _, key := range []string{"chat_logs", "memories", "kg_triples"} {
		if _, ok := resp[key]; !ok {
			t.Errorf("missing field %q", key)
		}
	}
}

func TestHandleSessionsListShadow(t *testing.T) {
	mux := http.NewServeMux()
	srv := setupTestServer()
	srv.RegisterRoutes(mux)

	req := httptest.NewRequest(http.MethodGet, "/sessions", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, rec.Code)
	}

	var resp map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to decode: %v", err)
	}
	if resp["status"] != "ok" {
		t.Errorf("status = %q, want ok", resp["status"])
	}
	if _, ok := resp["sessions"]; !ok {
		t.Error("missing field sessions")
	}
	if _, ok := resp["count"]; !ok {
		t.Error("missing field count")
	}
	if _, ok := resp["items"]; ok {
		t.Error("unexpected field items")
	}
}

func TestHandleSearchShadow(t *testing.T) {
	mux := http.NewServeMux()
	srv := setupTestServer()
	srv.RegisterRoutes(mux)

	body := `{"user_input":"hello","chat_session_id":"sess-123","top_k":5}`
	req := httptest.NewRequest(http.MethodPost, "/search", bytes.NewReader([]byte(body)))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, rec.Code)
	}

	var resp map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to decode: %v", err)
	}
	// 0.8 shape: items, injection_text, memory_count, fallback_count, has_fallback, total_count
	for _, key := range []string{"items", "injection_text", "memory_count", "fallback_count", "has_fallback", "total_count"} {
		if _, ok := resp[key]; !ok {
			t.Errorf("missing field %q", key)
		}
	}
}

func TestHandleSearchShadowBadJSON(t *testing.T) {
	mux := http.NewServeMux()
	srv := setupTestServer()
	srv.RegisterRoutes(mux)

	req := httptest.NewRequest(http.MethodPost, "/search", bytes.NewReader([]byte("not json")))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("expected status %d, got %d", http.StatusBadRequest, rec.Code)
	}
}

func TestHandleRetrievalIndexSnapshotShadow(t *testing.T) {
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
	if resp["status"] != "empty" {
		t.Errorf("status = %q, want empty", resp["status"])
	}
	if resp["chat_session_id"] != "sess-123" {
		t.Errorf("chat_session_id = %q, want sess-123", resp["chat_session_id"])
	}

	docs, ok := resp["documents"].([]any)
	if !ok {
		t.Fatalf("documents field missing or wrong type: %T", resp["documents"])
	}
	if len(docs) != 0 {
		t.Fatalf("documents length = %d, want 0 for empty shadow snapshot", len(docs))
	}
	schema, ok := resp["document_schema"].(map[string]any)
	if !ok || schema["version"] != "q1a.v1" || schema["index_version"] != "q1e.v1" {
		t.Fatalf("document_schema mismatch: %#v", resp["document_schema"])
	}
}

func TestHandleRetrievalIndexSnapshotMissingParam(t *testing.T) {
	mux := http.NewServeMux()
	srv := setupTestServer()
	srv.RegisterRoutes(mux)

	req := httptest.NewRequest(http.MethodGet, "/retrieval-index/", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	// Go ServeMux does not match an empty path value; returns 404.
	// The handler 400 guard is unreachable at the root pattern edge.
	if rec.Code != http.StatusNotFound {
		t.Errorf("expected status %d, got %d", http.StatusNotFound, rec.Code)
	}
}

func TestHandleActiveStatesShadow(t *testing.T) {
	mux := http.NewServeMux()
	srv := setupTestServer()
	srv.RegisterRoutes(mux)

	req := httptest.NewRequest(http.MethodGet, "/active-states/sess-456", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, rec.Code)
	}

	var resp map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to decode: %v", err)
	}
	if resp["status"] != "ok" {
		t.Errorf("status = %q, want ok", resp["status"])
	}
	if resp["chat_session_id"] != "sess-456" {
		t.Errorf("chat_session_id = %q, want sess-456", resp["chat_session_id"])
	}
	if _, ok := resp["states"]; !ok {
		t.Error("missing field states")
	}
	if _, ok := resp["count"]; !ok {
		t.Error("missing field count")
	}
}

func TestHandleAuditShadow(t *testing.T) {
	mux := http.NewServeMux()
	srv := setupTestServer()
	srv.RegisterRoutes(mux)

	req := httptest.NewRequest(http.MethodGet, "/audit", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, rec.Code)
	}

	var resp map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to decode: %v", err)
	}
	if resp["status"] != "ok" {
		t.Errorf("status = %q, want ok", resp["status"])
	}
	if _, ok := resp["items"]; !ok {
		t.Error("missing field items")
	}
	if _, ok := resp["limit"]; !ok {
		t.Error("missing field limit")
	}
	if _, ok := resp["total"]; !ok {
		t.Error("missing field total")
	}
	if _, ok := resp["count"]; ok {
		t.Error("unexpected field count")
	}
}

// ---------------------------------------------------------------------------
// DTO decode integration via search
// ---------------------------------------------------------------------------

func TestHandleSearchDecodesSearchRequestDTO(t *testing.T) {
	mux := http.NewServeMux()
	srv := setupTestServer()
	srv.RegisterRoutes(mux)

	body := `{"user_input":"test query","chat_session_id":"sess-abc","top_k":10,"wing":"left"}`
	req := httptest.NewRequest(http.MethodPost, "/search", bytes.NewReader([]byte(body)))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, rec.Code)
	}

	// Verify the handler returned the expected shadow shape
	var resp map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to decode: %v", err)
	}
	if _, ok := resp["items"]; !ok {
		t.Error("missing field items")
	}
}

// ---------------------------------------------------------------------------
// Proxy skeleton tests
// ---------------------------------------------------------------------------

func TestServerProxyPluginMainValidEndpointCallsUpstream(t *testing.T) {
	mux := http.NewServeMux()
	srv := setupTestServer()
	srv.RegisterRoutes(mux)

	oldClient := proxyHTTPClient
	proxyHTTPClient = &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		return &http.Response{
			StatusCode: http.StatusOK,
			Header:     make(http.Header),
			Body:       io.NopCloser(bytes.NewReader([]byte(`{"choices":[{"message":{"content":"ok"}}]}`))),
		}, nil
	})}
	defer func() { proxyHTTPClient = oldClient }()

	body := `{"provider":"openai","endpoint":"https://api.example.com/v1","model":"gpt-4","api_key":"sk-test","messages":[{"role":"user","content":"hi"}]}`
	req := httptest.NewRequest(http.MethodPost, "/proxy/plugin-main", bytes.NewReader([]byte(body)))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("expected status %d, got %d", http.StatusOK, rec.Code)
	}
}

func TestHandleProxyPluginMainInvalidEndpointReturns400(t *testing.T) {
	mux := http.NewServeMux()
	srv := setupTestServer()
	srv.RegisterRoutes(mux)

	body := `{"endpoint":"https://localhost/v1","model":"gpt-4","api_key":"sk-test"}`
	req := httptest.NewRequest(http.MethodPost, "/proxy/plugin-main", bytes.NewReader([]byte(body)))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("expected status %d, got %d", http.StatusBadRequest, rec.Code)
	}
}

func TestHandleProxyPluginMainMissingEndpointReturns400(t *testing.T) {
	mux := http.NewServeMux()
	srv := setupTestServer()
	srv.RegisterRoutes(mux)

	body := `{"model":"gpt-4"}`
	req := httptest.NewRequest(http.MethodPost, "/proxy/plugin-main", bytes.NewReader([]byte(body)))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("expected status %d, got %d", http.StatusBadRequest, rec.Code)
	}
}

// ---------------------------------------------------------------------------
// Prepare-turn shadow skeleton tests
// ---------------------------------------------------------------------------

func TestHandlePrepareTurnShadow(t *testing.T) {
	mux := http.NewServeMux()
	srv := setupTestServer()
	srv.RegisterRoutes(mux)

	body := `{"chat_session_id":"sess-abc","raw_user_input":"hello"}`
	req := httptest.NewRequest(http.MethodPost, "/prepare-turn", bytes.NewReader([]byte(body)))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, rec.Code)
	}
	var resp map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to decode: %v", err)
	}
	if resp["status"] != "ok" {
		t.Errorf("status = %q, want ok", resp["status"])
	}
	if resp["source"] != "shadow" {
		t.Errorf("source = %q, want shadow", resp["source"])
	}
	if resp["chat_session_id"] != "sess-abc" {
		t.Errorf("chat_session_id = %q, want sess-abc", resp["chat_session_id"])
	}
	if _, ok := resp["generation_packet"]; !ok {
		t.Error("missing generation_packet")
	}
	gp, ok := resp["generation_packet"].(map[string]any)
	if !ok {
		t.Fatalf("invalid generation_packet type")
	}
	if gp["packet_mode"] != "store_backed_shadow" {
		t.Errorf("packet_mode = %v, want store_backed_shadow", gp["packet_mode"])
	}
	if degraded, ok := gp["degraded"].(bool); !ok || degraded {
		t.Errorf("degraded = %v, want false", gp["degraded"])
	}
	if _, ok := resp["note"]; !ok {
		t.Error("missing note field")
	}
}

func TestHandlePrepareTurnMissingSessionID(t *testing.T) {
	mux := http.NewServeMux()
	srv := setupTestServer()
	srv.RegisterRoutes(mux)

	body := `{"raw_user_input":"hello"}`
	req := httptest.NewRequest(http.MethodPost, "/prepare-turn", bytes.NewReader([]byte(body)))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("expected status %d, got %d", http.StatusBadRequest, rec.Code)
	}
}

// ---------------------------------------------------------------------------
// Complete-turn shadow skeleton tests
// ---------------------------------------------------------------------------

func TestHandleCompleteTurnShadow(t *testing.T) {
	mux := http.NewServeMux()
	srv := setupTestServer()
	srv.RegisterRoutes(mux)

	body := `{"chat_session_id":"sess-xyz","turn_index":7,"user_input":"hi","assistant_content":"hello"}`
	req := httptest.NewRequest(http.MethodPost, "/complete-turn", bytes.NewReader([]byte(body)))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, rec.Code)
	}
	var resp map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to decode: %v", err)
	}
	if resp["status"] != "ok" {
		t.Errorf("status = %q, want ok", resp["status"])
	}
	if resp["source"] != "shadow" {
		t.Errorf("source = %q, want shadow", resp["source"])
	}
	if resp["chat_session_id"] != "sess-xyz" {
		t.Errorf("chat_session_id = %q, want sess-xyz", resp["chat_session_id"])
	}
	if _, ok := resp["trace_handoff"]; !ok {
		t.Error("missing trace_handoff")
	}
	if _, ok := resp["note"]; !ok {
		t.Error("missing note")
	}
}

func TestHandleCompleteTurnMissingSessionID(t *testing.T) {
	mux := http.NewServeMux()
	srv := setupTestServer()
	srv.RegisterRoutes(mux)

	body := `{"turn_index":7,"user_input":"hi"}`
	req := httptest.NewRequest(http.MethodPost, "/complete-turn", bytes.NewReader([]byte(body)))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("expected status %d, got %d", http.StatusBadRequest, rec.Code)
	}
}

func TestSeq02CoreRouteFlowHealthWakeupSearchTurnsCompleteRollbackStats(t *testing.T) {
	mux := http.NewServeMux()
	srv := setupTestServer()
	srv.RegisterRoutes(mux)

	// Health
	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("health status = %d, want 200", rec.Code)
	}
	var health map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &health); err != nil {
		t.Fatalf("health decode: %v", err)
	}
	if health["status"] != "ok" {
		t.Errorf("health status = %v, want ok", health["status"])
	}

	// Wakeup
	req = httptest.NewRequest(http.MethodGet, "/wakeup", nil)
	rec = httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("wakeup status = %d, want 200", rec.Code)
	}
	var wakeup map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &wakeup); err != nil {
		t.Fatalf("wakeup decode: %v", err)
	}
	if wakeup["status"] != "ok" {
		t.Errorf("wakeup status = %v, want ok", wakeup["status"])
	}

	// Search
	body := `{"user_input":"hello","chat_session_id":"sess-flow","top_k":5}`
	req = httptest.NewRequest(http.MethodPost, "/search", bytes.NewReader([]byte(body)))
	req.Header.Set("Content-Type", "application/json")
	rec = httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("search status = %d, want 200", rec.Code)
	}
	var search map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &search); err != nil {
		t.Fatalf("search decode: %v", err)
	}
	if _, ok := search["items"]; !ok {
		t.Errorf("search missing items")
	}

	// Prepare-turn (2.0 route)
	body = `{"chat_session_id":"sess-flow","turn_index":1,"raw_user_input":"hello"}`
	req = httptest.NewRequest(http.MethodPost, "/prepare-turn", bytes.NewReader([]byte(body)))
	req.Header.Set("Content-Type", "application/json")
	rec = httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("prepare-turn status = %d, want 200", rec.Code)
	}
	var prep map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &prep); err != nil {
		t.Fatalf("prepare-turn decode: %v", err)
	}
	if prep["status"] != "ok" {
		t.Errorf("prepare-turn status = %v, want ok", prep["status"])
	}

	// Complete-turn (2.0 route)
	body = `{"chat_session_id":"sess-flow","turn_index":1,"user_input":"hello","assistant_content":"world"}`
	req = httptest.NewRequest(http.MethodPost, "/complete-turn", bytes.NewReader([]byte(body)))
	req.Header.Set("Content-Type", "application/json")
	rec = httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("complete-turn status = %d, want 200", rec.Code)
	}
	var comp map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &comp); err != nil {
		t.Fatalf("complete-turn decode: %v", err)
	}
	if comp["status"] != "ok" {
		t.Errorf("complete-turn status = %v, want ok", comp["status"])
	}

	// Legacy /turns returns shadow_guard 503
	body = `{"chat_session_id":"sess-flow"}`
	req = httptest.NewRequest(http.MethodPost, "/turns", bytes.NewReader([]byte(body)))
	req.Header.Set("Content-Type", "application/json")
	rec = httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusServiceUnavailable {
		t.Errorf("/turns status = %d, want 503", rec.Code)
	}

	// Legacy /turns/complete returns shadow_guard 503
	req = httptest.NewRequest(http.MethodPost, "/turns/complete", bytes.NewReader([]byte(body)))
	req.Header.Set("Content-Type", "application/json")
	rec = httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusServiceUnavailable {
		t.Errorf("/turns/complete status = %d, want 503", rec.Code)
	}

	// Rollback
	req = httptest.NewRequest(http.MethodDelete, "/rollback/5?chat_session_id=sess-flow&req_source=adapter", nil)
	rec = httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("rollback status = %d, want 200", rec.Code)
	}
	var rb map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &rb); err != nil {
		t.Fatalf("rollback decode: %v", err)
	}
	if rb["status"] != "ok" {
		t.Errorf("rollback status = %v, want ok", rb["status"])
	}

	// Stats
	req = httptest.NewRequest(http.MethodGet, "/stats", nil)
	rec = httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("stats status = %d, want 200", rec.Code)
	}
	var stats map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &stats); err != nil {
		t.Fatalf("stats decode: %v", err)
	}
	for _, key := range []string{"chat_logs", "memories", "kg_triples"} {
		if _, ok := stats[key]; !ok {
			t.Errorf("stats missing field %q", key)
		}
	}
}

func TestSeq123P78Pre125PreparationScopeMarkers(t *testing.T) {
	srv := setupTestServer()
	mux := http.NewServeMux()
	srv.RegisterRoutes(mux)

	req := httptest.NewRequest(http.MethodGet, "/ready", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("readiness status = %d, want 200", rec.Code)
	}
	var resp readyResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if resp.Mode != "shadow" {
		t.Errorf("mode = %q, want shadow", resp.Mode)
	}
	if resp.Checks["live_cutover"] != "disabled" {
		t.Errorf("live_cutover = %q, want disabled", resp.Checks["live_cutover"])
	}
	if resp.Checks["shadow_mode"] != "active" {
		t.Errorf("shadow_mode = %q, want active", resp.Checks["shadow_mode"])
	}

	// Aggressive live/cutover modes must remain blocked before 12.5.
	for _, mode := range []config.Mode{config.ModeLive, config.ModeCutover} {
		cfg := config.Default()
		cfg.Mode = mode
		mux2 := http.NewServeMux()
		srv2 := NewServer(cfg)
		srv2.RegisterRoutes(mux2)
		req2 := httptest.NewRequest(http.MethodGet, "/ready", nil)
		rec2 := httptest.NewRecorder()
		mux2.ServeHTTP(rec2, req2)
		if rec2.Code != http.StatusServiceUnavailable {
			t.Errorf("mode %s: readiness status = %d, want 503", mode, rec2.Code)
		}
		var r2 readyResponse
		if err := json.Unmarshal(rec2.Body.Bytes(), &r2); err != nil {
			t.Fatalf("mode %s: decode: %v", mode, err)
		}
		if r2.Checks["live_cutover"] != "disabled" {
			t.Errorf("mode %s: live_cutover = %q, want disabled", mode, r2.Checks["live_cutover"])
		}
		if r2.Ready {
			t.Errorf("mode %s: Ready should be false", mode)
		}
	}
}
