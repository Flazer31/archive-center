package bench

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestIsSafeProbe(t *testing.T) {
	if !IsSafeProbe("GET", "/health") {
		t.Error("GET /health should be safe")
	}
	if !IsSafeProbe("GET", "/ready") {
		t.Error("GET /ready should be safe")
	}
	if !IsSafeProbe("GET", "/version") {
		t.Error("GET /version should be safe")
	}
	if IsSafeProbe("POST", "/health") {
		t.Error("POST /health should not be safe")
	}
	if IsSafeProbe("GET", "/prepare-turn") {
		t.Error("GET /prepare-turn should not be safe")
	}
	if IsSafeProbe("GET", "/turns") {
		t.Error("GET /turns should not be safe")
	}
	if IsSafeProbe("DELETE", "/health") {
		t.Error("DELETE /health should not be safe")
	}
}

func TestProbeSuccess(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("ok"))
	}))
	defer ts.Close()

	client := &http.Client{Timeout: 2 * time.Second}
	ctx := context.Background()
	res, err := Probe(ctx, client, http.MethodGet, ts.URL+"/health")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res.Error != "" {
		t.Errorf("unexpected error in result: %s", res.Error)
	}
	if res.StatusCode != http.StatusOK {
		t.Errorf("status = %d, want %d", res.StatusCode, http.StatusOK)
	}
	if res.Bytes != 2 {
		t.Errorf("bytes = %d, want 2", res.Bytes)
	}
	if res.Latency <= 0 {
		t.Error("latency should be positive")
	}
}

func TestProbeNon200(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusServiceUnavailable)
		w.Write([]byte("unavailable"))
	}))
	defer ts.Close()

	client := &http.Client{Timeout: 2 * time.Second}
	ctx := context.Background()
	res, err := Probe(ctx, client, http.MethodGet, ts.URL+"/ready")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res.Error != "" {
		t.Errorf("unexpected error in result: %s", res.Error)
	}
	if res.StatusCode != http.StatusServiceUnavailable {
		t.Errorf("status = %d, want %d", res.StatusCode, http.StatusServiceUnavailable)
	}
}

func TestProbeTimeout(t *testing.T) {
	// A server that never responds should trigger a timeout.
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(5 * time.Second)
		w.WriteHeader(http.StatusOK)
	}))
	defer ts.Close()

	client := &http.Client{Timeout: 100 * time.Millisecond}
	ctx := context.Background()
	res, err := Probe(ctx, client, http.MethodGet, ts.URL+"/health")
	if err != nil {
		t.Fatalf("unexpected error return: %v", err)
	}
	if res.Error == "" {
		t.Fatal("expected a timeout error in result")
	}
	if !strings.Contains(res.Error, "Client.Timeout") && !strings.Contains(res.Error, "context deadline exceeded") {
		t.Errorf("error does not indicate timeout: %s", res.Error)
	}
}

func TestSummarize(t *testing.T) {
	url := "http://127.0.0.1:8000/health"
	results := []Result{
		{URL: url, Method: "GET", StatusCode: 200, Latency: 10 * time.Millisecond, Bytes: 100},
		{URL: url, Method: "GET", StatusCode: 200, Latency: 20 * time.Millisecond, Bytes: 100},
		{URL: url, Method: "GET", StatusCode: 200, Latency: 30 * time.Millisecond, Bytes: 100},
		{URL: url, Method: "GET", StatusCode: 200, Latency: 40 * time.Millisecond, Bytes: 100},
		{URL: url, Method: "GET", StatusCode: 200, Latency: 50 * time.Millisecond, Bytes: 100},
	}

	summary := Summarize(results)
	if summary.Total != 5 {
		t.Errorf("total = %d, want 5", summary.Total)
	}
	if summary.Success != 5 {
		t.Errorf("success = %d, want 5", summary.Success)
	}
	if summary.Failure != 0 {
		t.Errorf("failure = %d, want 0", summary.Failure)
	}
	if summary.MinLatency != 10*time.Millisecond {
		t.Errorf("min = %v, want 10ms", summary.MinLatency)
	}
	if summary.MaxLatency != 50*time.Millisecond {
		t.Errorf("max = %v, want 50ms", summary.MaxLatency)
	}
	if summary.AvgLatency != 30*time.Millisecond {
		t.Errorf("avg = %v, want 30ms", summary.AvgLatency)
	}
	if summary.P50Latency != 30*time.Millisecond {
		t.Errorf("p50 = %v, want 30ms", summary.P50Latency)
	}
	if summary.TotalBytes != 500 {
		t.Errorf("bytes = %d, want 500", summary.TotalBytes)
	}
	if !summary.SafeProbe {
		t.Error("safe_probe should be true for GET /health")
	}
}

func TestSummarizeWithErrors(t *testing.T) {
	url := "http://127.0.0.1:8000/health"
	results := []Result{
		{URL: url, Method: "GET", StatusCode: 200, Latency: 10 * time.Millisecond, Bytes: 50},
		{URL: url, Method: "GET", Error: "connection refused"},
		{URL: url, Method: "GET", Error: "connection refused"},
	}

	summary := Summarize(results)
	if summary.Success != 1 {
		t.Errorf("success = %d, want 1", summary.Success)
	}
	if summary.Failure != 2 {
		t.Errorf("failure = %d, want 2", summary.Failure)
	}
	if summary.TotalBytes != 50 {
		t.Errorf("bytes = %d, want 50", summary.TotalBytes)
	}
	if summary.ErrorMessages["connection refused"] != 2 {
		t.Errorf("connection refused count = %d, want 2", summary.ErrorMessages["connection refused"])
	}
}

func TestPercentile(t *testing.T) {
	sorted := []time.Duration{
		10 * time.Millisecond,
		20 * time.Millisecond,
		30 * time.Millisecond,
		40 * time.Millisecond,
		50 * time.Millisecond,
	}
	if percentile(sorted, 0.00) != 10*time.Millisecond {
		t.Errorf("p0 = %v", percentile(sorted, 0.00))
	}
	if percentile(sorted, 1.00) != 50*time.Millisecond {
		t.Errorf("p100 = %v", percentile(sorted, 1.00))
	}
	if percentile(sorted, 0.50) != 30*time.Millisecond {
		t.Errorf("p50 = %v", percentile(sorted, 0.50))
	}
	if percentile([]time.Duration{}, 0.50) != 0 {
		t.Error("empty percentile should be 0")
	}
}

func TestMillis(t *testing.T) {
	if Millis(1234*time.Microsecond) != "1.2" {
		t.Errorf("millis = %s", Millis(1234*time.Microsecond))
	}
	if Millis(0) != "0.0" {
		t.Errorf("millis(0) = %s", Millis(0))
	}
}

func TestExtractPath(t *testing.T) {
	if extractPath("http://127.0.0.1:8000/health") != "/health" {
		t.Errorf("extractPath = %s", extractPath("http://127.0.0.1:8000/health"))
	}
	if extractPath("http://127.0.0.1:8000/") != "/" {
		t.Errorf("extractPath root = %s", extractPath("http://127.0.0.1:8000/"))
	}
}

func TestProbeConnectionRefused(t *testing.T) {
	// Use a port that is extremely unlikely to be open.
	client := &http.Client{Timeout: 500 * time.Millisecond}
	ctx := context.Background()
	res, err := Probe(ctx, client, http.MethodGet, "http://127.0.0.1:1/health")
	if err != nil {
		t.Fatalf("unexpected error return: %v", err)
	}
	if res.Error == "" {
		t.Fatal("expected an error in result")
	}
	if res.StatusCode != 0 {
		t.Errorf("status code should be 0 on error, got %d", res.StatusCode)
	}
}
