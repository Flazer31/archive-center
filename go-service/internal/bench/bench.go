// Package bench provides baseline capture primitives for the Archive Center 2.0
// migration. It is stdlib-only and safe for local probe use against the
// current 0.8 backend without mutating state.
package bench

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"sort"
	"strings"
	"time"
)

// RunConfig holds the parameters for a baseline capture run.
type RunConfig struct {
	BaseURL string
	Paths   []string
	Method  string
	Count   int
	Timeout time.Duration
}

// Result captures a single HTTP probe outcome.
type Result struct {
	URL        string
	Method     string
	StatusCode int
	Latency    time.Duration
	Bytes      int64
	Error      string
}

// RouteSummary aggregates Result values for one route.
type RouteSummary struct {
	URL           string
	Method        string
	Total         int
	Success       int
	Failure       int
	StatusCodes   map[int]int
	SafeProbe     bool
	Latencies     []time.Duration
	MinLatency    time.Duration
	AvgLatency    time.Duration
	P50Latency    time.Duration
	P95Latency    time.Duration
	MaxLatency    time.Duration
	TotalBytes    int64
	ErrorMessages map[string]int
}

// IsSafeProbe returns true for HTTP methods and path combinations that
// are known to be read-only and non-mutating for the current runtime.
func IsSafeProbe(method, path string) bool {
	method = strings.ToUpper(method)
	if method != http.MethodGet {
		return false
	}
	// Only static/readiness probe routes are considered safe for automatic
	// baseline probing because they do not mutate database, session, or vector
	// state.
	if path == "/health" || path == "/ready" || path == "/version" {
		return true
	}
	return false
}

// Probe executes a single HTTP request and records its latency and outcome.
func Probe(ctx context.Context, client *http.Client, method, url string) (*Result, error) {
	req, err := http.NewRequestWithContext(ctx, method, url, nil)
	if err != nil {
		return &Result{URL: url, Method: method, Error: err.Error()}, err
	}

	start := time.Now()
	resp, err := client.Do(req)
	latency := time.Since(start)

	if err != nil {
		return &Result{
			URL:     url,
			Method:  method,
			Latency: latency,
			Error:   err.Error(),
		}, nil
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return &Result{
			URL:        url,
			Method:     method,
			StatusCode: resp.StatusCode,
			Latency:    latency,
			Error:      err.Error(),
		}, nil
	}

	return &Result{
		URL:        url,
		Method:     method,
		StatusCode: resp.StatusCode,
		Latency:    latency,
		Bytes:      int64(len(body)),
	}, nil
}

// Summarize aggregates a slice of Results into a RouteSummary.
func Summarize(results []Result) RouteSummary {
	if len(results) == 0 {
		return RouteSummary{}
	}

	s := RouteSummary{
		URL:           results[0].URL,
		Method:        results[0].Method,
		Total:         len(results),
		StatusCodes:   make(map[int]int),
		ErrorMessages: make(map[string]int),
	}

	var totalLatency time.Duration
	var totalBytes int64

	for _, r := range results {
		if r.Error != "" {
			s.Failure++
			s.ErrorMessages[r.Error]++
			continue
		}
		s.Success++
		s.StatusCodes[r.StatusCode]++
		s.Latencies = append(s.Latencies, r.Latency)
		totalLatency += r.Latency
		totalBytes += r.Bytes
	}

	s.TotalBytes = totalBytes
	s.SafeProbe = IsSafeProbe(s.Method, extractPath(s.URL))

	if len(s.Latencies) > 0 {
		sort.Slice(s.Latencies, func(i, j int) bool {
			return s.Latencies[i] < s.Latencies[j]
		})
		s.MinLatency = s.Latencies[0]
		s.MaxLatency = s.Latencies[len(s.Latencies)-1]
		s.AvgLatency = totalLatency / time.Duration(len(s.Latencies))
		s.P50Latency = percentile(s.Latencies, 0.50)
		s.P95Latency = percentile(s.Latencies, 0.95)
	}

	return s
}

// percentile returns the percentile value from a sorted slice of durations.
// The caller must sort latencies before calling this function.
func percentile(sorted []time.Duration, p float64) time.Duration {
	if len(sorted) == 0 {
		return 0
	}
	if p <= 0 {
		return sorted[0]
	}
	if p >= 1 {
		return sorted[len(sorted)-1]
	}
	idx := float64(len(sorted)-1) * p
	lower := int(idx)
	frac := idx - float64(lower)
	if lower >= len(sorted)-1 {
		return sorted[len(sorted)-1]
	}
	v1 := sorted[lower]
	v2 := sorted[lower+1]
	return time.Duration(float64(v1) + frac*float64(v2-v1))
}

func extractPath(urlStr string) string {
	// Naive path extraction for the common case http://host:port/path
	parts := strings.SplitN(urlStr, "/", 4)
	if len(parts) >= 4 {
		return "/" + parts[3]
	}
	return urlStr
}

// Millis formats a duration as milliseconds with one decimal place.
func Millis(d time.Duration) string {
	if d == 0 {
		return "0.0"
	}
	return fmt.Sprintf("%.1f", float64(d.Nanoseconds())/1e6)
}
