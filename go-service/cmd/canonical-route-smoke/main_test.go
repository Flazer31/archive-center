package main

import (
	"encoding/json"
	"net/http"
	"strings"
	"testing"

	"github.com/risulongmemory/archive-center-go/internal/config"
	"github.com/risulongmemory/archive-center-go/internal/httpapi"
)

func TestBuildSmokeReportGuarded(t *testing.T) {
	r := buildSmokeReport(false, "", "sess-1")
	if r == nil {
		t.Fatal("expected non-nil report")
	}
	if r.Status != "guarded" {
		t.Fatalf("status = %q, want guarded", r.Status)
	}
	if r.Error != "" {
		t.Fatalf("unexpected error: %q", r.Error)
	}
}

func TestBuildSmokeReportMissingDSN(t *testing.T) {
	r := buildSmokeReport(true, "", "sess-1")
	if r == nil {
		t.Fatal("expected non-nil report")
	}
	if r.Status != "failed" {
		t.Fatalf("status = %q, want failed", r.Status)
	}
	if !strings.Contains(r.Error, "missing dsn") {
		t.Fatalf("expected missing dsn error, got %q", r.Error)
	}
}

func TestBuildSmokeReportExecuteWithDSN(t *testing.T) {
	r := buildSmokeReport(true, "user:pass@tcp(localhost:3306)/db", "sess-1")
	if r != nil {
		t.Fatalf("expected nil report, got %+v", r)
	}
}

func TestRunSmokeAllPass(t *testing.T) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		if r.Method == http.MethodPost {
			_ = json.NewEncoder(w).Encode(map[string]any{"source": "shadow", "saved": true})
			return
		}
		if strings.Contains(r.URL.Path, "effective-inputs") {
			_ = json.NewEncoder(w).Encode(map[string]any{"source": "shadow", "found": true})
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"source": "shadow", "count": 1})
	})

	report := runSmoke(handler, "sess-test")
	if report.Status != "ok" {
		t.Fatalf("status = %q, want ok", report.Status)
	}
	if len(report.Routes) != 16 {
		t.Fatalf("expected 16 routes, got %d", len(report.Routes))
	}
	for _, route := range report.Routes {
		if !route.Pass {
			t.Errorf("route %s %s failed with status %d", route.Method, route.Name, route.Status)
		}
	}
}

func TestRunSmokePostFailure(t *testing.T) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPost {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(map[string]any{"source": "shadow"})
	})

	report := runSmoke(handler, "sess-test")
	if report.Status != "failed" {
		t.Fatalf("status = %q, want failed", report.Status)
	}
	postFailures := 0
	for _, route := range report.Routes {
		if route.Method == "POST" && !route.Pass {
			postFailures++
		}
	}
	if postFailures != 8 {
		t.Fatalf("expected 8 POST failures, got %d", postFailures)
	}
}

func TestRunSmokeGetUnexpectedStatus(t *testing.T) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet {
			w.WriteHeader(http.StatusForbidden)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(map[string]any{"source": "shadow", "saved": true})
	})

	report := runSmoke(handler, "sess-test")
	if report.Status != "failed" {
		t.Fatalf("status = %q, want failed", report.Status)
	}
	getFailures := 0
	for _, route := range report.Routes {
		if route.Method == "GET" && !route.Pass {
			getFailures++
		}
	}
	if getFailures != 8 {
		t.Fatalf("expected 8 GET failures, got %d", getFailures)
	}
}

func TestRunSmokeWithRealServerNoopStore(t *testing.T) {
	cfg := config.Default()
	cfg.StoreMode = config.StoreModeDualShadow
	srv := httpapi.NewServer(cfg)
	mux := http.NewServeMux()
	srv.RegisterRoutes(mux)

	report := runSmoke(mux, "sess-real")
	if report.Status != "failed" {
		t.Fatalf("status = %q, want failed because noop reads must not prove persistence", report.Status)
	}
	readFailures := 0
	for _, route := range report.Routes {
		if route.Method == http.MethodGet && !route.Pass {
			readFailures++
		}
	}
	if readFailures == 0 {
		t.Fatal("expected noop read failures")
	}
}
