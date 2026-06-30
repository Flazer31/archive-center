package compare

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func writeJSON(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(v)
}

// ---------------------------------------------------------------------------
// Allowlist tests
// ---------------------------------------------------------------------------

func TestIsAllowed_ExactMatch(t *testing.T) {
	if !isAllowed("GET", "/health") {
		t.Error("GET /health should be allowed")
	}
	if !isAllowed("GET", "/ready") {
		t.Error("GET /ready should be allowed")
	}
	if !isAllowed("GET", "/wakeup") {
		t.Error("GET /wakeup should be allowed")
	}
	if !isAllowed("POST", "/search") {
		t.Error("POST /search should be allowed")
	}
	if !isAllowed("GET", "/kg/recall") {
		t.Error("GET /kg/recall should be allowed")
	}
	if !isAllowed("POST", "/kg/recall") {
		t.Error("POST /kg/recall should be allowed")
	}
	if !isAllowed("GET", "/sessions/compare") {
		t.Error("GET /sessions/compare should be allowed")
	}
	if !isAllowed("POST", "/chapters/dry-run") {
		t.Error("POST /chapters/dry-run should be allowed")
	}
	if !isAllowed("POST", "/chapters/search") {
		t.Error("POST /chapters/search should be allowed")
	}
	if !isAllowed("POST", "/episodes/search") {
		t.Error("POST /episodes/search should be allowed")
	}
}

func TestIsAllowed_PrefixMatch(t *testing.T) {
	if !isAllowed("GET", "/active-states/sess-123") {
		t.Error("GET /active-states/{id} should be allowed")
	}
	if !isAllowed("GET", "/retrieval-index/sess-456") {
		t.Error("GET /retrieval-index/{id} should be allowed")
	}
	if !isAllowed("GET", "/metrics/lc1c/sess-789") {
		t.Error("GET /metrics/lc1c/{id} should be allowed")
	}
	if !isAllowed("GET", "/world-rules/sess-abc/inherited") {
		t.Error("GET /world-rules/{id}/inherited should be allowed via prefix")
	}
}

func TestIsAllowed_Blocked(t *testing.T) {
	blocked := []struct {
		method string
		path   string
	}{
		{"POST", "/complete-turn"},
		{"POST", "/prepare-turn"},
		{"POST", "/turns"},
		{"DELETE", "/rollback/1"},
		{"POST", "/proxy/plugin-main"},
		{"PATCH", "/session/sess-123/active-scope"},
		{"POST", "/config/update"},
		{"GET", "/chapters/some-id"},
		{"POST", "/chapters/generate"},
		{"PATCH", "/episodes/1"},
		{"DELETE", "/episodes/1"},
	}
	for _, tc := range blocked {
		if isAllowed(tc.method, tc.path) {
			t.Errorf("%s %s should be blocked", tc.method, tc.path)
		}
	}
}

// ---------------------------------------------------------------------------
// Harness comparison tests
// ---------------------------------------------------------------------------

func TestHarness_CompareHealth(t *testing.T) {
	pySrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, map[string]any{"status": "ok", "service": "python"})
	}))
	goSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, map[string]any{"status": "ok", "service": "go"})
	}))
	defer pySrv.Close()
	defer goSrv.Close()

	h := NewHarness(pySrv.URL, goSrv.URL)
	res, err := h.Compare(context.Background(), "GET", "/health", nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !res.Allowed {
		t.Error("expected allowed")
	}
	if !res.StatusMatch {
		t.Errorf("status mismatch: py=%d go=%d", res.PythonStatus, res.GoStatus)
	}
	if !res.KeysMatch {
		t.Errorf("keys mismatch: missing=%v extra=%v", res.MissingKeys, res.ExtraKeys)
	}
	if res.DurationMS < 0 {
		t.Error("duration should be non-negative")
	}
}

func TestHarness_CompareSearch(t *testing.T) {
	pySrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, map[string]any{
			"status": "ok",
			"items":  []any{},
			"count":  0,
			"mode":   "python",
		})
	}))
	goSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, map[string]any{
			"status":         "ok",
			"items":          []any{},
			"injection_text": "",
			"memory_count":   0,
			"mode":           "go",
		})
	}))
	defer pySrv.Close()
	defer goSrv.Close()

	h := NewHarness(pySrv.URL, goSrv.URL)
	body := []byte(`{"user_input":"test","chat_session_id":"sess-1","top_k":5}`)
	res, err := h.Compare(context.Background(), "POST", "/search", body)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !res.Allowed {
		t.Error("expected allowed")
	}
	if res.StatusMatch {
		if res.PythonStatus != http.StatusOK {
			t.Errorf("python status = %d, want 200", res.PythonStatus)
		}
		if res.GoStatus != http.StatusOK {
			t.Errorf("go status = %d, want 200", res.GoStatus)
		}
	}
	// Python has "count" which Go lacks; Go has "injection_text" and "memory_count" which Python lacks.
	if len(res.MissingKeys) == 0 {
		t.Error("expected missing keys (count is python-only)")
	}
	if len(res.ExtraKeys) == 0 {
		t.Error("expected extra keys (injection_text/memory_count are go-only)")
	}
}

func TestHarness_BlocksUnsafeRoute(t *testing.T) {
	h := NewHarness("http://localhost", "http://localhost")
	res, err := h.Compare(context.Background(), "POST", "/complete-turn", []byte(`{}`))
	if err != ErrUnsafeRoute {
		t.Errorf("expected ErrUnsafeRoute, got %v", err)
	}
	if res == nil || res.Allowed {
		t.Error("expected blocked result")
	}
}

func TestHarness_BlocksPrepareTurn(t *testing.T) {
	h := NewHarness("http://localhost", "http://localhost")
	_, err := h.Compare(context.Background(), "POST", "/prepare-turn", []byte(`{}`))
	if err != ErrUnsafeRoute {
		t.Errorf("expected ErrUnsafeRoute, got %v", err)
	}
}

func TestHarness_BlocksProxyPluginMain(t *testing.T) {
	h := NewHarness("http://localhost", "http://localhost")
	_, err := h.Compare(context.Background(), "POST", "/proxy/plugin-main", []byte(`{}`))
	if err != ErrUnsafeRoute {
		t.Errorf("expected ErrUnsafeRoute, got %v", err)
	}
}

func TestHarness_BlocksDelete(t *testing.T) {
	h := NewHarness("http://localhost", "http://localhost")
	_, err := h.Compare(context.Background(), "DELETE", "/rollback/1", nil)
	if err != ErrUnsafeRoute {
		t.Errorf("expected ErrUnsafeRoute, got %v", err)
	}
}

func TestHarness_AllowsActiveStates(t *testing.T) {
	pySrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, map[string]any{"status": "ok", "states": []any{}})
	}))
	goSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, map[string]any{"status": "ok", "states": []any{}, "count": 0})
	}))
	defer pySrv.Close()
	defer goSrv.Close()

	h := NewHarness(pySrv.URL, goSrv.URL)
	res, err := h.Compare(context.Background(), "GET", "/active-states/sess-abc", nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !res.Allowed {
		t.Error("expected allowed")
	}
	if !res.StatusMatch {
		t.Errorf("status mismatch: py=%d go=%d", res.PythonStatus, res.GoStatus)
	}
}

func TestHarness_AllowsResumePack(t *testing.T) {
	pySrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, map[string]any{
			"pack_status":    "empty",
			"trigger":        "resume",
			"sources_used":   []any{},
			"layer_count":    0,
			"assembled_text": "",
			"saga":           nil,
			"arc":            nil,
			"chapter":        nil,
			"assembly_note":  "P-4c: read-only long-gap resume pack; not wired into injection or input_context",
		})
	}))
	goSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, map[string]any{
			"pack_status":    "empty",
			"trigger":        "resume",
			"sources_used":   []any{},
			"layer_count":    0,
			"assembled_text": "",
			"saga":           nil,
			"arc":            nil,
			"chapter":        nil,
			"assembly_note":  "P-4c: read-only long-gap resume pack; not wired into injection or input_context",
		})
	}))
	defer pySrv.Close()
	defer goSrv.Close()

	h := NewHarness(pySrv.URL, goSrv.URL)
	res, err := h.Compare(context.Background(), "GET", "/sessions/sess-123/resume-pack?continuity_trigger_mode=resume", nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !res.Allowed {
		t.Error("expected allowed")
	}
	if !res.StatusMatch {
		t.Errorf("status mismatch: py=%d go=%d", res.PythonStatus, res.GoStatus)
	}
	if !res.KeysMatch {
		t.Errorf("keys mismatch: missing=%v extra=%v", res.MissingKeys, res.ExtraKeys)
	}
}
