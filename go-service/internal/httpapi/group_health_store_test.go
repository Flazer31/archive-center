package httpapi

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/risulongmemory/archive-center-go/internal/config"
)

func TestHandleReadyDefaultStoreModeNoop(t *testing.T) {
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

	if resp.Checks["store_mode"] != "noop" {
		t.Errorf("store_mode = %q, want %q", resp.Checks["store_mode"], "noop")
	}
	if resp.Checks["store_shadow"] != "not_configured" {
		t.Errorf("store_shadow = %q, want %q", resp.Checks["store_shadow"], "not_configured")
	}
	if resp.Checks["store_shadow_failures"] != "0" {
		t.Errorf("store_shadow_failures = %q, want %q", resp.Checks["store_shadow_failures"], "0")
	}
	if resp.Checks["store_shadow_last_error"] != "none" {
		t.Errorf("store_shadow_last_error = %q, want %q", resp.Checks["store_shadow_last_error"], "none")
	}
	if resp.Checks["vector_engine_policy"] != "fallback" {
		t.Errorf("vector_engine_policy = %q, want %q", resp.Checks["vector_engine_policy"], "fallback")
	}
	if resp.Checks["mariadb_product_read"] != "disabled" {
		t.Errorf("mariadb_product_read = %q, want %q", resp.Checks["mariadb_product_read"], "disabled")
	}
	if resp.Checks["mariadb_authority"] != "disabled" {
		t.Errorf("mariadb_authority = %q, want %q", resp.Checks["mariadb_authority"], "disabled")
	}
}

func TestHandleReadyDualShadowStore(t *testing.T) {
	cfg := config.Default()
	cfg.StoreMode = config.StoreModeDualShadow

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

	if resp.Checks["store_mode"] != "dual_shadow" {
		t.Errorf("store_mode = %q, want %q", resp.Checks["store_mode"], "dual_shadow")
	}
	if resp.Checks["store_shadow"] != "active" {
		t.Errorf("store_shadow = %q, want %q", resp.Checks["store_shadow"], "active")
	}
	if resp.Checks["store_shadow_failures"] != "0" {
		t.Errorf("store_shadow_failures = %q, want %q", resp.Checks["store_shadow_failures"], "0")
	}
	if resp.Checks["store_shadow_last_error"] != "none" {
		t.Errorf("store_shadow_last_error = %q, want %q", resp.Checks["store_shadow_last_error"], "none")
	}
	if resp.Checks["store_open_error"] != "none" {
		t.Errorf("store_open_error = %q, want %q", resp.Checks["store_open_error"], "none")
	}
	if resp.Checks["vector_engine_policy"] != "fallback" {
		t.Errorf("vector_engine_policy = %q, want %q", resp.Checks["vector_engine_policy"], "fallback")
	}
}

func TestHandleReadyMariaDBShadowStore(t *testing.T) {
	cfg := config.Default()
	cfg.StoreMode = config.StoreModeMariaDBShadow
	cfg.MariaDBDSN = "user:pass@tcp(127.0.0.1:3306)/archive_center?parseTime=true"
	cfg.Readiness.MariaDBConfigured = true

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

	if resp.Checks["store_mode"] != "mariadb_shadow" {
		t.Errorf("store_mode = %q, want %q", resp.Checks["store_mode"], "mariadb_shadow")
	}
	if resp.Checks["mariadb"] != "configured" {
		t.Errorf("mariadb = %q, want %q", resp.Checks["mariadb"], "configured")
	}
	if resp.Checks["store_shadow"] != "active" {
		t.Errorf("store_shadow = %q, want %q", resp.Checks["store_shadow"], "active")
	}
	if resp.Checks["store_open_error"] != "none" {
		t.Errorf("store_open_error = %q, want %q", resp.Checks["store_open_error"], "none")
	}
	if resp.Checks["vector_engine_policy"] != "fallback" {
		t.Errorf("vector_engine_policy = %q, want %q", resp.Checks["vector_engine_policy"], "fallback")
	}
}

func TestHandleReadyMariaDBProductReadFlag(t *testing.T) {
	cfg := config.Default()
	cfg.StoreMode = config.StoreModeMariaDBReadShadow
	cfg.MariaDBDSN = "user:pass@tcp(127.0.0.1:3306)/archive_center?parseTime=true"
	cfg.Readiness.MariaDBConfigured = true
	cfg.MariaDBProductReadEnabled = true

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

	if resp.Checks["store_mode"] != "mariadb_read_shadow" {
		t.Errorf("store_mode = %q, want %q", resp.Checks["store_mode"], "mariadb_read_shadow")
	}
	if resp.Checks["mariadb_product_read"] != "enabled" {
		t.Errorf("mariadb_product_read = %q, want %q", resp.Checks["mariadb_product_read"], "enabled")
	}
	if resp.Checks["mariadb_authority"] != "disabled" {
		t.Errorf("mariadb_authority = %q, want %q", resp.Checks["mariadb_authority"], "disabled")
	}
}

func TestHandleReadyMariaDBAuthorityStore(t *testing.T) {
	cfg := config.Default()
	cfg.StoreMode = config.StoreModeMariaDBAuthority
	cfg.MariaDBDSN = "user:pass@tcp(127.0.0.1:3306)/archive_center?parseTime=true"
	cfg.Readiness.MariaDBConfigured = true

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

	if resp.Checks["store_mode"] != "mariadb_authority" {
		t.Errorf("store_mode = %q, want %q", resp.Checks["store_mode"], "mariadb_authority")
	}
	if resp.Checks["mariadb_product_read"] != "enabled" {
		t.Errorf("mariadb_product_read = %q, want %q", resp.Checks["mariadb_product_read"], "enabled")
	}
	if resp.Checks["mariadb_authority"] != "enabled" {
		t.Errorf("mariadb_authority = %q, want %q", resp.Checks["mariadb_authority"], "enabled")
	}
	if resp.Checks["store_shadow"] != "not_configured" {
		t.Errorf("store_shadow = %q, want %q", resp.Checks["store_shadow"], "not_configured")
	}
}

func TestHandleReadyLiveModeStoreChecksStillPresent(t *testing.T) {
	cfg := config.Default()
	cfg.Mode = config.ModeLive
	cfg.StoreMode = config.StoreModeDualShadow

	mux := http.NewServeMux()
	srv := NewServer(cfg)
	srv.RegisterRoutes(mux)

	req := httptest.NewRequest(http.MethodGet, "/ready", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected status %d, got %d", http.StatusServiceUnavailable, rec.Code)
	}

	var resp readyResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if resp.Checks["store_mode"] != "dual_shadow" {
		t.Errorf("store_mode = %q, want %q", resp.Checks["store_mode"], "dual_shadow")
	}
	if resp.Checks["store_shadow"] != "active" {
		t.Errorf("store_shadow = %q, want %q", resp.Checks["store_shadow"], "active")
	}
	if resp.Checks["live_cutover"] != "disabled" {
		t.Errorf("live_cutover = %q, want %q", resp.Checks["live_cutover"], "disabled")
	}
	if resp.Checks["vector_engine_policy"] != "fallback" {
		t.Errorf("vector_engine_policy = %q, want %q", resp.Checks["vector_engine_policy"], "fallback")
	}
}

func TestHandleReadyIgnoresMilvusStubConfigured(t *testing.T) {
	cfg := config.Default()
	cfg.MilvusStubEnabled = true
	cfg.MilvusLitePath = "/tmp/milvus_test.db"

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

	if resp.Checks["vector_engine_policy"] != "fallback" {
		t.Errorf("vector_engine_policy = %q, want %q", resp.Checks["vector_engine_policy"], "fallback")
	}
	if _, ok := resp.Checks["milvus_stub"]; ok {
		t.Errorf("milvus_stub should not be exposed in active readiness: %q", resp.Checks["milvus_stub"])
	}
}

func TestHandleReadyMilvusProductReadNoLongerReportsLiveEnabled(t *testing.T) {
	cfg := config.Default()
	cfg.MilvusSDKEnabled = true
	cfg.MilvusRecallReadEnabled = true
	cfg.MilvusProductReadEnabled = true
	cfg.MilvusEndpoint = "http://127.0.0.1:19530"
	cfg.Readiness.MilvusConfigured = true

	mux := http.NewServeMux()
	srv := setupTestServer()
	srv.Cfg = cfg
	srv.VectorOpenError = nil
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

	if resp.Checks["vector_engine_policy"] != "fallback" {
		t.Errorf("vector_engine_policy = %q, want %q", resp.Checks["vector_engine_policy"], "fallback")
	}
	if _, ok := resp.Checks["milvus_live_enabled"]; ok {
		t.Errorf("milvus_live_enabled should not be exposed in active readiness: %q", resp.Checks["milvus_live_enabled"])
	}
}
