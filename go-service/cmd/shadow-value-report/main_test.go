package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/risulongmemory/archive-center-go/internal/compare"
)

func TestBuildProbes_Count(t *testing.T) {
	probes := buildProbes("test-sid")
	if len(probes) != 55 {
		t.Fatalf("expected 55 probes, got %d", len(probes))
	}
}

func TestBuildProbes_NoR2Paths(t *testing.T) {
	probes := buildProbes("test-sid")
	if hasR2Path(probes) {
		t.Error("probe set must not contain any R2 write/mutation routes")
	}
}

func TestBuildProbes_SkipReasons(t *testing.T) {
	probes := buildProbes("test-sid")
	var skipCount int
	expectedSkips := map[string]string{
		"GET /ready":    "Go-only ops endpoint (not present in Python 0.8)",
		"GET /version":  "Go-only ops endpoint (not present in Python 0.8)",
		"GET /prompts":  "Go-only / 2.0-only prompt filesystem route (not present in Python 0.8)",
		"GET /prompts/": "Go-only / 2.0-only prompt filesystem route (not present in Python 0.8)",
	}
	for _, p := range probes {
		if p.skipReason != "" {
			skipCount++
			endpoint := p.method + " " + p.path
			want, ok := expectedSkips[endpoint]
			if !ok {
				t.Errorf("unexpected skip for %s: %s", endpoint, p.skipReason)
				continue
			}
			if !strings.Contains(p.skipReason, want) {
				t.Errorf("skip reason mismatch for %s: got %q, want containing %q", endpoint, p.skipReason, want)
			}
		}
	}
	if skipCount != 4 {
		t.Fatalf("expected 4 skipped probes, got %d", skipCount)
	}
}

func TestBuildProbes_ComparedCount(t *testing.T) {
	probes := buildProbes("test-sid")
	compared := 0
	for _, p := range probes {
		if p.skipReason == "" {
			compared++
		}
	}
	if compared != 51 {
		t.Fatalf("expected 51 compared probes, got %d", compared)
	}
}

func TestBuildProbes_JSAdapterReadMethods(t *testing.T) {
	probes := buildProbes("shadow-parity-fake-sid")
	required := map[string]bool{
		"GET /wakeup":            false,
		"GET /kg/recall":         false,
		"POST /kg/recall":        false,
		"POST /chapters/dry-run": false,
		"POST /chapters/search":  false,
		"POST /episodes/search":  false,
	}
	for _, p := range probes {
		endpoint := p.method + " " + p.path
		if _, ok := required[endpoint]; ok {
			required[endpoint] = true
		}
	}
	for endpoint, found := range required {
		if !found {
			t.Errorf("missing JS adapter read probe %s", endpoint)
		}
	}
}

func TestBuildProbes_DataBackedSessionAddsExplorerAndKGQueries(t *testing.T) {
	probes := buildProbes("real-session")
	found := map[string]bool{
		"GET /explorer/chat_logs?chat_session_id=real-session":                                       false,
		"GET /explorer/memories?chat_session_id=real-session":                                        false,
		"GET /explorer/direct-evidence?chat_session_id=real-session":                                 false,
		"GET /explorer/kg_triples?chat_session_id=real-session":                                      false,
		"GET /retrieval-index/runtime-config":                                                        false,
		"GET /intent-routing/runtime-config":                                                         false,
		"GET /retrieval-index/real-session/source-row?document_id=memory:1":                          false,
		"GET /characters/real-session/test-character":                                                false,
		"GET /characters/real-session/test-character/events":                                         false,
		"GET /episodes/detail/999999":                                                                false,
		"GET /kg/recall?chat_session_id=real-session&limit=20&offset=0":                              false,
		"GET /sessions/compare?session_ids=real-session,shadow-parity-secondary-sid&preview_limit=3": false,
	}
	for _, p := range probes {
		endpoint := p.method + " " + p.path
		if _, ok := found[endpoint]; ok {
			found[endpoint] = true
		}
	}
	for endpoint, ok := range found {
		if !ok {
			t.Fatalf("missing data-backed probe %s", endpoint)
		}
	}
}

func TestRun_JSONOutputAndSummary(t *testing.T) {
	// Create two httptest servers that return identical JSON.
	pySrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		fmt.Fprintln(w, `{"status":"ok","items":[]}`)
	}))
	defer pySrv.Close()

	goSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		fmt.Fprintln(w, `{"status":"ok","items":[]}`)
	}))
	defer goSrv.Close()

	dir := t.TempDir()
	mdPath := filepath.Join(dir, "report.md")
	jsonPath := filepath.Join(dir, "report.json")
	if err := run(pySrv.URL, goSrv.URL, mdPath, jsonPath, 10*time.Second, 20, "shadow-parity-fake-sid"); err != nil {
		t.Fatalf("run failed: %v", err)
	}
	md, err := os.ReadFile(mdPath)
	if err != nil {
		t.Fatalf("failed to read markdown output: %v", err)
	}
	if !strings.Contains(string(md), "Shadow Value Parity Report") {
		t.Fatalf("markdown output missing report header:\n%s", string(md))
	}

	data, err := os.ReadFile(jsonPath)
	if err != nil {
		t.Fatalf("failed to read json output: %v", err)
	}

	var payload struct {
		Timestamp   time.Time             `json:"timestamp"`
		PythonBase  string                `json:"python_base"`
		GoBase      string                `json:"go_base"`
		MaxDiffs    int                   `json:"max_diffs"`
		Summary     compare.ReportSummary `json:"summary"`
		Results     []compare.ValueResult `json:"results"`
		SkipReasons map[string]string     `json:"skip_reasons"`
	}
	if err := json.Unmarshal(data, &payload); err != nil {
		t.Fatalf("failed to unmarshal json output: %v", err)
	}

	if payload.Summary.Total != 55 {
		t.Errorf("expected summary.total=55, got %d", payload.Summary.Total)
	}
	if payload.Summary.Skipped != 4 {
		t.Errorf("expected summary.skipped=4, got %d", payload.Summary.Skipped)
	}
	if payload.Summary.Compared != 51 {
		t.Errorf("expected summary.compared=51, got %d", payload.Summary.Compared)
	}
	if payload.Summary.Allowed != 55 {
		t.Errorf("expected summary.allowed=55, got %d", payload.Summary.Allowed)
	}
	if payload.Summary.Blocked != 0 {
		t.Errorf("expected summary.blocked=0, got %d", payload.Summary.Blocked)
	}
	if payload.Summary.TransportErrorCount != 0 {
		t.Errorf("expected summary.transport_error_count=0, got %d", payload.Summary.TransportErrorCount)
	}
	if payload.Summary.StatusMismatchCount != 0 {
		t.Errorf("expected summary.status_mismatch_count=0, got %d", payload.Summary.StatusMismatchCount)
	}
	if payload.Summary.KeysMismatchCount != 0 {
		t.Errorf("expected summary.keys_mismatch_count=0, got %d", payload.Summary.KeysMismatchCount)
	}
	if payload.Summary.ExactMatches != 51 {
		t.Errorf("expected summary.exact_matches=51, got %d", payload.Summary.ExactMatches)
	}
	if payload.Summary.MismatchCount != 0 {
		t.Errorf("expected summary.mismatch_count=0, got %d", payload.Summary.MismatchCount)
	}

	if len(payload.Results) != 55 {
		t.Errorf("expected 55 results, got %d", len(payload.Results))
	}
	if len(payload.SkipReasons) != 4 {
		t.Errorf("expected 4 skip_reasons, got %d", len(payload.SkipReasons))
	}
}
