package compare

import (
	"strings"
	"testing"
	"time"
)

func TestReportWriteMarkdownSkipsOutsideMatchDenominator(t *testing.T) {
	report := &Report{
		Timestamp:  time.Date(2026, 5, 23, 0, 0, 0, 0, time.UTC),
		PythonBase: "http://127.0.0.1:8000",
		GoBase:     "http://127.0.0.1:28080",
		Results: []Result{
			{Endpoint: "GET /ready", Allowed: true},
			{Endpoint: "GET /stats", Allowed: true, StatusMatch: true, KeysMatch: true},
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

	if !strings.Contains(out, "- **Skipped**: 1") {
		t.Fatalf("missing skipped count:\n%s", out)
	}
	if !strings.Contains(out, "- **Status Match**: 1 / 1") {
		t.Fatalf("skipped endpoint should not be in status denominator:\n%s", out)
	}
	if !strings.Contains(out, "- **Keys Match**: 1 / 1") {
		t.Fatalf("skipped endpoint should not be in keys denominator:\n%s", out)
	}
	if !strings.Contains(out, "| GET /ready | true | none | none | none | none | none | none | none | SKIP: Go-only ops endpoint |") {
		t.Fatalf("skip row should preserve table column count:\n%s", out)
	}
}
