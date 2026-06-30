package bench

import (
	"encoding/json"
	"testing"
	"time"
)

func TestNewReportShape(t *testing.T) {
	summary := RouteSummary{
		URL:           "http://127.0.0.1:8000/health",
		Method:        "GET",
		Total:         5,
		Success:       5,
		Failure:       0,
		SafeProbe:     true,
		StatusCodes:   map[int]int{200: 5},
		MinLatency:    10 * time.Millisecond,
		AvgLatency:    30 * time.Millisecond,
		P50Latency:    30 * time.Millisecond,
		P95Latency:    45 * time.Millisecond,
		MaxLatency:    50 * time.Millisecond,
		TotalBytes:    500,
		ErrorMessages: map[string]int{},
	}

	r := NewReport(summary)
	if r.URL != summary.URL {
		t.Errorf("url = %q, want %q", r.URL, summary.URL)
	}
	if r.Method != summary.Method {
		t.Errorf("method = %q, want %q", r.Method, summary.Method)
	}
	if !r.SafeProbe {
		t.Error("safe_probe should be true")
	}
	if r.Total != 5 {
		t.Errorf("total = %d, want 5", r.Total)
	}
	if r.Success != 5 {
		t.Errorf("success = %d, want 5", r.Success)
	}
	if r.Failure != 0 {
		t.Errorf("failure = %d, want 0", r.Failure)
	}
	if r.TotalBytes != 500 {
		t.Errorf("total_bytes = %d, want 500", r.TotalBytes)
	}
	if r.LatencyMs.Min != 10.0 {
		t.Errorf("min = %f, want 10.0", r.LatencyMs.Min)
	}
	if r.LatencyMs.Avg != 30.0 {
		t.Errorf("avg = %f, want 30.0", r.LatencyMs.Avg)
	}
	if r.LatencyMs.P50 != 30.0 {
		t.Errorf("p50 = %f, want 30.0", r.LatencyMs.P50)
	}
	if r.LatencyMs.P95 != 45.0 {
		t.Errorf("p95 = %f, want 45.0", r.LatencyMs.P95)
	}
	if r.LatencyMs.Max != 50.0 {
		t.Errorf("max = %f, want 50.0", r.LatencyMs.Max)
	}
}

func TestNewReportZeroDurations(t *testing.T) {
	summary := RouteSummary{
		URL:           "http://127.0.0.1:8000/health",
		Method:        "GET",
		Total:         3,
		Success:       0,
		Failure:       3,
		SafeProbe:     true,
		StatusCodes:   map[int]int{},
		ErrorMessages: map[string]int{"connection refused": 3},
	}

	r := NewReport(summary)
	if r.LatencyMs.Min != 0 {
		t.Errorf("min = %f, want 0", r.LatencyMs.Min)
	}
	if r.LatencyMs.Avg != 0 {
		t.Errorf("avg = %f, want 0", r.LatencyMs.Avg)
	}
	if r.LatencyMs.P50 != 0 {
		t.Errorf("p50 = %f, want 0", r.LatencyMs.P50)
	}
	if r.LatencyMs.P95 != 0 {
		t.Errorf("p95 = %f, want 0", r.LatencyMs.P95)
	}
	if r.LatencyMs.Max != 0 {
		t.Errorf("max = %f, want 0", r.LatencyMs.Max)
	}
	if r.TotalBytes != 0 {
		t.Errorf("total_bytes = %d, want 0", r.TotalBytes)
	}
}

func TestNewReportJSONNoLatencies(t *testing.T) {
	summary := RouteSummary{
		URL:           "http://127.0.0.1:8000/health",
		Method:        "GET",
		Total:         2,
		Success:       2,
		Failure:       0,
		SafeProbe:     true,
		StatusCodes:   map[int]int{200: 2},
		Latencies:     []time.Duration{10 * time.Millisecond, 20 * time.Millisecond},
		MinLatency:    10 * time.Millisecond,
		AvgLatency:    15 * time.Millisecond,
		P50Latency:    15 * time.Millisecond,
		P95Latency:    19 * time.Millisecond,
		MaxLatency:    20 * time.Millisecond,
		TotalBytes:    200,
		ErrorMessages: map[string]int{},
	}

	r := NewReport(summary)
	b, err := json.Marshal(r)
	if err != nil {
		t.Fatalf("json marshal failed: %v", err)
	}

	var raw map[string]any
	if err := json.Unmarshal(b, &raw); err != nil {
		t.Fatalf("json unmarshal failed: %v", err)
	}

	// Latencies must NOT appear in JSON output.
	if _, ok := raw["latencies"]; ok {
		t.Error("JSON output should not contain 'latencies'")
	}

	// latency_ms must be present and contain numeric values.
	lat, ok := raw["latency_ms"].(map[string]any)
	if !ok {
		t.Fatal("latency_ms should be a JSON object")
	}
	for _, key := range []string{"min", "avg", "p50", "p95", "max"} {
		if _, ok := lat[key]; !ok {
			t.Errorf("latency_ms missing key %q", key)
		}
		if _, ok := lat[key].(float64); !ok {
			t.Errorf("latency_ms.%s should be a number", key)
		}
	}
}

func TestBaselineRunReportShape(t *testing.T) {
	report := BaselineRunReport{
		Status:     "ok",
		CapturedAt: "2026-05-24T00:00:00Z",
		Scope:      "safe_baseline",
		BaseURL:    "http://127.0.0.1:8000",
		Paths:      []string{"/health"},
		Method:     "GET",
		Count:      5,
		TimeoutSec: 2,
		HTTPReports: []Report{
			NewReport(RouteSummary{
				URL:           "http://127.0.0.1:8000/health",
				Method:        "GET",
				Total:         5,
				Success:       5,
				Failure:       0,
				SafeProbe:     true,
				StatusCodes:   map[int]int{200: 5},
				MinLatency:    10 * time.Millisecond,
				AvgLatency:    20 * time.Millisecond,
				P50Latency:    20 * time.Millisecond,
				P95Latency:    30 * time.Millisecond,
				MaxLatency:    40 * time.Millisecond,
				TotalBytes:    500,
				ErrorMessages: map[string]int{},
			}),
		},
		Readiness: ReadinessInfo{
			Enabled:   true,
			Success:   true,
			Attempts:  1,
			ElapsedMs: 15,
		},
		Process: ProcessInfo{
			PID:      1234,
			Status:   "captured",
			RSSBytes: 104857600,
			RSSMB:    100.0,
		},
		BlockedMetrics: DefaultBlockedMetrics(),
	}

	b, err := json.Marshal(report)
	if err != nil {
		t.Fatalf("marshal failed: %v", err)
	}

	var raw map[string]any
	if err := json.Unmarshal(b, &raw); err != nil {
		t.Fatalf("unmarshal failed: %v", err)
	}

	required := []string{"status", "captured_at", "scope", "base_url", "paths", "method", "count", "timeout_sec", "http_reports", "readiness", "process", "blocked_metrics"}
	for _, key := range required {
		if _, ok := raw[key]; !ok {
			t.Errorf("missing top-level key %q", key)
		}
	}

	if r, ok := raw["readiness"].(map[string]any); ok {
		for _, key := range []string{"enabled", "success", "attempts", "elapsed_ms", "error"} {
			if _, ok := r[key]; !ok {
				t.Errorf("readiness missing key %q", key)
			}
		}
	} else {
		t.Error("readiness should be an object")
	}

	if p, ok := raw["process"].(map[string]any); ok {
		for _, key := range []string{"pid", "status", "rss_bytes", "rss_mb", "error"} {
			if _, ok := p[key]; !ok {
				t.Errorf("process missing key %q", key)
			}
		}
	} else {
		t.Error("process should be an object")
	}

	if bm, ok := raw["blocked_metrics"].(map[string]any); ok {
		for _, key := range []string{"prepare_turn_overhead", "complete_turn_overhead", "retrieval_latency", "chroma_primary_retrieval", "reindex_or_backfill_cost"} {
			if _, ok := bm[key]; !ok {
				t.Errorf("blocked_metrics missing key %q", key)
			}
		}
	} else {
		t.Error("blocked_metrics should be an object")
	}

	if hr, ok := raw["http_reports"].([]any); ok {
		if len(hr) != 1 {
			t.Errorf("http_reports length = %d, want 1", len(hr))
		}
	} else {
		t.Error("http_reports should be an array")
	}
}
