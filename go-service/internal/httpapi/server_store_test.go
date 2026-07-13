package httpapi

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/risulongmemory/archive-center-go/internal/config"
	"github.com/risulongmemory/archive-center-go/internal/store"
	"github.com/risulongmemory/archive-center-go/internal/vector"
)

func TestNewServerDefaultStoreIsNoop(t *testing.T) {
	cfg := config.Default()
	srv := NewServer(cfg)

	if srv.Store == nil {
		t.Fatal("Store should not be nil")
	}

	// A noop store returns nil for SaveChatLog.
	if err := srv.Store.SaveChatLog(context.Background(), &store.ChatLog{ChatSessionID: "s1", TurnIndex: 1}); err != nil {
		t.Errorf("unexpected error from default noop store: %v", err)
	}

	// It does not implement ShadowStatusReporter.
	if _, ok := srv.Store.(store.ShadowStatusReporter); ok {
		t.Error("default store should not implement ShadowStatusReporter")
	}
}

func TestNewServerDualShadowStore(t *testing.T) {
	cfg := config.Default()
	cfg.StoreMode = config.StoreModeDualShadow
	srv := NewServer(cfg)

	if srv.Store == nil {
		t.Fatal("Store should not be nil")
	}

	// It must implement ShadowStatusReporter.
	rep, ok := srv.Store.(store.ShadowStatusReporter)
	if !ok {
		t.Fatal("dual_shadow store should implement ShadowStatusReporter")
	}

	// SaveChatLog should succeed (both sides are noop).
	if err := srv.Store.SaveChatLog(context.Background(), &store.ChatLog{ChatSessionID: "s1", TurnIndex: 1}); err != nil {
		t.Errorf("unexpected error from dual-write store: %v", err)
	}

	failures, lastErr := rep.ShadowStatus()
	if failures != 0 {
		t.Errorf("expected 0 shadow failures, got %d", failures)
	}
	if lastErr != nil {
		t.Errorf("expected nil lastErr, got %v", lastErr)
	}
}

func TestNewServerIgnoresMariaDBDSNByDefault(t *testing.T) {
	// Even with a MariaDB DSN present, the default store mode is noop.
	// No real connection should be attempted.
	cfg := config.Default()
	cfg.MariaDBDSN = "user:pass@tcp(127.0.0.1:3306)/archive_center?parseTime=true"
	cfg.Readiness.MariaDBConfigured = true
	srv := NewServer(cfg)

	if _, ok := srv.Store.(store.ShadowStatusReporter); ok {
		t.Error("store should remain noop even when MariaDB DSN is present")
	}
}

func TestNewServerMariaDBShadowStore(t *testing.T) {
	cfg := config.Default()
	cfg.StoreMode = config.StoreModeMariaDBShadow
	cfg.MariaDBDSN = "user:pass@tcp(127.0.0.1:3306)/archive_center?parseTime=true"
	cfg.Readiness.MariaDBConfigured = true
	srv := NewServer(cfg)

	if srv.Store == nil {
		t.Fatal("Store should not be nil")
	}
	if srv.StoreOpenError != nil {
		t.Fatalf("StoreOpenError = %v, want nil", srv.StoreOpenError)
	}
	if _, ok := srv.Store.(store.ShadowStatusReporter); !ok {
		t.Fatal("mariadb_shadow store should use dual-write wrapper and implement ShadowStatusReporter")
	}
}

func TestNewServerMariaDBShadowWithoutDSNRecordsOpenError(t *testing.T) {
	cfg := config.Default()
	cfg.StoreMode = config.StoreModeMariaDBShadow
	srv := NewServer(cfg)

	if srv.Store == nil {
		t.Fatal("Store should fall back to noop store")
	}
	if !errors.Is(srv.StoreOpenError, store.ErrNotEnabled) {
		t.Fatalf("StoreOpenError = %v, want ErrNotEnabled", srv.StoreOpenError)
	}
	if _, ok := srv.Store.(store.ShadowStatusReporter); ok {
		t.Fatal("store should not report shadow status when MariaDB shadow open failed")
	}
}

func TestNewServerStoreModeExplicitNoop(t *testing.T) {
	cfg := config.Default()
	cfg.StoreMode = config.StoreModeNoop
	srv := NewServer(cfg)

	if _, ok := srv.Store.(store.ShadowStatusReporter); ok {
		t.Error("explicit noop store should not implement ShadowStatusReporter")
	}
}

func TestCompleteTurnNoopSaveDisabled(t *testing.T) {
	cfg := config.Default()
	srv := NewServer(cfg)
	mux := http.NewServeMux()
	srv.RegisterRoutes(mux)
	ts := httptest.NewServer(mux)
	defer ts.Close()

	body := map[string]any{
		"chat_session_id":   "test-session",
		"turn_index":        1,
		"user_input":        "hello",
		"assistant_content": "world",
	}
	b, _ := json.Marshal(body)
	resp, err := http.Post(ts.URL+"/complete-turn", "application/json", bytes.NewReader(b))
	if err != nil {
		t.Fatalf("post failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200", resp.StatusCode)
	}

	var result map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		t.Fatalf("decode failed: %v", err)
	}

	if result["save_ok"] != false {
		t.Errorf("save_ok = %v, want false", result["save_ok"])
	}
	if !strings.Contains(result["save_error"].(string), "shadow_mode") {
		t.Errorf("save_error = %q, want shadow_mode reference", result["save_error"])
	}
	if result["source"] != "shadow" {
		t.Errorf("source = %q, want shadow", result["source"])
	}
}

func TestCompleteTurnDualShadowSaveOK(t *testing.T) {
	cfg := config.Default()
	cfg.StoreMode = config.StoreModeDualShadow
	srv := NewServer(cfg)
	mux := http.NewServeMux()
	srv.RegisterRoutes(mux)
	ts := httptest.NewServer(mux)
	defer ts.Close()

	body := map[string]any{
		"chat_session_id":   "session-dual",
		"turn_index":        2,
		"user_input":        "user query",
		"assistant_content": "assistant answer",
	}
	b, _ := json.Marshal(body)
	resp, err := http.Post(ts.URL+"/complete-turn", "application/json", bytes.NewReader(b))
	if err != nil {
		t.Fatalf("post failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200", resp.StatusCode)
	}

	var result map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		t.Fatalf("decode failed: %v", err)
	}

	if result["save_ok"] != true {
		t.Errorf("save_ok = %v, want true", result["save_ok"])
	}
	if result["save_error"] != "" {
		t.Errorf("save_error = %q, want empty", result["save_error"])
	}
	if result["source"] != "dual_shadow" {
		t.Errorf("source = %q, want dual_shadow", result["source"])
	}

	// Ensure no live/cutover/authority claims.
	raw, _ := json.Marshal(result)
	lower := strings.ToLower(string(raw))
	for _, forbidden := range []string{"live", "cutover", "authority"} {
		if strings.Contains(lower, forbidden) {
			t.Errorf("response contains forbidden word %q", forbidden)
		}
	}

	// Verify store implements ShadowStatusReporter.
	rep, ok := srv.Store.(store.ShadowStatusReporter)
	if !ok {
		t.Fatal("store should implement ShadowStatusReporter in dual_shadow")
	}
	failures, lastErr := rep.ShadowStatus()
	if failures != 0 {
		t.Errorf("shadow failures = %d, want 0", failures)
	}
	if lastErr != nil {
		t.Errorf("shadow lastErr = %v, want nil", lastErr)
	}
}

func TestEffectiveInputsDualShadowSaveOK(t *testing.T) {
	cfg := config.Default()
	cfg.StoreMode = config.StoreModeDualShadow
	srv := NewServer(cfg)
	mux := http.NewServeMux()
	srv.RegisterRoutes(mux)
	ts := httptest.NewServer(mux)
	defer ts.Close()

	body := map[string]any{
		"chat_session_id": "session-eff-dual",
		"turn_index":      5,
		"effective_input": "refined user intent",
	}
	b, _ := json.Marshal(body)
	resp, err := http.Post(ts.URL+"/effective-inputs", "application/json", bytes.NewReader(b))
	if err != nil {
		t.Fatalf("post failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200", resp.StatusCode)
	}

	var result map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		t.Fatalf("decode failed: %v", err)
	}

	if result["status"] != "ok" {
		t.Errorf("status = %q, want ok", result["status"])
	}
	if result["save_ok"] != true {
		t.Errorf("save_ok = %v, want true", result["save_ok"])
	}
	if result["save_error"] != "" {
		t.Errorf("save_error = %q, want empty", result["save_error"])
	}
	if result["source"] != "dual_shadow" {
		t.Errorf("source = %q, want dual_shadow", result["source"])
	}

	rep, ok := srv.Store.(store.ShadowStatusReporter)
	if !ok {
		t.Fatal("store should implement ShadowStatusReporter in dual_shadow")
	}
	failures, lastErr := rep.ShadowStatus()
	if failures != 0 {
		t.Errorf("shadow failures = %d, want 0", failures)
	}
	if lastErr != nil {
		t.Errorf("shadow lastErr = %v, want nil", lastErr)
	}
}

func TestNewServerMariaDBReadShadowStore(t *testing.T) {
	cfg := config.Default()
	cfg.StoreMode = config.StoreModeMariaDBReadShadow
	cfg.MariaDBDSN = "user:pass@tcp(127.0.0.1:3306)/archive_center?parseTime=true"
	cfg.Readiness.MariaDBConfigured = true
	srv := NewServer(cfg)

	if srv.Store == nil {
		t.Fatal("Store should not be nil")
	}
	if srv.StoreOpenError != nil {
		t.Fatalf("StoreOpenError = %v, want nil", srv.StoreOpenError)
	}
	// It must NOT implement ShadowStatusReporter because it is not a shadow write store.
	if _, ok := srv.Store.(store.ShadowStatusReporter); ok {
		t.Fatal("mariadb_read_shadow store should NOT implement ShadowStatusReporter")
	}
	// It must not allow writes.
	if err := srv.Store.SaveChatLog(context.Background(), &store.ChatLog{ChatSessionID: "s1", TurnIndex: 1}); !errors.Is(err, store.ErrNotEnabled) {
		t.Fatalf("expected ErrNotEnabled for writes, got %v", err)
	}
}

func TestNewServerMariaDBReadShadowWithoutDSNFallsBack(t *testing.T) {
	cfg := config.Default()
	cfg.StoreMode = config.StoreModeMariaDBReadShadow
	srv := NewServer(cfg)

	if srv.Store == nil {
		t.Fatal("Store should fall back to noop store")
	}
	if !errors.Is(srv.StoreOpenError, store.ErrNotEnabled) {
		t.Fatalf("StoreOpenError = %v, want ErrNotEnabled", srv.StoreOpenError)
	}
	if _, ok := srv.Store.(store.ShadowStatusReporter); ok {
		t.Fatal("store should not report shadow status when MariaDB read shadow open failed")
	}
}

func TestNewServerMariaDBReadShadowDoesNotEnableShadowWrites(t *testing.T) {
	cfg := config.Default()
	cfg.StoreMode = config.StoreModeMariaDBReadShadow
	cfg.MariaDBDSN = "user:pass@tcp(127.0.0.1:3306)/archive_center?parseTime=true"
	srv := NewServer(cfg)
	if srv.usesShadowWriteStore() {
		t.Error("usesShadowWriteStore should be false for mariadb_read_shadow")
	}
}

func TestNewServerMariaDBAuthorityEnablesStoreWrites(t *testing.T) {
	cfg := config.Default()
	cfg.StoreMode = config.StoreModeMariaDBAuthority
	cfg.MariaDBDSN = "user:pass@tcp(127.0.0.1:3306)/archive_center?parseTime=true"
	srv := NewServer(cfg)

	if srv.Store == nil {
		t.Fatal("Store should not be nil")
	}
	if srv.StoreOpenError != nil {
		t.Fatalf("StoreOpenError = %v, want nil", srv.StoreOpenError)
	}
	if !srv.usesShadowWriteStore() {
		t.Error("mariadb_authority should enable the migrated Store write path")
	}
	if _, ok := srv.Store.(store.ShadowStatusReporter); ok {
		t.Fatal("mariadb_authority should be direct authority store, not dual shadow reporter")
	}
}

func TestNewServerDefaultVectorIsFake(t *testing.T) {
	cfg := config.Default()
	srv := NewServer(cfg)

	if srv.Vector == nil {
		t.Fatal("Vector should not be nil")
	}
	if srv.VectorOpenError != nil {
		t.Errorf("VectorOpenError should be nil by default, got %v", srv.VectorOpenError)
	}
	// Verify it's the fake store by asserting Search returns ErrNotFound.
	_, err := srv.Vector.Search(context.Background(), "s1", []float32{0.1}, 1, "")
	if !errors.Is(err, vector.ErrNotFound) {
		t.Errorf("expected fake store to return ErrNotFound, got %v", err)
	}
}

func TestNewServerChromaEndpointUsesChromaVectorStore(t *testing.T) {
	chroma := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.Method + " " + r.URL.Path {
		case "GET /api/v2/tenants/default_tenant/databases/default_database/collections/archive_center_vectors":
			_, _ = w.Write([]byte(`{"id":"collection-1","name":"archive_center_vectors"}`))
		case "POST /api/v2/tenants/default_tenant/databases/default_database/collections/collection-1/query":
			_, _ = w.Write([]byte(`{"ids":[["doc-1"]],"documents":[["from chroma"]],"metadatas":[[{"chat_session_id":"s1","tier":"memory"}]]}`))
		default:
			http.Error(w, r.Method+" "+r.URL.Path, http.StatusNotFound)
		}
	}))
	defer chroma.Close()

	cfg := config.Default()
	cfg.ChromaEndpoint = chroma.URL
	cfg.Readiness.ChromaConfigured = true
	cfg.RuntimeProfile = config.RuntimeProfileFullLocal
	cfg.VectorMode = config.VectorModeBundled
	cfg.ChromaEnabled = true
	srv := NewServer(cfg)

	if srv.Vector == nil {
		t.Fatal("Vector should not be nil")
	}
	if srv.VectorOpenError != nil {
		t.Fatalf("VectorOpenError = %v, want nil", srv.VectorOpenError)
	}
	docs, err := srv.Vector.Search(context.Background(), "s1", []float32{0.1}, 1, "")
	if err != nil {
		t.Fatalf("Search should use Chroma vector store: %v", err)
	}
	if len(docs) != 1 || docs[0].DocumentText != "from chroma" {
		t.Fatalf("unexpected docs: %+v", docs)
	}
}
