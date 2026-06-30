// Package bench provides baseline capture primitives.
package bench

import "time"

// LatencyMs holds latency values expressed in milliseconds.
type LatencyMs struct {
	Min float64 `json:"min"`
	Avg float64 `json:"avg"`
	P50 float64 `json:"p50"`
	P95 float64 `json:"p95"`
	Max float64 `json:"max"`
}

// Report is a stable JSON DTO for baseline capture output.
// It does not expose raw Latencies or time.Duration internals.
type Report struct {
	URL           string         `json:"url"`
	Method        string         `json:"method"`
	SafeProbe     bool           `json:"safe_probe"`
	Total         int            `json:"total"`
	Success       int            `json:"success"`
	Failure       int            `json:"failure"`
	StatusCodes   map[int]int    `json:"status_codes"`
	LatencyMs     LatencyMs      `json:"latency_ms"`
	TotalBytes    int64          `json:"total_bytes"`
	ErrorMessages map[string]int `json:"error_messages"`
}

// NewReport converts a RouteSummary into a stable Report DTO.
func NewReport(s RouteSummary) Report {
	return Report{
		URL:           s.URL,
		Method:        s.Method,
		SafeProbe:     s.SafeProbe,
		Total:         s.Total,
		Success:       s.Success,
		Failure:       s.Failure,
		StatusCodes:   s.StatusCodes,
		TotalBytes:    s.TotalBytes,
		ErrorMessages: s.ErrorMessages,
		LatencyMs: LatencyMs{
			Min: durToMs(s.MinLatency),
			Avg: durToMs(s.AvgLatency),
			P50: durToMs(s.P50Latency),
			P95: durToMs(s.P95Latency),
			Max: durToMs(s.MaxLatency),
		},
	}
}

func durToMs(d time.Duration) float64 {
	if d <= 0 {
		return 0
	}
	return float64(d.Nanoseconds()) / 1e6
}

// BaselineRunReport is the top-level safe baseline run report.
type BaselineRunReport struct {
	Status         string            `json:"status"`
	CapturedAt     string            `json:"captured_at"`
	Scope          string            `json:"scope"`
	BaseURL        string            `json:"base_url"`
	Paths          []string          `json:"paths"`
	Method         string            `json:"method"`
	Count          int               `json:"count"`
	TimeoutSec     int               `json:"timeout_sec"`
	HTTPReports    []Report          `json:"http_reports"`
	Readiness      ReadinessInfo     `json:"readiness"`
	Process        ProcessInfo       `json:"process"`
	BlockedMetrics map[string]string `json:"blocked_metrics"`
}

// ReadinessInfo records the result of the optional wait-ready probe.
type ReadinessInfo struct {
	Enabled   bool   `json:"enabled"`
	Success   bool   `json:"success"`
	Attempts  int    `json:"attempts"`
	ElapsedMs int64  `json:"elapsed_ms"`
	Error     string `json:"error"`
}

// ProcessInfo records the optional process RSS snapshot.
type ProcessInfo struct {
	PID      int     `json:"pid"`
	Status   string  `json:"status"`
	RSSBytes int64   `json:"rss_bytes"`
	RSSMB    float64 `json:"rss_mb"`
	Error    string  `json:"error"`
}

// DefaultBlockedMetrics returns the set of metrics that are intentionally
// not auto-measured in the current slice.
func DefaultBlockedMetrics() map[string]string {
	return map[string]string{
		"prepare_turn_overhead":    "not_measured",
		"complete_turn_overhead":   "not_measured",
		"retrieval_latency":        "not_measured",
		"chroma_primary_retrieval": "not_measured",
		"reindex_or_backfill_cost": "not_measured",
	}
}
