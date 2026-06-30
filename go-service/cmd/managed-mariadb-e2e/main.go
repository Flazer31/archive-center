// managed-mariadb-e2e is a safe managed runner that detects MariaDB providers,
// emits clear JSON when unavailable, and defines the contract for temp MariaDB E2E
// without switching authority.
//
// It does not accept a user-prepared DSN; 2.0 manages DB creation/bootstrap.
package main

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"sync/atomic"
	"time"

	_ "github.com/go-sql-driver/mysql"
)

type providerInfo struct {
	Name      string `json:"name"`
	Path      string `json:"path"`
	Available bool   `json:"available"`
	Type      string `json:"type"`
}

type routeSmokeLiveConfig struct {
	Enabled   bool
	HTTPWait  time.Duration
	Critic    routeSmokeLLMConfig
	Embedding routeSmokeLLMConfig
}

type routeSmokeLLMConfig struct {
	Provider            string
	Endpoint            string
	APIKey              string
	Model               string
	TimeoutMs           int64
	Temperature         float64
	MaxTokens           int64
	MaxCompletionTokens int64
	ReasoningEffort     string
}

type tempPlan struct {
	DataDir     string `json:"data_dir"`
	Port        int    `json:"port"`
	DSNRedacted string `json:"dsn_redacted"`
}

type childPlanStep struct {
	Name string `json:"name"`
	Note string `json:"note,omitempty"`
}

type executedStep struct {
	Name       string         `json:"name"`
	Status     string         `json:"status"`
	Command    string         `json:"command,omitempty"`
	Redacted   string         `json:"redacted_command,omitempty"`
	ExitCode   int            `json:"exit_code"`
	DurationMs int64          `json:"duration_ms"`
	Error      string         `json:"error,omitempty"`
	Note       string         `json:"note,omitempty"`
	Details    map[string]any `json:"details,omitempty"`
}

type report struct {
	Status                string          `json:"status"`
	GeneratedAt           string          `json:"generated_at"`
	SourceMode            string          `json:"source_mode"`
	SQLiteDB              string          `json:"sqlite_db,omitempty"`
	ExportDir             string          `json:"export_dir,omitempty"`
	Execute               bool            `json:"execute"`
	KeepTemp              bool            `json:"keep_temp"`
	ProductReadProof      bool            `json:"product_read_proof"`
	RouteWriteSmoke       map[string]any  `json:"route_write_smoke,omitempty"`
	BackupRestore         map[string]any  `json:"backup_restore_drill,omitempty"`
	AuthorityCutover      map[string]any  `json:"authority_cutover_replay,omitempty"`
	DefaultSwitch         map[string]any  `json:"default_switch_rehearsal,omitempty"`
	DefaultRuntime        map[string]any  `json:"default_runtime_switch,omitempty"`
	SessionIsolationSmoke map[string]any  `json:"session_isolation_smoke,omitempty"`
	ProviderStatus        string          `json:"provider_status"`
	ProvidersChecked      []providerInfo  `json:"providers_checked"`
	TempPlan              tempPlan        `json:"temp_plan"`
	ChildPlan             []childPlanStep `json:"child_plan"`
	ExecutedSteps         []executedStep  `json:"executed_steps"`
	RollbackProof         map[string]any  `json:"rollback_proof,omitempty"`
	VectorRuntime         map[string]any  `json:"vector_runtime"`
	SafetyFlags           map[string]bool `json:"safety_flags"`
	Errors                []string        `json:"errors"`
	Warnings              []string        `json:"warnings"`
	SchemaTables          []string        `json:"schema_tables,omitempty"`
	StoreSaveTables       []string        `json:"store_save_tables,omitempty"`
	StoreListTables       []string        `json:"store_list_tables,omitempty"`
}

var (
	knownStoreSaveTables = []string{
		"chat_logs",
		"effective_input_logs",
		"memories",
		"direct_evidence_records",
		"kg_triples",
		"audit_logs",
		"critic_feedback",
		"character_events",
		"entities",
		"trust_states",
		"storylines",
		"world_rules",
		"character_states",
		"pending_threads",
		"active_states",
	}
	knownStoreListTables = []string{
		"chat_logs",
		"effective_input_logs",
		"memories",
		"direct_evidence_records",
		"kg_triples",
		"audit_logs",
		"critic_feedback",
		"character_events",
		"storylines",
		"world_rules",
		"character_states",
		"pending_threads",
		"active_states",
		"canonical_state_layers",
		"episode_summaries",
	}
)

func discoverSchemaSQL() string {
	wd, _ := os.Getwd()
	exe, err := osExecutable()
	if err != nil {
		return ""
	}
	baseDir := filepath.Dir(exe)
	candidates := []string{
		filepath.Join(wd, "migrations", "001_schema.sql"),
		filepath.Join(wd, "..", "migrations", "001_schema.sql"),
		filepath.Join(baseDir, "migrations", "001_schema.sql"),
		filepath.Join(baseDir, "..", "migrations", "001_schema.sql"),
		filepath.Join(baseDir, "..", "..", "migrations", "001_schema.sql"),
		filepath.Join(baseDir, "..", "..", "..", "migrations", "001_schema.sql"),
		filepath.Join(baseDir, "go-service", "migrations", "001_schema.sql"),
		filepath.Join(baseDir, "..", "go-service", "migrations", "001_schema.sql"),
	}
	for _, c := range candidates {
		if info, err := os.Stat(c); err == nil && !info.IsDir() {
			if abs, err := filepath.Abs(c); err == nil {
				return abs
			}
			return c
		}
	}
	return ""
}

func parseSchemaTables(path string) []string {
	if path == "" {
		return nil
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return nil
	}
	var tables []string
	for _, line := range strings.Split(string(data), "\n") {
		line = strings.TrimSpace(line)
		const prefix = "CREATE TABLE IF NOT EXISTS "
		if !strings.HasPrefix(line, prefix) {
			continue
		}
		rest := strings.TrimPrefix(line, prefix)
		rest = strings.TrimSpace(rest)
		if idx := strings.IndexFunc(rest, func(r rune) bool { return r == ' ' || r == '(' }); idx >= 0 {
			rest = rest[:idx]
		}
		rest = strings.Trim(rest, "`")
		if rest != "" {
			tables = append(tables, rest)
		}
	}
	return tables
}

type providerLookup interface {
	lookup(name string) (path string, ok bool)
}

type typedProviderLookup interface {
	lookupTyped(name string, defaultType string) (path string, ok bool, providerType string)
}

type osExecLookup struct{}

func (osExecLookup) lookup(name string) (string, bool) {
	path, err := execLookPath(name)
	if err != nil {
		return "", false
	}
	return path, true
}

// execLookPath is a thin wrapper so tests can override via build-time or
// we can swap the lookup implementation in tests.
var execLookPath = osExecLookPath

func osExecLookPath(name string) (string, error) {
	return exec.LookPath(name)
}

// osExecutable is a testable wrapper around os.Executable.
var osExecutable = os.Executable

// discoverBundled searches for a MariaDB provider binary relative to the
// current executable directory, covering common 2.0 deployment layouts.
func discoverBundled(name string) (string, bool) {
	exe, err := osExecutable()
	if err != nil {
		return "", false
	}
	return discoverBundledIn(name, filepath.Dir(exe))
}

func discoverBundledIn(name string, baseDir string) (string, bool) {
	if name != "mariadbd" && name != "mysqld" {
		return "", false
	}

	roots := []string{
		filepath.Join(baseDir, "mariadb", "bin"),
		filepath.Join(baseDir, "MariaDB", "bin"),
		filepath.Join(baseDir, "runtime", "mariadb", "bin"),
		filepath.Join(baseDir, "runtime", "MariaDB", "bin"),
		filepath.Join(baseDir, "..", "mariadb", "bin"),
		filepath.Join(baseDir, "..", "MariaDB", "bin"),
		filepath.Join(baseDir, "..", "runtime", "mariadb", "bin"),
		filepath.Join(baseDir, "..", "runtime", "MariaDB", "bin"),
		filepath.Join(baseDir, "..", "vendor", "mariadb", "bin"),
		filepath.Join(baseDir, "..", "vendor", "MariaDB", "bin"),
		filepath.Join(baseDir, "..", "resources", "mariadb", "bin"),
		filepath.Join(baseDir, "..", "resources", "MariaDB", "bin"),
		filepath.Join(baseDir, "..", "..", "mariadb", "bin"),
		filepath.Join(baseDir, "..", "..", "MariaDB", "bin"),
		filepath.Join(baseDir, "..", "..", "runtime", "mariadb", "bin"),
		filepath.Join(baseDir, "..", "..", "runtime", "MariaDB", "bin"),
	}

	names := []string{name}
	if runtime.GOOS == "windows" {
		names = append(names, name+".exe")
	}
	for _, root := range roots {
		for _, candidateName := range names {
			c := filepath.Join(root, candidateName)
			if info, err := os.Stat(c); err == nil && !info.IsDir() {
				if runtime.GOOS != "windows" && info.Mode()&0111 == 0 {
					continue
				}
				if abs, err := filepath.Abs(c); err == nil {
					return abs, true
				}
				return c, true
			}
		}
	}
	return "", false
}

// bundledLookup checks for a bundled MariaDB provider relative to the
// executable before falling back to PATH lookup.
type bundledLookup struct {
	fallback providerLookup
}

func (b bundledLookup) lookup(name string) (string, bool) {
	path, ok, _ := b.lookupTyped(name, "direct")
	return path, ok
}

func (b bundledLookup) lookupTyped(name string, defaultType string) (string, bool, string) {
	if path, ok := discoverBundled(name); ok {
		return path, true, "bundled_direct"
	}
	if b.fallback == nil {
		return "", false, defaultType
	}
	path, ok := b.fallback.lookup(name)
	return path, ok, defaultType
}

func lookupProvider(lookup providerLookup, name string, defaultType string) providerInfo {
	if typed, ok := lookup.(typedProviderLookup); ok {
		path, available, providerType := typed.lookupTyped(name, defaultType)
		return providerInfo{Name: name, Path: path, Available: available, Type: providerType}
	}
	path, available := lookup.lookup(name)
	return providerInfo{Name: name, Path: path, Available: available, Type: defaultType}
}

// commandExecutor abstracts os/exec for testability.
type commandExecutor interface {
	LookPath(name string) (string, error)
	Run(ctx context.Context, name string, arg ...string) ([]byte, error)
	Start(ctx context.Context, name string, arg ...string) (*exec.Cmd, error)
	StartWithEnv(ctx context.Context, name string, env []string, arg ...string) (*exec.Cmd, error)
	Kill(cmd *exec.Cmd) error
}

type osExecutor struct{}

func (osExecutor) LookPath(name string) (string, error) {
	return exec.LookPath(name)
}

func (osExecutor) Run(ctx context.Context, name string, arg ...string) ([]byte, error) {
	cmd := exec.CommandContext(ctx, name, arg...)
	return cmd.CombinedOutput()
}

func (osExecutor) Start(ctx context.Context, name string, arg ...string) (*exec.Cmd, error) {
	cmd := exec.CommandContext(ctx, name, arg...)
	cmd.Stdin = nil
	cmd.Stdout = nil
	cmd.Stderr = nil
	if err := cmd.Start(); err != nil {
		return nil, err
	}
	return cmd, nil
}

func (osExecutor) StartWithEnv(ctx context.Context, name string, env []string, arg ...string) (*exec.Cmd, error) {
	cmd := exec.CommandContext(ctx, name, arg...)
	cmd.Env = env
	cmd.Stdin = nil
	cmd.Stdout = nil
	cmd.Stderr = nil
	if err := cmd.Start(); err != nil {
		return nil, err
	}
	return cmd, nil
}

func (osExecutor) Kill(cmd *exec.Cmd) error {
	if cmd == nil || cmd.Process == nil {
		return nil
	}
	return cmd.Process.Kill()
}

var defaultExecutor commandExecutor = osExecutor{}

type directProviderConfig struct {
	ProviderName          string
	ProviderPath          string
	DataDir               string
	Port                  int
	SessionID             string
	SQLiteDB              string
	ExportDir             string
	KeepTemp              bool
	GoBinPath             string
	GoHTTPPort            int
	PythonBaseURL         string
	PythonFallbackSrc     string
	PythonFallbackPort    int
	ProductReadProof      bool
	RouteWriteSmoke       bool
	BackupRestore         bool
	AuthorityCutover      bool
	DefaultSwitch         bool
	DefaultSwitchActual   bool
	SessionIsolationSmoke bool
}

func (cfg directProviderConfig) skipDefaultReadShadow() bool {
	return cfg.SessionIsolationSmoke &&
		!cfg.ProductReadProof &&
		!cfg.RouteWriteSmoke &&
		!cfg.BackupRestore &&
		!cfg.AuthorityCutover &&
		!cfg.DefaultSwitch &&
		!cfg.DefaultSwitchActual
}

type directProviderRunner interface {
	run(ctx context.Context, cfg directProviderConfig) ([]executedStep, error)
}

type osDirectProviderRunner struct {
	exec commandExecutor
}

func newOSDirectProviderRunner() *osDirectProviderRunner {
	return &osDirectProviderRunner{exec: defaultExecutor}
}

type pythonFallbackProcess struct {
	cmd    *exec.Cmd
	stdout *os.File
	stderr *os.File
}

func (p *pythonFallbackProcess) stop() {
	if p == nil || p.cmd == nil {
		return
	}
	if p.cmd.Process != nil {
		_ = p.cmd.Process.Kill()
	}
	_ = p.cmd.Wait()
	if p.stdout != nil {
		_ = p.stdout.Close()
	}
	if p.stderr != nil {
		_ = p.stderr.Close()
	}
}

// degradedError signals that temp DB setup succeeded but value reporting failed.
type degradedError struct {
	msg string
}

func (e *degradedError) Error() string { return e.msg }

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

func copyPythonFallbackSource(ctx context.Context, srcDir, dstDir string) error {
	if strings.TrimSpace(srcDir) == "" {
		return fmt.Errorf("python fallback source directory is empty")
	}
	if runtime.GOOS != "windows" {
		return fmt.Errorf("python fallback temp-copy is currently implemented for Windows robocopy only")
	}
	if err := os.MkdirAll(dstDir, 0755); err != nil {
		return err
	}
	cmd := exec.CommandContext(ctx, "robocopy", srcDir, dstDir, "/E", "/XD", ".venv", "/NP", "/NFL", "/NDL")
	out, err := cmd.CombinedOutput()
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			code := exitErr.ExitCode()
			if code < 8 {
				return nil
			}
			return fmt.Errorf("robocopy failed with code %d: %s", code, string(out))
		}
		return fmt.Errorf("robocopy failed: %w: %s", err, string(out))
	}
	return nil
}

func startPythonFallbackBackend(ctx context.Context, tempDir string, port int) (*pythonFallbackProcess, executedStep, error) {
	stdout, err := os.Create(filepath.Join(tempDir, "python-fallback.stdout.log"))
	if err != nil {
		return nil, executedStep{Name: "python-fallback-start", Status: "failed", Error: err.Error()}, err
	}
	stderr, err := os.Create(filepath.Join(tempDir, "python-fallback.stderr.log"))
	if err != nil {
		_ = stdout.Close()
		return nil, executedStep{Name: "python-fallback-start", Status: "failed", Error: err.Error()}, err
	}
	args := []string{"-m", "uvicorn", "--app-dir", tempDir, "backend.main:app", "--host", "127.0.0.1", "--port", strconv.Itoa(port)}
	cmd := exec.CommandContext(ctx, "python", args...)
	cmd.Dir = tempDir
	cmd.Env = append(os.Environ(), "PYTHONDONTWRITEBYTECODE=1")
	cmd.Stdout = stdout
	cmd.Stderr = stderr
	if err := cmd.Start(); err != nil {
		_ = stdout.Close()
		_ = stderr.Close()
		return nil, executedStep{
			Name:    "python-fallback-start",
			Status:  "failed",
			Command: "python " + strings.Join(args, " "),
			Error:   err.Error(),
		}, err
	}
	return &pythonFallbackProcess{cmd: cmd, stdout: stdout, stderr: stderr}, executedStep{
		Name:    "python-fallback-start",
		Status:  "ok",
		Command: "python " + strings.Join(args, " "),
	}, nil
}

func waitPythonFallbackReady(ctx context.Context, port int) executedStep {
	baseURL := fmt.Sprintf("http://127.0.0.1:%d", port)
	deadline := time.Now().Add(2 * time.Minute)
	for time.Now().Before(deadline) {
		probe, err := probeGET(ctx, baseURL+"/health")
		if err == nil && probeStatusOK(probe) {
			return executedStep{Name: "python-fallback-wait-ready", Status: "ok"}
		}
		select {
		case <-ctx.Done():
			return executedStep{Name: "python-fallback-wait-ready", Status: "failed", Error: ctx.Err().Error()}
		case <-time.After(500 * time.Millisecond):
		}
	}
	return executedStep{Name: "python-fallback-wait-ready", Status: "failed", Error: "health check timed out"}
}

type goBackendStartConfig struct {
	BinPath          string
	Port             int
	DSN              string
	StoreMode        string
	ProductReadProof bool
}

func startGoBackend(ctx context.Context, exec commandExecutor, cfg goBackendStartConfig) (*exec.Cmd, executedStep, error) {
	env := os.Environ()
	env = append(env, fmt.Sprintf("AC_BIND_ADDR=127.0.0.1:%d", cfg.Port))
	env = append(env, "AC_STORE_MODE="+cfg.StoreMode)
	if strings.TrimSpace(cfg.DSN) != "" {
		env = append(env, "AC_MARIADB_DSN="+cfg.DSN)
	}
	if cfg.ProductReadProof {
		env = append(env, "AC_MARIADB_PRODUCT_READ_ENABLED=true")
	}

	parts := []string{
		fmt.Sprintf("AC_STORE_MODE=%s", cfg.StoreMode),
		fmt.Sprintf("AC_BIND_ADDR=127.0.0.1:%d", cfg.Port),
	}
	if strings.TrimSpace(cfg.DSN) != "" {
		parts = append(parts, "AC_MARIADB_DSN="+redactDSN(cfg.DSN))
	}
	if cfg.ProductReadProof {
		parts = append(parts, "AC_MARIADB_PRODUCT_READ_ENABLED=true")
	}
	if endpoint := strings.TrimSpace(os.Getenv("AC_CHROMA_ENDPOINT")); endpoint != "" {
		parts = append(parts, "AC_CHROMA_ENDPOINT="+routeSmokeEndpointHost(endpoint))
	}
	parts = append(parts, cfg.BinPath)
	redactedCommand := strings.Join(parts, " ")
	cmd, err := exec.StartWithEnv(ctx, cfg.BinPath, env)
	if err != nil {
		return nil, executedStep{
			Name:     "start-go-backend",
			Status:   "failed",
			Command:  redactedCommand,
			Redacted: redactedCommand,
			ExitCode: -1,
			Error:    err.Error(),
		}, err
	}

	step := executedStep{
		Name:     "start-go-backend",
		Status:   "ok",
		Command:  redactedCommand,
		Redacted: redactedCommand,
	}
	return cmd, step, nil
}

func waitGoReady(ctx context.Context, port int) executedStep {
	start := time.Now()
	url := fmt.Sprintf("http://127.0.0.1:%d/health", port)
	ticker := time.NewTicker(200 * time.Millisecond)
	defer ticker.Stop()

	deadline, hasDeadline := ctx.Deadline()
	if !hasDeadline {
		deadline = time.Now().Add(30 * time.Second)
	} else if capDeadline := time.Now().Add(30 * time.Second); capDeadline.Before(deadline) {
		deadline = capDeadline
	}

	for {
		select {
		case <-ctx.Done():
			if time.Now().After(deadline) {
				return executedStep{
					Name:       "wait-go-ready",
					Status:     "failed",
					DurationMs: time.Since(start).Milliseconds(),
					Error:      fmt.Sprintf("go backend not ready on port %d within timeout", port),
				}
			}
			return executedStep{
				Name:       "wait-go-ready",
				Status:     "failed",
				DurationMs: time.Since(start).Milliseconds(),
				Error:      "context cancelled before go backend became ready",
			}
		case <-ticker.C:
			req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
			if err != nil {
				continue
			}
			resp, err := http.DefaultClient.Do(req)
			if err == nil {
				_ = resp.Body.Close()
				if resp.StatusCode == http.StatusOK {
					return executedStep{
						Name:       "wait-go-ready",
						Status:     "ok",
						DurationMs: time.Since(start).Milliseconds(),
					}
				}
			}
			if time.Now().After(deadline) {
				return executedStep{
					Name:       "wait-go-ready",
					Status:     "failed",
					DurationMs: time.Since(start).Milliseconds(),
					Error:      fmt.Sprintf("go backend not ready on port %d within timeout", port),
				}
			}
		}
	}
}

func fetchReadyChecks(ctx context.Context, port int) (map[string]string, error) {
	url := fmt.Sprintf("http://127.0.0.1:%d/ready", port)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("ready status %d", resp.StatusCode)
	}
	var body struct {
		Checks map[string]string `json:"checks"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		return nil, err
	}
	if body.Checks == nil {
		return nil, fmt.Errorf("ready response missing checks")
	}
	return body.Checks, nil
}

func runRouteWriteSmoke(ctx context.Context, port int, dsn string, sessionID string, storeMode string) (map[string]any, error) {
	baseURL := fmt.Sprintf("http://127.0.0.1:%d", port)
	smokeSession := safeSessionID(sessionID) + "-route-write"
	before, err := routeSmokeCounts(ctx, dsn, smokeSession)
	if err != nil {
		return nil, fmt.Errorf("route smoke pre-count: %w", err)
	}

	routes := []map[string]any{}
	liveCfg := routeSmokeLiveConfigFromEnv()
	clientMeta := map[string]any{}
	criticProvider := "managed_stub_openai_compatible"
	embeddingProvider := "not_configured"
	var criticStub *routeSmokeCriticStub
	if liveCfg.Enabled {
		clientMeta = routeSmokeLiveClientMeta(liveCfg)
		criticProvider = "configured_live_provider"
		embeddingProvider = "configured_live_provider"
	} else {
		criticStub = startRouteSmokeCriticStub()
		defer criticStub.Close()
		clientMeta = routeSmokeStubClientMeta(criticStub.URL())
	}

	firstCompleteBody := routeSmokeCompleteTurnBodyWithClientMeta(smokeSession, 9101, "first", clientMeta, liveCfg.Enabled)
	firstCompleteRoute, err := postJSONWithTimeout(ctx, baseURL+"/complete-turn", firstCompleteBody, liveCfg.HTTPWait)
	routes = append(routes, firstCompleteRoute)
	if err != nil {
		return routeSmokeReport("failed", baseURL, smokeSession, before, nil, routes, storeMode), err
	}
	afterOneTurn, err := routeSmokeCounts(ctx, dsn, smokeSession)
	if err != nil {
		return routeSmokeReport("failed", baseURL, smokeSession, before, nil, routes, storeMode), fmt.Errorf("route smoke one-turn count: %w", err)
	}

	secondCompleteBody := routeSmokeCompleteTurnBodyWithClientMeta(smokeSession, 9101, "second", clientMeta, liveCfg.Enabled)
	secondCompleteRoute, err := postJSONWithTimeout(ctx, baseURL+"/complete-turn", secondCompleteBody, liveCfg.HTTPWait)
	routes = append(routes, secondCompleteRoute)
	if err != nil {
		return routeSmokeReport("failed", baseURL, smokeSession, before, nil, routes, storeMode), err
	}
	afterTwoTurns, err := routeSmokeCounts(ctx, dsn, smokeSession)
	if err != nil {
		return routeSmokeReport("failed", baseURL, smokeSession, before, nil, routes, storeMode), fmt.Errorf("route smoke two-turn count: %w", err)
	}

	effectiveBody := map[string]any{
		"chat_session_id": smokeSession,
		"turn_index":      9102,
		"effective_input": "route smoke effective input",
	}
	effectiveRoute, err := postJSON(ctx, baseURL+"/effective-inputs", effectiveBody)
	routes = append(routes, effectiveRoute)
	if err != nil {
		return routeSmokeReport("failed", baseURL, smokeSession, before, nil, routes, storeMode), err
	}

	canonicalRoutes := []struct {
		path string
		body map[string]any
	}{
		{"/canonical/" + smokeSession + "/chat-logs", map[string]any{
			"turn_index": 9201,
			"role":       "assistant",
			"content":    "canonical route smoke chat log",
		}},
		{"/canonical/" + smokeSession + "/effective-inputs", map[string]any{
			"turn_index":      9202,
			"effective_input": "canonical route smoke effective input",
		}},
		{"/canonical/" + smokeSession + "/memories", map[string]any{
			"turn_index":             9203,
			"summary_json":           `{"summary":"canonical route smoke memory"}`,
			"embedding":              "[]",
			"embedding_model":        "route-smoke",
			"importance":             0.5,
			"emotional_boost":        0.1,
			"evidence":               `{"text":"canonical route smoke evidence text"}`,
			"emotional_intensity":    0.2,
			"narrative_significance": 0.3,
			"place_wing":             "test",
			"place_room":             "route-smoke",
		}},
		{"/canonical/" + smokeSession + "/evidence", map[string]any{
			"evidence_kind":     "direct",
			"evidence_text":     "canonical route smoke direct evidence",
			"source_turn_start": 9201,
			"source_turn_end":   9203,
			"turn_anchor":       9203,
			"source_hash":       "route-smoke",
			"archive_state":     "active",
			"capture_stage":     "route_smoke",
		}},
		{"/canonical/" + smokeSession + "/kg-triples", map[string]any{
			"subject":     "RouteSmoke",
			"predicate":   "touches",
			"object":      "MariaDB",
			"valid_from":  9201,
			"valid_to":    0,
			"source_turn": 9203,
		}},
		{"/canonical/" + smokeSession + "/audit-logs", map[string]any{
			"event_type":   "route_smoke",
			"target_type":  "managed_mariadb_e2e",
			"target_id":    9203,
			"summary":      "canonical route smoke audit",
			"details_json": `{"source":"managed_mariadb_e2e"}`,
			"source":       "route_smoke",
		}},
		{"/canonical/" + smokeSession + "/critic-feedback", map[string]any{
			"target_type":    "turn",
			"target_id":      9203,
			"feedback_value": "ok",
			"feedback_note":  "canonical route smoke feedback",
			"source":         "route_smoke",
		}},
		{"/canonical/" + smokeSession + "/character-events", map[string]any{
			"character_name": "RouteSmoke",
			"turn_index":     9203,
			"event_type":     "smoke",
			"details_json":   `{"source":"managed_mariadb_e2e"}`,
		}},
	}
	for _, route := range canonicalRoutes {
		routeResult, err := postJSON(ctx, baseURL+route.path, route.body)
		routes = append(routes, routeResult)
		if err != nil {
			return routeSmokeReport("failed", baseURL, smokeSession, before, nil, routes, storeMode), err
		}
	}

	after, err := routeSmokeCounts(ctx, dsn, smokeSession)
	if err != nil {
		return routeSmokeReport("failed", baseURL, smokeSession, before, nil, routes, storeMode), fmt.Errorf("route smoke post-count: %w", err)
	}

	delta := routeSmokeDelta(before, after)
	oneTurnDelta := routeSmokeDelta(before, afterOneTurn)
	twoTurnDelta := routeSmokeDelta(before, afterTwoTurns)
	nonCompleteRouteDelta := routeSmokeDelta(afterTwoTurns, after)
	contentChecks, contentErr := routeSmokeContentChecks(ctx, dsn, smokeSession, liveCfg)
	if contentErr != nil {
		return routeSmokeReport("failed", baseURL, smokeSession, before, after, routes, storeMode), fmt.Errorf("route smoke content check: %w", contentErr)
	}
	liveProviderChecks := routeSmokeLiveProviderResponseChecks(routes, liveCfg)
	expectedMin := map[string]int{
		"chat_logs":               5,
		"effective_input_logs":    4,
		"memories":                3,
		"direct_evidence_records": 3,
		"kg_triples":              3,
		"audit_logs":              5,
		"critic_feedback":         3,
		"character_events":        3,
		"entities":                2,
		"trust_states":            2,
		"storylines":              2,
		"world_rules":             2,
		"character_states":        2,
		"pending_threads":         2,
		"active_states":           6,
	}
	expectedCompleteMin := map[string]int{
		"chat_logs":               4,
		"effective_input_logs":    2,
		"memories":                2,
		"direct_evidence_records": 2,
		"kg_triples":              2,
		"audit_logs":              4,
		"critic_feedback":         2,
		"character_events":        2,
		"entities":                2,
		"trust_states":            2,
		"storylines":              2,
		"world_rules":             2,
		"character_states":        2,
		"pending_threads":         2,
		"active_states":           6,
	}
	ok := true
	for table, expected := range expectedMin {
		if delta[table] < expected {
			ok = false
			break
		}
	}
	for table, expected := range expectedCompleteMin {
		if twoTurnDelta[table] < expected {
			ok = false
			break
		}
	}
	if all, _ := contentChecks["all_expected_content_found"].(bool); !all {
		ok = false
	}
	if liveCfg.Enabled {
		if all, _ := liveProviderChecks["all_live_provider_checks_passed"].(bool); !all {
			ok = false
		}
	}
	status := "ok"
	if !ok {
		status = "failed"
	}
	report := routeSmokeReport(status, baseURL, smokeSession, before, after, routes, storeMode)
	report["delta_counts"] = delta
	report["one_turn_delta_counts"] = oneTurnDelta
	report["two_turn_delta_counts"] = twoTurnDelta
	report["complete_turn_delta_counts"] = twoTurnDelta
	report["non_complete_route_delta_counts"] = nonCompleteRouteDelta
	report["expected_min_delta"] = expectedMin
	report["expected_complete_turn_min_delta"] = expectedCompleteMin
	report["content_checks"] = contentChecks
	report["live_provider_checks"] = liveProviderChecks
	report["critic_stub_calls"] = routeSmokeCriticStubCalls(criticStub)
	report["critic_provider"] = criticProvider
	report["critic_provider_detail"] = routeSmokeLLMReport(liveCfg.Critic, liveCfg.Enabled)
	report["embedding_provider"] = embeddingProvider
	report["embedding_provider_detail"] = routeSmokeLLMReport(liveCfg.Embedding, liveCfg.Enabled)
	report["provider_mode"] = routeSmokeProviderMode(liveCfg)
	report["authority_switch"] = storeMode == "mariadb_authority"
	report["persistent_switch"] = false
	report["go_default_switch"] = false
	if !ok {
		return report, fmt.Errorf("route write smoke count delta below expectation: %+v", delta)
	}
	return report, nil
}

func runSessionIsolationSmoke(ctx context.Context, baseURL string, criticStub *routeSmokeCriticStub, sessionPrefix string) (map[string]any, error) {
	sessionA := sessionPrefix + "-a"
	sessionB := sessionPrefix + "-b"
	routes := []map[string]any{}
	steps := []struct {
		session string
		turn    int
		label   string
	}{
		{session: sessionA, turn: 1, label: "session-a-first"},
		{session: sessionA, turn: 1, label: "session-a-second-stale-request"},
		{session: sessionB, turn: 1, label: "session-b-first"},
	}
	for _, step := range steps {
		result, err := postJSON(ctx, baseURL+"/complete-turn", routeSmokeCompleteTurnBody(step.session, step.turn, step.label, criticStub.URL()))
		routes = append(routes, result)
		if err != nil {
			return map[string]any{
				"status":   "failed",
				"sessions": []string{sessionA, sessionB},
				"routes":   routes,
			}, err
		}
	}

	sessionsResult, sessionsErr := probeGET(ctx, baseURL+"/sessions")
	timelineA, timelineAErr := probeGET(ctx, baseURL+"/timeline?sessionId="+url.QueryEscape(sessionA)+"&limit=40")
	timelineB, timelineBErr := probeGET(ctx, baseURL+"/timeline?sessionId="+url.QueryEscape(sessionB)+"&limit=40")

	sessionChecks := checkSessionListIsolation(sessionsResult, map[string]int{sessionA: 4, sessionB: 2})
	timelineACheck := checkTimelineIsolation(timelineA, sessionA, 4)
	timelineBCheck := checkTimelineIsolation(timelineB, sessionB, 2)
	ok := sessionsErr == nil && timelineAErr == nil && timelineBErr == nil &&
		boolField(sessionChecks, "ok") &&
		boolField(timelineACheck, "ok") &&
		boolField(timelineBCheck, "ok")

	status := "ok"
	if !ok {
		status = "failed"
	}
	report := map[string]any{
		"status":         status,
		"base_url":       baseURL,
		"sessions":       []string{sessionA, sessionB},
		"routes":         routes,
		"sessions_probe": sessionsResult,
		"timeline_a":     timelineA,
		"timeline_b":     timelineB,
		"checks": map[string]any{
			"sessions_list": sessionChecks,
			"timeline_a":    timelineACheck,
			"timeline_b":    timelineBCheck,
		},
		"expected_chat_log_counts": map[string]int{
			sessionA: 4,
			sessionB: 2,
		},
	}
	if !ok {
		errs := []string{}
		for _, err := range []error{sessionsErr, timelineAErr, timelineBErr} {
			if err != nil {
				errs = append(errs, err.Error())
			}
		}
		if len(errs) == 0 {
			errs = append(errs, "session isolation checks failed")
		}
		report["errors"] = errs
		return report, errors.New(strings.Join(errs, "; "))
	}
	return report, nil
}

func checkSessionListIsolation(probe map[string]any, expected map[string]int) map[string]any {
	out := map[string]any{
		"ok":       false,
		"expected": expected,
		"found":    map[string]any{},
	}
	if probe == nil || probe["status"] != "ok" {
		out["error"] = "sessions probe failed"
		return out
	}
	body, _ := probe["json"].(map[string]any)
	rows, _ := body["sessions"].([]any)
	found := map[string]any{}
	ok := true
	for _, row := range rows {
		obj, _ := row.(map[string]any)
		sid := strings.TrimSpace(fmt.Sprint(obj["chat_session_id"]))
		if _, wants := expected[sid]; !wants {
			continue
		}
		chatCount := intFromAny(obj["chat_logs_count"])
		found[sid] = map[string]any{
			"chat_logs_count":  chatCount,
			"memories_count":   intFromAny(obj["memories_count"]),
			"kg_triples_count": intFromAny(obj["kg_triples_count"]),
		}
		if chatCount != expected[sid] {
			ok = false
		}
	}
	for sid := range expected {
		if _, exists := found[sid]; !exists {
			ok = false
		}
	}
	out["found"] = found
	out["ok"] = ok
	return out
}

func checkTimelineIsolation(probe map[string]any, sessionID string, expectedChatLogs int) map[string]any {
	out := map[string]any{
		"ok":                 false,
		"session_id":         sessionID,
		"expected_chat_logs": expectedChatLogs,
	}
	if probe == nil || probe["status"] != "ok" {
		out["error"] = "timeline probe failed"
		return out
	}
	body, _ := probe["json"].(map[string]any)
	items, _ := body["items"].([]any)
	meta, _ := body["meta"].(map[string]any)
	sourceCounts, _ := meta["source_counts"].(map[string]any)
	chatLogs := intFromAny(sourceCounts["chat_logs"])
	foreignItems := []string{}
	chatLogItems := 0
	for _, item := range items {
		obj, _ := item.(map[string]any)
		itemSession := strings.TrimSpace(fmt.Sprint(obj["chat_session_id"]))
		itemType := strings.TrimSpace(fmt.Sprint(obj["type"]))
		if itemSession != "" && itemSession != sessionID {
			foreignItems = append(foreignItems, itemSession)
		}
		if itemType == "chat_log" {
			chatLogItems++
		}
	}
	ok := chatLogs == expectedChatLogs && chatLogItems == expectedChatLogs && len(foreignItems) == 0
	out["source_counts_chat_logs"] = chatLogs
	out["chat_log_items"] = chatLogItems
	out["total_items"] = len(items)
	out["foreign_items"] = foreignItems
	out["ok"] = ok
	return out
}

func checkSessionDelete(deleteProbe map[string]any, sessionsProbe map[string]any, timelineProbe map[string]any, sessionID string) map[string]any {
	out := map[string]any{
		"ok":         false,
		"session_id": sessionID,
	}
	deleteOK := false
	if deleteProbe != nil && deleteProbe["status"] == "ok" {
		body, _ := deleteProbe["response"].(map[string]any)
		deleteOK = body["deleted"] == true && body["mutation_enabled"] == true
		out["delete_status"] = body["status"]
		out["delete_source"] = body["source"]
		out["delete_ok"] = deleteOK
	} else {
		out["delete_error"] = "delete probe failed"
	}

	stillListed := false
	if sessionsProbe != nil && sessionsProbe["status"] == "ok" {
		body, _ := sessionsProbe["json"].(map[string]any)
		rows, _ := body["sessions"].([]any)
		for _, row := range rows {
			obj, _ := row.(map[string]any)
			if strings.TrimSpace(fmt.Sprint(obj["chat_session_id"])) == sessionID {
				stillListed = true
				break
			}
		}
		out["still_listed"] = stillListed
	} else {
		out["sessions_after_delete_error"] = "sessions probe failed"
	}

	timelineEmpty := false
	timelineForeign := []string{}
	if timelineProbe != nil && timelineProbe["status"] == "ok" {
		body, _ := timelineProbe["json"].(map[string]any)
		items, _ := body["items"].([]any)
		for _, item := range items {
			obj, _ := item.(map[string]any)
			timelineForeign = append(timelineForeign, strings.TrimSpace(fmt.Sprint(obj["chat_session_id"])))
		}
		timelineEmpty = len(items) == 0
		out["timeline_items_after_delete"] = len(items)
		out["timeline_sessions_after_delete"] = timelineForeign
	} else {
		out["timeline_after_delete_error"] = "timeline probe failed"
	}

	out["ok"] = deleteOK && !stillListed && timelineEmpty
	return out
}

func checkRollbackMutation(deleteProbe map[string]any, chatProbe map[string]any, memProbe map[string]any, kgProbe map[string]any, auditProbe map[string]any, sessionID string, fromTurn int) map[string]any {
	out := map[string]any{
		"ok":         false,
		"session_id": sessionID,
		"from_turn":  fromTurn,
	}

	rollbackOK := false
	deletionsOK := false
	if deleteProbe != nil && deleteProbe["status"] == "ok" {
		body := responseJSONFromProbe(deleteProbe)
		plan := mapFromAny(body["rollback_plan"])
		deletions := mapFromAny(body["deletions"])
		required := []string{
			"chat_logs",
			"effective_inputs",
			"memories",
			"direct_evidence",
			"kg_triples",
			"critic_feedback",
			"character_events",
			"entities",
			"trust_states",
			"storylines",
			"world_rules",
			"character_states",
			"pending_threads",
			"active_states",
			"canonical_state_layers",
			"episode_summaries",
			"vectors",
			"rollback_audit",
		}
		missing := []string{}
		for _, key := range required {
			item := mapFromAny(deletions[key])
			if item["ok"] != true {
				missing = append(missing, key)
			}
		}
		deletionsOK = len(missing) == 0
		rollbackOK = body["status"] == "ok" &&
			plan["status"] == "executed" &&
			plan["mutation_enabled"] == true &&
			plan["would_delete"] == true
		out["rollback_status"] = body["status"]
		out["rollback_source"] = body["source"]
		out["rollback_plan"] = plan
		out["delete_keys_ok"] = deletionsOK
		out["delete_keys_missing_or_failed"] = missing
	} else {
		out["rollback_error"] = "rollback delete probe failed"
	}

	chatCheck := checkSessionScopedItems(chatProbe, sessionID, 2, "total")
	memCheck := checkSessionScopedItems(memProbe, sessionID, 1, "total")
	auditCheck := checkAuditTotal(auditProbe, 1)

	kgOK := false
	kgTotal := 0
	kgInvalidated := 0
	kgStillOpen := 0
	kgForeign := []string{}
	if kgProbe != nil && kgProbe["status"] == "ok" {
		body := responseJSONFromProbe(kgProbe)
		items, _ := body["items"].([]any)
		kgTotal = intFromAny(body["total"])
		for _, item := range items {
			obj := mapFromAny(item)
			itemSession := strings.TrimSpace(fmt.Sprint(obj["chat_session_id"]))
			if itemSession != "" && itemSession != sessionID {
				kgForeign = append(kgForeign, itemSession)
			}
			validTo := intFromAny(obj["valid_to"])
			if validTo == fromTurn-1 {
				kgInvalidated++
			}
			if validTo == 0 || validTo >= fromTurn {
				kgStillOpen++
			}
		}
		kgOK = kgTotal == 2 && len(items) == 2 && kgInvalidated >= 1 && kgStillOpen >= 1 && len(kgForeign) == 0
	}
	kgCheck := map[string]any{
		"ok":                   kgOK,
		"total":                kgTotal,
		"invalidated_valid_to": kgInvalidated,
		"still_open":           kgStillOpen,
		"foreign_items":        kgForeign,
		"expected_total":       2,
		"expected_valid_to":    fromTurn - 1,
	}

	out["checks"] = map[string]any{
		"rollback_response": rollbackOK,
		"delete_keys":       deletionsOK,
		"chat_logs":         chatCheck,
		"memories":          memCheck,
		"kg_valid_to":       kgCheck,
		"audit":             auditCheck,
	}
	out["ok"] = rollbackOK &&
		deletionsOK &&
		boolField(chatCheck, "ok") &&
		boolField(memCheck, "ok") &&
		boolField(kgCheck, "ok") &&
		boolField(auditCheck, "ok")
	return out
}

func responseJSONFromProbe(probe map[string]any) map[string]any {
	if probe == nil {
		return map[string]any{}
	}
	if body, ok := probe["json"].(map[string]any); ok {
		return body
	}
	if body, ok := probe["response"].(map[string]any); ok {
		return body
	}
	return map[string]any{}
}

func checkSessionScopedItems(probe map[string]any, sessionID string, expectedItems int, countField string) map[string]any {
	out := map[string]any{
		"ok":             false,
		"session_id":     sessionID,
		"expected_items": expectedItems,
		"count_field":    countField,
	}
	if probe == nil || probe["status"] != "ok" {
		out["error"] = "probe failed"
		return out
	}
	body := responseJSONFromProbe(probe)
	items, _ := body["items"].([]any)
	foreignItems := []string{}
	for _, item := range items {
		obj, _ := item.(map[string]any)
		itemSession := strings.TrimSpace(fmt.Sprint(obj["chat_session_id"]))
		if itemSession != "" && itemSession != sessionID {
			foreignItems = append(foreignItems, itemSession)
		}
	}
	count := len(items)
	if countField != "" {
		if value, exists := body[countField]; exists {
			count = intFromAny(value)
		}
	}
	out["item_count"] = len(items)
	out["count"] = count
	out["foreign_items"] = foreignItems
	out["ok"] = count == expectedItems && len(items) == expectedItems && len(foreignItems) == 0
	return out
}

func checkStatsCounts(probe map[string]any, expected map[string]int) map[string]any {
	out := map[string]any{"ok": false, "expected": expected}
	if probe == nil || probe["status"] != "ok" {
		out["error"] = "probe failed"
		return out
	}
	body := responseJSONFromProbe(probe)
	actual := map[string]int{}
	ok := true
	for key, want := range expected {
		got := intFromAny(body[key])
		actual[key] = got
		if got != want {
			ok = false
		}
	}
	out["actual"] = actual
	out["ok"] = ok
	return out
}

func firstItemID(probe map[string]any) int64 {
	body := responseJSONFromProbe(probe)
	items, _ := body["items"].([]any)
	if len(items) == 0 {
		return 0
	}
	item, _ := items[0].(map[string]any)
	return int64(intFromAny(item["id"]))
}

func checkFeedbackPost(probe map[string]any, targetID int64) map[string]any {
	out := map[string]any{"ok": false, "target_id": targetID}
	if targetID <= 0 {
		out["error"] = "target id missing"
		return out
	}
	if probe == nil || probe["status"] != "ok" {
		out["error"] = "probe failed"
		return out
	}
	body := responseJSONFromProbe(probe)
	out["response_status"] = body["status"]
	out["feedback_value"] = body["feedback_value"]
	out["feedback_id"] = body["feedback_id"]
	out["ok"] = body["status"] == "ok" && body["ok"] == true && body["feedback_value"] == "up"
	return out
}

func checkFeedbackLatest(probe map[string]any, targetID int64, expectedValue string) map[string]any {
	out := map[string]any{"ok": false, "target_id": targetID, "expected_value": expectedValue}
	if targetID <= 0 {
		out["error"] = "target id missing"
		return out
	}
	if probe == nil || probe["status"] != "ok" {
		out["error"] = "probe failed"
		return out
	}
	body := responseJSONFromProbe(probe)
	feedbacks, _ := body["feedbacks"].(map[string]any)
	item, _ := feedbacks[strconv.FormatInt(targetID, 10)].(map[string]any)
	out["count"] = intFromAny(body["count"])
	out["feedbacks_count"] = len(feedbacks)
	out["actual_value"] = item["feedback_value"]
	out["ok"] = body["status"] == "ok" && item["feedback_value"] == expectedValue
	return out
}

func checkAuditTotal(probe map[string]any, minTotal int) map[string]any {
	out := map[string]any{"ok": false, "min_total": minTotal}
	if probe == nil || probe["status"] != "ok" {
		out["error"] = "probe failed"
		return out
	}
	body := responseJSONFromProbe(probe)
	total := intFromAny(body["total"])
	items, _ := body["items"].([]any)
	out["total"] = total
	out["item_count"] = len(items)
	out["ok"] = body["status"] == "ok" && total >= minTotal && len(items) >= minTotal
	return out
}

func checkShadowGuard(probe map[string]any) map[string]any {
	out := map[string]any{"ok": false}
	if probe == nil {
		out["error"] = "probe missing"
		return out
	}
	body := responseJSONFromProbe(probe)
	out["http_status"] = intFromAny(probe["http_status"])
	out["code"] = body["code"]
	out["status"] = body["status"]
	out["ok"] = intFromAny(probe["http_status"]) == http.StatusServiceUnavailable && body["code"] == "shadow_guard"
	return out
}

func checkSessionsCompare(probe map[string]any, expected map[string]map[string]int) map[string]any {
	out := map[string]any{"ok": false, "expected": expected}
	if probe == nil || probe["status"] != "ok" {
		out["error"] = "probe failed"
		return out
	}
	body := responseJSONFromProbe(probe)
	sessions, _ := body["sessions"].(map[string]any)
	actual := map[string]map[string]int{}
	ok := body["status"] == "ok"
	for sid, expectedCounts := range expected {
		payload, _ := sessions[sid].(map[string]any)
		counts, _ := payload["counts"].(map[string]any)
		actualCounts := map[string]int{}
		for key, want := range expectedCounts {
			got := intFromAny(counts[key])
			actualCounts[key] = got
			if got != want {
				ok = false
			}
		}
		logs, _ := payload["logs_preview"].([]any)
		memories, _ := payload["memories_preview"].([]any)
		kgTriples, _ := payload["kg_triples"].([]any)
		actualCounts["logs_preview"] = len(logs)
		actualCounts["memories_preview"] = len(memories)
		actualCounts["kg_preview"] = len(kgTriples)
		if len(logs) == 0 || len(memories) == 0 || len(kgTriples) == 0 {
			ok = false
		}
		actual[sid] = actualCounts
	}
	out["actual"] = actual
	out["ok"] = ok
	return out
}

func runSessionIsolationSmokeStandalone(ctx context.Context, port int, sessionPrefix string) (map[string]any, error) {
	baseURL := fmt.Sprintf("http://127.0.0.1:%d", port)
	sessionA := safeSessionID(sessionPrefix) + "-rmg03-standalone-a"
	sessionB := safeSessionID(sessionPrefix) + "-rmg03-standalone-b"
	routes := []map[string]any{}

	criticStub := startRouteSmokeCriticStub()
	defer criticStub.Close()

	// Session A: two distinct turns
	for _, turn := range []int{1, 2} {
		body := routeSmokeCompleteTurnBody(sessionA, turn, fmt.Sprintf("standalone-a-turn-%d", turn), criticStub.URL())
		result, err := postJSON(ctx, baseURL+"/complete-turn", body)
		routes = append(routes, result)
		if err != nil {
			return map[string]any{
				"status":   "failed",
				"sessions": []string{sessionA, sessionB},
				"routes":   routes,
			}, err
		}
	}

	// Session B: one turn
	bodyB := routeSmokeCompleteTurnBody(sessionB, 1, "standalone-b-turn-1", criticStub.URL())
	resultB, err := postJSON(ctx, baseURL+"/complete-turn", bodyB)
	routes = append(routes, resultB)
	if err != nil {
		return map[string]any{
			"status":   "failed",
			"sessions": []string{sessionA, sessionB},
			"routes":   routes,
		}, err
	}

	sessionsResult, sessionsErr := probeGET(ctx, baseURL+"/sessions")
	timelineA, timelineAErr := probeGET(ctx, baseURL+"/timeline?sessionId="+url.QueryEscape(sessionA)+"&limit=40")
	timelineB, timelineBErr := probeGET(ctx, baseURL+"/timeline?sessionId="+url.QueryEscape(sessionB)+"&limit=40")
	searchA, searchAErr := postJSON(ctx, baseURL+"/search", map[string]any{"chat_session_id": sessionA, "user_input": "route smoke search A", "top_k": 10})
	searchB, searchBErr := postJSON(ctx, baseURL+"/search", map[string]any{"chat_session_id": sessionB, "user_input": "route smoke search B", "top_k": 10})
	explorerChatA, explorerChatAErr := probeGET(ctx, baseURL+"/explorer/chat_logs?chat_session_id="+url.QueryEscape(sessionA)+"&limit=40")
	explorerChatB, explorerChatBErr := probeGET(ctx, baseURL+"/explorer/chat_logs?chat_session_id="+url.QueryEscape(sessionB)+"&limit=40")
	explorerMemA, explorerMemAErr := probeGET(ctx, baseURL+"/explorer/memories?chat_session_id="+url.QueryEscape(sessionA)+"&limit=40")
	explorerMemB, explorerMemBErr := probeGET(ctx, baseURL+"/explorer/memories?chat_session_id="+url.QueryEscape(sessionB)+"&limit=40")
	explorerKGA, explorerKGAErr := probeGET(ctx, baseURL+"/explorer/kg_triples?chat_session_id="+url.QueryEscape(sessionA)+"&limit=40")
	explorerKGB, explorerKGBErr := probeGET(ctx, baseURL+"/explorer/kg_triples?chat_session_id="+url.QueryEscape(sessionB)+"&limit=40")
	statsBeforeDelete, statsBeforeDeleteErr := probeGET(ctx, baseURL+"/stats")

	// Each /complete-turn writes 2 chat logs (user + assistant) plus critic-derived rows.
	// With the managed stub, sessionA (2 turns) -> 4 chat logs, sessionB (1 turn) -> 2 chat logs.
	sessionChecks := checkSessionListIsolation(sessionsResult, map[string]int{sessionA: 4, sessionB: 2})
	timelineACheck := checkTimelineIsolation(timelineA, sessionA, 4)
	timelineBCheck := checkTimelineIsolation(timelineB, sessionB, 2)
	searchACheck := checkSessionScopedItems(searchA, sessionA, 2, "memory_count")
	searchBCheck := checkSessionScopedItems(searchB, sessionB, 1, "memory_count")
	explorerChatACheck := checkSessionScopedItems(explorerChatA, sessionA, 4, "total")
	explorerChatBCheck := checkSessionScopedItems(explorerChatB, sessionB, 2, "total")
	explorerMemACheck := checkSessionScopedItems(explorerMemA, sessionA, 2, "total")
	explorerMemBCheck := checkSessionScopedItems(explorerMemB, sessionB, 1, "total")
	explorerKGACheck := checkSessionScopedItems(explorerKGA, sessionA, 2, "total")
	explorerKGBCheck := checkSessionScopedItems(explorerKGB, sessionB, 1, "total")
	statsCheck := checkStatsCounts(statsBeforeDelete, map[string]int{"chat_logs": 6, "memories": 3, "kg_triples": 3})
	memoryAID := firstItemID(explorerMemA)
	feedbackPost := map[string]any{"status": "skipped", "reason": "no memory id found for session A"}
	var feedbackPostErr error
	feedbackLatest := map[string]any{"status": "skipped", "reason": "no memory id found for session A"}
	var feedbackLatestErr error
	auditFeedback := map[string]any{"status": "skipped", "reason": "feedback post skipped"}
	var auditFeedbackErr error
	protectedMutation := map[string]any{"status": "skipped", "reason": "no memory id found for session A"}
	var protectedMutationErr error
	if memoryAID > 0 {
		feedbackPost, feedbackPostErr = postJSON(ctx, baseURL+"/feedback", map[string]any{
			"chat_session_id": sessionA,
			"target_type":     "memory",
			"target_id":       memoryAID,
			"feedback_value":  "up",
			"feedback_note":   "rmg23 managed smoke",
		})
		feedbackLatest, feedbackLatestErr = probeGET(ctx, baseURL+"/feedback/latest?chat_session_id="+url.QueryEscape(sessionA)+"&target_type=memory&target_ids="+strconv.FormatInt(memoryAID, 10))
		auditFeedback, auditFeedbackErr = probeGET(ctx, baseURL+"/audit?chat_session_id="+url.QueryEscape(sessionA)+"&event_type=critic_feedback&limit=10")
		protectedMutation, protectedMutationErr = patchJSONProbe(ctx, baseURL+"/explorer/memories/"+strconv.FormatInt(memoryAID, 10), map[string]any{
			"chat_session_id": sessionB,
			"importance":      0.1,
		})
	}
	feedbackPostCheck := checkFeedbackPost(feedbackPost, memoryAID)
	feedbackLatestCheck := checkFeedbackLatest(feedbackLatest, memoryAID, "up")
	auditFeedbackCheck := checkAuditTotal(auditFeedback, 1)
	protectedMutationCheck := checkShadowGuard(protectedMutation)
	compareAB, compareABErr := probeGET(ctx, baseURL+"/sessions/compare?session_ids="+url.QueryEscape(sessionA+","+sessionB)+"&preview_limit=2")
	compareABCheck := checkSessionsCompare(compareAB, map[string]map[string]int{
		sessionA: map[string]int{"chat_logs": 4, "memories": 2, "kg_triples": 2, "feedback_up": 1},
		sessionB: map[string]int{"chat_logs": 2, "memories": 1, "kg_triples": 1, "feedback_up": 0},
	})
	deleteB, deleteBErr := deleteJSON(ctx, baseURL+"/sessions/"+url.PathEscape(sessionB))
	sessionsAfterDelete, sessionsAfterDeleteErr := probeGET(ctx, baseURL+"/sessions")
	timelineBAfterDelete, timelineBAfterDeleteErr := probeGET(ctx, baseURL+"/timeline?sessionId="+url.QueryEscape(sessionB)+"&limit=40")
	deleteBCheck := checkSessionDelete(deleteB, sessionsAfterDelete, timelineBAfterDelete, sessionB)
	rollbackA, rollbackAErr := deleteJSON(ctx, baseURL+"/rollback/2?chat_session_id="+url.QueryEscape(sessionA)+"&req_source=auto_rollback")
	explorerChatAAfterRollback, explorerChatAAfterRollbackErr := probeGET(ctx, baseURL+"/explorer/chat_logs?chat_session_id="+url.QueryEscape(sessionA)+"&limit=40")
	explorerMemAAfterRollback, explorerMemAAfterRollbackErr := probeGET(ctx, baseURL+"/explorer/memories?chat_session_id="+url.QueryEscape(sessionA)+"&limit=40")
	explorerKGAAfterRollback, explorerKGAAfterRollbackErr := probeGET(ctx, baseURL+"/explorer/kg_triples?chat_session_id="+url.QueryEscape(sessionA)+"&limit=40")
	auditRollbackA, auditRollbackAErr := probeGET(ctx, baseURL+"/audit?chat_session_id="+url.QueryEscape(sessionA)+"&event_type=rollback&limit=10")
	timelineAAfterRollback, timelineAAfterRollbackErr := probeGET(ctx, baseURL+"/timeline?sessionId="+url.QueryEscape(sessionA)+"&limit=40")
	rollbackACheck := checkRollbackMutation(rollbackA, explorerChatAAfterRollback, explorerMemAAfterRollback, explorerKGAAfterRollback, auditRollbackA, sessionA, 2)
	timelineAAfterRollbackCheck := checkTimelineIsolation(timelineAAfterRollback, sessionA, 2)
	ok := sessionsErr == nil && timelineAErr == nil && timelineBErr == nil &&
		searchAErr == nil && searchBErr == nil &&
		explorerChatAErr == nil && explorerChatBErr == nil &&
		explorerMemAErr == nil && explorerMemBErr == nil &&
		explorerKGAErr == nil && explorerKGBErr == nil &&
		statsBeforeDeleteErr == nil &&
		feedbackPostErr == nil && feedbackLatestErr == nil && auditFeedbackErr == nil && protectedMutationErr == nil && compareABErr == nil &&
		deleteBErr == nil && sessionsAfterDeleteErr == nil && timelineBAfterDeleteErr == nil &&
		rollbackAErr == nil &&
		explorerChatAAfterRollbackErr == nil && explorerMemAAfterRollbackErr == nil && explorerKGAAfterRollbackErr == nil &&
		auditRollbackAErr == nil && timelineAAfterRollbackErr == nil &&
		boolField(sessionChecks, "ok") &&
		boolField(timelineACheck, "ok") &&
		boolField(timelineBCheck, "ok") &&
		boolField(searchACheck, "ok") &&
		boolField(searchBCheck, "ok") &&
		boolField(explorerChatACheck, "ok") &&
		boolField(explorerChatBCheck, "ok") &&
		boolField(explorerMemACheck, "ok") &&
		boolField(explorerMemBCheck, "ok") &&
		boolField(explorerKGACheck, "ok") &&
		boolField(explorerKGBCheck, "ok") &&
		boolField(statsCheck, "ok") &&
		boolField(feedbackPostCheck, "ok") &&
		boolField(feedbackLatestCheck, "ok") &&
		boolField(auditFeedbackCheck, "ok") &&
		boolField(protectedMutationCheck, "ok") &&
		boolField(compareABCheck, "ok") &&
		boolField(deleteBCheck, "ok") &&
		boolField(rollbackACheck, "ok") &&
		boolField(timelineAAfterRollbackCheck, "ok")

	status := "ok"
	if !ok {
		status = "failed"
	}
	report := map[string]any{
		"status":         status,
		"base_url":       baseURL,
		"sessions":       []string{sessionA, sessionB},
		"routes":         routes,
		"sessions_probe": sessionsResult,
		"timeline_a":     timelineA,
		"timeline_b":     timelineB,
		"search_a":       searchA,
		"search_b":       searchB,
		"explorer": map[string]any{
			"chat_logs_a":  explorerChatA,
			"chat_logs_b":  explorerChatB,
			"memories_a":   explorerMemA,
			"memories_b":   explorerMemB,
			"kg_triples_a": explorerKGA,
			"kg_triples_b": explorerKGB,
		},
		"rmg23": map[string]any{
			"status":              status,
			"memory_a_id":         memoryAID,
			"stats":               statsBeforeDelete,
			"feedback_post":       feedbackPost,
			"feedback_latest":     feedbackLatest,
			"audit_feedback":      auditFeedback,
			"sessions_compare":    compareAB,
			"protected_mutation":  protectedMutation,
			"scope":               "SEQ-02 canonical read/control proof for stats, audit, feedback, compare, and guarded DB editing",
			"product_green":       false,
			"remaining_rmg23_gap": "manual DB editing and edit-history audit are still guarded/not product-green",
		},
		"delete_session_b": map[string]any{
			"delete_probe":          deleteB,
			"sessions_after_delete": sessionsAfterDelete,
			"timeline_after_delete": timelineBAfterDelete,
		},
		"rmg04": map[string]any{
			"status":                    status,
			"rollback_probe":            rollbackA,
			"chat_logs_after_rollback":  explorerChatAAfterRollback,
			"memories_after_rollback":   explorerMemAAfterRollback,
			"kg_triples_after_rollback": explorerKGAAfterRollback,
			"audit_after_rollback":      auditRollbackA,
			"timeline_after_rollback":   timelineAAfterRollback,
			"scope":                     "SEQ-03 actual rollback mutation proof on a writable MariaDB authority store",
			"product_green":             false,
			"remaining_rmg04_gap":       "JS live RisuAI session-delete UI update, repair-replay rebuild, and later guidance/session-active-scope/maintenance cleanup remain open",
		},
		"checks": map[string]any{
			"sessions_list":             sessionChecks,
			"timeline_a":                timelineACheck,
			"timeline_b":                timelineBCheck,
			"search_a":                  searchACheck,
			"search_b":                  searchBCheck,
			"explorer_chat_a":           explorerChatACheck,
			"explorer_chat_b":           explorerChatBCheck,
			"explorer_mem_a":            explorerMemACheck,
			"explorer_mem_b":            explorerMemBCheck,
			"explorer_kg_a":             explorerKGACheck,
			"explorer_kg_b":             explorerKGBCheck,
			"stats":                     statsCheck,
			"feedback_post":             feedbackPostCheck,
			"feedback_latest":           feedbackLatestCheck,
			"audit_feedback":            auditFeedbackCheck,
			"protected_edit":            protectedMutationCheck,
			"sessions_compare":          compareABCheck,
			"delete_session_b":          deleteBCheck,
			"rollback_session_a":        rollbackACheck,
			"timeline_a_after_rollback": timelineAAfterRollbackCheck,
		},
		"expected_chat_log_counts": map[string]int{
			sessionA: 4,
			sessionB: 2,
		},
	}
	if !ok {
		errs := []string{}
		for _, err := range []error{
			sessionsErr, timelineAErr, timelineBErr,
			searchAErr, searchBErr,
			explorerChatAErr, explorerChatBErr,
			explorerMemAErr, explorerMemBErr,
			explorerKGAErr, explorerKGBErr,
			statsBeforeDeleteErr,
			feedbackPostErr, feedbackLatestErr, auditFeedbackErr, protectedMutationErr, compareABErr,
			deleteBErr, sessionsAfterDeleteErr, timelineBAfterDeleteErr,
			rollbackAErr, explorerChatAAfterRollbackErr, explorerMemAAfterRollbackErr, explorerKGAAfterRollbackErr,
			auditRollbackAErr, timelineAAfterRollbackErr,
		} {
			if err != nil {
				errs = append(errs, err.Error())
			}
		}
		if len(errs) == 0 {
			errs = append(errs, "session isolation checks failed")
		}
		report["errors"] = errs
		return report, errors.New(strings.Join(errs, "; "))
	}
	return report, nil
}
func boolField(m map[string]any, key string) bool {
	v, _ := m[key].(bool)
	return v
}

func intFromAny(v any) int {
	switch t := v.(type) {
	case int:
		return t
	case int64:
		return int(t)
	case float64:
		return int(t)
	case json.Number:
		i, _ := t.Int64()
		return int(i)
	default:
		i, _ := strconv.Atoi(strings.TrimSpace(fmt.Sprint(v)))
		return i
	}
}

func mapFromAny(v any) map[string]any {
	if m, ok := v.(map[string]any); ok {
		return m
	}
	return map[string]any{}
}

type routeSmokeCriticStub struct {
	server *httptest.Server
	calls  atomic.Int64
}

func routeSmokeLiveConfigFromEnv() routeSmokeLiveConfig {
	enabled := routeSmokeEnvBool("AC_RMG02_ROUTE_SMOKE_LIVE_PROVIDER")
	waitMs := routeSmokeEnvInt64("AC_RMG02_ROUTE_SMOKE_HTTP_TIMEOUT_MS", 180000)
	if waitMs <= 0 {
		waitMs = 180000
	}
	return routeSmokeLiveConfig{
		Enabled:  enabled,
		HTTPWait: time.Duration(waitMs) * time.Millisecond,
		Critic: routeSmokeLLMConfig{
			Provider:            routeSmokeEnv("AC_RMG02_CRITIC_PROVIDER", "openai"),
			Endpoint:            routeSmokeEnv("AC_RMG02_CRITIC_ENDPOINT", "http://127.0.0.1:11434/v1"),
			APIKey:              routeSmokeEnv("AC_RMG02_CRITIC_API_KEY", "ollama-local"),
			Model:               routeSmokeEnv("AC_RMG02_CRITIC_MODEL", "glm-5.1:cloud"),
			TimeoutMs:           routeSmokeEnvInt64("AC_RMG02_CRITIC_TIMEOUT_MS", 120000),
			Temperature:         routeSmokeEnvFloat("AC_RMG02_CRITIC_TEMPERATURE", 0),
			MaxTokens:           routeSmokeEnvInt64("AC_RMG02_CRITIC_MAX_TOKENS", 1800),
			MaxCompletionTokens: routeSmokeEnvInt64("AC_RMG02_CRITIC_MAX_COMPLETION_TOKENS", 1800),
			ReasoningEffort:     routeSmokeEnv("AC_RMG02_CRITIC_REASONING_EFFORT", ""),
		},
		Embedding: routeSmokeLLMConfig{
			Provider:  routeSmokeEnv("AC_RMG02_EMBEDDING_PROVIDER", "ollama"),
			Endpoint:  routeSmokeEnv("AC_RMG02_EMBEDDING_ENDPOINT", "http://127.0.0.1:11434"),
			APIKey:    routeSmokeEnv("AC_RMG02_EMBEDDING_API_KEY", "ollama-local"),
			Model:     routeSmokeEnv("AC_RMG02_EMBEDDING_MODEL", "nomic-embed-text"),
			TimeoutMs: routeSmokeEnvInt64("AC_RMG02_EMBEDDING_TIMEOUT_MS", 60000),
		},
	}
}

func routeSmokeEnv(name string, fallback string) string {
	if value := strings.TrimSpace(os.Getenv(name)); value != "" {
		return value
	}
	return fallback
}

func routeSmokeEnvBool(name string) bool {
	switch strings.ToLower(strings.TrimSpace(os.Getenv(name))) {
	case "1", "true", "yes", "y", "on", "live":
		return true
	default:
		return false
	}
}

func routeSmokeEnvInt64(name string, fallback int64) int64 {
	raw := strings.TrimSpace(os.Getenv(name))
	if raw == "" {
		return fallback
	}
	value, err := strconv.ParseInt(raw, 10, 64)
	if err != nil {
		return fallback
	}
	return value
}

func routeSmokeEnvFloat(name string, fallback float64) float64 {
	raw := strings.TrimSpace(os.Getenv(name))
	if raw == "" {
		return fallback
	}
	value, err := strconv.ParseFloat(raw, 64)
	if err != nil {
		return fallback
	}
	return value
}

func routeSmokeStubClientMeta(criticEndpoint string) map[string]any {
	return map[string]any{
		"critic": map[string]any{
			"provider":    "openai",
			"endpoint":    criticEndpoint,
			"api_key":     "managed-route-smoke-key",
			"model":       "route-smoke-critic",
			"timeout_ms":  10000,
			"temperature": 0,
			"max_tokens":  800,
		},
	}
}

func routeSmokeLiveClientMeta(cfg routeSmokeLiveConfig) map[string]any {
	critic := map[string]any{
		"provider":              cfg.Critic.Provider,
		"endpoint":              cfg.Critic.Endpoint,
		"api_key":               cfg.Critic.APIKey,
		"model":                 cfg.Critic.Model,
		"timeout_ms":            cfg.Critic.TimeoutMs,
		"temperature":           cfg.Critic.Temperature,
		"max_tokens":            cfg.Critic.MaxTokens,
		"max_completion_tokens": cfg.Critic.MaxCompletionTokens,
	}
	if strings.TrimSpace(cfg.Critic.ReasoningEffort) != "" {
		critic["reasoning_effort"] = cfg.Critic.ReasoningEffort
	}
	return map[string]any{
		"critic": critic,
		"embedding": map[string]any{
			"provider":   cfg.Embedding.Provider,
			"endpoint":   cfg.Embedding.Endpoint,
			"api_key":    cfg.Embedding.APIKey,
			"model":      cfg.Embedding.Model,
			"timeout_ms": cfg.Embedding.TimeoutMs,
		},
	}
}

func routeSmokeProviderMode(cfg routeSmokeLiveConfig) string {
	if cfg.Enabled {
		return "configured_live_provider"
	}
	return "managed_stub_openai_compatible"
}

func routeSmokeLLMReport(cfg routeSmokeLLMConfig, enabled bool) map[string]any {
	if !enabled {
		return map[string]any{"configured": false}
	}
	return map[string]any{
		"configured":    true,
		"provider":      cfg.Provider,
		"endpoint_host": routeSmokeEndpointHost(cfg.Endpoint),
		"model":         cfg.Model,
		"timeout_ms":    cfg.TimeoutMs,
	}
}

func routeSmokeEndpointHost(endpoint string) string {
	parsed, err := url.Parse(strings.TrimSpace(endpoint))
	if err == nil && parsed.Host != "" {
		return parsed.Host
	}
	return strings.TrimSpace(endpoint)
}

func routeSmokeCriticStubCalls(stub *routeSmokeCriticStub) int64 {
	if stub == nil {
		return 0
	}
	return stub.Calls()
}

func startRouteSmokeCriticStub() *routeSmokeCriticStub {
	stub := &routeSmokeCriticStub{}
	stub.server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		stub.calls.Add(1)
		raw, _ := io.ReadAll(io.LimitReader(r.Body, 1<<20))
		evidenceExcerpt := "route smoke first assistant content"
		if strings.Contains(string(raw), "route smoke second assistant content") {
			evidenceExcerpt = "route smoke second assistant content"
		}
		extraction := map[string]any{
			"turn_summary":           "Route smoke critic saved a durable memory.",
			"importance_score":       7,
			"emotional_intensity":    0.42,
			"narrative_significance": 0.66,
			"relationship_memory": map[string]any{
				"bond_and_distance": "Nova trusts Orion after the route smoke check.",
				"target_name":       "Orion",
				"trust":             0.74,
			},
			"entities": map[string]any{
				"characters": []any{map[string]any{
					"name":           "Nova",
					"role":           "character",
					"status_emotion": "focused",
					"confidence":     0.91,
				}},
			},
			"kg_triples": []any{map[string]any{
				"subject":    "Nova",
				"predicate":  "trusts",
				"object":     "Orion",
				"valid_from": 1,
			}},
			"evidence_excerpts": []any{evidenceExcerpt},
			"character_deltas": []any{map[string]any{
				"name":          "Nova",
				"status":        map[string]any{"mood": "focused"},
				"relationships": map[string]any{"Orion": "trusted"},
				"events": []any{map[string]any{
					"type":   "route_smoke",
					"detail": "Nova completed a managed route smoke check.",
				}},
			}},
			"world_rules": []any{map[string]any{
				"scope":     "session",
				"category":  "migration_smoke",
				"key":       "route_smoke_rule",
				"value":     "Managed route smoke writes must be visible in MariaDB.",
				"source":    "managed_mariadb_e2e",
				"source_id": "route-write-smoke",
			}},
			"pending_threads": []any{map[string]any{
				"title":       "Route smoke continuity check",
				"details":     "Follow up if managed route smoke writes are missing.",
				"thread_type": "migration_smoke",
				"priority":    2,
				"confidence":  0.85,
			}},
			"state_deltas": map[string]any{
				"scene_pressure": "steady",
			},
		}
		extractionBytes, _ := json.Marshal(extraction)
		resp := map[string]any{
			"id":      "route-smoke-critic",
			"object":  "chat.completion",
			"model":   "route-smoke-critic",
			"choices": []any{map[string]any{"message": map[string]any{"role": "assistant", "content": string(extractionBytes)}}},
			"usage":   map[string]any{"prompt_tokens": 1, "completion_tokens": 1, "total_tokens": 2},
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(resp)
	}))
	return stub
}

func (s *routeSmokeCriticStub) URL() string {
	if s == nil || s.server == nil {
		return ""
	}
	return s.server.URL
}

func (s *routeSmokeCriticStub) Close() {
	if s != nil && s.server != nil {
		s.server.Close()
	}
}

func (s *routeSmokeCriticStub) Calls() int64 {
	if s == nil {
		return 0
	}
	return s.calls.Load()
}

func routeSmokeCompleteTurnBody(sessionID string, requestedTurn int, label string, criticEndpoint string) map[string]any {
	return routeSmokeCompleteTurnBodyWithClientMeta(sessionID, requestedTurn, label, routeSmokeStubClientMeta(criticEndpoint), false)
}

func routeSmokeCompleteTurnBodyWithClientMeta(sessionID string, requestedTurn int, label string, clientMeta map[string]any, liveProvider bool) map[string]any {
	userInput := fmt.Sprintf("route smoke %s user input", label)
	assistantContent := fmt.Sprintf("route smoke %s assistant content", label)
	if liveProvider {
		userInput = fmt.Sprintf("route smoke %s user input: Nova tells Orion that she trusts him with the lighthouse key and asks him to remember the Archive Hall promise.", label)
		assistantContent = fmt.Sprintf("route smoke %s assistant content: Nova gives Orion the silver compass in the Archive Hall. Nova says exactly, \"I trust you with the lighthouse key.\" Orion accepts responsibility. The world rule is that the lighthouse key opens the north archive only during moonrise. The unresolved storyline is to repair the clock bridge before dawn. Nova feels focused and relieved.", label)
	}
	return map[string]any{
		"chat_session_id":   sessionID,
		"turn_index":        requestedTurn,
		"user_input":        userInput,
		"assistant_content": assistantContent,
		"request_type":      "model",
		"context_messages": []map[string]any{
			{"role": "critic", "content": "route smoke", "score": 1},
		},
		"improvement_trace": map[string]any{"score": 1, "source": "managed_mariadb_e2e", "label": label},
		"client_meta":       clientMeta,
	}
}

func routeSmokeDelta(before map[string]int, after map[string]int) map[string]int {
	delta := map[string]int{}
	for key, afterValue := range after {
		delta[key] = afterValue - before[key]
	}
	return delta
}

func postJSON(ctx context.Context, url string, payload map[string]any) (map[string]any, error) {
	return postJSONWithTimeout(ctx, url, payload, 10*time.Second)
}

func postJSONWithTimeout(ctx context.Context, url string, payload map[string]any, timeout time.Duration) (map[string]any, error) {
	if timeout <= 0 {
		timeout = 10 * time.Second
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return map[string]any{"url": url, "status": "failed", "error": err.Error()}, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return map[string]any{"url": url, "status": "failed", "error": err.Error()}, err
	}
	req.Header.Set("Content-Type", "application/json")
	client := &http.Client{Timeout: timeout}
	resp, err := client.Do(req)
	if err != nil {
		return map[string]any{"url": url, "status": "failed", "error": err.Error()}, err
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	decoded := map[string]any{}
	if len(raw) > 0 {
		_ = json.Unmarshal(raw, &decoded)
	}
	status := "ok"
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		status = "failed"
	}
	out := map[string]any{
		"url":         url,
		"method":      http.MethodPost,
		"http_status": resp.StatusCode,
		"status":      status,
		"response":    decoded,
	}
	if status != "ok" {
		return out, fmt.Errorf("POST %s returned HTTP %d", url, resp.StatusCode)
	}
	return out, nil
}

func deleteJSON(ctx context.Context, url string) (map[string]any, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodDelete, url, nil)
	if err != nil {
		return map[string]any{"url": url, "status": "failed", "error": err.Error()}, err
	}
	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return map[string]any{"url": url, "status": "failed", "error": err.Error()}, err
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	decoded := map[string]any{}
	if len(raw) > 0 {
		_ = json.Unmarshal(raw, &decoded)
	}
	status := "ok"
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		status = "failed"
	}
	out := map[string]any{
		"url":         url,
		"method":      http.MethodDelete,
		"http_status": resp.StatusCode,
		"status":      status,
		"response":    decoded,
	}
	if status != "ok" {
		return out, fmt.Errorf("DELETE %s returned HTTP %d", url, resp.StatusCode)
	}
	return out, nil
}

func patchJSONProbe(ctx context.Context, url string, payload map[string]any) (map[string]any, error) {
	body, err := json.Marshal(payload)
	if err != nil {
		return map[string]any{"url": url, "status": "failed", "error": err.Error()}, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPatch, url, bytes.NewReader(body))
	if err != nil {
		return map[string]any{"url": url, "status": "failed", "error": err.Error()}, err
	}
	req.Header.Set("Content-Type", "application/json")
	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return map[string]any{"url": url, "status": "failed", "error": err.Error()}, err
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	decoded := map[string]any{}
	if len(raw) > 0 {
		_ = json.Unmarshal(raw, &decoded)
	}
	status := "ok"
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		status = "failed"
	}
	return map[string]any{
		"url":         url,
		"method":      http.MethodPatch,
		"http_status": resp.StatusCode,
		"status":      status,
		"response":    decoded,
	}, nil
}

func routeSmokeCounts(ctx context.Context, dsn string, sessionID string) (map[string]int, error) {
	db, err := sql.Open("mysql", dsn)
	if err != nil {
		return nil, err
	}
	defer db.Close()
	out := map[string]int{}
	for _, table := range []string{
		"chat_logs",
		"effective_input_logs",
		"memories",
		"direct_evidence_records",
		"kg_triples",
		"audit_logs",
		"critic_feedback",
		"character_events",
		"entities",
		"trust_states",
		"storylines",
		"world_rules",
		"character_states",
		"pending_threads",
		"active_states",
	} {
		var count int
		query := fmt.Sprintf("SELECT COUNT(*) FROM %s WHERE chat_session_id = ?", table)
		if err := db.QueryRowContext(ctx, query, sessionID).Scan(&count); err != nil {
			return nil, fmt.Errorf("%s count: %w", table, err)
		}
		out[table] = count
	}
	return out, nil
}

func routeSmokeContentChecks(ctx context.Context, dsn string, sessionID string, liveCfg routeSmokeLiveConfig) (map[string]any, error) {
	if liveCfg.Enabled {
		return routeSmokeLiveContentChecks(ctx, dsn, sessionID, liveCfg)
	}
	db, err := sql.Open("mysql", dsn)
	if err != nil {
		return nil, err
	}
	defer db.Close()
	checks := map[string]bool{}
	specs := []struct {
		name  string
		query string
		args  []any
	}{
		{
			name:  "complete_turn_first_user_log",
			query: "SELECT COUNT(*) FROM chat_logs WHERE chat_session_id = ? AND role = 'user' AND content = ?",
			args:  []any{sessionID, "route smoke first user input"},
		},
		{
			name:  "complete_turn_first_assistant_log",
			query: "SELECT COUNT(*) FROM chat_logs WHERE chat_session_id = ? AND role = 'assistant' AND content = ?",
			args:  []any{sessionID, "route smoke first assistant content"},
		},
		{
			name:  "complete_turn_second_user_log",
			query: "SELECT COUNT(*) FROM chat_logs WHERE chat_session_id = ? AND role = 'user' AND content = ?",
			args:  []any{sessionID, "route smoke second user input"},
		},
		{
			name:  "complete_turn_second_assistant_log",
			query: "SELECT COUNT(*) FROM chat_logs WHERE chat_session_id = ? AND role = 'assistant' AND content = ?",
			args:  []any{sessionID, "route smoke second assistant content"},
		},
		{
			name:  "complete_turn_effective_input",
			query: "SELECT COUNT(*) FROM effective_input_logs WHERE chat_session_id = ? AND effective_input LIKE ?",
			args:  []any{sessionID, "%route smoke first user input%"},
		},
		{
			name:  "critic_memory_summary",
			query: "SELECT COUNT(*) FROM memories WHERE chat_session_id = ? AND CAST(summary_json AS CHAR) LIKE ?",
			args:  []any{sessionID, "%Route smoke critic saved a durable memory%"},
		},
		{
			name:  "critic_direct_evidence",
			query: "SELECT COUNT(*) FROM direct_evidence_records WHERE chat_session_id = ? AND evidence_text IN (?, ?)",
			args:  []any{sessionID, "route smoke first assistant content", "route smoke second assistant content"},
		},
		{
			name:  "critic_kg_triple",
			query: "SELECT COUNT(*) FROM kg_triples WHERE chat_session_id = ? AND subject = ? AND predicate = ? AND object = ?",
			args:  []any{sessionID, "Nova", "trusts", "Orion"},
		},
		{
			name:  "critic_entity",
			query: "SELECT COUNT(*) FROM entities WHERE chat_session_id = ? AND name = ?",
			args:  []any{sessionID, "Nova"},
		},
		{
			name:  "critic_trust_state",
			query: "SELECT COUNT(*) FROM trust_states WHERE chat_session_id = ? AND target_name = ?",
			args:  []any{sessionID, "Orion"},
		},
		{
			name:  "critic_world_rule",
			query: "SELECT COUNT(*) FROM world_rules WHERE chat_session_id = ? AND `key` = ?",
			args:  []any{sessionID, "route_smoke_rule"},
		},
		{
			name:  "critic_storyline",
			query: "SELECT COUNT(*) FROM storylines WHERE chat_session_id = ? AND name = ?",
			args:  []any{sessionID, "Route smoke continuity check"},
		},
		{
			name:  "critic_character_state",
			query: "SELECT COUNT(*) FROM character_states WHERE chat_session_id = ? AND character_name = ?",
			args:  []any{sessionID, "Nova"},
		},
		{
			name:  "critic_character_event",
			query: "SELECT COUNT(*) FROM character_events WHERE chat_session_id = ? AND character_name = ? AND event_type = ?",
			args:  []any{sessionID, "Nova", "route_smoke"},
		},
		{
			name:  "critic_pending_thread",
			query: "SELECT COUNT(*) FROM pending_threads WHERE chat_session_id = ? AND description = ?",
			args:  []any{sessionID, "Follow up if managed route smoke writes are missing."},
		},
		{
			name:  "critic_active_state_entities",
			query: "SELECT COUNT(*) FROM active_states WHERE chat_session_id = ? AND state_type = ?",
			args:  []any{sessionID, "entities"},
		},
	}
	all := true
	for _, spec := range specs {
		ok, err := routeSmokeQueryExists(ctx, db, spec.query, spec.args...)
		if err != nil {
			return nil, fmt.Errorf("%s: %w", spec.name, err)
		}
		checks[spec.name] = ok
		if !ok {
			all = false
		}
	}
	return map[string]any{
		"checks":                     checks,
		"all_expected_content_found": all,
		"scope":                      "complete_turn_route_rows_and_critic_artifacts",
	}, nil
}

func routeSmokeLiveContentChecks(ctx context.Context, dsn string, sessionID string, liveCfg routeSmokeLiveConfig) (map[string]any, error) {
	db, err := sql.Open("mysql", dsn)
	if err != nil {
		return nil, err
	}
	defer db.Close()
	countChecks := map[string]int{
		"chat_logs":               4,
		"effective_input_logs":    2,
		"memories":                2,
		"direct_evidence_records": 2,
		"kg_triples":              2,
		"entities":                2,
		"trust_states":            2,
		"world_rules":             2,
		"storylines":              2,
		"character_states":        2,
		"character_events":        2,
		"pending_threads":         2,
		"active_states":           2,
	}
	checks := map[string]bool{}
	counts := map[string]int{}
	all := true
	for table, minCount := range countChecks {
		query := fmt.Sprintf("SELECT COUNT(*) FROM %s WHERE chat_session_id = ?", table)
		count, err := routeSmokeQueryCount(ctx, db, query, sessionID)
		if err != nil {
			return nil, fmt.Errorf("%s count: %w", table, err)
		}
		counts[table] = count
		ok := count >= minCount
		checks[table+"_min_count"] = ok
		if !ok {
			all = false
		}
	}
	embeddingModel := strings.TrimSpace(liveCfg.Embedding.Model)
	if embeddingModel != "" {
		count, err := routeSmokeQueryCount(ctx, db, "SELECT COUNT(*) FROM memories WHERE chat_session_id = ? AND embedding_model = ? AND TRIM(embedding) <> '' AND TRIM(embedding) <> '[]'", sessionID, embeddingModel)
		if err != nil {
			return nil, fmt.Errorf("memory embedding model count: %w", err)
		}
		counts["memories_with_live_embedding_model"] = count
		ok := count >= 2
		checks["memories_with_live_embedding_model"] = ok
		if !ok {
			all = false
		}
	}
	placeholderKG, err := routeSmokeQueryCount(ctx, db, "SELECT COUNT(*) FROM kg_triples WHERE chat_session_id = ? AND (subject LIKE 'char\\_%' OR subject LIKE 'cid\\_%' OR object LIKE 'turn\\_%' OR object LIKE 'char\\_%' OR predicate = 'has_turn')", sessionID)
	if err != nil {
		return nil, fmt.Errorf("placeholder kg count: %w", err)
	}
	counts["placeholder_kg_triples"] = placeholderKG
	checks["no_placeholder_kg_triples"] = placeholderKG == 0
	if placeholderKG != 0 {
		all = false
	}
	rawEvidence, err := routeSmokeQueryCount(ctx, db, "SELECT COUNT(*) FROM direct_evidence_records WHERE chat_session_id = ? AND (CHAR_LENGTH(evidence_text) > 320 OR evidence_text LIKE '%route smoke first user input%route smoke first assistant content%')", sessionID)
	if err != nil {
		return nil, fmt.Errorf("raw direct evidence count: %w", err)
	}
	counts["raw_or_whole_turn_direct_evidence"] = rawEvidence
	checks["no_raw_or_whole_turn_direct_evidence"] = rawEvidence == 0
	if rawEvidence != 0 {
		all = false
	}
	return map[string]any{
		"checks":                     checks,
		"counts":                     counts,
		"all_expected_content_found": all,
		"scope":                      "complete_turn_live_provider_rows_and_quality_guards",
	}, nil
}

func routeSmokeQueryExists(ctx context.Context, db *sql.DB, query string, args ...any) (bool, error) {
	var count int
	if err := db.QueryRowContext(ctx, query, args...).Scan(&count); err != nil {
		return false, err
	}
	return count > 0, nil
}

func routeSmokeQueryCount(ctx context.Context, db *sql.DB, query string, args ...any) (int, error) {
	var count int
	if err := db.QueryRowContext(ctx, query, args...).Scan(&count); err != nil {
		return 0, err
	}
	return count, nil
}

func routeSmokeLiveProviderResponseChecks(routes []map[string]any, liveCfg routeSmokeLiveConfig) map[string]any {
	out := map[string]any{
		"requested": liveCfg.Enabled,
	}
	if !liveCfg.Enabled {
		out["status"] = "not_requested"
		out["all_live_provider_checks_passed"] = true
		return out
	}
	completeRoutes := []map[string]any{}
	for _, route := range routes {
		if strings.HasSuffix(strings.TrimSpace(fmt.Sprint(route["url"])), "/complete-turn") {
			completeRoutes = append(completeRoutes, route)
		}
	}
	checks := map[string]bool{
		"two_complete_turn_responses": len(completeRoutes) >= 2,
	}
	details := []map[string]any{}
	all := checks["two_complete_turn_responses"]
	for index, route := range completeRoutes {
		resp := mapFromAny(route["response"])
		trace := mapFromAny(resp["trace_handoff"])
		llmTrace := mapFromAny(resp["llm_config_trace"])
		criticTrace := mapFromAny(llmTrace["critic"])
		embeddingTrace := mapFromAny(llmTrace["embedding"])
		detail := map[string]any{
			"index":                   index,
			"critic_triggered":        resp["critic_triggered"],
			"derived_artifacts_saved": resp["derived_artifacts_saved"],
			"embedding_status":        trace["embedding_status"],
			"vector_status":           trace["vector_status"],
			"critic_configured":       criticTrace["configured"],
			"embedding_configured":    embeddingTrace["configured"],
			"memories_saved":          resp["memories_saved"],
			"evidence_saved":          resp["evidence_saved"],
			"kg_triples_saved":        resp["kg_triples_saved"],
			"entities_saved":          resp["entities_saved"],
			"trust_states_saved":      resp["trust_states_saved"],
			"world_rules_saved":       resp["world_rules_saved"],
		}
		details = append(details, detail)
		ok := resp["critic_triggered"] == true &&
			intFromAny(resp["derived_artifacts_saved"]) > 0 &&
			fmt.Sprint(trace["embedding_status"]) == "ok" &&
			criticTrace["configured"] == true &&
			embeddingTrace["configured"] == true &&
			intFromAny(resp["memories_saved"]) > 0 &&
			intFromAny(resp["evidence_saved"]) > 0 &&
			intFromAny(resp["kg_triples_saved"]) > 0
		checks[fmt.Sprintf("complete_turn_%d_live_provider_artifacts", index+1)] = ok
		if !ok {
			all = false
		}
	}
	out["status"] = "ok"
	if !all {
		out["status"] = "failed"
	}
	out["all_live_provider_checks_passed"] = all
	out["checks"] = checks
	out["details"] = details
	return out
}

func routeSmokeReport(status string, baseURL string, sessionID string, before map[string]int, after map[string]int, routes []map[string]any, storeMode string) map[string]any {
	if strings.TrimSpace(storeMode) == "" {
		storeMode = "mariadb_shadow"
	}
	return map[string]any{
		"requested":       true,
		"status":          status,
		"base_url":        baseURL,
		"store_mode":      storeMode,
		"chat_session_id": sessionID,
		"before_counts":   before,
		"after_counts":    after,
		"routes":          routes,
	}
}

func runDefaultCandidateProbe(ctx context.Context, port int, actual bool) (map[string]any, error) {
	baseURL := fmt.Sprintf("http://127.0.0.1:%d", port)
	probes, err := probeReadEndpoints(ctx, baseURL, []string{"/health", "/ready", "/version", "/stats"})
	selectedRuntime := "go_rehearsal"
	switchScope := "managed_disposable_rehearsal"
	if actual {
		selectedRuntime = "go"
		switchScope = "managed_disposable_actual"
	}
	report := map[string]any{
		"requested":              true,
		"role":                   "go_default_candidate",
		"base_url":               baseURL,
		"selected_runtime":       selectedRuntime,
		"switch_scope":           switchScope,
		"candidate_store_mode":   "mariadb_read_shadow",
		"candidate_product_read": true,
		"authority_switch":       false,
		"go_default_switch":      actual,
		"persistent_switch":      false,
		"python_runtime_retired": false,
		"probes":                 probes,
	}
	readyChecks := map[string]any{}
	if ready, ok := probes["/ready"]; ok {
		if body, ok := ready["json"].(map[string]any); ok {
			if checks, ok := body["checks"].(map[string]any); ok {
				readyChecks = checks
			}
		}
	}
	report["ready_checks"] = readyChecks
	ok := err == nil &&
		allProbeStatusOK(probes) &&
		readyChecks["store_mode"] == "mariadb_read_shadow" &&
		readyChecks["mariadb_product_read"] == "enabled" &&
		readyChecks["mariadb_authority"] == "disabled"
	status := "ok"
	if !ok {
		status = "failed"
	}
	report["status"] = status
	if err != nil {
		report["error"] = err.Error()
		return report, err
	}
	if !ok {
		return report, fmt.Errorf("go default candidate probe failed")
	}
	return report, nil
}

func runAuthorityCutoverReplay(ctx context.Context, exec commandExecutor, cfg directProviderConfig, goBinPath string, dsn string, pythonBaseURL string, startPort int) (map[string]any, []executedStep, error) {
	base := map[string]any{
		"requested":              true,
		"store_mode":             "mariadb_authority",
		"authority_switch":       true,
		"persistent_switch":      false,
		"go_default_switch":      false,
		"rollback_required":      true,
		"python_runtime_retired": false,
	}
	var steps []executedStep
	port, err := findAvailablePort("127.0.0.1", startPort, startPort+30)
	if err != nil {
		base["status"] = "failed"
		base["error"] = err.Error()
		return base, []executedStep{{Name: "authority-start-go-backend", Status: "failed", Error: err.Error()}}, err
	}

	cmd, startStep, err := startGoBackend(ctx, exec, goBackendStartConfig{
		BinPath:   goBinPath,
		Port:      port,
		DSN:       dsn,
		StoreMode: "mariadb_authority",
	})
	startStep.Name = "authority-start-go-backend"
	steps = append(steps, startStep)
	if err != nil {
		base["status"] = "failed"
		base["error"] = err.Error()
		return base, steps, err
	}
	stopped := false
	defer func() {
		if !stopped {
			_ = exec.Kill(cmd)
		}
	}()

	waitStep := waitGoReady(ctx, port)
	waitStep.Name = "authority-wait-go-ready"
	steps = append(steps, waitStep)
	if waitStep.Status != "ok" {
		base["status"] = "failed"
		base["error"] = waitStep.Error
		return base, steps, fmt.Errorf("authority backend not ready: %s", waitStep.Error)
	}

	checks, err := fetchReadyChecks(ctx, port)
	readyStatus := "ok"
	readyErr := ""
	if err != nil {
		readyStatus = "failed"
		readyErr = err.Error()
	} else if checks["store_mode"] != "mariadb_authority" || checks["mariadb_authority"] != "enabled" || checks["mariadb_product_read"] != "enabled" {
		readyStatus = "failed"
		readyErr = fmt.Sprintf("unexpected authority checks: store_mode=%s mariadb_authority=%s mariadb_product_read=%s", checks["store_mode"], checks["mariadb_authority"], checks["mariadb_product_read"])
	}
	steps = append(steps, executedStep{
		Name:   "authority-ready-check",
		Status: readyStatus,
		Error:  readyErr,
		Note:   fmt.Sprintf("store_mode=%s mariadb_product_read=%s mariadb_authority=%s", checks["store_mode"], checks["mariadb_product_read"], checks["mariadb_authority"]),
	})
	base["ready_checks"] = checks
	if readyStatus != "ok" {
		base["status"] = "failed"
		base["error"] = readyErr
		return base, steps, fmt.Errorf("authority ready check failed: %s", readyErr)
	}

	start := time.Now()
	smokeDetails, err := runRouteWriteSmoke(ctx, port, dsn, cfg.SessionID+"-authority", "mariadb_authority")
	smokeStatus := "ok"
	smokeErr := ""
	if err != nil {
		smokeStatus = "failed"
		smokeErr = err.Error()
	}
	steps = append(steps, executedStep{
		Name:       "authority-route-write-smoke",
		Status:     smokeStatus,
		DurationMs: time.Since(start).Milliseconds(),
		Error:      smokeErr,
		Details:    smokeDetails,
	})
	base["route_write_smoke"] = smokeDetails
	if err != nil {
		base["status"] = "failed"
		base["error"] = err.Error()
		return base, steps, fmt.Errorf("authority route write smoke: %w", err)
	}

	start = time.Now()
	reportArgs := []string{
		"-go-base", fmt.Sprintf("http://127.0.0.1:%d", port),
		"-session-id", cfg.SessionID,
		"-out", filepath.Join(cfg.DataDir, "authority-shadow-value-report.md"),
		"-json-out", filepath.Join(cfg.DataDir, "authority-shadow-value-report.json"),
	}
	if strings.TrimSpace(pythonBaseURL) != "" {
		reportArgs = append(reportArgs, "-python-base", pythonBaseURL)
	}
	reportCmd, reportRunArgs, reportDisplay := managedCommand(exec, "shadow-value-report", reportArgs...)
	replay := map[string]any{
		"requested": true,
		"command":   reportDisplay,
		"json_out":  filepath.Join(cfg.DataDir, "authority-shadow-value-report.json"),
	}
	if out, err := exec.Run(ctx, reportCmd, reportRunArgs...); err != nil {
		replay["status"] = "failed"
		replay["error"] = fmt.Sprintf("%v: %s", err, string(out))
		steps = append(steps, executedStep{
			Name:       "authority-post-cutover-replay",
			Status:     "failed",
			Command:    reportDisplay,
			ExitCode:   -1,
			DurationMs: time.Since(start).Milliseconds(),
			Error:      replay["error"].(string),
		})
		base["post_cutover_replay"] = replay
		base["status"] = "failed"
		base["error"] = replay["error"]
		return base, steps, fmt.Errorf("authority post-cutover replay: %w", err)
	}
	replay["status"] = "ok"
	steps = append(steps, executedStep{
		Name:       "authority-post-cutover-replay",
		Status:     "ok",
		Command:    reportDisplay,
		DurationMs: time.Since(start).Milliseconds(),
		Details:    replay,
	})
	base["post_cutover_replay"] = replay

	stopStep := stopGoBackend(exec, cmd)
	stopStep.Name = "authority-stop-go-backend"
	steps = append(steps, stopStep)
	stopped = true
	if stopStep.Status != "ok" {
		base["status"] = "failed"
		base["error"] = stopStep.Error
		return base, steps, fmt.Errorf("authority backend stop failed: %s", stopStep.Error)
	}

	base["base_url"] = fmt.Sprintf("http://127.0.0.1:%d", port)
	base["status"] = "ok"
	base["post_cutover_replay_ok"] = true
	return base, steps, nil
}

func runPythonFallbackProbe(ctx context.Context, baseURL string) (map[string]any, error) {
	baseURL = strings.TrimRight(strings.TrimSpace(baseURL), "/")
	report := map[string]any{
		"requested":              true,
		"role":                   "python_fallback",
		"base_url":               baseURL,
		"selected_runtime":       "python_fallback",
		"fallback_available":     false,
		"authority_switch":       false,
		"go_default_switch":      false,
		"python_runtime_retired": false,
	}
	if baseURL == "" {
		report["status"] = "failed"
		report["error"] = "python fallback base URL is empty"
		return report, fmt.Errorf("python fallback base URL is empty")
	}
	probes, _ := probeReadEndpoints(ctx, baseURL, []string{"/health", "/ready", "/version", "/stats"})
	report["probes"] = probes
	ok := probeStatusOK(probes["/health"]) && probeStatusOK(probes["/stats"])
	report["fallback_available"] = ok
	report["required_probes"] = []string{"/health", "/stats"}
	report["optional_probes"] = []string{"/ready", "/version"}
	status := "ok"
	if !ok {
		status = "failed"
	}
	report["status"] = status
	if !ok {
		return report, fmt.Errorf("python fallback probe failed")
	}
	return report, nil
}

func probeReadEndpoints(ctx context.Context, baseURL string, paths []string) (map[string]map[string]any, error) {
	out := map[string]map[string]any{}
	var firstErr error
	for _, path := range paths {
		result, err := probeGET(ctx, baseURL+path)
		out[path] = result
		if err != nil && firstErr == nil {
			firstErr = err
		}
	}
	return out, firstErr
}

func probeGET(ctx context.Context, url string) (map[string]any, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return map[string]any{"url": url, "status": "failed", "error": err.Error()}, err
	}
	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return map[string]any{"url": url, "status": "failed", "error": err.Error()}, err
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	decoded := map[string]any{}
	if len(raw) > 0 {
		_ = json.Unmarshal(raw, &decoded)
	}
	status := "ok"
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		status = "failed"
	}
	result := map[string]any{
		"url":         url,
		"method":      http.MethodGet,
		"http_status": resp.StatusCode,
		"status":      status,
		"json":        decoded,
	}
	if status != "ok" {
		return result, fmt.Errorf("GET %s returned HTTP %d", url, resp.StatusCode)
	}
	return result, nil
}

func allProbeStatusOK(probes map[string]map[string]any) bool {
	for _, probe := range probes {
		if !probeStatusOK(probe) {
			return false
		}
	}
	return true
}

func probeStatusOK(probe map[string]any) bool {
	if probe == nil {
		return false
	}
	status, _ := probe["status"].(string)
	switch httpStatus := probe["http_status"].(type) {
	case int:
		return status == "ok" && httpStatus >= 200 && httpStatus < 300
	case float64:
		return status == "ok" && httpStatus >= 200 && httpStatus < 300
	default:
		return false
	}
}

func runBackupRestoreDrill(ctx context.Context, port int, dataDir string) (map[string]any, error) {
	const sourceDB = "archive_center_temp"
	const restoreDB = "archive_center_restore_temp"
	rootDSN := fmt.Sprintf("root@tcp(127.0.0.1:%d)/?timeout=10s&parseTime=true", port)
	db, err := sql.Open("mysql", rootDSN)
	if err != nil {
		return backupRestoreReport("failed", sourceDB, restoreDB, nil, err.Error(), ""), err
	}
	defer db.Close()
	if err := db.PingContext(ctx); err != nil {
		return backupRestoreReport("failed", sourceDB, restoreDB, nil, err.Error(), ""), err
	}

	tables, err := listBaseTables(ctx, db, sourceDB)
	if err != nil {
		return backupRestoreReport("failed", sourceDB, restoreDB, nil, err.Error(), ""), err
	}
	if len(tables) == 0 {
		err := fmt.Errorf("source database %s has no base tables", sourceDB)
		return backupRestoreReport("failed", sourceDB, restoreDB, nil, err.Error(), ""), err
	}

	if _, err := db.ExecContext(ctx, "DROP DATABASE IF EXISTS "+escapeIdentifier(restoreDB)); err != nil {
		return backupRestoreReport("failed", sourceDB, restoreDB, nil, err.Error(), ""), err
	}
	if _, err := db.ExecContext(ctx, "CREATE DATABASE "+escapeIdentifier(restoreDB)+" CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci"); err != nil {
		return backupRestoreReport("failed", sourceDB, restoreDB, nil, err.Error(), ""), err
	}

	tableReports := make([]map[string]any, 0, len(tables))
	for _, table := range tables {
		source := escapeIdentifier(sourceDB) + "." + escapeIdentifier(table)
		restore := escapeIdentifier(restoreDB) + "." + escapeIdentifier(table)
		if _, err := db.ExecContext(ctx, "CREATE TABLE "+restore+" LIKE "+source); err != nil {
			return backupRestoreReport("failed", sourceDB, restoreDB, tableReports, err.Error(), ""), err
		}
		if _, err := db.ExecContext(ctx, "INSERT INTO "+restore+" SELECT * FROM "+source); err != nil {
			return backupRestoreReport("failed", sourceDB, restoreDB, tableReports, err.Error(), ""), err
		}
		sourceCount, err := tableCount(ctx, db, sourceDB, table)
		if err != nil {
			return backupRestoreReport("failed", sourceDB, restoreDB, tableReports, err.Error(), ""), err
		}
		restoreCount, err := tableCount(ctx, db, restoreDB, table)
		if err != nil {
			return backupRestoreReport("failed", sourceDB, restoreDB, tableReports, err.Error(), ""), err
		}
		tableReports = append(tableReports, map[string]any{
			"table":         table,
			"source_rows":   sourceCount,
			"restored_rows": restoreCount,
			"match":         sourceCount == restoreCount,
		})
		if sourceCount != restoreCount {
			err := fmt.Errorf("restore count mismatch for %s: source=%d restored=%d", table, sourceCount, restoreCount)
			return backupRestoreReport("failed", sourceDB, restoreDB, tableReports, err.Error(), ""), err
		}
	}

	manifestPath := filepath.Join(dataDir, "backup-restore-drill.json")
	report := backupRestoreReport("ok", sourceDB, restoreDB, tableReports, "", manifestPath)
	manifest, _ := json.MarshalIndent(report, "", "  ")
	manifest = append(manifest, '\n')
	if err := os.WriteFile(manifestPath, manifest, 0644); err != nil {
		return backupRestoreReport("failed", sourceDB, restoreDB, tableReports, err.Error(), ""), err
	}
	return report, nil
}

func listBaseTables(ctx context.Context, db *sql.DB, database string) ([]string, error) {
	rows, err := db.QueryContext(ctx, "SHOW FULL TABLES FROM "+escapeIdentifier(database)+" WHERE Table_type = 'BASE TABLE'")
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var tables []string
	for rows.Next() {
		var table string
		var tableType string
		if err := rows.Scan(&table, &tableType); err != nil {
			return nil, err
		}
		tables = append(tables, table)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	sort.Strings(tables)
	return tables, nil
}

func tableCount(ctx context.Context, db *sql.DB, database string, table string) (int, error) {
	var count int
	query := "SELECT COUNT(*) FROM " + escapeIdentifier(database) + "." + escapeIdentifier(table)
	if err := db.QueryRowContext(ctx, query).Scan(&count); err != nil {
		return 0, err
	}
	return count, nil
}

func backupRestoreReport(status string, sourceDB string, restoreDB string, tables []map[string]any, errText string, manifestPath string) map[string]any {
	totalSource := 0
	totalRestored := 0
	allMatch := status == "ok"
	for _, table := range tables {
		if v, ok := table["source_rows"].(int); ok {
			totalSource += v
		}
		if v, ok := table["restored_rows"].(int); ok {
			totalRestored += v
		}
		if v, ok := table["match"].(bool); ok && !v {
			allMatch = false
		}
	}
	out := map[string]any{
		"requested":           true,
		"status":              status,
		"method":              "managed_sql_clone_restore",
		"source_database":     sourceDB,
		"restored_database":   restoreDB,
		"tables_checked":      len(tables),
		"source_rows_total":   totalSource,
		"restored_rows_total": totalRestored,
		"row_count_match":     allMatch && totalSource == totalRestored,
		"tables":              tables,
		"authority_switch":    false,
		"go_default_switch":   false,
	}
	if manifestPath != "" {
		out["manifest_path"] = manifestPath
	}
	if errText != "" {
		out["error"] = errText
	}
	return out
}

func stopGoBackend(exec commandExecutor, cmd *exec.Cmd) executedStep {
	if cmd == nil {
		return executedStep{Name: "stop-go-backend", Status: "ok", Note: "no process to stop"}
	}
	if err := exec.Kill(cmd); err != nil {
		return executedStep{Name: "stop-go-backend", Status: "failed", Error: err.Error()}
	}
	return executedStep{Name: "stop-go-backend", Status: "ok"}
}

func goServiceRoot() string {
	wd, err := os.Getwd()
	if err != nil {
		return ""
	}
	for {
		if info, err := os.Stat(filepath.Join(wd, "go.mod")); err == nil && !info.IsDir() {
			return wd
		}
		parent := filepath.Dir(wd)
		if parent == wd {
			return ""
		}
		wd = parent
	}
}

func managedCommand(exec commandExecutor, name string, args ...string) (string, []string, string) {
	if path, err := exec.LookPath(name); err == nil {
		return path, args, name + " " + strings.Join(args, " ")
	}
	if root := goServiceRoot(); root != "" {
		runArgs := append([]string{"run", "-buildvcs=false", "./cmd/" + name}, args...)
		return "go", runArgs, "go " + strings.Join(runArgs, " ")
	}
	return name, args, name + " " + strings.Join(args, " ")
}

func (r *osDirectProviderRunner) run(ctx context.Context, cfg directProviderConfig) (steps []executedStep, err error) {
	// Step: create temp data dir
	start := time.Now()
	if !cfg.KeepTemp && strings.TrimSpace(cfg.DataDir) != "" {
		_ = os.RemoveAll(cfg.DataDir)
	}
	if err := os.MkdirAll(cfg.DataDir, 0750); err != nil {
		steps = append(steps, executedStep{
			Name:       "create-datadir",
			Status:     "failed",
			Command:    fmt.Sprintf("mkdir %s", cfg.DataDir),
			ExitCode:   -1,
			DurationMs: time.Since(start).Milliseconds(),
			Error:      err.Error(),
		})
		return steps, fmt.Errorf("create datadir: %w", err)
	}
	steps = append(steps, executedStep{
		Name:       "create-datadir",
		Status:     "ok",
		DurationMs: time.Since(start).Milliseconds(),
	})

	// Step: check port availability
	start = time.Now()
	ln, err := net.Listen("tcp", fmt.Sprintf("127.0.0.1:%d", cfg.Port))
	if err != nil {
		steps = append(steps, executedStep{
			Name:       "port-check",
			Status:     "failed",
			Command:    fmt.Sprintf("net.Listen tcp 127.0.0.1:%d", cfg.Port),
			ExitCode:   -1,
			DurationMs: time.Since(start).Milliseconds(),
			Error:      err.Error(),
		})
		steps = append(steps, cleanupTempStep(cfg.DataDir, cfg.KeepTemp))
		return steps, fmt.Errorf("port check: %w", err)
	}
	_ = ln.Close()
	steps = append(steps, executedStep{
		Name:       "port-check",
		Status:     "ok",
		DurationMs: time.Since(start).Milliseconds(),
	})

	// Step: init data dir
	start = time.Now()
	initCmd, initArgs := buildInitCommand(cfg.ProviderPath, cfg.DataDir)
	if out, err := r.exec.Run(ctx, initCmd, initArgs...); err != nil {
		steps = append(steps, executedStep{
			Name:       "init-datadir",
			Status:     "failed",
			Command:    initCmd + " " + strings.Join(initArgs, " "),
			ExitCode:   -1,
			DurationMs: time.Since(start).Milliseconds(),
			Error:      fmt.Sprintf("%v: %s", err, string(out)),
		})
		steps = append(steps, cleanupTempStep(cfg.DataDir, cfg.KeepTemp))
		return steps, fmt.Errorf("init datadir: %w", err)
	}
	steps = append(steps, executedStep{
		Name:       "init-datadir",
		Status:     "ok",
		DurationMs: time.Since(start).Milliseconds(),
	})

	// Step: start server
	start = time.Now()
	serverArgs := buildStartArgs(cfg.ProviderPath, cfg.DataDir, cfg.Port)
	serverCmd, err := r.exec.Start(ctx, cfg.ProviderPath, serverArgs...)
	if err != nil {
		steps = append(steps, executedStep{
			Name:       "start-server",
			Status:     "failed",
			Command:    cfg.ProviderPath + " " + strings.Join(serverArgs, " "),
			ExitCode:   -1,
			DurationMs: time.Since(start).Milliseconds(),
			Error:      err.Error(),
		})
		steps = append(steps, cleanupTempStep(cfg.DataDir, cfg.KeepTemp))
		return steps, fmt.Errorf("start server: %w", err)
	}
	steps = append(steps, executedStep{
		Name:       "start-server",
		Status:     "ok",
		DurationMs: time.Since(start).Milliseconds(),
	})

	defer func() {
		steps = append(steps, stopServerStep(cfg, serverCmd, r.exec)...)
		steps = append(steps, cleanupTempStep(cfg.DataDir, cfg.KeepTemp))
	}()

	// Step: wait for server readiness
	start = time.Now()
	if err := waitForServerReady(ctx, cfg.Port); err != nil {
		steps = append(steps, executedStep{
			Name:       "wait-ready",
			Status:     "failed",
			DurationMs: time.Since(start).Milliseconds(),
			Error:      err.Error(),
		})
		return steps, fmt.Errorf("wait ready: %w", err)
	}
	steps = append(steps, executedStep{
		Name:       "wait-ready",
		Status:     "ok",
		DurationMs: time.Since(start).Milliseconds(),
	})

	// Step: bootstrap database
	start = time.Now()
	if err := bootstrapDatabase(ctx, cfg); err != nil {
		steps = append(steps, executedStep{
			Name:       "bootstrap-database",
			Status:     "failed",
			DurationMs: time.Since(start).Milliseconds(),
			Error:      err.Error(),
		})
		return steps, fmt.Errorf("bootstrap database: %w", err)
	}
	steps = append(steps, executedStep{
		Name:       "bootstrap-database",
		Status:     "ok",
		DurationMs: time.Since(start).Milliseconds(),
	})

	// Optional: sqlite-export
	effectiveExportDir := cfg.ExportDir
	if strings.TrimSpace(cfg.SQLiteDB) != "" {
		start = time.Now()
		effectiveExportDir = filepath.Join(cfg.DataDir, "export")
		exportArgs := []string{"-db", cfg.SQLiteDB, "-out", effectiveExportDir, "-all"}
		exportCmd, exportRunArgs, exportDisplay := managedCommand(r.exec, "sqlite-export", exportArgs...)
		if out, err := r.exec.Run(ctx, exportCmd, exportRunArgs...); err != nil {
			steps = append(steps, executedStep{
				Name:       "sqlite-export",
				Status:     "failed",
				Command:    exportDisplay,
				ExitCode:   -1,
				DurationMs: time.Since(start).Milliseconds(),
				Error:      fmt.Sprintf("%v: %s", err, string(out)),
			})
			return steps, fmt.Errorf("sqlite export: %w", err)
		}
		steps = append(steps, executedStep{
			Name:       "sqlite-export",
			Status:     "ok",
			DurationMs: time.Since(start).Milliseconds(),
		})
	}

	// Step: mariadb-schema
	start = time.Now()
	dsn := buildInternalDSN(cfg.DataDir, cfg.Port, cfg.SessionID)
	schemaArgs := []string{"-dsn", dsn, "-execute"}
	schemaCmd, schemaRunArgs, schemaDisplay := managedCommand(r.exec, "mariadb-schema", schemaArgs...)
	if out, err := r.exec.Run(ctx, schemaCmd, schemaRunArgs...); err != nil {
		steps = append(steps, executedStep{
			Name:       "mariadb-schema",
			Status:     "failed",
			Command:    strings.ReplaceAll(schemaDisplay, dsn, redactDSN(dsn)),
			ExitCode:   -1,
			DurationMs: time.Since(start).Milliseconds(),
			Error:      fmt.Sprintf("%v: %s", err, string(out)),
		})
		return steps, fmt.Errorf("mariadb schema: %w", err)
	}
	steps = append(steps, executedStep{
		Name:       "mariadb-schema",
		Status:     "ok",
		DurationMs: time.Since(start).Milliseconds(),
	})

	if !cfg.skipDefaultReadShadow() {
		// Step: mariadb-import
		start = time.Now()
		importArgs := []string{"-export-dir", effectiveExportDir, "-dsn", dsn, "-execute"}
		importCmd, importRunArgs, importDisplay := managedCommand(r.exec, "mariadb-import", importArgs...)
		if out, err := r.exec.Run(ctx, importCmd, importRunArgs...); err != nil {
			steps = append(steps, executedStep{
				Name:       "mariadb-import",
				Status:     "failed",
				Command:    strings.ReplaceAll(importDisplay, dsn, redactDSN(dsn)),
				ExitCode:   -1,
				DurationMs: time.Since(start).Milliseconds(),
				Error:      fmt.Sprintf("%v: %s", err, string(out)),
			})
			return steps, fmt.Errorf("mariadb import: %w", err)
		}
		steps = append(steps, executedStep{
			Name:       "mariadb-import",
			Status:     "ok",
			DurationMs: time.Since(start).Milliseconds(),
		})

		// Step: mariadb-compare
		start = time.Now()
		compareArgs := []string{"-export-dir", effectiveExportDir, "-dsn", dsn}
		compareCmd, compareRunArgs, compareDisplay := managedCommand(r.exec, "mariadb-compare", compareArgs...)
		if out, err := r.exec.Run(ctx, compareCmd, compareRunArgs...); err != nil {
			steps = append(steps, executedStep{
				Name:       "mariadb-compare",
				Status:     "failed",
				Command:    strings.ReplaceAll(compareDisplay, dsn, redactDSN(dsn)),
				ExitCode:   -1,
				DurationMs: time.Since(start).Milliseconds(),
				Error:      fmt.Sprintf("%v: %s", err, string(out)),
			})
			return steps, fmt.Errorf("mariadb compare: %w", err)
		}
		steps = append(steps, executedStep{
			Name:       "mariadb-compare",
			Status:     "ok",
			DurationMs: time.Since(start).Milliseconds(),
		})
	}

	goBinPath := cfg.GoBinPath
	if goBinPath == "" {
		if p, err := r.exec.LookPath("archive-center-go"); err == nil {
			goBinPath = p
		}
	}
	if goBinPath == "" {
		steps = append(steps, executedStep{
			Name:   "start-go-backend",
			Status: "failed",
			Error:  "archive-center-go binary not found in PATH and no -go-bin provided",
		})
		return steps, fmt.Errorf("go backend binary not found")
	}

	effectivePythonBaseURL := cfg.PythonBaseURL
	if (cfg.DefaultSwitch || cfg.AuthorityCutover) && strings.TrimSpace(cfg.PythonFallbackSrc) != "" {
		fallbackDir := filepath.Join(cfg.DataDir, "python-fallback-0.8")
		start = time.Now()
		err := copyPythonFallbackSource(ctx, cfg.PythonFallbackSrc, fallbackDir)
		status := "ok"
		errText := ""
		if err != nil {
			status = "failed"
			errText = err.Error()
		}
		steps = append(steps, executedStep{
			Name:       "python-fallback-copy",
			Status:     status,
			DurationMs: time.Since(start).Milliseconds(),
			Error:      errText,
			Details: map[string]any{
				"source_dir": cfg.PythonFallbackSrc,
				"temp_dir":   fallbackDir,
			},
		})
		if err != nil {
			return steps, fmt.Errorf("python fallback copy: %w", err)
		}

		fallbackPort := cfg.PythonFallbackPort
		if fallbackPort <= 0 {
			fallbackPort = 18106
		}
		fallbackPort, err = findAvailablePort("127.0.0.1", fallbackPort, fallbackPort+20)
		if err != nil {
			steps = append(steps, executedStep{Name: "python-fallback-start", Status: "failed", Error: err.Error()})
			return steps, fmt.Errorf("python fallback port: %w", err)
		}
		effectivePythonBaseURL = fmt.Sprintf("http://127.0.0.1:%d", fallbackPort)
		start = time.Now()
		pythonFallback, step, err := startPythonFallbackBackend(ctx, fallbackDir, fallbackPort)
		step.DurationMs = time.Since(start).Milliseconds()
		step.Details = map[string]any{
			"base_url": effectivePythonBaseURL,
			"temp_dir": fallbackDir,
		}
		steps = append(steps, step)
		if err != nil {
			return steps, fmt.Errorf("python fallback start: %w", err)
		}
		defer pythonFallback.stop()

		waitStep := waitPythonFallbackReady(ctx, fallbackPort)
		steps = append(steps, waitStep)
		if waitStep.Status != "ok" {
			return steps, fmt.Errorf("python fallback not ready: %s", waitStep.Error)
		}
	}

	if cfg.RouteWriteSmoke {
		routePort, err := findAvailablePort("127.0.0.1", cfg.GoHTTPPort, cfg.GoHTTPPort+20)
		if err != nil {
			steps = append(steps, executedStep{
				Name:   "route-start-go-backend",
				Status: "failed",
				Error:  err.Error(),
			})
			return steps, fmt.Errorf("find route write go backend port: %w", err)
		}

		routeCmd, routeStart, err := startGoBackend(ctx, r.exec, goBackendStartConfig{
			BinPath:          goBinPath,
			Port:             routePort,
			DSN:              dsn,
			StoreMode:        "mariadb_shadow",
			ProductReadProof: false,
		})
		routeStart.Name = "route-start-go-backend"
		steps = append(steps, routeStart)
		if err != nil {
			return steps, fmt.Errorf("start route write go backend: %w", err)
		}
		routeStopped := false
		defer func() {
			if !routeStopped {
				step := stopGoBackend(r.exec, routeCmd)
				step.Name = "route-stop-go-backend"
				steps = append(steps, step)
			}
		}()

		waitStep := waitGoReady(ctx, routePort)
		waitStep.Name = "route-wait-go-ready"
		steps = append(steps, waitStep)
		if waitStep.Status != "ok" {
			return steps, fmt.Errorf("route write go backend not ready: %s", waitStep.Error)
		}

		start = time.Now()
		smokeDetails, err := runRouteWriteSmoke(ctx, routePort, dsn, cfg.SessionID, "mariadb_shadow")
		status := "ok"
		errText := ""
		if err != nil {
			status = "failed"
			errText = err.Error()
		}
		steps = append(steps, executedStep{
			Name:       "route-write-smoke",
			Status:     status,
			DurationMs: time.Since(start).Milliseconds(),
			Error:      errText,
			Details:    smokeDetails,
		})
		stopRoute := stopGoBackend(r.exec, routeCmd)
		stopRoute.Name = "route-stop-go-backend"
		steps = append(steps, stopRoute)
		routeStopped = true
		if err != nil {
			return steps, fmt.Errorf("route write smoke: %w", err)
		}

	}

	if cfg.SessionIsolationSmoke {
		isoPort, err := findAvailablePort("127.0.0.1", cfg.GoHTTPPort, cfg.GoHTTPPort+20)
		if err != nil {
			steps = append(steps, executedStep{
				Name:   "session-isolation-start-go-backend",
				Status: "failed",
				Error:  err.Error(),
			})
			return steps, fmt.Errorf("find session isolation go backend port: %w", err)
		}

		isoCmd, isoStart, err := startGoBackend(ctx, r.exec, goBackendStartConfig{
			BinPath:   goBinPath,
			Port:      isoPort,
			DSN:       dsn,
			StoreMode: "mariadb_authority",
		})
		isoStart.Name = "session-isolation-start-go-backend"
		steps = append(steps, isoStart)
		if err != nil {
			return steps, fmt.Errorf("start session isolation go backend: %w", err)
		}
		isoStopped := false
		defer func() {
			if !isoStopped {
				step := stopGoBackend(r.exec, isoCmd)
				step.Name = "session-isolation-stop-go-backend"
				steps = append(steps, step)
			}
		}()

		waitStep := waitGoReady(ctx, isoPort)
		waitStep.Name = "session-isolation-wait-go-ready"
		steps = append(steps, waitStep)
		if waitStep.Status != "ok" {
			return steps, fmt.Errorf("session isolation go backend not ready: %s", waitStep.Error)
		}

		start = time.Now()
		isoDetails, err := runSessionIsolationSmokeStandalone(ctx, isoPort, cfg.SessionID)
		status := "ok"
		errText := ""
		if err != nil {
			status = "failed"
			errText = err.Error()
		}
		steps = append(steps, executedStep{
			Name:       "session-isolation-smoke",
			Status:     status,
			DurationMs: time.Since(start).Milliseconds(),
			Error:      errText,
			Details:    isoDetails,
		})
		stopIso := stopGoBackend(r.exec, isoCmd)
		stopIso.Name = "session-isolation-stop-go-backend"
		steps = append(steps, stopIso)
		isoStopped = true
		if err != nil {
			return steps, fmt.Errorf("session isolation smoke: %w", err)
		}
	}

	if cfg.skipDefaultReadShadow() {
		return steps, nil
	}

	if cfg.BackupRestore {
		start = time.Now()
		restoreDetails, err := runBackupRestoreDrill(ctx, cfg.Port, cfg.DataDir)
		status := "ok"
		errText := ""
		if err != nil {
			status = "failed"
			errText = err.Error()
		}
		steps = append(steps, executedStep{
			Name:       "backup-restore-drill",
			Status:     status,
			DurationMs: time.Since(start).Milliseconds(),
			Error:      errText,
			Details:    restoreDetails,
		})
		if err != nil {
			return steps, fmt.Errorf("backup restore drill: %w", err)
		}
	}

	// --- Go backend steps ---
	goPort, err := findAvailablePort("127.0.0.1", cfg.GoHTTPPort, cfg.GoHTTPPort+20)
	if err != nil {
		steps = append(steps, executedStep{
			Name:   "start-go-backend",
			Status: "failed",
			Error:  err.Error(),
		})
		return steps, fmt.Errorf("find go backend port: %w", err)
	}

	goCmd, goStartStep, err := startGoBackend(ctx, r.exec, goBackendStartConfig{
		BinPath:          goBinPath,
		Port:             goPort,
		DSN:              dsn,
		StoreMode:        "mariadb_read_shadow",
		ProductReadProof: cfg.ProductReadProof,
	})
	steps = append(steps, goStartStep)
	if err != nil {
		return steps, fmt.Errorf("start go backend: %w", err)
	}
	// Ensure Go backend is stopped before server cleanup.
	goStopped := false
	defer func() {
		if !goStopped {
			steps = append(steps, stopGoBackend(r.exec, goCmd))
		}
	}()

	steps = append(steps, waitGoReady(ctx, goPort))
	last := steps[len(steps)-1]
	if last.Status != "ok" {
		return steps, fmt.Errorf("go backend not ready: %s", last.Error)
	}

	// Step: shadow-value-report
	start = time.Now()
	reportArgs := []string{
		"-go-base", fmt.Sprintf("http://127.0.0.1:%d", goPort),
		"-session-id", cfg.SessionID,
		"-out", filepath.Join(cfg.DataDir, "shadow-value-report.md"),
		"-json-out", filepath.Join(cfg.DataDir, "shadow-value-report.json"),
	}
	if effectivePythonBaseURL != "" {
		reportArgs = append(reportArgs, "-python-base", effectivePythonBaseURL)
	}
	reportCmd, reportRunArgs, reportDisplay := managedCommand(r.exec, "shadow-value-report", reportArgs...)
	if out, err := r.exec.Run(ctx, reportCmd, reportRunArgs...); err != nil {
		steps = append(steps, executedStep{
			Name:       "shadow-value-report",
			Status:     "failed",
			Command:    reportDisplay,
			ExitCode:   -1,
			DurationMs: time.Since(start).Milliseconds(),
			Error:      fmt.Sprintf("%v: %s", err, string(out)),
		})
		return steps, &degradedError{msg: fmt.Sprintf("shadow-value-report failed: %v", err)}
	}
	steps = append(steps, executedStep{
		Name:       "shadow-value-report",
		Status:     "ok",
		Command:    reportDisplay,
		DurationMs: time.Since(start).Milliseconds(),
	})

	if cfg.DefaultSwitch {
		start = time.Now()
		candidateDetails, err := runDefaultCandidateProbe(ctx, goPort, cfg.DefaultSwitchActual)
		status := "ok"
		errText := ""
		if err != nil {
			status = "failed"
			errText = err.Error()
		}
		stepName := "go-default-candidate-probe"
		if cfg.DefaultSwitchActual {
			stepName = "go-default-actual-switch-gate"
		}
		steps = append(steps, executedStep{
			Name:       stepName,
			Status:     status,
			DurationMs: time.Since(start).Milliseconds(),
			Error:      errText,
			Details:    candidateDetails,
		})
		if err != nil {
			return steps, fmt.Errorf("go default candidate probe: %w", err)
		}
	}

	if cfg.AuthorityCutover {
		start = time.Now()
		authorityDetails, authoritySteps, err := runAuthorityCutoverReplay(ctx, r.exec, cfg, goBinPath, dsn, effectivePythonBaseURL, goPort+31)
		for _, step := range authoritySteps {
			if step.DurationMs == 0 && step.Name == "authority-start-go-backend" {
				step.DurationMs = time.Since(start).Milliseconds()
			}
			steps = append(steps, step)
		}
		status := "ok"
		errText := ""
		if err != nil {
			status = "failed"
			errText = err.Error()
		}
		steps = append(steps, executedStep{
			Name:       "authority-cutover-summary",
			Status:     status,
			DurationMs: time.Since(start).Milliseconds(),
			Error:      errText,
			Details:    authorityDetails,
		})
		if err != nil {
			return steps, fmt.Errorf("authority cutover replay: %w", err)
		}
	}

	if cfg.ProductReadProof {
		stopStep := stopGoBackend(r.exec, goCmd)
		stopStep.Name = "rollback-stop-product-go-backend"
		steps = append(steps, stopStep)
		goStopped = true

		if cfg.DefaultSwitch || cfg.AuthorityCutover {
			start = time.Now()
			fallbackDetails, err := runPythonFallbackProbe(ctx, effectivePythonBaseURL)
			status := "ok"
			errText := ""
			if err != nil {
				status = "failed"
				errText = err.Error()
			}
			steps = append(steps, executedStep{
				Name:       "python-fallback-replay",
				Status:     status,
				DurationMs: time.Since(start).Milliseconds(),
				Error:      errText,
				Details:    fallbackDetails,
			})
			if err != nil {
				return steps, fmt.Errorf("python fallback replay: %w", err)
			}
		}

		rollbackPort, err := findAvailablePort("127.0.0.1", goPort+1, goPort+30)
		if err != nil {
			steps = append(steps, executedStep{
				Name:   "rollback-start-go-backend",
				Status: "failed",
				Error:  err.Error(),
			})
			return steps, fmt.Errorf("find rollback go backend port: %w", err)
		}

		rollbackCmd, rollbackStart, err := startGoBackend(ctx, r.exec, goBackendStartConfig{
			BinPath:   goBinPath,
			Port:      rollbackPort,
			StoreMode: "noop",
		})
		rollbackStart.Name = "rollback-start-go-backend"
		steps = append(steps, rollbackStart)
		if err != nil {
			return steps, fmt.Errorf("start rollback go backend: %w", err)
		}

		rollbackStopped := false
		defer func() {
			if !rollbackStopped {
				step := stopGoBackend(r.exec, rollbackCmd)
				step.Name = "rollback-stop-go-backend"
				steps = append(steps, step)
			}
		}()

		waitStep := waitGoReady(ctx, rollbackPort)
		waitStep.Name = "rollback-wait-go-ready"
		steps = append(steps, waitStep)
		if waitStep.Status != "ok" {
			return steps, fmt.Errorf("rollback go backend not ready: %s", waitStep.Error)
		}

		checks, err := fetchReadyChecks(ctx, rollbackPort)
		if err != nil {
			steps = append(steps, executedStep{
				Name:   "rollback-ready-check",
				Status: "failed",
				Error:  err.Error(),
			})
			return steps, fmt.Errorf("rollback ready check: %w", err)
		}
		rolledBack := checks["store_mode"] == "noop" &&
			checks["mariadb_product_read"] == "disabled" &&
			checks["mariadb_authority"] == "disabled"
		status := "ok"
		if !rolledBack {
			status = "failed"
		}
		steps = append(steps, executedStep{
			Name:   "rollback-ready-check",
			Status: status,
			Note:   fmt.Sprintf("store_mode=%s mariadb_product_read=%s mariadb_authority=%s", checks["store_mode"], checks["mariadb_product_read"], checks["mariadb_authority"]),
		})
		stopRollback := stopGoBackend(r.exec, rollbackCmd)
		stopRollback.Name = "rollback-stop-go-backend"
		steps = append(steps, stopRollback)
		rollbackStopped = true
		if !rolledBack {
			return steps, fmt.Errorf("rollback proof failed")
		}
	}

	return steps, nil
}

func waitForServerReady(ctx context.Context, port int) error {
	addr := fmt.Sprintf("127.0.0.1:%d", port)
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}
		conn, err := net.DialTimeout("tcp", addr, 2*time.Second)
		if err != nil {
			time.Sleep(500 * time.Millisecond)
			continue
		}
		_ = conn.Close()
		return nil
	}
}

func bootstrapDatabase(ctx context.Context, cfg directProviderConfig) error {
	rootDSN := fmt.Sprintf("root@tcp(127.0.0.1:%d)/?timeout=3s&readTimeout=3s&writeTimeout=3s", cfg.Port)
	db, err := sql.Open("mysql", rootDSN)
	if err != nil {
		return err
	}
	defer db.Close()

	if err := db.PingContext(ctx); err != nil {
		return err
	}

	user := "ac_root"
	password := buildPassword(cfg.SessionID)
	dbName := "archive_center_temp"
	userLit := quoteStringLiteral(user)
	passwordLit := quoteStringLiteral(password)
	dbIdent := escapeIdentifier(dbName)

	createDBSQL := fmt.Sprintf("CREATE DATABASE IF NOT EXISTS %s CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci", dbIdent)
	if _, err := db.ExecContext(ctx, createDBSQL); err != nil {
		return fmt.Errorf("create database: %w", err)
	}

	createUserSQL := fmt.Sprintf("CREATE USER IF NOT EXISTS %s@'127.0.0.1' IDENTIFIED BY %s", userLit, passwordLit)
	if _, err := db.ExecContext(ctx, createUserSQL); err != nil {
		return fmt.Errorf("create user: %w", err)
	}

	grantSQL := fmt.Sprintf("GRANT ALL PRIVILEGES ON %s.* TO %s@'127.0.0.1'", dbIdent, userLit)
	if _, err := db.ExecContext(ctx, grantSQL); err != nil {
		return fmt.Errorf("grant privileges: %w", err)
	}

	createLocalhostSQL := fmt.Sprintf("CREATE USER IF NOT EXISTS %s@'localhost' IDENTIFIED BY %s", userLit, passwordLit)
	if _, err := db.ExecContext(ctx, createLocalhostSQL); err == nil {
		grantLocalhostSQL := fmt.Sprintf("GRANT ALL PRIVILEGES ON %s.* TO %s@'localhost'", dbIdent, userLit)
		_, _ = db.ExecContext(ctx, grantLocalhostSQL)
	}
	_, _ = db.ExecContext(ctx, "FLUSH PRIVILEGES")

	return nil
}

func buildInitArgs(providerPath, dataDir string) []string {
	return []string{"--no-defaults", "--initialize-insecure", "--datadir", dataDir}
}

func buildInitCommand(providerPath, dataDir string) (string, []string) {
	if runtime.GOOS == "windows" {
		binDir := filepath.Dir(providerPath)
		for _, name := range []string{"mariadb-install-db.exe", "mysql_install_db.exe"} {
			candidate := filepath.Join(binDir, name)
			if info, err := os.Stat(candidate); err == nil && !info.IsDir() {
				return candidate, []string{"--datadir=" + dataDir, "--password="}
			}
		}
	}
	return providerPath, buildInitArgs(providerPath, dataDir)
}

func buildStartArgs(providerPath, dataDir string, port int) []string {
	return []string{
		"--no-defaults",
		"--datadir", dataDir,
		"--port", strconv.Itoa(port),
		"--socket", filepath.Join(dataDir, "mysql.sock"),
		"--skip-networking=0",
		"--bind-address=127.0.0.1",
		"--pid-file", filepath.Join(dataDir, "mysqld.pid"),
	}
}

func stopServerStep(cfg directProviderConfig, serverCmd *exec.Cmd, exec commandExecutor) []executedStep {
	var out []executedStep
	if serverCmd != nil && serverCmd.Process != nil {
		_ = exec.Kill(serverCmd)
		out = append(out, executedStep{Name: "stop-server", Status: "ok"})
	} else {
		out = append(out, executedStep{Name: "stop-server", Status: "ok", Note: "no running process"})
	}
	return out
}

func cleanupTempStep(dataDir string, keep bool) executedStep {
	if keep {
		return executedStep{Name: "cleanup", Status: "retained", Note: "keep-temp=true"}
	}
	if err := os.RemoveAll(dataDir); err != nil {
		return executedStep{Name: "cleanup", Status: "failed", Error: err.Error()}
	}
	return executedStep{Name: "cleanup", Status: "ok"}
}

var defaultDirectRunner directProviderRunner = newOSDirectProviderRunner()

func main() {
	sqliteDB := flag.String("sqlite-db", "", "Path to SQLite database")
	exportDir := flag.String("export-dir", "", "Path to export directory")
	pythonBaseURL := flag.String("python-base", "http://127.0.0.1:8000", "Python 0.8 backend base URL for shadow-value-report")
	outPath := flag.String("out", "", "JSON report path; stdout if empty")
	execute := flag.Bool("execute", false, "Execute the plan (default: guarded)")
	keepTemp := flag.Bool("keep-temp", false, "Keep temporary directory after run")
	sessionID := flag.String("session-id", "managed-mariadb-e2e", "Session ID for temp resources")
	productReadProof := flag.Bool("product-read-proof", false, "Enable R2 MariaDB product-read flag and rollback proof on the disposable Go backend")
	routeWriteSmoke := flag.Bool("route-write-smoke", false, "Run disposable Go HTTP route write smoke against temp MariaDB in mariadb_shadow mode")
	sessionIsolationSmoke := flag.Bool("session-isolation-smoke", false, "Run disposable RMG-03 session isolation smoke against temp MariaDB in mariadb_authority mode")
	backupRestoreDrill := flag.Bool("backup-restore-drill", false, "Clone source MariaDB into a restored database and verify table row counts before cutover")
	authorityCutoverReplay := flag.Bool("authority-cutover-replay", false, "Run a managed disposable MariaDB authority cutover replay with post-cutover replay and rollback proof")
	defaultSwitchRehearsal := flag.Bool("default-switch-rehearsal", false, "Probe Go as a disposable default-runtime candidate, stop it, then prove Python fallback remains reachable")
	defaultSwitchActual := flag.Bool("default-switch-actual", false, "Run a managed disposable Go default-runtime actual switch gate with post-switch replay and Python fallback proof")
	pythonFallbackSrc := flag.String("python-fallback-src-dir", "", "Optional 0.8 source tree to temp-copy and start as Python fallback for default-switch rehearsal")
	pythonFallbackPort := flag.Int("python-fallback-port", 18106, "Preferred Python fallback temp backend port")
	goBin := flag.String("go-bin", "", "Optional archive-center-go binary path for read-shadow value report")
	providerBin := flag.String("provider-bin", "", "Explicit path to MariaDB server binary (mariadbd or mysqld)")
	flag.Parse()
	if *providerBin == "" {
		*providerBin = os.Getenv("AC_MARIADB_PROVIDER_BIN")
	}

	if *defaultSwitchActual {
		*defaultSwitchRehearsal = true
	}
	if *authorityCutoverReplay {
		*productReadProof = true
	}
	r := runWithOptions(*sqliteDB, *exportDir, *pythonBaseURL, *execute, *keepTemp, *sessionID, *productReadProof, *routeWriteSmoke, *backupRestoreDrill, *authorityCutoverReplay, *defaultSwitchRehearsal, *defaultSwitchActual, *sessionIsolationSmoke, *pythonFallbackSrc, *pythonFallbackPort, *goBin, *providerBin, bundledLookup{fallback: osExecLookup{}})
	reportJSON, err := json.MarshalIndent(r, "", "  ")
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to marshal report: %v\n", err)
		os.Exit(1)
	}
	reportJSON = append(reportJSON, '\n')
	if *outPath != "" {
		if err := os.WriteFile(*outPath, reportJSON, 0644); err != nil {
			fmt.Fprintf(os.Stderr, "write report: %v\n", err)
			os.Exit(1)
		}
	} else {
		_, _ = os.Stdout.Write(reportJSON)
	}
	switch r.Status {
	case "ok":
		os.Exit(0)
	case "blocked":
		os.Exit(3)
	case "degraded":
		os.Exit(4)
	default:
		os.Exit(2)
	}
}

func run(sqliteDB, exportDir, pythonBaseURL string, execute, keepTemp bool, sessionID string, lookup providerLookup) *report {
	return runWithOptions(sqliteDB, exportDir, pythonBaseURL, execute, keepTemp, sessionID, false, false, false, false, false, false, false, "", 0, "", "", lookup)
}

func runWithOptions(sqliteDB, exportDir, pythonBaseURL string, execute, keepTemp bool, sessionID string, productReadProof bool, routeWriteSmoke bool, backupRestoreDrill bool, authorityCutoverReplay bool, defaultSwitchRehearsal bool, defaultSwitchActual bool, sessionIsolationSmoke bool, pythonFallbackSrc string, pythonFallbackPort int, goBinPath string, explicitProviderBin string, lookup providerLookup) *report {
	sessionOnly := (directProviderConfig{
		ProductReadProof:      productReadProof,
		RouteWriteSmoke:       routeWriteSmoke,
		BackupRestore:         backupRestoreDrill,
		AuthorityCutover:      authorityCutoverReplay,
		DefaultSwitch:         defaultSwitchRehearsal,
		DefaultSwitchActual:   defaultSwitchActual,
		SessionIsolationSmoke: sessionIsolationSmoke,
	}).skipDefaultReadShadow()
	r := &report{
		Status:                "ok",
		GeneratedAt:           time.Now().UTC().Format(time.RFC3339),
		SourceMode:            deriveSourceMode(sqliteDB, exportDir),
		SQLiteDB:              sqliteDB,
		ExportDir:             exportDir,
		Execute:               execute,
		KeepTemp:              keepTemp,
		ProductReadProof:      productReadProof,
		RouteWriteSmoke:       summarizeRouteWriteSmoke(nil, routeWriteSmoke),
		SessionIsolationSmoke: summarizeSessionIsolationSmoke(nil, sessionIsolationSmoke),
		BackupRestore:         summarizeBackupRestore(nil, backupRestoreDrill),
		AuthorityCutover:      summarizeAuthorityCutover(nil, authorityCutoverReplay),
		DefaultSwitch:         summarizeDefaultSwitchRehearsal(nil, defaultSwitchRehearsal),
		DefaultRuntime:        summarizeDefaultRuntimeSwitch(nil, defaultSwitchActual),
		RollbackProof:         summarizeRollbackProof(nil, productReadProof),
		VectorRuntime:         summarizeVectorRuntime(),
		SafetyFlags: map[string]bool{
			"authority_switch":                  false,
			"mariadb_product_read_persisted":    false,
			"mariadb_authority_default_enabled": false,
			"chromadb_required":                 true,
			"chromadb_endpoint_configured":      strings.TrimSpace(os.Getenv("AC_CHROMA_ENDPOINT")) != "",
			"milvus_required":                   false,
			"live_milvus":                       false,
			"chroma_retired":                    false,
			"go_default_switch":                 false,
		},
		SchemaTables:    parseSchemaTables(discoverSchemaSQL()),
		StoreSaveTables: knownStoreSaveTables,
		StoreListTables: knownStoreListTables,
	}

	hasSQLite := strings.TrimSpace(sqliteDB) != ""
	hasExport := strings.TrimSpace(exportDir) != ""

	if !hasSQLite && !hasExport && !sessionOnly {
		r.Status = "failed"
		r.Errors = append(r.Errors, "missing source: provide -sqlite-db or -export-dir")
		return r
	}

	if hasSQLite && hasExport {
		r.Status = "failed"
		r.Errors = append(r.Errors, "ambiguous source: provide only one of -sqlite-db or -export-dir")
		return r
	}

	if !execute {
		r.Status = "guarded"
		r.Warnings = append(r.Warnings, "execute=false: no provider start, no DB touch")
		return r
	}
	if defaultSwitchActual && !productReadProof {
		r.Status = "failed"
		r.Errors = append(r.Errors, "default-switch-actual requires -product-read-proof so rollback can be proven")
		return r
	}
	if authorityCutoverReplay && !productReadProof {
		r.Status = "failed"
		r.Errors = append(r.Errors, "authority-cutover-replay requires -product-read-proof so rollback can be proven")
		return r
	}

	directProviders := []string{"mariadbd", "mysqld"}
	containerProviders := []string{"docker", "podman", "nerdctl"}

	detected := ""
	detectedPath := ""
	detectedType := ""

	// Check explicit provider first.
	explicit := strings.TrimSpace(explicitProviderBin)
	if explicit != "" {
		name := filepath.Base(explicit)
		name = strings.TrimSuffix(name, filepath.Ext(name))
		pi := providerInfo{Name: name, Path: explicit, Available: false, Type: "explicit_direct"}
		if info, err := os.Stat(explicit); err == nil && !info.IsDir() {
			if name == "mariadbd" || name == "mysqld" {
				pi.Available = true
			}
		}
		r.ProvidersChecked = append(r.ProvidersChecked, pi)
		if pi.Available {
			detected = pi.Name
			detectedPath = pi.Path
			detectedType = pi.Type
		}
	}

	if detected == "" {
		for _, name := range directProviders {
			pi := lookupProvider(lookup, name, "direct")
			r.ProvidersChecked = append(r.ProvidersChecked, pi)
			if pi.Available && detected == "" {
				detected = name
				detectedPath = pi.Path
				detectedType = pi.Type
			}
		}
		for _, name := range containerProviders {
			pi := lookupProvider(lookup, name, "container")
			r.ProvidersChecked = append(r.ProvidersChecked, pi)
			if pi.Available && detected == "" {
				detected = name
				detectedPath = pi.Path
				detectedType = pi.Type
			}
		}
	}

	if detected == "" {
		if explicit != "" {
			r.Status = "blocked"
			r.ProviderStatus = "missing_explicit"
			r.Errors = append(r.Errors, fmt.Sprintf("explicit MariaDB provider not found or not a server binary: %s", explicit))
			return r
		}
		r.Status = "blocked"
		r.ProviderStatus = "missing"
		r.Errors = append(r.Errors, "no MariaDB provider available")
		return r
	}

	safeID := safeSessionID(sessionID)
	tempDataDir := filepath.Join(os.TempDir(), fmt.Sprintf("archive-center-mariadb-%s", safeID))
	r.TempPlan = tempPlan{
		DataDir:     tempDataDir,
		Port:        13306,
		DSNRedacted: redactDSN(buildInternalDSN(tempDataDir, 13306, safeID)),
	}

	if hasSQLite {
		r.ChildPlan = append(r.ChildPlan, childPlanStep{
			Name: "sqlite-export",
			Note: "sqlite-export -all -db " + sqliteDB,
		})
	}
	r.ChildPlan = append(r.ChildPlan, childPlanStep{Name: "mariadb-schema", Note: "apply schema to temp instance"})
	if !sessionOnly {
		r.ChildPlan = append(r.ChildPlan, []childPlanStep{
			{Name: "mariadb-import", Note: "import canonical NDJSON into temp instance"},
			{Name: "mariadb-compare", Note: "compare temp instance against source"},
		}...)
		r.ChildPlan = append(r.ChildPlan, []childPlanStep{
			{Name: "start-go-backend", Note: "start Go shadow backend against temp MariaDB"},
			{Name: "wait-go-ready", Note: "wait for Go backend health"},
			{Name: "shadow-value-report", Note: "mariadb_read_shadow report with -go-base"},
			{Name: "stop-go-backend", Note: "stop Go shadow backend"},
		}...)
	}
	if productReadProof {
		rollbackSteps := []childPlanStep{
			{Name: "rollback-stop-product-go-backend", Note: "stop product-read proof backend"},
			{Name: "rollback-start-go-backend", Note: "restart Go backend with AC_STORE_MODE=noop and without AC_MARIADB_PRODUCT_READ_ENABLED"},
			{Name: "rollback-wait-go-ready", Note: "wait for rollback backend health"},
			{Name: "rollback-ready-check", Note: "prove store_mode=noop, mariadb_product_read=disabled, and mariadb_authority=disabled"},
			{Name: "rollback-stop-go-backend", Note: "stop rollback backend"},
		}
		if defaultSwitchRehearsal || authorityCutoverReplay {
			rollbackSteps = append([]childPlanStep{rollbackSteps[0], childPlanStep{
				Name: "python-fallback-replay",
				Note: "with Go candidate stopped, prove the Python fallback base remains reachable",
			}}, rollbackSteps[1:]...)
		}
		r.ChildPlan = append(r.ChildPlan, rollbackSteps...)
	}
	if defaultSwitchRehearsal {
		stepName := "go-default-candidate-probe"
		stepNote := "probe Go as the selected default-runtime candidate while keeping authority/default flags non-persistent"
		if defaultSwitchActual {
			stepName = "go-default-actual-switch-gate"
			stepNote = "run the managed disposable Go default-runtime actual switch gate with post-switch replay evidence"
		}
		r.ChildPlan = append(r.ChildPlan, childPlanStep{
			Name: stepName,
			Note: stepNote,
		})
	}
	if authorityCutoverReplay {
		r.ChildPlan = append(r.ChildPlan, []childPlanStep{
			{Name: "authority-start-go-backend", Note: "start Go backend in AC_STORE_MODE=mariadb_authority against temp MariaDB"},
			{Name: "authority-wait-go-ready", Note: "wait for authority backend health"},
			{Name: "authority-ready-check", Note: "prove store_mode=mariadb_authority, mariadb_product_read=enabled, and mariadb_authority=enabled"},
			{Name: "authority-route-write-smoke", Note: "POST migrated write routes, then verify MariaDB row deltas through the authority store"},
			{Name: "authority-post-cutover-replay", Note: "run read replay against the authority backend"},
			{Name: "authority-stop-go-backend", Note: "stop authority backend before rollback proof"},
		}...)
	}
	if routeWriteSmoke {
		r.ChildPlan = append(r.ChildPlan, []childPlanStep{
			{Name: "route-start-go-backend", Note: "start Go backend in AC_STORE_MODE=mariadb_shadow against temp MariaDB for disposable write smoke"},
			{Name: "route-wait-go-ready", Note: "wait for route write smoke backend health"},
			{Name: "route-write-smoke", Note: "POST /complete-turn and /effective-inputs, then verify MariaDB row deltas"},
			{Name: "route-stop-go-backend", Note: "stop route write smoke backend"},
		}...)
	}
	if sessionIsolationSmoke {
		r.ChildPlan = append(r.ChildPlan, []childPlanStep{
			{Name: "session-isolation-start-go-backend", Note: "start disposable Go backend in AC_STORE_MODE=mariadb_authority for RMG-03 session isolation smoke"},
			{Name: "session-isolation-wait-go-ready", Note: "wait for session isolation smoke backend health"},
			{Name: "session-isolation-smoke", Note: "POST /complete-turn for two sessions, verify /sessions and /timeline isolation"},
			{Name: "session-isolation-stop-go-backend", Note: "stop session isolation smoke backend"},
		}...)
	}
	if backupRestoreDrill {
		r.ChildPlan = append(r.ChildPlan, childPlanStep{
			Name: "backup-restore-drill",
			Note: "clone temp MariaDB source database into a restored database and verify all table row counts",
		})
	}

	if detectedType == "container" {
		r.Status = "blocked"
		r.ProviderStatus = "detected_not_implemented"
		r.Warnings = append(r.Warnings, fmt.Sprintf("provider %q detected but container bootstrap not yet implemented", detected))
		return r
	}

	// Direct provider execution flow.
	switch detectedType {
	case "explicit_direct":
		r.ProviderStatus = "detected_explicit_direct"
	case "bundled_direct":
		r.ProviderStatus = "detected_bundled_direct"
	default:
		r.ProviderStatus = "detected_direct"
	}
	cfg := directProviderConfig{
		ProviderName:          detected,
		ProviderPath:          detectedPath,
		DataDir:               tempDataDir,
		Port:                  13306,
		SessionID:             safeID,
		SQLiteDB:              sqliteDB,
		ExportDir:             exportDir,
		KeepTemp:              keepTemp,
		GoHTTPPort:            28180,
		PythonBaseURL:         pythonBaseURL,
		PythonFallbackSrc:     pythonFallbackSrc,
		PythonFallbackPort:    pythonFallbackPort,
		ProductReadProof:      productReadProof,
		RouteWriteSmoke:       routeWriteSmoke,
		SessionIsolationSmoke: sessionIsolationSmoke,
		BackupRestore:         backupRestoreDrill,
		AuthorityCutover:      authorityCutoverReplay,
		DefaultSwitch:         defaultSwitchRehearsal,
		DefaultSwitchActual:   defaultSwitchActual,
		GoBinPath:             goBinPath,
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	defer cancel()
	steps, runErr := defaultDirectRunner.run(ctx, cfg)
	r.ExecutedSteps = steps
	r.RollbackProof = summarizeRollbackProof(steps, productReadProof)
	r.RouteWriteSmoke = summarizeRouteWriteSmoke(steps, routeWriteSmoke)
	r.SessionIsolationSmoke = summarizeSessionIsolationSmoke(steps, sessionIsolationSmoke)
	r.BackupRestore = summarizeBackupRestore(steps, backupRestoreDrill)
	r.AuthorityCutover = summarizeAuthorityCutover(steps, authorityCutoverReplay)
	r.DefaultSwitch = summarizeDefaultSwitchRehearsal(steps, defaultSwitchRehearsal)
	r.DefaultRuntime = summarizeDefaultRuntimeSwitch(steps, defaultSwitchActual)
	if runErr != nil {
		var degr *degradedError
		if errors.As(runErr, &degr) {
			r.Status = "degraded"
			r.Errors = append(r.Errors, runErr.Error())
			r.Warnings = append(r.Warnings, "temp MariaDB import/compare succeeded but shadow-value-report could not complete")
		} else {
			r.Status = "failed"
			r.Errors = append(r.Errors, runErr.Error())
		}
		return r
	}
	r.Status = "ok"
	return r
}

func deriveSourceMode(sqliteDB, exportDir string) string {
	hasSQLite := strings.TrimSpace(sqliteDB) != ""
	hasExport := strings.TrimSpace(exportDir) != ""
	switch {
	case hasSQLite && hasExport:
		return "ambiguous"
	case hasSQLite:
		return "sqlite-db"
	case hasExport:
		return "export-dir"
	default:
		return "none"
	}
}

func summarizeVectorRuntime() map[string]any {
	endpoint := strings.TrimSpace(os.Getenv("AC_CHROMA_ENDPOINT"))
	collection := strings.TrimSpace(os.Getenv("AC_CHROMA_COLLECTION"))
	if collection == "" {
		collection = "archive_center_vectors"
	}
	apiPath := strings.TrimSpace(os.Getenv("AC_CHROMA_API_PATH"))
	if apiPath == "" {
		apiPath = "/api/v1"
	}
	return map[string]any{
		"accelerator":                             "chromadb",
		"chromadb_required":                       true,
		"chromadb_endpoint_configured":            endpoint != "",
		"chromadb_endpoint_host":                  routeSmokeEndpointHost(endpoint),
		"chromadb_collection":                     collection,
		"chromadb_api_path":                       apiPath,
		"live_cutover_requires_chromadb_endpoint": true,
		"milvus_required":                         false,
		"milvus_default":                          false,
		"milvus_optional_experimental":            true,
	}
}

func summarizeRollbackProof(steps []executedStep, requested bool) map[string]any {
	out := map[string]any{
		"requested":   requested,
		"rolled_back": false,
	}
	if !requested {
		out["status"] = "not_requested"
		return out
	}
	for _, step := range steps {
		if step.Name == "rollback-ready-check" {
			out["status"] = step.Status
			out["rolled_back"] = step.Status == "ok"
			out["note"] = step.Note
			if step.Error != "" {
				out["error"] = step.Error
			}
			return out
		}
	}
	out["status"] = "missing"
	return out
}

func summarizeRouteWriteSmoke(steps []executedStep, requested bool) map[string]any {
	out := map[string]any{
		"requested": requested,
	}
	if !requested {
		out["status"] = "not_requested"
		return out
	}
	for _, step := range steps {
		if step.Name != "route-write-smoke" {
			continue
		}
		if step.Details != nil {
			for k, v := range step.Details {
				out[k] = v
			}
		}
		out["status"] = step.Status
		if step.Error != "" {
			out["error"] = step.Error
		}
		return out
	}
	out["status"] = "missing"
	return out
}

func summarizeSessionIsolationSmoke(steps []executedStep, requested bool) map[string]any {
	out := map[string]any{
		"requested": requested,
	}
	if !requested {
		out["status"] = "not_requested"
		return out
	}
	for _, step := range steps {
		if step.Name != "session-isolation-smoke" {
			continue
		}
		if step.Details != nil {
			for k, v := range step.Details {
				out[k] = v
			}
		}
		out["status"] = step.Status
		if step.Error != "" {
			out["error"] = step.Error
		}
		return out
	}
	out["status"] = "missing"
	return out
}
func summarizeBackupRestore(steps []executedStep, requested bool) map[string]any {
	out := map[string]any{
		"requested": requested,
	}
	if !requested {
		out["status"] = "not_requested"
		return out
	}
	for _, step := range steps {
		if step.Name != "backup-restore-drill" {
			continue
		}
		if step.Details != nil {
			for k, v := range step.Details {
				out[k] = v
			}
		}
		out["status"] = step.Status
		if step.Error != "" {
			out["error"] = step.Error
		}
		return out
	}
	out["status"] = "missing"
	return out
}

func summarizeAuthorityCutover(steps []executedStep, requested bool) map[string]any {
	out := map[string]any{
		"requested": requested,
	}
	if !requested {
		out["status"] = "not_requested"
		return out
	}
	var summary map[string]any
	readyOK := false
	routeOK := false
	replayOK := false
	rollbackOK := false
	fallbackOK := false
	for _, step := range steps {
		switch step.Name {
		case "authority-cutover-summary":
			summary = map[string]any{"status": step.Status}
			if step.Details != nil {
				for k, v := range step.Details {
					summary[k] = v
				}
			}
			if step.Error != "" {
				summary["error"] = step.Error
			}
		case "authority-ready-check":
			readyOK = step.Status == "ok"
		case "authority-route-write-smoke":
			routeOK = step.Status == "ok"
			if step.Details != nil {
				out["route_write_smoke"] = step.Details
			}
		case "authority-post-cutover-replay":
			replayOK = step.Status == "ok"
		case "rollback-ready-check":
			rollbackOK = step.Status == "ok"
		case "python-fallback-replay":
			fallbackOK = step.Status == "ok"
		}
	}
	if summary != nil {
		for k, v := range summary {
			out[k] = v
		}
	}
	out["store_mode"] = "mariadb_authority"
	out["authority_switch"] = true
	out["persistent_switch"] = false
	out["go_default_switch"] = false
	out["python_runtime_retired"] = false
	out["ready_check"] = readyOK
	out["route_write_smoke_ok"] = routeOK
	out["post_cutover_replay"] = replayOK
	out["rollback_available"] = rollbackOK
	out["fallback_available"] = fallbackOK
	if readyOK && routeOK && replayOK && rollbackOK {
		out["status"] = "ok"
		return out
	}
	out["status"] = "missing"
	if summary != nil || readyOK || routeOK || replayOK || rollbackOK || fallbackOK {
		out["status"] = "failed"
	}
	return out
}

func summarizeDefaultSwitchRehearsal(steps []executedStep, requested bool) map[string]any {
	out := map[string]any{
		"requested": requested,
	}
	if !requested {
		out["status"] = "not_requested"
		return out
	}
	var goCandidate map[string]any
	var pythonFallback map[string]any
	for _, step := range steps {
		switch step.Name {
		case "go-default-candidate-probe":
			goCandidate = map[string]any{
				"status": step.Status,
			}
			if step.Details != nil {
				for k, v := range step.Details {
					goCandidate[k] = v
				}
			}
			if step.Error != "" {
				goCandidate["error"] = step.Error
			}
		case "python-fallback-replay":
			pythonFallback = map[string]any{
				"status": step.Status,
			}
			if step.Details != nil {
				for k, v := range step.Details {
					pythonFallback[k] = v
				}
			}
			if step.Error != "" {
				pythonFallback["error"] = step.Error
			}
		}
	}
	out["go_candidate"] = goCandidate
	out["python_fallback"] = pythonFallback
	out["selected_runtime"] = "go_rehearsal"
	out["fallback_runtime"] = "python_fallback"
	out["authority_switch"] = false
	out["go_default_switch"] = false
	out["python_runtime_retired"] = false
	ok := statusOKMap(goCandidate) && statusOKMap(pythonFallback)
	if ok {
		out["status"] = "ok"
		out["fallback_available"] = true
		return out
	}
	out["status"] = "missing"
	out["fallback_available"] = false
	if goCandidate != nil || pythonFallback != nil {
		out["status"] = "failed"
	}
	return out
}

func summarizeDefaultRuntimeSwitch(steps []executedStep, requested bool) map[string]any {
	out := map[string]any{
		"requested": requested,
	}
	if !requested {
		out["status"] = "not_requested"
		return out
	}
	var gate map[string]any
	var pythonFallback map[string]any
	shadowReplayOK := false
	rollbackOK := false
	for _, step := range steps {
		switch step.Name {
		case "go-default-actual-switch-gate":
			gate = map[string]any{
				"status": step.Status,
			}
			if step.Details != nil {
				for k, v := range step.Details {
					gate[k] = v
				}
			}
			if step.Error != "" {
				gate["error"] = step.Error
			}
		case "python-fallback-replay":
			pythonFallback = map[string]any{
				"status": step.Status,
			}
			if step.Details != nil {
				for k, v := range step.Details {
					pythonFallback[k] = v
				}
			}
			if step.Error != "" {
				pythonFallback["error"] = step.Error
			}
		case "shadow-value-report":
			shadowReplayOK = step.Status == "ok"
		case "rollback-ready-check":
			rollbackOK = step.Status == "ok"
		}
	}
	out["go_gate"] = gate
	out["python_fallback"] = pythonFallback
	out["selected_runtime"] = "go"
	out["fallback_runtime"] = "python_fallback"
	out["switch_scope"] = "managed_disposable_actual"
	out["authority_switch"] = false
	out["go_default_switch"] = true
	out["persistent_switch"] = false
	out["python_runtime_retired"] = false
	out["post_switch_replay"] = shadowReplayOK
	out["rollback_available"] = rollbackOK
	out["fallback_available"] = statusOKMap(pythonFallback)
	if statusOKMap(gate) && statusOKMap(pythonFallback) && shadowReplayOK && rollbackOK {
		out["status"] = "ok"
		return out
	}
	out["status"] = "missing"
	if gate != nil || pythonFallback != nil || shadowReplayOK || rollbackOK {
		out["status"] = "failed"
	}
	return out
}

func statusOKMap(m map[string]any) bool {
	if m == nil {
		return false
	}
	status, _ := m["status"].(string)
	return status == "ok"
}

func buildPassword(sessionID string) string {
	return safeSessionID(sessionID) + "-pass"
}

func buildInternalDSN(dataDir string, port int, sessionID string) string {
	user := "ac_root"
	password := buildPassword(sessionID)
	dbName := "archive_center_temp"
	return fmt.Sprintf("%s:%s@tcp(127.0.0.1:%d)/%s?parseTime=true&timeout=3s&readTimeout=3s&writeTimeout=3s", user, password, port, dbName)
}

func redactDSN(dsn string) string {
	atIdx := strings.Index(dsn, "@")
	if atIdx == -1 {
		return dsn
	}
	prefix := dsn[:atIdx]
	suffix := dsn[atIdx:]
	colonIdx := strings.Index(prefix, ":")
	if colonIdx == -1 {
		return dsn
	}
	return prefix[:colonIdx+1] + "***" + suffix
}

func quoteStringLiteral(s string) string {
	return "'" + strings.ReplaceAll(s, "'", "''") + "'"
}

func escapeIdentifier(s string) string {
	return "`" + strings.ReplaceAll(s, "`", "``") + "`"
}

func safeSessionID(sessionID string) string {
	trimmed := strings.TrimSpace(sessionID)
	if trimmed == "" {
		return "session"
	}
	var b strings.Builder
	for _, r := range trimmed {
		switch {
		case r >= 'a' && r <= 'z':
			b.WriteRune(r)
		case r >= 'A' && r <= 'Z':
			b.WriteRune(r)
		case r >= '0' && r <= '9':
			b.WriteRune(r)
		case r == '-' || r == '_':
			b.WriteRune(r)
		default:
			b.WriteByte('_')
		}
		if b.Len() >= 80 {
			break
		}
	}
	out := strings.Trim(b.String(), "-_")
	if out == "" {
		return "session"
	}
	return out
}
