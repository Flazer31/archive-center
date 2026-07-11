// managed-mariadb-e2e is a safe managed runner that detects MariaDB providers,
// emits clear JSON when unavailable, and defines the contract for temp MariaDB E2E
// without switching authority.
//
// It does not accept a user-prepared DSN; 2.0 manages DB creation/bootstrap.
package main

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
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
