package main

import (
	"context"
	"flag"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

const defaultFixtureExportDirName = "sqlite-export-r1-52-2026-05-26-a"

var (
	pythonPort   = flag.Int("python-port", 18093, "Python backend port")
	goPort       = flag.Int("go-port", 28293, "Go backend port")
	srcDir       = flag.String("src-dir", `M:\risulongmemory\Archive Center Beta 0.8(fix)`, "Python 0.8 source directory")
	goServiceDir = flag.String("go-service-dir", `M:\risulongmemory\Archive Center 2.0\go-service`, "Go service directory")
	benchmarkDir = flag.String("benchmark-dir", `M:\risulongmemory\Archive Center 2.0\benchmarks`, "Benchmark output directory")
	fixtureDir   = flag.String("fixture-dir", "", "Go fixture NDJSON export directory; defaults to benchmarks/sqlite-export-r1-52-2026-05-26-a")
	r1Tag        = flag.String("r1-tag", "r1-77-fixture-live", "Report R1 tag")
	maxDiffs     = flag.Int("max-diffs", 80, "Max diffs for shadow-value-report")
	timeout      = flag.Duration("timeout", 2*time.Minute, "Health check timeout per backend")
)

type managedProcess struct {
	cmd     *exec.Cmd
	logFile *os.File
}

func (p *managedProcess) stop() {
	if p == nil || p.cmd == nil {
		return
	}
	if p.cmd.Process != nil {
		pid := strconv.Itoa(p.cmd.Process.Pid)
		if err := exec.Command("taskkill", "/T", "/F", "/PID", pid).Run(); err != nil {
			_ = p.cmd.Process.Kill()
		}
	}
	_ = p.cmd.Wait()
	if p.logFile != nil {
		_ = p.logFile.Close()
	}
	time.Sleep(250 * time.Millisecond)
}

func main() {
	flag.Parse()
	if err := run(); err != nil {
		log.Fatalf("fixture-live-runner: %v", err)
	}
}

func run() error {
	ctx := context.Background()

	tempDir, err := os.MkdirTemp("", "fixture-live-runner-0.8-*")
	if err != nil {
		return fmt.Errorf("create temp dir: %w", err)
	}
	defer func() {
		if err := os.RemoveAll(tempDir); err != nil {
			log.Printf("warning: failed to remove temp dir %s: %v", tempDir, err)
		}
	}()

	resolvedFixtureDir := resolveFixtureDir(*fixtureDir, *benchmarkDir)
	if err := validateFixtureDir(resolvedFixtureDir); err != nil {
		return err
	}

	if err := copy08ToTemp(*srcDir, tempDir); err != nil {
		return fmt.Errorf("copy 0.8 to temp: %w", err)
	}

	pythonBase := fmt.Sprintf("http://127.0.0.1:%d", *pythonPort)
	pythonProc, err := startPythonBackend(tempDir, *pythonPort)
	if err != nil {
		return fmt.Errorf("start python backend: %w", err)
	}
	defer pythonProc.stop()

	goBase := fmt.Sprintf("http://127.0.0.1:%d", *goPort)
	goProc, err := startGoBackend(*goServiceDir, *goPort, resolvedFixtureDir, tempDir, tempDir)
	if err != nil {
		return fmt.Errorf("start go backend: %w", err)
	}
	defer goProc.stop()

	if err := waitForHealthy(ctx, pythonBase, *timeout); err != nil {
		return fmt.Errorf("python backend health: %w", err)
	}
	if err := waitForHealthy(ctx, goBase, *timeout); err != nil {
		return fmt.Errorf("go backend health: %w", err)
	}

	dateStr := time.Now().Format("2006-01-02")
	outPath := filepath.Join(*benchmarkDir, fmt.Sprintf("shadow-value-parity-report-%s-%s.md", dateStr, *r1Tag))
	jsonPath := filepath.Join(*benchmarkDir, fmt.Sprintf("shadow-value-parity-report-%s-%s.json", dateStr, *r1Tag))

	if err := runShadowValueReport(*goServiceDir, pythonBase, goBase, outPath, jsonPath, *maxDiffs); err != nil {
		return fmt.Errorf("shadow-value-report: %w", err)
	}

	log.Printf("Reports generated: %s, %s", outPath, jsonPath)
	return nil
}

func resolveFixtureDir(explicit, benchmarkDir string) string {
	if strings.TrimSpace(explicit) != "" {
		return explicit
	}
	return filepath.Join(benchmarkDir, defaultFixtureExportDirName)
}

func validateFixtureDir(path string) error {
	info, err := os.Stat(path)
	if err != nil {
		return fmt.Errorf("fixture dir %q: %w", path, err)
	}
	if !info.IsDir() {
		return fmt.Errorf("fixture dir %q is not a directory", path)
	}
	return nil
}

func copy08ToTemp(src, dst string) error {
	cmd := exec.Command("robocopy", src, dst, "/E", "/XD", ".venv", "/NP", "/NFL", "/NDL")
	out, err := cmd.CombinedOutput()
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			code := exitErr.ExitCode()
			if code >= 8 {
				return fmt.Errorf("robocopy failed with code %d: %s", code, string(out))
			}
		} else {
			return fmt.Errorf("robocopy failed: %w: %s", err, string(out))
		}
	}
	return nil
}

func makeJunction(dst, src string) error {
	cmd := exec.Command("cmd", "/c", "mklink", "/J", dst, src)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("mklink /J failed: %w: %s", err, string(out))
	}
	return nil
}

func startPythonBackend(tempDir string, port int) (*managedProcess, error) {
	cmd := exec.Command("python", "-m", "uvicorn", "--app-dir", tempDir, "backend.main:app", "--host", "127.0.0.1", "--port", fmt.Sprintf("%d", port))
	cmd.Env = append(os.Environ(), "PYTHONDONTWRITEBYTECODE=1")
	logFile, err := os.Create(filepath.Join(tempDir, "python-backend.log"))
	if err != nil {
		return nil, err
	}
	cmd.Stdout = logFile
	cmd.Stderr = logFile
	cmd.Dir = tempDir
	if err := cmd.Start(); err != nil {
		logFile.Close()
		return nil, err
	}
	return &managedProcess{cmd: cmd, logFile: logFile}, nil
}

func goBackendEnv(port int, fixtureDir string, referenceDir ...string) []string {
	env := append(os.Environ(),
		fmt.Sprintf("AC_BIND_ADDR=127.0.0.1:%d", port),
		"AC_STORE_MODE=fixture_shadow",
		"AC_STORE_FIXTURE_DIR="+fixtureDir,
	)
	if len(referenceDir) > 0 {
		if model := readDotEnvValue(filepath.Join(referenceDir[0], "backend", ".env"), "PROJECT_EMBEDDING_MODEL"); model != "" {
			env = append(env, "PROJECT_EMBEDDING_MODEL="+model)
		}
		env = append(env, "AC_CHROMA_SHADOW_PERSIST_DIR="+filepath.Join(referenceDir[0], ".chroma_shadow"))
	}
	return env
}

func startGoBackend(goServiceDir string, port int, fixtureDir string, logDir string, referenceDir string) (*managedProcess, error) {
	cmd := exec.Command("go", "run", "./cmd/archive-center-go")
	cmd.Dir = goServiceDir
	cmd.Env = goBackendEnv(port, fixtureDir, referenceDir)
	logFile, err := os.Create(filepath.Join(logDir, "go-backend.log"))
	if err != nil {
		return nil, err
	}
	cmd.Stdout = logFile
	cmd.Stderr = logFile
	if err := cmd.Start(); err != nil {
		logFile.Close()
		return nil, err
	}
	return &managedProcess{cmd: cmd, logFile: logFile}, nil
}

func readDotEnvValue(path, key string) string {
	raw, err := os.ReadFile(path)
	if err != nil {
		return ""
	}
	prefix := key + "="
	for _, line := range strings.Split(string(raw), "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") || !strings.HasPrefix(line, prefix) {
			continue
		}
		value := strings.TrimSpace(strings.TrimPrefix(line, prefix))
		value = strings.Trim(value, `"'`)
		return value
	}
	return ""
}

func waitForHealthy(ctx context.Context, baseURL string, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	client := &http.Client{Timeout: 5 * time.Second}
	for time.Now().Before(deadline) {
		req, err := http.NewRequestWithContext(ctx, "GET", baseURL+"/health", nil)
		if err != nil {
			return err
		}
		resp, err := client.Do(req)
		if err == nil {
			_, _ = io.Copy(io.Discard, resp.Body)
			resp.Body.Close()
			if resp.StatusCode == http.StatusOK {
				return nil
			}
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(500 * time.Millisecond):
		}
	}
	return fmt.Errorf("health check failed after %v", timeout)
}

func runShadowValueReport(goServiceDir, pythonBase, goBase, outPath, jsonPath string, maxDiffs int) error {
	cmd := exec.Command("go", "run", "./cmd/shadow-value-report",
		"-python-base", pythonBase,
		"-go-base", goBase,
		"-out", outPath,
		"-json-out", jsonPath,
		"-max-diffs", fmt.Sprintf("%d", maxDiffs),
	)
	cmd.Dir = goServiceDir
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}
