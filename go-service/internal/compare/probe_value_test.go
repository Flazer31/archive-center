package compare

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestHarness_ProbeValue_ExactMatch(t *testing.T) {
	pySrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, map[string]any{"status": "ok", "count": 1})
	}))
	goSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, map[string]any{"status": "ok", "count": 1})
	}))
	defer pySrv.Close()
	defer goSrv.Close()

	h := NewHarness(pySrv.URL, goSrv.URL)
	res := h.ProbeValue(context.Background(), "GET", "/health", nil, 10)
	if !res.Allowed {
		t.Error("expected allowed")
	}
	if !res.StatusMatch {
		t.Errorf("status mismatch: py=%d go=%d", res.PythonStatus, res.GoStatus)
	}
	if !res.KeysMatch {
		t.Errorf("keys mismatch")
	}
	if !res.ExactJSONMatch {
		t.Error("expected exact JSON match")
	}
	if !res.BehaviorMatch {
		t.Error("expected behavior match")
	}
	if !res.TypeMatch {
		t.Error("expected type match")
	}
	if len(res.Diffs) != 0 {
		t.Errorf("expected no diffs, got %v", res.Diffs)
	}
}

func TestHarness_ProbeValue_ScalarDiff(t *testing.T) {
	pySrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, map[string]any{"status": "ok", "count": 1})
	}))
	goSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, map[string]any{"status": "ok", "count": 2})
	}))
	defer pySrv.Close()
	defer goSrv.Close()

	h := NewHarness(pySrv.URL, goSrv.URL)
	res := h.ProbeValue(context.Background(), "GET", "/health", nil, 10)
	if !res.StatusMatch {
		t.Errorf("status mismatch")
	}
	if res.ExactJSONMatch {
		t.Error("expected no exact JSON match")
	}
	if res.BehaviorMatch {
		t.Error("expected no behavior match for scalar diff")
	}
	if !res.TypeMatch {
		t.Error("expected type match despite scalar diff")
	}
	if len(res.Diffs) != 1 {
		t.Fatalf("expected 1 diff, got %d", len(res.Diffs))
	}
	if res.Diffs[0].Type != "scalar" {
		t.Errorf("expected scalar diff, got %s", res.Diffs[0].Type)
	}
}

func TestHarness_ProbeValue_TypeDiff(t *testing.T) {
	pySrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, map[string]any{"count": 1})
	}))
	goSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, map[string]any{"count": "one"})
	}))
	defer pySrv.Close()
	defer goSrv.Close()

	h := NewHarness(pySrv.URL, goSrv.URL)
	res := h.ProbeValue(context.Background(), "GET", "/health", nil, 10)
	if res.ExactJSONMatch {
		t.Error("expected no exact JSON match")
	}
	if res.TypeMatch {
		t.Error("expected no type match")
	}
}

func TestHarness_ProbeValue_VolatileOnlyDiffIsBehaviorMatch(t *testing.T) {
	pySrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, map[string]any{"status": "ok", "generated_at": "2026-05-27T00:00:00Z"})
	}))
	goSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, map[string]any{"status": "ok", "generated_at": "2026-05-27T00:00:01Z"})
	}))
	defer pySrv.Close()
	defer goSrv.Close()

	h := NewHarness(pySrv.URL, goSrv.URL)
	res := h.ProbeValue(context.Background(), "GET", "/health", nil, 10)
	if res.ExactJSONMatch {
		t.Error("expected raw exact JSON mismatch")
	}
	if !res.VolatileOnlyDiffs {
		t.Error("expected volatile-only diff marker")
	}
	if !res.BehaviorMatch {
		t.Error("expected behavior match for volatile-only diff")
	}
}

func TestHarness_ProbeValue_BlocksUnsafeRoute(t *testing.T) {
	h := NewHarness("http://localhost", "http://localhost")
	res := h.ProbeValue(context.Background(), "POST", "/complete-turn", []byte(`{}`), 10)
	if res.Allowed {
		t.Error("expected blocked result")
	}
	if res.Error != ErrUnsafeRoute.Error() {
		t.Errorf("expected ErrUnsafeRoute, got %s", res.Error)
	}
}
