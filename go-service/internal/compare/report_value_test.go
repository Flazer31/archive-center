package compare

import (
	"encoding/json"
	"strings"
	"testing"
	"time"
)

func TestValueReportWriteMarkdown(t *testing.T) {
	report := &ValueReport{
		Timestamp:  time.Date(2026, 5, 23, 0, 0, 0, 0, time.UTC),
		PythonBase: "http://127.0.0.1:8000",
		GoBase:     "http://127.0.0.1:28080",
		MaxDiffs:   20,
		Results: []ValueResult{
			{
				Endpoint:       "GET /health",
				Allowed:        true,
				StatusMatch:    true,
				KeysMatch:      true,
				ExactJSONMatch: true,
				BehaviorMatch:  true,
				TypeMatch:      true,
				PythonStatus:   200,
				GoStatus:       200,
				Diffs:          nil,
			},
			{
				Endpoint:       "GET /stats",
				Allowed:        true,
				StatusMatch:    true,
				KeysMatch:      true,
				ExactJSONMatch: false,
				BehaviorMatch:  false,
				TypeMatch:      true,
				PythonStatus:   200,
				GoStatus:       200,
				Diffs: []ValueDiff{
					{Path: ".count", Type: "scalar", PythonVal: "1", GoVal: "2"},
				},
			},
			{
				Endpoint: "GET /ready",
				Allowed:  true,
			},
		},
		SkipReasons: map[string]string{
			"GET /ready": "Go-only ops endpoint",
		},
	}

	var b strings.Builder
	if err := report.WriteMarkdown(&b); err != nil {
		t.Fatalf("WriteMarkdown failed: %v", err)
	}
	out := b.String()

	if !strings.Contains(out, "R1 Report-Only") {
		t.Error("missing R1 report-only header")
	}
	if !strings.Contains(out, "Disclaimer") {
		t.Error("missing disclaimer")
	}
	if !strings.Contains(out, "Volatile Field Policy") {
		t.Error("missing volatile field policy")
	}
	if !strings.Contains(out, "**Exact JSON Matches**: 1 / 2") {
		t.Errorf("missing exact JSON match count:\n%s", out)
	}
	if !strings.Contains(out, "**Behavior Matches**: 1 / 2") {
		t.Errorf("missing behavior match count:\n%s", out)
	}
	if !strings.Contains(out, "**Type Matches**: 2 / 2") {
		t.Errorf("missing type match count:\n%s", out)
	}
	if !strings.Contains(out, "**Endpoints with Value/List-Length Mismatches**: 1 / 2") {
		t.Errorf("missing value mismatch count:\n%s", out)
	}
	if !strings.Contains(out, "SKIP: Go-only ops endpoint") {
		t.Errorf("missing skip reason:\n%s", out)
	}
	if !strings.Contains(out, ".count") {
		t.Errorf("missing detailed diff:\n%s", out)
	}
}

func TestValueReportWriteJSONSummary(t *testing.T) {
	report := &ValueReport{
		Timestamp:  time.Date(2026, 5, 26, 0, 0, 0, 0, time.UTC),
		PythonBase: "http://127.0.0.1:8000",
		GoBase:     "http://127.0.0.1:28080",
		MaxDiffs:   20,
		Results: []ValueResult{
			{
				Endpoint:       "GET /health",
				Allowed:        true,
				StatusMatch:    true,
				KeysMatch:      true,
				ExactJSONMatch: true,
				BehaviorMatch:  true,
				TypeMatch:      true,
			},
			{
				Endpoint:       "GET /stats",
				Allowed:        true,
				StatusMatch:    true,
				KeysMatch:      true,
				ExactJSONMatch: false,
				BehaviorMatch:  false,
				TypeMatch:      true,
				Diffs: []ValueDiff{
					{Path: ".count", Type: "scalar", PythonVal: "1", GoVal: "2"},
				},
			},
			{
				Endpoint:       "GET /sessions",
				Allowed:        true,
				StatusMatch:    false,
				KeysMatch:      false,
				ExactJSONMatch: false,
				BehaviorMatch:  false,
				TypeMatch:      false,
				Error:          "python backend: connection refused",
			},
			{
				Endpoint: "GET /ready",
				Allowed:  true,
			},
		},
		SkipReasons: map[string]string{
			"GET /ready": "Go-only ops endpoint",
		},
	}

	var b strings.Builder
	if err := report.WriteJSON(&b); err != nil {
		t.Fatalf("WriteJSON failed: %v", err)
	}

	var decoded struct {
		PythonBase  string            `json:"python_base"`
		GoBase      string            `json:"go_base"`
		MaxDiffs    int               `json:"max_diffs"`
		Summary     ReportSummary     `json:"summary"`
		SkipReasons map[string]string `json:"skip_reasons"`
		Results     []struct {
			Endpoint       string      `json:"endpoint"`
			ExactJSONMatch bool        `json:"exact_json_match"`
			Diffs          []ValueDiff `json:"diffs"`
		} `json:"results"`
	}
	if err := json.Unmarshal([]byte(b.String()), &decoded); err != nil {
		t.Fatalf("json decode failed: %v\n%s", err, b.String())
	}

	if decoded.PythonBase != report.PythonBase || decoded.GoBase != report.GoBase || decoded.MaxDiffs != report.MaxDiffs {
		t.Fatalf("top-level metadata mismatch: %#v", decoded)
	}
	if got := decoded.Summary.Total; got != 4 {
		t.Fatalf("summary.total = %d, want 4", got)
	}
	if got := decoded.Summary.Compared; got != 3 {
		t.Fatalf("summary.compared = %d, want 3", got)
	}
	if got := decoded.Summary.Skipped; got != 1 {
		t.Fatalf("summary.skipped = %d, want 1", got)
	}
	if got := decoded.Summary.Allowed; got != 4 {
		t.Fatalf("summary.allowed = %d, want 4", got)
	}
	if got := decoded.Summary.ExactMatches; got != 1 {
		t.Fatalf("summary.exact_matches = %d, want 1", got)
	}
	if got := decoded.Summary.BehaviorMatches; got != 1 {
		t.Fatalf("summary.behavior_matches = %d, want 1", got)
	}
	if got := decoded.Summary.MismatchCount; got != 2 {
		t.Fatalf("summary.mismatch_count = %d, want 2", got)
	}
	if got := decoded.Summary.BehaviorMismatchCount; got != 2 {
		t.Fatalf("summary.behavior_mismatch_count = %d, want 2", got)
	}
	if got := decoded.Summary.TransportErrorCount; got != 1 {
		t.Fatalf("summary.transport_error_count = %d, want 1", got)
	}
	if got := decoded.Summary.StatusMismatchCount; got != 1 {
		t.Fatalf("summary.status_mismatch_count = %d, want 1", got)
	}
	if got := decoded.Summary.KeysMismatchCount; got != 1 {
		t.Fatalf("summary.keys_mismatch_count = %d, want 1", got)
	}
	if got := decoded.Summary.TypeMismatchCount; got != 1 {
		t.Fatalf("summary.type_mismatch_count = %d, want 1", got)
	}
	if got := decoded.Summary.ValueMismatchCount; got != 1 {
		t.Fatalf("summary.value_mismatch_count = %d, want 1", got)
	}
	if decoded.SkipReasons["GET /ready"] == "" {
		t.Fatal("skip reason missing from JSON")
	}
	if len(decoded.Results) != 4 || decoded.Results[1].Diffs[0].Path != ".count" {
		t.Fatalf("result diffs missing from JSON: %#v", decoded.Results)
	}
}

func TestValueReportSummaryCountsVolatileOnlyAsBehaviorMatch(t *testing.T) {
	report := &ValueReport{
		Results: []ValueResult{
			{
				Endpoint:          "GET /narrative-control/test",
				Allowed:           true,
				StatusMatch:       true,
				KeysMatch:         true,
				ExactJSONMatch:    false,
				BehaviorMatch:     true,
				VolatileOnlyDiffs: true,
				TypeMatch:         true,
				Diffs: []ValueDiff{
					{Path: ".generated_at", Type: "volatile", PythonVal: "2026-05-27T00:00:00Z", GoVal: "2026-05-27T00:00:01Z"},
				},
			},
		},
		SkipReasons: map[string]string{},
	}

	summary := report.Summarize()
	if summary.ExactMatches != 0 {
		t.Fatalf("exact_matches = %d, want 0", summary.ExactMatches)
	}
	if summary.MismatchCount != 1 {
		t.Fatalf("mismatch_count = %d, want 1 raw mismatch", summary.MismatchCount)
	}
	if summary.BehaviorMatches != 1 {
		t.Fatalf("behavior_matches = %d, want 1", summary.BehaviorMatches)
	}
	if summary.BehaviorMismatchCount != 0 {
		t.Fatalf("behavior_mismatch_count = %d, want 0", summary.BehaviorMismatchCount)
	}
	if summary.VolatileOnlyCount != 1 {
		t.Fatalf("volatile_only_count = %d, want 1", summary.VolatileOnlyCount)
	}
}
