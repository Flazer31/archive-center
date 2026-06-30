// go-shadow-launch-smoke starts the Archive Center 2.0 Go shadow backend in
// explicit noop mode and proves the runtime can launch without MariaDB.
package main

import (
	"bytes"
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"time"
)

type report struct {
	Status          string            `json:"status"`
	GeneratedAt     string            `json:"generated_at"`
	Scope           string            `json:"scope"`
	Note            string            `json:"note"`
	CommandMode     string            `json:"command_mode"`
	Command         []string          `json:"command"`
	GoServiceRoot   string            `json:"go_service_root,omitempty"`
	Port            int               `json:"port"`
	BaseURL         string            `json:"base_url"`
	StartupAttempts int               `json:"startup_attempts"`
	StartupElapsed  int64             `json:"startup_elapsed_ms"`
	Probes          []probeResult     `json:"probes"`
	SafetyFlags     map[string]bool   `json:"safety_flags"`
	OpenGaps        []string          `json:"open_gaps"`
	Errors          []string          `json:"errors,omitempty"`
	Process         processSummary    `json:"process"`
	EnvSummary      map[string]string `json:"env_summary"`
}

type processSummary struct {
	Started bool   `json:"started"`
	Stopped bool   `json:"stopped"`
	PID     int    `json:"pid,omitempty"`
	Error   string `json:"error,omitempty"`
}

type probeResult struct {
	Path         string   `json:"path"`
	Status       string   `json:"status"`
	HTTPStatus   int      `json:"http_status"`
	JSONValid    bool     `json:"json_valid"`
	TopLevelKeys []string `json:"top_level_keys,omitempty"`
	Error        string   `json:"error,omitempty"`
}

type launchSpec struct {
	BinPath       string
	GoServiceRoot string
	Port          int
	TempDir       string
}

func main() {
	binPath := flag.String("bin", "", "Optional archive-center-go binary path. If omitted, builds archive-center-go into a temp dir and runs that binary.")
	goServiceRoot := flag.String("go-service-root", ".", "Go service root used when -bin is omitted.")
	outPath := flag.String("out", "", "JSON report path. Defaults to stdout.")
	startupTimeoutSec := flag.Int("startup-timeout", 45, "Seconds to wait for /health.")
	probeTimeoutSec := flag.Int("probe-timeout", 3, "Per-probe timeout in seconds.")
	flag.Parse()

	r := run(*binPath, *goServiceRoot, time.Duration(*startupTimeoutSec)*time.Second, time.Duration(*probeTimeoutSec)*time.Second)
	if err := writeReport(*outPath, r); err != nil {
		fmt.Fprintf(os.Stderr, "write report: %v\n", err)
		os.Exit(1)
	}
	if r.Status != "ok" {
		os.Exit(1)
	}
}

func run(binPath, goServiceRoot string, startupTimeout, probeTimeout time.Duration) report {
	r := report{
		Status:        "ok",
		GeneratedAt:   time.Now().UTC().Format(time.RFC3339),
		Scope:         "R1 DB-less Go shadow backend launch smoke",
		Note:          "Starts Archive Center 2.0 Go backend in explicit shadow/noop mode. This proves process launch without MariaDB; it is not product cutover evidence.",
		GoServiceRoot: goServiceRoot,
		SafetyFlags: map[string]bool{
			"authority_switch":       false,
			"mariadb_required":       false,
			"mariadb_authority":      false,
			"milvus_required":        false,
			"chromadb_required":      true,
			"chroma_retired":         false,
			"go_default_switch":      false,
			"product_cutover_claim":  false,
			"python_runtime_retired": false,
		},
		OpenGaps: []string{
			"MariaDB-backed read/write evidence remains open.",
			"ChromaDB endpoint-backed vector retrieval evidence remains open.",
			"Python 0.8 fallback and product cutover gates remain open.",
			"main.py decomposition is tracked as 2.0-side route/store/package migration, not as edits to the 0.8 source file.",
		},
		EnvSummary: map[string]string{
			"AC_MODE":            "shadow",
			"AC_STORE_MODE":      "noop",
			"AC_MARIADB_DSN":     "unset",
			"AC_CHROMA_ENDPOINT": "unset",
		},
	}

	port, err := findAvailablePort("127.0.0.1", 28220, 28280)
	if err != nil {
		r.Status = "failed"
		r.Errors = append(r.Errors, err.Error())
		return r
	}
	r.Port = port
	r.BaseURL = fmt.Sprintf("http://127.0.0.1:%d", port)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	tempDir := ""
	if strings.TrimSpace(binPath) == "" {
		tempDir, err = os.MkdirTemp("", "archive-center-go-shadow-smoke-*")
		if err != nil {
			r.Status = "failed"
			r.Errors = append(r.Errors, fmt.Sprintf("create temp build dir: %v", err))
			return r
		}
		defer os.RemoveAll(tempDir)
	}

	spec := launchSpec{BinPath: strings.TrimSpace(binPath), GoServiceRoot: goServiceRoot, Port: port, TempDir: tempDir}
	cmd, commandMode, commandDisplay, err := buildCommand(ctx, spec)
	r.CommandMode = commandMode
	r.Command = commandDisplay
	if err != nil {
		r.Status = "failed"
		r.Errors = append(r.Errors, err.Error())
		return r
	}

	var stderr limitBuffer
	var stdout limitBuffer
	cmd.Stderr = &stderr
	cmd.Stdout = &stdout

	if err := cmd.Start(); err != nil {
		r.Status = "failed"
		r.Process.Error = err.Error()
		r.Errors = append(r.Errors, fmt.Sprintf("start backend: %v", err))
		return r
	}
	r.Process.Started = true
	if cmd.Process != nil {
		r.Process.PID = cmd.Process.Pid
	}

	waitDone := make(chan error, 1)
	go func() {
		waitDone <- cmd.Wait()
	}()

	startupCtx, startupCancel := context.WithTimeout(context.Background(), startupTimeout)
	attempts, elapsed, readyErr := waitForHealth(startupCtx, r.BaseURL, 250*time.Millisecond, probeTimeout)
	startupCancel()
	r.StartupAttempts = attempts
	r.StartupElapsed = elapsed.Milliseconds()
	if readyErr != nil {
		r.Status = "failed"
		r.Errors = append(r.Errors, fmt.Sprintf("wait for /health: %v", readyErr))
		r.Process.Error = collectProcessOutput(stdout.String(), stderr.String())
		stopProcess(cmd, cancel, waitDone)
		r.Process.Stopped = true
		return r
	}

	client := &http.Client{Timeout: probeTimeout}
	for _, path := range []string{"/health", "/ready", "/version", "/stats"} {
		pr := probe(client, r.BaseURL, path)
		r.Probes = append(r.Probes, pr)
		if pr.Status != "ok" {
			r.Status = "failed"
		}
	}

	stopProcess(cmd, cancel, waitDone)
	r.Process.Stopped = true
	if r.Status != "ok" {
		r.Process.Error = collectProcessOutput(stdout.String(), stderr.String())
	}
	return r
}

func buildCommand(ctx context.Context, spec launchSpec) (*exec.Cmd, string, []string, error) {
	env := forcedNoopEnv(os.Environ(), spec.Port)
	if spec.BinPath != "" {
		cmd := exec.CommandContext(ctx, spec.BinPath)
		cmd.Env = env
		return cmd, "binary", []string{spec.BinPath}, nil
	}

	root, err := filepath.Abs(spec.GoServiceRoot)
	if err != nil {
		return nil, "", nil, err
	}
	if _, err := os.Stat(filepath.Join(root, "go.mod")); err != nil {
		return nil, "", nil, fmt.Errorf("go service root must contain go.mod: %w", err)
	}
	if spec.TempDir == "" {
		return nil, "", nil, fmt.Errorf("temp dir is required when -bin is omitted")
	}
	binaryName := "archive-center-go"
	if runtime.GOOS == "windows" {
		binaryName += ".exe"
	}
	tempBinary := filepath.Join(spec.TempDir, binaryName)
	buildArgs := []string{"build", "-buildvcs=false", "-o", tempBinary, "./cmd/archive-center-go"}
	build := exec.CommandContext(ctx, "go", buildArgs...)
	build.Dir = root
	build.Env = env
	if out, err := build.CombinedOutput(); err != nil {
		return nil, "", nil, fmt.Errorf("build temp archive-center-go binary: %w: %s", err, strings.TrimSpace(string(out)))
	}

	cmd := exec.CommandContext(ctx, tempBinary)
	cmd.Env = env
	return cmd, "temp_binary", append(append([]string{"go"}, buildArgs...), "&&", tempBinary), nil
}

func forcedNoopEnv(base []string, port int) []string {
	blocked := map[string]bool{
		"AC_BIND_ADDR":           true,
		"AC_MODE":                true,
		"AC_STORE_MODE":          true,
		"AC_MARIADB_DSN":         true,
		"AC_STORE_FIXTURE_DIR":   true,
		"AC_CHROMA_ENDPOINT":     true,
		"AC_CHROMA_COLLECTION":   true,
		"AC_CHROMA_API_PATH":     true,
		"AC_MILVUS_ENDPOINT":     true,
		"AC_MILVUS_LITE_PATH":    true,
		"AC_MILVUS_STUB_ENABLED": true,
		"AC_MILVUS_SDK_ENABLED":  true,
		"AC_MILVUS_LIVE_ENABLED": true,
	}
	out := make([]string, 0, len(base)+3)
	for _, kv := range base {
		key, _, ok := strings.Cut(kv, "=")
		if !ok || blocked[key] {
			continue
		}
		out = append(out, kv)
	}
	out = append(out,
		fmt.Sprintf("AC_BIND_ADDR=127.0.0.1:%d", port),
		"AC_MODE=shadow",
		"AC_STORE_MODE=noop",
	)
	return out
}

func findAvailablePort(host string, startPort, maxPort int) (int, error) {
	for p := startPort; p <= maxPort; p++ {
		ln, err := net.Listen("tcp", fmt.Sprintf("%s:%d", host, p))
		if err == nil {
			_ = ln.Close()
			return p, nil
		}
	}
	return 0, fmt.Errorf("no available port in range %d-%d", startPort, maxPort)
}

func waitForHealth(ctx context.Context, baseURL string, interval time.Duration, probeTimeout time.Duration) (int, time.Duration, error) {
	start := time.Now()
	client := &http.Client{Timeout: probeTimeout}
	attempts := 0
	for {
		attempts++
		pr := probe(client, baseURL, "/health")
		if pr.Status == "ok" {
			return attempts, time.Since(start), nil
		}
		if err := ctx.Err(); err != nil {
			return attempts, time.Since(start), err
		}
		time.Sleep(interval)
	}
}

func probe(client *http.Client, baseURL string, path string) probeResult {
	url := strings.TrimRight(baseURL, "/") + path
	resp, err := client.Get(url)
	if err != nil {
		return probeResult{Path: path, Status: "failed", Error: err.Error()}
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return probeResult{Path: path, Status: "failed", HTTPStatus: resp.StatusCode, Error: err.Error()}
	}
	var decoded map[string]any
	jsonValid := json.Unmarshal(body, &decoded) == nil
	pr := probeResult{
		Path:       path,
		HTTPStatus: resp.StatusCode,
		JSONValid:  jsonValid,
	}
	if jsonValid {
		pr.TopLevelKeys = sortedKeys(decoded)
	}
	if resp.StatusCode >= 200 && resp.StatusCode < 300 && jsonValid {
		pr.Status = "ok"
	} else {
		pr.Status = "failed"
		if !jsonValid {
			pr.Error = "invalid_json"
		}
	}
	return pr
}

func sortedKeys(m map[string]any) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

func stopProcess(cmd *exec.Cmd, cancel context.CancelFunc, waitDone <-chan error) {
	cancel()
	select {
	case <-waitDone:
		return
	case <-time.After(2 * time.Second):
	}
	if cmd != nil && cmd.Process != nil {
		_ = cmd.Process.Kill()
	}
	select {
	case <-waitDone:
	case <-time.After(2 * time.Second):
	}
}

func collectProcessOutput(stdout, stderr string) string {
	combined := strings.TrimSpace(strings.Join([]string{stdout, stderr}, "\n"))
	if combined == "" {
		return ""
	}
	return combined
}

func writeReport(path string, r report) error {
	data, err := json.MarshalIndent(r, "", "  ")
	if err != nil {
		return err
	}
	data = append(data, '\n')
	if path == "" {
		_, err = os.Stdout.Write(data)
		return err
	}
	return os.WriteFile(path, data, 0644)
}

type limitBuffer struct {
	bytes.Buffer
}

func (b *limitBuffer) Write(p []byte) (int, error) {
	if b.Len() > 64*1024 {
		return len(p), nil
	}
	return b.Buffer.Write(p)
}
