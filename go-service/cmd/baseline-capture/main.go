// baseline-capture measures the current Archive Center 0.8 backend HTTP baseline
// without mutating state. It is safe to run against a local development instance.
//
// Default behavior probes GET /health only.
package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/risulongmemory/archive-center-go/internal/bench"
)

func main() {
	baseURL := flag.String("base", "http://127.0.0.1:8000", "Base URL of the backend")
	pathsRaw := flag.String("paths", "/health", "Comma-separated list of paths to probe")
	method := flag.String("method", "GET", "HTTP method (GET only for now)")
	count := flag.Int("n", 5, "Number of requests per path")
	timeoutSec := flag.Int("timeout", 2, "Per-request timeout in seconds")
	jsonOut := flag.Bool("json", false, "Emit JSON output instead of table")
	reportPath := flag.String("report", "", "Write safe baseline run report JSON to file")
	pid := flag.Int("pid", 0, "Process ID to capture RSS for (0 = not requested)")
	waitReady := flag.Bool("wait-ready", false, "Wait for /health to succeed before probing")
	startupTimeout := flag.Int("startup-timeout", 30, "Max seconds to wait for ready (wait-ready only)")
	startupIntervalMs := flag.Int("startup-interval-ms", 500, "Interval between ready probes in ms (wait-ready only)")
	flag.Parse()

	if err := ValidateFlags(*count, *timeoutSec, *startupTimeout, *startupIntervalMs, *pid, *waitReady); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(2)
	}

	if !strings.EqualFold(*method, "GET") {
		fmt.Fprintf(os.Stderr, "error: only GET is supported in this slice\n")
		os.Exit(2)
	}

	paths := splitPaths(*pathsRaw)
	if len(paths) == 0 {
		fmt.Fprintf(os.Stderr, "error: no paths provided\n")
		os.Exit(2)
	}

	for _, p := range paths {
		if !bench.IsSafeProbe(*method, p) {
			fmt.Fprintf(os.Stderr, "error: path %q with method %q is not classified as safe_probe in this slice\n", p, *method)
			os.Exit(2)
		}
	}

	timeout := time.Duration(*timeoutSec) * time.Second
	client := &http.Client{Timeout: timeout}

	// Optional wait-for-ready
	readiness := bench.ReadinessInfo{Enabled: *waitReady}
	if *waitReady {
		readyURL := strings.TrimRight(*baseURL, "/") + "/health"
		readyCtx, cancel := context.WithTimeout(context.Background(), time.Duration(*startupTimeout)*time.Second)
		defer cancel()
		attempts, elapsed, err := WaitForReady(readyCtx, client, readyURL, time.Duration(*startupIntervalMs)*time.Millisecond)
		readiness.Attempts = attempts
		readiness.ElapsedMs = elapsed.Milliseconds()
		if err != nil {
			readiness.Error = err.Error()
		} else {
			readiness.Success = true
		}
	}

	summaries := executeCapture(client, *method, *baseURL, paths, *count, readiness)
	reports := make([]bench.Report, len(summaries))
	for i, s := range summaries {
		reports[i] = bench.NewReport(s)
	}

	// Optional process RSS
	process := bench.ProcessInfo{PID: *pid, Status: "not_requested"}
	if *pid > 0 {
		rss, err := bench.GetProcessRSS(*pid)
		if err != nil {
			process.Status = "error"
			process.Error = err.Error()
		} else {
			process.Status = "captured"
			process.RSSBytes = rss
			process.RSSMB = float64(rss) / (1024 * 1024)
		}
	}

	// Determine overall status
	status := "ok"
	if readiness.Enabled && !readiness.Success {
		status = "failed_readiness"
	} else {
		for _, s := range summaries {
			if s.Success == 0 && s.Total > 0 {
				status = "degraded"
				break
			}
		}
	}

	runReport := bench.BaselineRunReport{
		Status:         status,
		CapturedAt:     time.Now().UTC().Format(time.RFC3339),
		Scope:          "safe_baseline",
		BaseURL:        *baseURL,
		Paths:          paths,
		Method:         *method,
		Count:          *count,
		TimeoutSec:     *timeoutSec,
		HTTPReports:    reports,
		Readiness:      readiness,
		Process:        process,
		BlockedMetrics: bench.DefaultBlockedMetrics(),
	}

	if *reportPath != "" {
		if err := writeReport(*reportPath, runReport); err != nil {
			fmt.Fprintf(os.Stderr, "error writing report: %v\n", err)
			os.Exit(1)
		}
	}

	if *jsonOut {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		if err := enc.Encode(reports); err != nil {
			fmt.Fprintf(os.Stderr, "error encoding JSON: %v\n", err)
			os.Exit(1)
		}
		return
	}

	printTable(summaries)
}

func executeCapture(client *http.Client, method, baseURL string, paths []string, count int, readiness bench.ReadinessInfo) []bench.RouteSummary {
	if readiness.Enabled && !readiness.Success {
		return nil
	}
	summaries := make([]bench.RouteSummary, 0, len(paths))
	for _, p := range paths {
		url := strings.TrimRight(baseURL, "/") + p
		results := make([]bench.Result, 0, count)
		for i := 0; i < count; i++ {
			ctx := context.Background()
			res, _ := bench.Probe(ctx, client, method, url)
			results = append(results, *res)
		}
		summaries = append(summaries, bench.Summarize(results))
	}
	return summaries
}

func writeReport(path string, report bench.BaselineRunReport) error {
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()
	enc := json.NewEncoder(f)
	enc.SetIndent("", "  ")
	return enc.Encode(report)
}

// WaitForReady polls url until it returns HTTP 200 or ctx is cancelled.
func WaitForReady(ctx context.Context, client *http.Client, url string, interval time.Duration) (int, time.Duration, error) {
	start := time.Now()
	attempts := 0
	for {
		attempts++
		res, _ := bench.Probe(ctx, client, http.MethodGet, url)
		if res.Error == "" && res.StatusCode == http.StatusOK {
			return attempts, time.Since(start), nil
		}
		if ctx.Err() != nil {
			return attempts, time.Since(start), ctx.Err()
		}
		time.Sleep(interval)
	}
}

// ValidateFlags checks that CLI flags are within acceptable ranges.
func ValidateFlags(count, timeout, startupTimeout, startupIntervalMs, pid int, waitReady bool) error {
	if count <= 0 {
		return errors.New("request count must be > 0")
	}
	if timeout <= 0 {
		return errors.New("timeout must be > 0")
	}
	if startupIntervalMs <= 0 {
		return errors.New("startup-interval-ms must be > 0")
	}
	if startupTimeout < 0 {
		return errors.New("startup-timeout must be >= 0")
	}
	if waitReady && startupTimeout == 0 {
		return errors.New("startup-timeout must be > 0 when wait-ready is enabled")
	}
	if pid < 0 {
		return errors.New("pid must be >= 0")
	}
	return nil
}

func splitPaths(raw string) []string {
	parts := strings.Split(raw, ",")
	var out []string
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p != "" {
			if !strings.HasPrefix(p, "/") {
				p = "/" + p
			}
			out = append(out, p)
		}
	}
	return out
}

func printTable(summaries []bench.RouteSummary) {
	fmt.Println("Route                          | Safe | OK | FAIL | Min    | Avg    | P50    | P95    | Max    | Bytes")
	fmt.Println("-------------------------------|------|----|------|--------|--------|--------|--------|--------|--------")
	for _, s := range summaries {
		codes := make([]string, 0, len(s.StatusCodes))
		for code, n := range s.StatusCodes {
			codes = append(codes, fmt.Sprintf("%d=%d", code, n))
		}
		fmt.Printf(
			"%-30s | %-4v | %2d | %4d | %6s | %6s | %6s | %6s | %6s | %d\n",
			s.Method+" "+s.URL,
			s.SafeProbe,
			s.Success,
			s.Failure,
			bench.Millis(s.MinLatency),
			bench.Millis(s.AvgLatency),
			bench.Millis(s.P50Latency),
			bench.Millis(s.P95Latency),
			bench.Millis(s.MaxLatency),
			s.TotalBytes,
		)
		if len(codes) > 0 {
			fmt.Printf("  status_codes: %s\n", strings.Join(codes, ", "))
		}
		if len(s.ErrorMessages) > 0 {
			for msg, n := range s.ErrorMessages {
				fmt.Printf("  error: %q x%d\n", msg, n)
			}
		}
	}
}
