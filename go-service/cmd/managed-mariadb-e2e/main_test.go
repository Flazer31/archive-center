package main

import (
	"context"
	"errors"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"
)

type fakeLookup struct {
	paths map[string]string
}

func (f *fakeLookup) lookup(name string) (string, bool) {
	if p, ok := f.paths[name]; ok {
		return p, true
	}
	return "", false
}

func writeFakeExecutable(t *testing.T, path string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		t.Fatalf("mkdir executable parent: %v", err)
	}
	if err := os.WriteFile(path, []byte("fake executable"), 0755); err != nil {
		t.Fatalf("write fake executable: %v", err)
	}
}

type fakeDirectRunner struct {
	steps   []executedStep
	err     error
	lastCfg directProviderConfig
}

func (f *fakeDirectRunner) run(ctx context.Context, cfg directProviderConfig) ([]executedStep, error) {
	f.lastCfg = cfg
	return append([]executedStep(nil), f.steps...), f.err
}

func setFakeDirectRunner(t *testing.T, steps []executedStep, err error) *fakeDirectRunner {
	fr := &fakeDirectRunner{steps: steps, err: err}
	old := defaultDirectRunner
	defaultDirectRunner = fr
	t.Cleanup(func() { defaultDirectRunner = old })
	return fr
}

type fakeExecutor struct {
	lookPathFn func(name string) (string, error)
	runFn      func(ctx context.Context, name string, arg ...string) ([]byte, error)
	startFn    func(ctx context.Context, name string, arg ...string) (*exec.Cmd, error)
	killFn     func(cmd *exec.Cmd) error
	calls      []string
}

func (f *fakeExecutor) LookPath(name string) (string, error) {
	f.calls = append(f.calls, "LookPath:"+name)
	if f.lookPathFn != nil {
		return f.lookPathFn(name)
	}
	return "", errors.New("not found")
}

func (f *fakeExecutor) Run(ctx context.Context, name string, arg ...string) ([]byte, error) {
	f.calls = append(f.calls, "Run:"+name)
	if f.runFn != nil {
		return f.runFn(ctx, name, arg...)
	}
	return nil, errors.New("run failed")
}

func (f *fakeExecutor) Start(ctx context.Context, name string, arg ...string) (*exec.Cmd, error) {
	f.calls = append(f.calls, "Start:"+name)
	if f.startFn != nil {
		return f.startFn(ctx, name, arg...)
	}
	return nil, errors.New("start failed")
}

func (f *fakeExecutor) StartWithEnv(ctx context.Context, name string, env []string, arg ...string) (*exec.Cmd, error) {
	f.calls = append(f.calls, "StartWithEnv:"+name)
	if f.startFn != nil {
		return f.startFn(ctx, name, arg...)
	}
	return nil, errors.New("start failed")
}

func (f *fakeExecutor) Kill(cmd *exec.Cmd) error {
	f.calls = append(f.calls, "Kill")
	if f.killFn != nil {
		return f.killFn(cmd)
	}
	return nil
}

func TestRunGuardedNoProviderNeeded(t *testing.T) {
	lookup := &fakeLookup{paths: map[string]string{}}
	r := run("/db.sqlite", "", "", false, false, "sess-1", lookup)

	if r.Status != "guarded" {
		t.Fatalf("status = %q, want guarded", r.Status)
	}
	if r.ProviderStatus != "" {
		t.Fatalf("provider_status = %q, want empty", r.ProviderStatus)
	}
	if len(r.ProvidersChecked) != 0 {
		t.Fatalf("expected 0 providers checked, got %d", len(r.ProvidersChecked))
	}
	if len(r.Warnings) == 0 || !strings.Contains(r.Warnings[0], "execute=false") {
		t.Fatalf("expected execute=false warning, got %v", r.Warnings)
	}
}

func TestRunMissingSource(t *testing.T) {
	lookup := &fakeLookup{paths: map[string]string{}}
	r := run("", "", "", true, false, "sess-1", lookup)

	if r.Status != "failed" {
		t.Fatalf("status = %q, want failed", r.Status)
	}
	if len(r.Errors) == 0 || !strings.Contains(r.Errors[0], "missing source") {
		t.Fatalf("expected missing source error, got %v", r.Errors)
	}
}

func TestRunAmbiguousSource(t *testing.T) {
	lookup := &fakeLookup{paths: map[string]string{}}
	r := run("/db.sqlite", "/export", "", true, false, "sess-1", lookup)

	if r.Status != "failed" {
		t.Fatalf("status = %q, want failed", r.Status)
	}
	if len(r.Errors) == 0 || !strings.Contains(r.Errors[0], "ambiguous source") {
		t.Fatalf("expected ambiguous source error, got %v", r.Errors)
	}
	if r.SourceMode != "ambiguous" {
		t.Fatalf("source_mode = %q, want ambiguous", r.SourceMode)
	}
}

func TestRunNoProvidersAvailable(t *testing.T) {
	lookup := &fakeLookup{paths: map[string]string{}}
	r := run("/db.sqlite", "", "", true, false, "sess-1", lookup)

	if r.Status != "blocked" {
		t.Fatalf("status = %q, want blocked", r.Status)
	}
	if r.ProviderStatus != "missing" {
		t.Fatalf("provider_status = %q, want missing", r.ProviderStatus)
	}
	if len(r.ProvidersChecked) != 5 {
		t.Fatalf("expected 5 providers checked, got %d", len(r.ProvidersChecked))
	}

	expectedNames := []string{"mariadbd", "mysqld", "docker", "podman", "nerdctl"}
	for i, exp := range expectedNames {
		if r.ProvidersChecked[i].Name != exp {
			t.Fatalf("provider %d name = %q, want %q", i, r.ProvidersChecked[i].Name, exp)
		}
		if r.ProvidersChecked[i].Available {
			t.Fatalf("provider %q should not be available", exp)
		}
	}
	if len(r.Errors) == 0 || !strings.Contains(r.Errors[0], "no MariaDB provider available") {
		t.Fatalf("expected no provider error, got %v", r.Errors)
	}
}

func TestRunSafetyFlagsStayFalse(t *testing.T) {
	lookup := &fakeLookup{paths: map[string]string{}}
	r := run("/db.sqlite", "", "", false, false, "sess-1", lookup)

	for _, key := range []string{"authority_switch", "mariadb_product_read_persisted", "mariadb_authority_default_enabled", "live_milvus", "chroma_retired", "go_default_switch"} {
		value, ok := r.SafetyFlags[key]
		if !ok {
			t.Fatalf("missing safety flag %q", key)
		}
		if value {
			t.Fatalf("safety flag %q = true, want false", key)
		}
	}
	if r.SafetyFlags["milvus_required"] {
		t.Fatal("milvus_required should remain false for the 2.0 MariaDB+ChromaDB path")
	}
	if !r.SafetyFlags["chromadb_required"] {
		t.Fatal("chromadb_required should be true for the 2.0 vector accelerator path")
	}
	if r.VectorRuntime["accelerator"] != "chromadb" {
		t.Fatalf("vector accelerator = %v, want chromadb", r.VectorRuntime["accelerator"])
	}
	if r.VectorRuntime["milvus_required"] != false {
		t.Fatalf("vector milvus_required = %v, want false", r.VectorRuntime["milvus_required"])
	}
	if r.VectorRuntime["chromadb_required"] != true {
		t.Fatalf("vector chromadb_required = %v, want true", r.VectorRuntime["chromadb_required"])
	}
}

func TestRunVectorRuntimeReportsConfiguredChromaEndpoint(t *testing.T) {
	t.Setenv("AC_CHROMA_ENDPOINT", "http://127.0.0.1:8000")
	t.Setenv("AC_CHROMA_COLLECTION", "archive_center_test_vectors")
	lookup := &fakeLookup{paths: map[string]string{}}
	r := run("/db.sqlite", "", "", false, false, "sess-1", lookup)

	if !r.SafetyFlags["chromadb_endpoint_configured"] {
		t.Fatal("chromadb_endpoint_configured safety flag should be true")
	}
	if r.VectorRuntime["chromadb_endpoint_configured"] != true {
		t.Fatalf("vector chromadb_endpoint_configured = %v, want true", r.VectorRuntime["chromadb_endpoint_configured"])
	}
	if r.VectorRuntime["chromadb_endpoint_host"] != "127.0.0.1:8000" {
		t.Fatalf("chromadb endpoint host = %v", r.VectorRuntime["chromadb_endpoint_host"])
	}
	if r.VectorRuntime["chromadb_collection"] != "archive_center_test_vectors" {
		t.Fatalf("chromadb collection = %v", r.VectorRuntime["chromadb_collection"])
	}
}

// TestRunFakeProviderDetectedNotImplemented now tests a container provider
// to preserve the original assertion that detected-but-not-implemented stays blocked.
func TestDiscoverBundledIn_FoundMariadbd(t *testing.T) {
	dir := t.TempDir()
	installDir := filepath.Join(dir, "install")
	exeDir := filepath.Join(installDir, "bin")
	binDir := filepath.Join(installDir, "mariadb", "bin")
	if err := os.MkdirAll(exeDir, 0755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.MkdirAll(binDir, 0755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	exeName := "mariadbd"
	if runtime.GOOS == "windows" {
		exeName = "mariadbd.exe"
	}
	writeFakeExecutable(t, filepath.Join(binDir, exeName))

	found, ok := discoverBundledIn("mariadbd", exeDir)
	if !ok {
		t.Fatal("expected bundled provider to be found")
	}
	if !strings.Contains(found, "mariadb") {
		t.Fatalf("expected path to contain mariadb, got %q", found)
	}
}

func TestDiscoverBundledIn_FoundMysqld(t *testing.T) {
	dir := t.TempDir()
	exeDir := filepath.Join(dir, "go-service", "cmd", "managed-mariadb-e2e")
	binDir := filepath.Join(dir, "go-service", "mariadb", "bin")
	if err := os.MkdirAll(exeDir, 0755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.MkdirAll(binDir, 0755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	exeName := "mysqld"
	if runtime.GOOS == "windows" {
		exeName = "mysqld.exe"
	}
	writeFakeExecutable(t, filepath.Join(binDir, exeName))

	found, ok := discoverBundledIn("mysqld", exeDir)
	if !ok {
		t.Fatal("expected bundled provider to be found")
	}
	if !strings.Contains(found, "mariadb") {
		t.Fatalf("expected path to contain mariadb, got %q", found)
	}
}

func TestDiscoverBundledIn_NotFound(t *testing.T) {
	dir := t.TempDir()
	_, ok := discoverBundledIn("mariadbd", dir)
	if ok {
		t.Fatal("expected no bundled provider")
	}
}

func TestBundledLookup_Priority(t *testing.T) {
	dir := t.TempDir()
	installDir := filepath.Join(dir, "install")
	exeDir := filepath.Join(installDir, "bin")
	binDir := filepath.Join(installDir, "mariadb", "bin")
	if err := os.MkdirAll(exeDir, 0755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.MkdirAll(binDir, 0755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	exeName := "mariadbd"
	if runtime.GOOS == "windows" {
		exeName = "mariadbd.exe"
	}
	writeFakeExecutable(t, filepath.Join(binDir, exeName))

	old := osExecutable
	osExecutable = func() (string, error) { return filepath.Join(exeDir, "managed-mariadb-e2e"), nil }
	t.Cleanup(func() { osExecutable = old })

	fallback := &fakeLookup{paths: map[string]string{"mariadbd": "/usr/bin/mariadbd"}}
	bl := bundledLookup{fallback: fallback}

	path, ok, providerType := bl.lookupTyped("mariadbd", "direct")
	if !ok {
		t.Fatal("expected lookup to succeed")
	}
	if providerType != "bundled_direct" {
		t.Fatalf("provider type = %q, want bundled_direct", providerType)
	}
	if !strings.Contains(path, "mariadb") {
		t.Fatalf("expected bundled path, got %q", path)
	}
}

func TestBundledLookup_Fallback(t *testing.T) {
	fallback := &fakeLookup{paths: map[string]string{"mariadbd": "/usr/bin/mariadbd"}}
	bl := bundledLookup{fallback: fallback}

	path, ok := bl.lookup("mariadbd")
	if !ok {
		t.Fatal("expected lookup to succeed")
	}
	if path != "/usr/bin/mariadbd" {
		t.Fatalf("expected fallback path, got %q", path)
	}
}

func TestRun_WithBundledProvider(t *testing.T) {
	dir := t.TempDir()
	installDir := filepath.Join(dir, "install")
	exeDir := filepath.Join(installDir, "bin")
	binDir := filepath.Join(installDir, "mariadb", "bin")
	if err := os.MkdirAll(exeDir, 0755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.MkdirAll(binDir, 0755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	exeName := "mariadbd"
	if runtime.GOOS == "windows" {
		exeName = "mariadbd.exe"
	}
	writeFakeExecutable(t, filepath.Join(binDir, exeName))

	old := osExecutable
	osExecutable = func() (string, error) { return filepath.Join(exeDir, "managed-mariadb-e2e"), nil }
	t.Cleanup(func() { osExecutable = old })

	fr := setFakeDirectRunner(t, nil, nil)
	r := run("/db.sqlite", "", "", true, false, "sess-1", bundledLookup{fallback: &fakeLookup{}})

	if r.Status != "ok" {
		t.Fatalf("status = %q, want ok", r.Status)
	}
	if fr.lastCfg.ProviderName != "mariadbd" {
		t.Fatalf("provider name = %q, want mariadbd", fr.lastCfg.ProviderName)
	}
	if r.ProviderStatus != "detected_bundled_direct" {
		t.Fatalf("provider_status = %q, want detected_bundled_direct", r.ProviderStatus)
	}
	if len(r.ProvidersChecked) == 0 || r.ProvidersChecked[0].Type != "bundled_direct" {
		t.Fatalf("expected first provider type bundled_direct, got %#v", r.ProvidersChecked)
	}
	if !strings.Contains(fr.lastCfg.ProviderPath, "mariadb") {
		t.Fatalf("expected bundled path, got %q", fr.lastCfg.ProviderPath)
	}
}

func TestRun_BlockedWithBundledLookupMissing(t *testing.T) {
	bl := bundledLookup{fallback: &fakeLookup{paths: map[string]string{}}}
	r := run("/db.sqlite", "", "", true, false, "sess-1", bl)

	if r.Status != "blocked" {
		t.Fatalf("status = %q, want blocked", r.Status)
	}
	if r.ProviderStatus != "missing" {
		t.Fatalf("provider_status = %q, want missing", r.ProviderStatus)
	}
	if len(r.Errors) == 0 || !strings.Contains(r.Errors[0], "no MariaDB provider available") {
		t.Fatalf("expected no provider error, got %v", r.Errors)
	}
}

func TestRunFakeProviderDetectedNotImplemented(t *testing.T) {
	lookup := &fakeLookup{paths: map[string]string{"docker": "/usr/bin/docker"}}
	r := run("/db.sqlite", "", "", true, false, "sess-1", lookup)

	if r.Status != "blocked" {
		t.Fatalf("status = %q, want blocked", r.Status)
	}
	if r.ProviderStatus != "detected_not_implemented" {
		t.Fatalf("provider_status = %q, want detected_not_implemented", r.ProviderStatus)
	}
	if r.TempPlan.DataDir == "" {
		t.Fatal("expected temp data dir in temp_plan")
	}
	if r.TempPlan.Port != 13306 {
		t.Fatalf("port = %d, want 13306", r.TempPlan.Port)
	}
	if !strings.Contains(r.TempPlan.DSNRedacted, "***") {
		t.Fatalf("expected redacted DSN to contain ***, got %q", r.TempPlan.DSNRedacted)
	}
	if len(r.Warnings) == 0 || !strings.Contains(r.Warnings[0], "docker") {
		t.Fatalf("expected provider not implemented warning, got %v", r.Warnings)
	}
}

func TestRunChildPlanIncludesSQLiteExport(t *testing.T) {
	setFakeDirectRunner(t, []executedStep{{Name: "ok", Status: "ok"}}, nil)
	lookup := &fakeLookup{paths: map[string]string{"mariadbd": "/usr/bin/mariadbd"}}
	r := run("/db.sqlite", "", "", true, false, "sess-1", lookup)

	found := false
	for _, step := range r.ChildPlan {
		if step.Name == "sqlite-export" {
			found = true
			if !strings.Contains(step.Note, "-all") {
				t.Fatalf("expected sqlite-export note to contain -all, got %q", step.Note)
			}
		}
	}
	if !found {
		t.Fatal("expected child_plan to contain sqlite-export step")
	}
}

func TestRunDirectProviderCleanupOnFailure(t *testing.T) {
	steps := []executedStep{
		{Name: "create-datadir", Status: "ok"},
		{Name: "port-check", Status: "ok"},
		{Name: "init-datadir", Status: "failed", ExitCode: 1, Error: "init failed"},
		{Name: "cleanup", Status: "ok"},
	}
	setFakeDirectRunner(t, steps, errors.New("init failed"))
	lookup := &fakeLookup{paths: map[string]string{"mariadbd": "/usr/bin/mariadbd"}}
	r := run("/db.sqlite", "", "", true, false, "sess-fail", lookup)

	if r.Status != "failed" {
		t.Fatalf("status = %q, want failed", r.Status)
	}
	foundCleanup := false
	for _, s := range r.ExecutedSteps {
		if s.Name == "cleanup" {
			foundCleanup = true
		}
	}
	if !foundCleanup {
		t.Fatal("expected cleanup step in executed steps")
	}
}

func TestRunDirectProviderSQLiteExportFirst(t *testing.T) {
	steps := []executedStep{
		{Name: "create-datadir", Status: "ok"},
		{Name: "port-check", Status: "ok"},
		{Name: "init-datadir", Status: "ok"},
		{Name: "start-server", Status: "ok"},
		{Name: "wait-ready", Status: "ok"},
		{Name: "bootstrap-database", Status: "ok"},
		{Name: "sqlite-export", Status: "ok"},
		{Name: "mariadb-schema", Status: "ok"},
	}
	setFakeDirectRunner(t, steps, nil)
	lookup := &fakeLookup{paths: map[string]string{"mysqld": "/usr/bin/mysqld"}}
	r := run("/db.sqlite", "", "", true, false, "sess-sqlite", lookup)

	if r.Status != "ok" {
		t.Fatalf("status = %q, want ok", r.Status)
	}
	found := false
	for _, s := range r.ExecutedSteps {
		if s.Name == "sqlite-export" {
			found = true
		}
	}
	if !found {
		t.Fatal("expected sqlite-export in executed steps")
	}
}

func TestRunDirectProviderExportDirSkipsSQLiteExport(t *testing.T) {
	steps := []executedStep{
		{Name: "create-datadir", Status: "ok"},
		{Name: "port-check", Status: "ok"},
		{Name: "init-datadir", Status: "ok"},
		{Name: "start-server", Status: "ok"},
		{Name: "wait-ready", Status: "ok"},
		{Name: "bootstrap-database", Status: "ok"},
		{Name: "mariadb-schema", Status: "ok"},
		{Name: "mariadb-import", Status: "ok"},
		{Name: "mariadb-compare", Status: "ok"},
	}
	setFakeDirectRunner(t, steps, nil)
	lookup := &fakeLookup{paths: map[string]string{"mysqld": "/usr/bin/mysqld"}}
	r := run("", "/export", "", true, false, "sess-export", lookup)

	if r.Status != "ok" {
		t.Fatalf("status = %q, want ok", r.Status)
	}
	for _, s := range r.ExecutedSteps {
		if s.Name == "sqlite-export" {
			t.Fatal("expected no sqlite-export step for export-dir source")
		}
	}
}

func TestRunContainerProviderBlocked(t *testing.T) {
	lookup := &fakeLookup{paths: map[string]string{"docker": "/usr/bin/docker"}}
	r := run("/db.sqlite", "", "", true, false, "sess-container", lookup)

	if r.Status != "blocked" {
		t.Fatalf("status = %q, want blocked", r.Status)
	}
	if r.ProviderStatus != "detected_not_implemented" {
		t.Fatalf("provider_status = %q, want detected_not_implemented", r.ProviderStatus)
	}
}

func TestRunDirectProviderSequenceIncludesGoBackendSteps(t *testing.T) {
	steps := []executedStep{
		{Name: "create-datadir", Status: "ok"},
		{Name: "port-check", Status: "ok"},
		{Name: "init-datadir", Status: "ok"},
		{Name: "start-server", Status: "ok"},
		{Name: "wait-ready", Status: "ok"},
		{Name: "bootstrap-database", Status: "ok"},
		{Name: "mariadb-schema", Status: "ok"},
		{Name: "mariadb-import", Status: "ok"},
		{Name: "mariadb-compare", Status: "ok"},
		{Name: "start-go-backend", Status: "ok"},
		{Name: "wait-go-ready", Status: "ok"},
		{Name: "shadow-value-report", Status: "ok"},
		{Name: "stop-go-backend", Status: "ok"},
		{Name: "stop-server", Status: "ok"},
		{Name: "cleanup", Status: "ok"},
	}
	setFakeDirectRunner(t, steps, nil)
	lookup := &fakeLookup{paths: map[string]string{"mariadbd": "/usr/bin/mariadbd"}}
	r := run("/db.sqlite", "", "", true, false, "sess-seq", lookup)

	if r.Status != "ok" {
		t.Fatalf("status = %q, want ok", r.Status)
	}
	expected := []string{"start-go-backend", "wait-go-ready", "shadow-value-report", "stop-go-backend"}
	for _, name := range expected {
		found := false
		for _, s := range r.ExecutedSteps {
			if s.Name == name {
				found = true
				break
			}
		}
		if !found {
			t.Fatalf("expected executed step %q", name)
		}
	}
	for _, name := range expected {
		found := false
		for _, s := range r.ChildPlan {
			if s.Name == name {
				found = true
				break
			}
		}
		if !found {
			t.Fatalf("expected child plan step %q", name)
		}
	}
}

func TestRunDirectProviderValueReportFailureDegraded(t *testing.T) {
	steps := []executedStep{
		{Name: "create-datadir", Status: "ok"},
		{Name: "port-check", Status: "ok"},
		{Name: "init-datadir", Status: "ok"},
		{Name: "start-server", Status: "ok"},
		{Name: "wait-ready", Status: "ok"},
		{Name: "bootstrap-database", Status: "ok"},
		{Name: "mariadb-schema", Status: "ok"},
		{Name: "mariadb-import", Status: "ok"},
		{Name: "mariadb-compare", Status: "ok"},
		{Name: "start-go-backend", Status: "ok"},
		{Name: "wait-go-ready", Status: "ok"},
		{Name: "shadow-value-report", Status: "failed", Error: "python backend unreachable"},
		{Name: "stop-go-backend", Status: "ok"},
		{Name: "stop-server", Status: "ok"},
		{Name: "cleanup", Status: "ok"},
	}
	setFakeDirectRunner(t, steps, &degradedError{msg: "shadow-value-report failed: python backend unreachable"})
	lookup := &fakeLookup{paths: map[string]string{"mariadbd": "/usr/bin/mariadbd"}}
	r := run("/db.sqlite", "", "", true, false, "sess-degraded", lookup)

	if r.Status != "degraded" {
		t.Fatalf("status = %q, want degraded", r.Status)
	}
	if len(r.Errors) == 0 || !strings.Contains(r.Errors[0], "python backend unreachable") {
		t.Fatalf("expected degraded error, got %v", r.Errors)
	}
	foundWarning := false
	for _, w := range r.Warnings {
		if strings.Contains(w, "shadow-value-report could not complete") {
			foundWarning = true
			break
		}
	}
	if !foundWarning {
		t.Fatalf("expected degraded warning, got %v", r.Warnings)
	}
}

func TestRunDirectProviderGoBackendEnvRedaction(t *testing.T) {
	fr := setFakeDirectRunner(t, []executedStep{{Name: "ok", Status: "ok"}}, nil)
	lookup := &fakeLookup{paths: map[string]string{"mariadbd": "/usr/bin/mariadbd"}}
	r := run("/db.sqlite", "", "http://127.0.0.1:9000", true, false, "sess-env", lookup)

	if r.Status != "ok" {
		t.Fatalf("status = %q, want ok", r.Status)
	}
	if fr.lastCfg.PythonBaseURL != "http://127.0.0.1:9000" {
		t.Fatalf("python_base_url = %q, want http://127.0.0.1:9000", fr.lastCfg.PythonBaseURL)
	}
	if fr.lastCfg.GoHTTPPort != 28180 {
		t.Fatalf("go_http_port = %d, want 28180", fr.lastCfg.GoHTTPPort)
	}
}

func TestRunWithProductReadProofCarriesFlagAndRollbackSummary(t *testing.T) {
	steps := []executedStep{
		{Name: "create-datadir", Status: "ok"},
		{Name: "mariadb-schema", Status: "ok"},
		{Name: "mariadb-import", Status: "ok"},
		{Name: "mariadb-compare", Status: "ok"},
		{Name: "start-go-backend", Status: "ok"},
		{Name: "wait-go-ready", Status: "ok"},
		{Name: "shadow-value-report", Status: "ok"},
		{Name: "rollback-stop-product-go-backend", Status: "ok"},
		{Name: "rollback-start-go-backend", Status: "ok"},
		{Name: "rollback-wait-go-ready", Status: "ok"},
		{Name: "rollback-ready-check", Status: "ok", Note: "store_mode=noop mariadb_product_read=disabled mariadb_authority=disabled"},
		{Name: "rollback-stop-go-backend", Status: "ok"},
	}
	fr := setFakeDirectRunner(t, steps, nil)
	lookup := &fakeLookup{paths: map[string]string{"mariadbd": "/usr/bin/mariadbd"}}
	r := runWithOptions("/db.sqlite", "", "http://127.0.0.1:9000", true, false, "sess-product", true, false, false, false, false, false, false, "", 0, "/bin/archive-center-go", "", lookup)

	if r.Status != "ok" {
		t.Fatalf("status = %q, want ok", r.Status)
	}
	if !r.ProductReadProof {
		t.Fatal("expected product_read_proof=true")
	}
	if !fr.lastCfg.ProductReadProof {
		t.Fatal("expected direct runner to receive ProductReadProof=true")
	}
	if fr.lastCfg.GoBinPath != "/bin/archive-center-go" {
		t.Fatalf("go bin path = %q, want /bin/archive-center-go", fr.lastCfg.GoBinPath)
	}
	if got, _ := r.RollbackProof["rolled_back"].(bool); !got {
		t.Fatalf("rollback proof = %#v, want rolled_back=true", r.RollbackProof)
	}
	foundRollbackPlan := false
	for _, step := range r.ChildPlan {
		if step.Name == "rollback-ready-check" {
			foundRollbackPlan = true
			break
		}
	}
	if !foundRollbackPlan {
		t.Fatal("expected rollback-ready-check in child plan")
	}
	if r.SafetyFlags["authority_switch"] {
		t.Fatal("authority_switch safety flag must remain false")
	}
}

func TestRunWithRouteWriteSmokeCarriesFlagAndSummary(t *testing.T) {
	steps := []executedStep{
		{Name: "create-datadir", Status: "ok"},
		{Name: "mariadb-schema", Status: "ok"},
		{Name: "mariadb-import", Status: "ok"},
		{Name: "mariadb-compare", Status: "ok"},
		{Name: "route-start-go-backend", Status: "ok"},
		{Name: "route-wait-go-ready", Status: "ok"},
		{Name: "route-write-smoke", Status: "ok", Details: map[string]any{
			"requested":         true,
			"status":            "ok",
			"store_mode":        "mariadb_shadow",
			"delta_counts":      map[string]any{"chat_logs": 2, "effective_input_logs": 2, "audit_logs": 2, "critic_feedback": 1},
			"authority_switch":  false,
			"go_default_switch": false,
		}},
		{Name: "route-stop-go-backend", Status: "ok"},
		{Name: "start-go-backend", Status: "ok"},
		{Name: "wait-go-ready", Status: "ok"},
		{Name: "shadow-value-report", Status: "ok"},
	}
	fr := setFakeDirectRunner(t, steps, nil)
	lookup := &fakeLookup{paths: map[string]string{"mariadbd": "/usr/bin/mariadbd"}}
	r := runWithOptions("/db.sqlite", "", "http://127.0.0.1:9000", true, false, "sess-route", false, true, false, false, false, false, false, "", 0, "/bin/archive-center-go", "", lookup)

	if r.Status != "ok" {
		t.Fatalf("status = %q, want ok", r.Status)
	}
	if !fr.lastCfg.RouteWriteSmoke {
		t.Fatal("expected direct runner to receive RouteWriteSmoke=true")
	}
	if r.RouteWriteSmoke["status"] != "ok" {
		t.Fatalf("route_write_smoke = %#v, want status ok", r.RouteWriteSmoke)
	}
	if r.RouteWriteSmoke["store_mode"] != "mariadb_shadow" {
		t.Fatalf("route_write_smoke store_mode = %#v", r.RouteWriteSmoke["store_mode"])
	}
	foundRoutePlan := false
	for _, step := range r.ChildPlan {
		if step.Name == "route-write-smoke" {
			foundRoutePlan = true
			break
		}
	}
	if !foundRoutePlan {
		t.Fatal("expected route-write-smoke in child plan")
	}
	if r.SafetyFlags["authority_switch"] || r.SafetyFlags["go_default_switch"] {
		t.Fatal("route write smoke must not enable authority/default switch")
	}
}

func TestRunWithSessionIsolationSmokeCarriesFlagAndSummary(t *testing.T) {
	steps := []executedStep{
		{Name: "create-datadir", Status: "ok"},
		{Name: "mariadb-schema", Status: "ok"},
		{Name: "mariadb-import", Status: "ok"},
		{Name: "mariadb-compare", Status: "ok"},
		{Name: "session-isolation-start-go-backend", Status: "ok"},
		{Name: "session-isolation-wait-go-ready", Status: "ok"},
		{Name: "session-isolation-smoke", Status: "ok", Details: map[string]any{
			"status":   "ok",
			"sessions": []string{"sess-rmg03-standalone-a", "sess-rmg03-standalone-b"},
		}},
		{Name: "session-isolation-stop-go-backend", Status: "ok"},
		{Name: "start-go-backend", Status: "ok"},
		{Name: "wait-go-ready", Status: "ok"},
		{Name: "shadow-value-report", Status: "ok"},
	}
	fr := setFakeDirectRunner(t, steps, nil)
	lookup := &fakeLookup{paths: map[string]string{"mariadbd": "/usr/bin/mariadbd"}}
	r := runWithOptions("/db.sqlite", "", "http://127.0.0.1:9000", true, false, "sess", false, false, false, false, false, false, true, "", 0, "/bin/archive-center-go", "", lookup)

	if r.Status != "ok" {
		t.Fatalf("status = %q, want ok", r.Status)
	}
	if !fr.lastCfg.SessionIsolationSmoke {
		t.Fatal("expected direct runner to receive SessionIsolationSmoke=true")
	}
	if r.SessionIsolationSmoke["status"] != "ok" {
		t.Fatalf("session_isolation_smoke = %#v, want status ok", r.SessionIsolationSmoke)
	}
	foundIsoPlan := false
	for _, step := range r.ChildPlan {
		if step.Name == "session-isolation-smoke" {
			foundIsoPlan = true
			break
		}
	}
	if !foundIsoPlan {
		t.Fatal("expected session-isolation-smoke in child plan")
	}
}

func TestCheckSessionScopedItemsRejectsForeignRows(t *testing.T) {
	probe := map[string]any{
		"status": "ok",
		"json": map[string]any{
			"total": 2,
			"items": []any{
				map[string]any{"chat_session_id": "sess-a"},
				map[string]any{"chat_session_id": "sess-b"},
			},
		},
	}
	check := checkSessionScopedItems(probe, "sess-a", 2, "total")
	if boolField(check, "ok") {
		t.Fatalf("expected foreign session row to fail isolation check: %#v", check)
	}
	foreign, _ := check["foreign_items"].([]string)
	if len(foreign) != 1 || foreign[0] != "sess-b" {
		t.Fatalf("foreign_items = %#v, want sess-b", check["foreign_items"])
	}
}

func TestStartGoBackendSetsEnvAndRedactsDSN(t *testing.T) {
	fe := &fakeExecutor{
		startFn: func(ctx context.Context, name string, arg ...string) (*exec.Cmd, error) {
			cmd := exec.Command("true")
			return cmd, nil
		},
	}
	ctx := context.Background()
	cmd, step, err := startGoBackend(ctx, fe, goBackendStartConfig{
		BinPath:   "/bin/archive-center-go",
		Port:      28180,
		DSN:       "user:secret@tcp(127.0.0.1:13306)/db?parseTime=true",
		StoreMode: "mariadb_read_shadow",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cmd == nil {
		t.Fatal("expected cmd, got nil")
	}
	if step.Status != "ok" {
		t.Fatalf("status = %q, want ok", step.Status)
	}
	if strings.Contains(step.Command, "secret") || strings.Contains(step.Redacted, "secret") {
		t.Fatalf("command fields must not leak DSN password: command=%q redacted=%q", step.Command, step.Redacted)
	}
	if !strings.Contains(step.Redacted, "AC_MARIADB_DSN=user:***@tcp") {
		t.Fatalf("expected redacted command to contain redacted DSN, got %q", step.Redacted)
	}
	if !strings.Contains(step.Command, "AC_STORE_MODE=mariadb_read_shadow") {
		t.Fatalf("expected command to contain AC_STORE_MODE=mariadb_read_shadow, got %q", step.Command)
	}
	if !strings.Contains(step.Command, "AC_BIND_ADDR=127.0.0.1:28180") {
		t.Fatalf("expected command to contain AC_BIND_ADDR=127.0.0.1:28180, got %q", step.Command)
	}
}

func TestStartGoBackendProductReadEnv(t *testing.T) {
	fe := &fakeExecutor{
		startFn: func(ctx context.Context, name string, arg ...string) (*exec.Cmd, error) {
			cmd := exec.Command("true")
			return cmd, nil
		},
	}
	ctx := context.Background()
	_, step, err := startGoBackend(ctx, fe, goBackendStartConfig{
		BinPath:          "/bin/archive-center-go",
		Port:             28181,
		DSN:              "user:secret@tcp(127.0.0.1:13306)/db?parseTime=true",
		StoreMode:        "mariadb_read_shadow",
		ProductReadProof: true,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(step.Command, "AC_MARIADB_PRODUCT_READ_ENABLED=true") {
		t.Fatalf("expected product read env in command, got %q", step.Command)
	}
	if strings.Contains(step.Command, "secret") || strings.Contains(step.Redacted, "secret") {
		t.Fatalf("command fields must not leak DSN password: command=%q redacted=%q", step.Command, step.Redacted)
	}
}

func TestWaitGoReadySuccess(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	port := ln.Addr().(*net.TCPAddr).Port
	go http.Serve(ln, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer ln.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	step := waitGoReady(ctx, port)
	if step.Status != "ok" {
		t.Fatalf("status = %q, want ok: %s", step.Status, step.Error)
	}
}

func TestWaitGoReadyTimeout(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
	defer cancel()
	step := waitGoReady(ctx, 59999) // unlikely used port
	if step.Status != "failed" {
		t.Fatalf("status = %q, want failed", step.Status)
	}
	if !strings.Contains(step.Error, "not ready") {
		t.Fatalf("expected timeout error, got %q", step.Error)
	}
}

func TestStopGoBackendCallsKill(t *testing.T) {
	fe := &fakeExecutor{
		killFn: func(cmd *exec.Cmd) error {
			return nil
		},
	}
	cmd := exec.Command("true")
	step := stopGoBackend(fe, cmd)
	if step.Status != "ok" {
		t.Fatalf("status = %q, want ok", step.Status)
	}
	found := false
	for _, c := range fe.calls {
		if c == "Kill" {
			found = true
		}
	}
	if !found {
		t.Fatal("expected Kill to be called")
	}
}

func TestQuoteStringLiteral(t *testing.T) {
	cases := []struct {
		input string
		want  string
	}{
		{"hello", "'hello'"},
		{"it''s", "'it''''s'"},
		{"no quotes", "'no quotes'"},
		{"'leading", "'''leading'"},
		{"trailing'", "'trailing'''"},
		{"", "''"},
	}
	for _, tc := range cases {
		got := quoteStringLiteral(tc.input)
		if got != tc.want {
			t.Fatalf("quoteStringLiteral(%q) = %q, want %q", tc.input, got, tc.want)
		}
	}
}

func TestEscapeIdentifier(t *testing.T) {
	cases := []struct {
		input string
		want  string
	}{
		{"archive_center_temp", "`archive_center_temp`"},
		{"ac_root", "`ac_root`"},
		{"`backtick", "```backtick`"},
		{"a`b", "`a``b`"},
		{"", "``"},
	}
	for _, tc := range cases {
		got := escapeIdentifier(tc.input)
		if got != tc.want {
			t.Fatalf("escapeIdentifier(%q) = %q, want %q", tc.input, got, tc.want)
		}
	}
}

func TestSafeSessionID(t *testing.T) {
	cases := []struct {
		input string
		want  string
	}{
		{"sess-99", "sess-99"},
		{" ../evil\\path ", "evil_path"},
		{"??? session", "session"},
		{"___", "session"},
		{"", "session"},
	}
	for _, tc := range cases {
		got := safeSessionID(tc.input)
		if got != tc.want {
			t.Fatalf("safeSessionID(%q) = %q, want %q", tc.input, got, tc.want)
		}
	}
}

func TestCleanupTempStepRemovesOrRetains(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "managed-temp")
	if err := os.MkdirAll(dir, 0750); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	step := cleanupTempStep(dir, false)
	if step.Status != "ok" {
		t.Fatalf("cleanup status = %q, want ok: %s", step.Status, step.Error)
	}
	if _, err := os.Stat(dir); !os.IsNotExist(err) {
		t.Fatalf("expected temp dir to be removed, stat err=%v", err)
	}

	retained := filepath.Join(t.TempDir(), "managed-retained")
	if err := os.MkdirAll(retained, 0750); err != nil {
		t.Fatalf("mkdir retained: %v", err)
	}
	step = cleanupTempStep(retained, true)
	if step.Status != "retained" {
		t.Fatalf("cleanup status = %q, want retained", step.Status)
	}
	if _, err := os.Stat(retained); err != nil {
		t.Fatalf("expected retained temp dir to remain: %v", err)
	}
}

func TestRunExplicitProviderAvailable(t *testing.T) {
	lookup := &fakeLookup{paths: map[string]string{}}
	fakeBin := filepath.Join(t.TempDir(), "mariadbd.exe")
	writeFakeExecutable(t, fakeBin)

	fr := setFakeDirectRunner(t, nil, nil)
	r := runWithOptions("/db.sqlite", "", "http://127.0.0.1:9000", true, false, "sess-explicit", false, false, false, false, false, false, false, "", 0, "", fakeBin, lookup)

	if r.Status != "ok" {
		t.Fatalf("status = %q, want ok: errors=%v", r.Status, r.Errors)
	}
	if r.ProviderStatus != "detected_explicit_direct" {
		t.Fatalf("provider_status = %q, want detected_explicit_direct", r.ProviderStatus)
	}
	if len(r.ProvidersChecked) == 0 {
		t.Fatal("expected at least 1 provider checked")
	}
	found := false
	for _, pi := range r.ProvidersChecked {
		if pi.Type == "explicit_direct" {
			found = true
			if !pi.Available {
				t.Fatalf("explicit provider should be available")
			}
			if pi.Path != fakeBin {
				t.Fatalf("explicit provider path = %q, want %q", pi.Path, fakeBin)
			}
		}
	}
	if !found {
		t.Fatal("expected explicit_direct in providers_checked")
	}
	if fr.lastCfg.ProviderPath != fakeBin {
		t.Fatalf("runner ProviderPath = %q, want %q", fr.lastCfg.ProviderPath, fakeBin)
	}
}

func TestRunExplicitProviderMissing(t *testing.T) {
	lookup := &fakeLookup{paths: map[string]string{}}
	fakeBin := filepath.Join(t.TempDir(), "nonexistent", "mariadbd.exe")

	r := runWithOptions("/db.sqlite", "", "http://127.0.0.1:9000", true, false, "sess-missing", false, false, false, false, false, false, false, "", 0, "", fakeBin, lookup)

	if r.Status != "blocked" {
		t.Fatalf("status = %q, want blocked", r.Status)
	}
	if r.ProviderStatus != "missing_explicit" {
		t.Fatalf("provider_status = %q, want missing_explicit", r.ProviderStatus)
	}
	found := false
	for _, pi := range r.ProvidersChecked {
		if pi.Type == "explicit_direct" {
			found = true
			if pi.Available {
				t.Fatalf("missing explicit provider should not be available")
			}
		}
	}
	if !found {
		t.Fatal("expected explicit_direct in providers_checked")
	}
	if len(r.Errors) == 0 || !strings.Contains(r.Errors[0], "explicit MariaDB provider not found") {
		t.Fatalf("expected explicit missing error, got %v", r.Errors)
	}
}

func TestBuildInitCommandUsesWindowsInstallDBWhenPresent(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("windows-only MariaDB ZIP initialization path")
	}
	dir := t.TempDir()
	binDir := filepath.Join(dir, "bin")
	provider := filepath.Join(binDir, "mariadbd.exe")
	installDB := filepath.Join(binDir, "mariadb-install-db.exe")
	writeFakeExecutable(t, provider)
	writeFakeExecutable(t, installDB)

	cmd, args := buildInitCommand(provider, filepath.Join(dir, "data"))

	if cmd != installDB {
		t.Fatalf("init command = %q, want %q", cmd, installDB)
	}
	got := strings.Join(args, " ")
	if !strings.Contains(got, "--datadir=") || !strings.Contains(got, "--password=") {
		t.Fatalf("init args = %v, want datadir and blank password", args)
	}
}

func TestManagedCommandFallsBackToGoRun(t *testing.T) {
	exec := &fakeExecutor{}

	cmd, args, display := managedCommand(exec, "mariadb-schema", "-dsn", "root:secret@tcp(127.0.0.1:13306)/x", "-execute")

	if cmd != "go" {
		t.Fatalf("cmd = %q, want go", cmd)
	}
	if len(args) < 4 || args[0] != "run" || args[1] != "-buildvcs=false" || args[2] != "./cmd/mariadb-schema" {
		t.Fatalf("args = %v, want go run ./cmd/mariadb-schema", args)
	}
	if !strings.HasPrefix(display, "go run -buildvcs=false ./cmd/mariadb-schema") {
		t.Fatalf("display = %q, want go run display", display)
	}
}

func TestRouteWriteSmokeNotRequested(t *testing.T) {
	fr := setFakeDirectRunner(t, nil, nil)
	lookup := &fakeLookup{paths: map[string]string{"mariadbd": "/usr/bin/mariadbd"}}
	r := runWithOptions("/db.sqlite", "", "http://127.0.0.1:9000", true, false, "sess-no-smoke", false, false, false, false, false, false, false, "", 0, "", "", lookup)

	if r.Status != "ok" {
		t.Fatalf("status = %q, want ok", r.Status)
	}
	if fr.lastCfg.RouteWriteSmoke {
		t.Fatal("expected RouteWriteSmoke=false")
	}
	if r.RouteWriteSmoke["status"] != "not_requested" {
		t.Fatalf("route_write_smoke status = %q, want not_requested", r.RouteWriteSmoke["status"])
	}
	found := false
	for _, step := range r.ChildPlan {
		if step.Name == "route-write-smoke" {
			found = true
			break
		}
	}
	if found {
		t.Fatal("expected no route-write-smoke in child plan when not requested")
	}
}

func TestRouteWriteSmokeFailurePropagated(t *testing.T) {
	steps := []executedStep{
		{Name: "create-datadir", Status: "ok"},
		{Name: "mariadb-schema", Status: "ok"},
		{Name: "mariadb-import", Status: "ok"},
		{Name: "mariadb-compare", Status: "ok"},
		{Name: "route-start-go-backend", Status: "ok"},
		{Name: "route-wait-go-ready", Status: "ok"},
		{Name: "route-write-smoke", Status: "failed", Error: "route write smoke count delta below expectation", Details: map[string]any{
			"requested":         true,
			"status":            "failed",
			"store_mode":        "mariadb_shadow",
			"authority_switch":  false,
			"go_default_switch": false,
		}},
		{Name: "route-stop-go-backend", Status: "ok"},
		{Name: "start-go-backend", Status: "ok"},
		{Name: "wait-go-ready", Status: "ok"},
		{Name: "shadow-value-report", Status: "ok"},
	}
	fr := setFakeDirectRunner(t, steps, errors.New("route write smoke: route write smoke count delta below expectation"))
	lookup := &fakeLookup{paths: map[string]string{"mariadbd": "/usr/bin/mariadbd"}}
	r := runWithOptions("/db.sqlite", "", "http://127.0.0.1:9000", true, false, "sess-fail", false, true, false, false, false, false, false, "", 0, "", "", lookup)

	if r.Status != "failed" {
		t.Fatalf("status = %q, want failed", r.Status)
	}
	if !fr.lastCfg.RouteWriteSmoke {
		t.Fatal("expected RouteWriteSmoke=true")
	}
	if r.RouteWriteSmoke["status"] != "failed" {
		t.Fatalf("route_write_smoke status = %q, want failed", r.RouteWriteSmoke["status"])
	}
	if r.SafetyFlags["authority_switch"] || r.SafetyFlags["go_default_switch"] {
		t.Fatal("route write smoke must not enable authority/default switch even on failure")
	}
}

func TestRunWithBackupRestoreDrillCarriesFlagAndSummary(t *testing.T) {
	steps := []executedStep{
		{Name: "create-datadir", Status: "ok"},
		{Name: "mariadb-schema", Status: "ok"},
		{Name: "mariadb-import", Status: "ok"},
		{Name: "mariadb-compare", Status: "ok"},
		{Name: "backup-restore-drill", Status: "ok", Details: map[string]any{
			"requested":           true,
			"status":              "ok",
			"method":              "managed_sql_clone_restore",
			"tables_checked":      15,
			"source_rows_total":   21,
			"restored_rows_total": 21,
			"row_count_match":     true,
			"authority_switch":    false,
			"go_default_switch":   false,
		}},
		{Name: "start-go-backend", Status: "ok"},
		{Name: "wait-go-ready", Status: "ok"},
		{Name: "shadow-value-report", Status: "ok"},
	}
	fr := setFakeDirectRunner(t, steps, nil)
	lookup := &fakeLookup{paths: map[string]string{"mariadbd": "/usr/bin/mariadbd"}}
	r := runWithOptions("/db.sqlite", "", "http://127.0.0.1:9000", true, false, "sess-backup", false, false, true, false, false, false, false, "", 0, "/bin/archive-center-go", "", lookup)

	if r.Status != "ok" {
		t.Fatalf("status = %q, want ok", r.Status)
	}
	if !fr.lastCfg.BackupRestore {
		t.Fatal("expected direct runner to receive BackupRestore=true")
	}
	if r.BackupRestore["status"] != "ok" {
		t.Fatalf("backup_restore_drill = %#v, want status ok", r.BackupRestore)
	}
	if r.BackupRestore["row_count_match"] != true {
		t.Fatalf("backup_restore_drill row_count_match = %#v", r.BackupRestore["row_count_match"])
	}
	found := false
	for _, step := range r.ChildPlan {
		if step.Name == "backup-restore-drill" {
			found = true
			break
		}
	}
	if !found {
		t.Fatal("expected backup-restore-drill in child plan")
	}
	if r.SafetyFlags["authority_switch"] || r.SafetyFlags["go_default_switch"] {
		t.Fatal("backup restore drill must not enable authority/default switch")
	}
}

func TestRunWithDefaultSwitchRehearsalCarriesFlagAndSummary(t *testing.T) {
	steps := []executedStep{
		{Name: "create-datadir", Status: "ok"},
		{Name: "mariadb-schema", Status: "ok"},
		{Name: "mariadb-import", Status: "ok"},
		{Name: "mariadb-compare", Status: "ok"},
		{Name: "start-go-backend", Status: "ok"},
		{Name: "wait-go-ready", Status: "ok"},
		{Name: "shadow-value-report", Status: "ok"},
		{Name: "go-default-candidate-probe", Status: "ok", Details: map[string]any{
			"requested":              true,
			"role":                   "go_default_candidate",
			"selected_runtime":       "go",
			"candidate_store_mode":   "mariadb_read_shadow",
			"candidate_product_read": true,
			"authority_switch":       false,
			"go_default_switch":      false,
		}},
		{Name: "rollback-stop-product-go-backend", Status: "ok"},
		{Name: "python-fallback-replay", Status: "ok", Details: map[string]any{
			"requested":              true,
			"role":                   "python_fallback",
			"selected_runtime":       "python_fallback",
			"fallback_available":     true,
			"authority_switch":       false,
			"go_default_switch":      false,
			"python_runtime_retired": false,
		}},
		{Name: "rollback-start-go-backend", Status: "ok"},
		{Name: "rollback-wait-go-ready", Status: "ok"},
		{Name: "rollback-ready-check", Status: "ok", Note: "store_mode=noop mariadb_product_read=disabled mariadb_authority=disabled"},
		{Name: "rollback-stop-go-backend", Status: "ok"},
	}
	fr := setFakeDirectRunner(t, steps, nil)
	lookup := &fakeLookup{paths: map[string]string{"mariadbd": "/usr/bin/mariadbd"}}
	r := runWithOptions("/db.sqlite", "", "http://127.0.0.1:9000", true, false, "sess-default", true, false, false, false, true, false, false, "", 0, "/bin/archive-center-go", "", lookup)

	if r.Status != "ok" {
		t.Fatalf("status = %q, want ok", r.Status)
	}
	if !fr.lastCfg.DefaultSwitch {
		t.Fatal("expected direct runner to receive DefaultSwitch=true")
	}
	if r.DefaultSwitch["status"] != "ok" {
		t.Fatalf("default_switch_rehearsal = %#v, want status ok", r.DefaultSwitch)
	}
	if r.DefaultSwitch["fallback_available"] != true {
		t.Fatalf("fallback_available = %#v, want true", r.DefaultSwitch["fallback_available"])
	}
	foundGo := false
	foundFallback := false
	for _, step := range r.ChildPlan {
		if step.Name == "go-default-candidate-probe" {
			foundGo = true
		}
		if step.Name == "python-fallback-replay" {
			foundFallback = true
		}
	}
	if !foundGo || !foundFallback {
		t.Fatalf("expected default switch plan entries, go=%v fallback=%v", foundGo, foundFallback)
	}
	if r.SafetyFlags["authority_switch"] || r.SafetyFlags["go_default_switch"] {
		t.Fatal("default switch rehearsal must not persist authority/default switch")
	}
}

func TestRunWithDefaultSwitchActualCarriesManagedGateSummary(t *testing.T) {
	steps := []executedStep{
		{Name: "create-datadir", Status: "ok"},
		{Name: "mariadb-schema", Status: "ok"},
		{Name: "mariadb-import", Status: "ok"},
		{Name: "mariadb-compare", Status: "ok"},
		{Name: "start-go-backend", Status: "ok"},
		{Name: "wait-go-ready", Status: "ok"},
		{Name: "shadow-value-report", Status: "ok"},
		{Name: "go-default-actual-switch-gate", Status: "ok", Details: map[string]any{
			"requested":              true,
			"role":                   "go_default_candidate",
			"selected_runtime":       "go",
			"switch_scope":           "managed_disposable_actual",
			"candidate_store_mode":   "mariadb_read_shadow",
			"candidate_product_read": true,
			"authority_switch":       false,
			"go_default_switch":      true,
			"persistent_switch":      false,
			"python_runtime_retired": false,
		}},
		{Name: "rollback-stop-product-go-backend", Status: "ok"},
		{Name: "python-fallback-replay", Status: "ok", Details: map[string]any{
			"requested":              true,
			"role":                   "python_fallback",
			"selected_runtime":       "python_fallback",
			"fallback_available":     true,
			"authority_switch":       false,
			"go_default_switch":      false,
			"python_runtime_retired": false,
		}},
		{Name: "rollback-start-go-backend", Status: "ok"},
		{Name: "rollback-wait-go-ready", Status: "ok"},
		{Name: "rollback-ready-check", Status: "ok", Note: "store_mode=noop mariadb_product_read=disabled mariadb_authority=disabled"},
		{Name: "rollback-stop-go-backend", Status: "ok"},
	}
	fr := setFakeDirectRunner(t, steps, nil)
	lookup := &fakeLookup{paths: map[string]string{"mariadbd": "/usr/bin/mariadbd"}}
	r := runWithOptions("/db.sqlite", "", "http://127.0.0.1:9000", true, false, "sess-default-actual", true, true, true, false, true, true, false, "", 0, "/bin/archive-center-go", "", lookup)

	if r.Status != "ok" {
		t.Fatalf("status = %q, want ok", r.Status)
	}
	if !fr.lastCfg.DefaultSwitch || !fr.lastCfg.DefaultSwitchActual {
		t.Fatalf("expected direct runner default switch flags, got rehearsal=%v actual=%v", fr.lastCfg.DefaultSwitch, fr.lastCfg.DefaultSwitchActual)
	}
	if r.DefaultRuntime["status"] != "ok" {
		t.Fatalf("default_runtime_switch = %#v, want status ok", r.DefaultRuntime)
	}
	if r.DefaultRuntime["go_default_switch"] != true {
		t.Fatalf("go_default_switch = %#v, want true in managed actual gate", r.DefaultRuntime["go_default_switch"])
	}
	if r.DefaultRuntime["persistent_switch"] != false {
		t.Fatalf("persistent_switch = %#v, want false", r.DefaultRuntime["persistent_switch"])
	}
	if r.DefaultRuntime["post_switch_replay"] != true || r.DefaultRuntime["rollback_available"] != true {
		t.Fatalf("default runtime gate missing replay/rollback evidence: %#v", r.DefaultRuntime)
	}
	foundActualPlan := false
	for _, step := range r.ChildPlan {
		if step.Name == "go-default-actual-switch-gate" {
			foundActualPlan = true
			break
		}
	}
	if !foundActualPlan {
		t.Fatal("expected go-default-actual-switch-gate in child plan")
	}
	if r.SafetyFlags["authority_switch"] || r.SafetyFlags["go_default_switch"] {
		t.Fatal("managed actual switch gate must not persist authority/default switch")
	}
}

func TestRunWithDefaultSwitchActualRequiresProductReadProof(t *testing.T) {
	lookup := &fakeLookup{paths: map[string]string{"mariadbd": "/usr/bin/mariadbd"}}
	r := runWithOptions("/db.sqlite", "", "http://127.0.0.1:9000", true, false, "sess-default-actual-missing", false, false, false, false, true, true, false, "", 0, "/bin/archive-center-go", "", lookup)
	if r.Status != "failed" {
		t.Fatalf("status = %q, want failed", r.Status)
	}
	if len(r.Errors) == 0 || !strings.Contains(r.Errors[0], "default-switch-actual requires -product-read-proof") {
		t.Fatalf("expected product-read-proof requirement error, got %v", r.Errors)
	}
}

func TestRunWithAuthorityCutoverReplayCarriesSummary(t *testing.T) {
	steps := []executedStep{
		{Name: "create-datadir", Status: "ok"},
		{Name: "mariadb-schema", Status: "ok"},
		{Name: "mariadb-import", Status: "ok"},
		{Name: "mariadb-compare", Status: "ok"},
		{Name: "start-go-backend", Status: "ok"},
		{Name: "wait-go-ready", Status: "ok"},
		{Name: "shadow-value-report", Status: "ok"},
		{Name: "authority-start-go-backend", Status: "ok"},
		{Name: "authority-wait-go-ready", Status: "ok"},
		{Name: "authority-ready-check", Status: "ok", Note: "store_mode=mariadb_authority mariadb_product_read=enabled mariadb_authority=enabled"},
		{Name: "authority-route-write-smoke", Status: "ok", Details: map[string]any{
			"requested":        true,
			"status":           "ok",
			"store_mode":       "mariadb_authority",
			"delta_counts":     map[string]any{"chat_logs": 3, "effective_input_logs": 3},
			"authority_switch": true,
		}},
		{Name: "authority-post-cutover-replay", Status: "ok", Details: map[string]any{
			"requested": true,
			"status":    "ok",
		}},
		{Name: "authority-stop-go-backend", Status: "ok"},
		{Name: "authority-cutover-summary", Status: "ok", Details: map[string]any{
			"requested":              true,
			"status":                 "ok",
			"store_mode":             "mariadb_authority",
			"authority_switch":       true,
			"persistent_switch":      false,
			"go_default_switch":      false,
			"python_runtime_retired": false,
			"post_cutover_replay_ok": true,
		}},
		{Name: "rollback-stop-product-go-backend", Status: "ok"},
		{Name: "python-fallback-replay", Status: "ok", Details: map[string]any{
			"requested":          true,
			"status":             "ok",
			"fallback_available": true,
		}},
		{Name: "rollback-start-go-backend", Status: "ok"},
		{Name: "rollback-wait-go-ready", Status: "ok"},
		{Name: "rollback-ready-check", Status: "ok", Note: "store_mode=noop mariadb_product_read=disabled mariadb_authority=disabled"},
		{Name: "rollback-stop-go-backend", Status: "ok"},
	}
	fr := setFakeDirectRunner(t, steps, nil)
	lookup := &fakeLookup{paths: map[string]string{"mariadbd": "/usr/bin/mariadbd"}}
	r := runWithOptions("/db.sqlite", "", "http://127.0.0.1:9000", true, false, "sess-authority", true, false, true, true, false, false, false, "", 0, "/bin/archive-center-go", "", lookup)

	if r.Status != "ok" {
		t.Fatalf("status = %q, want ok", r.Status)
	}
	if !fr.lastCfg.AuthorityCutover {
		t.Fatal("expected direct runner to receive AuthorityCutover=true")
	}
	if r.AuthorityCutover["status"] != "ok" {
		t.Fatalf("authority_cutover_replay = %#v, want status ok", r.AuthorityCutover)
	}
	if r.AuthorityCutover["authority_switch"] != true {
		t.Fatalf("authority_switch = %#v, want true inside replay", r.AuthorityCutover["authority_switch"])
	}
	if r.AuthorityCutover["persistent_switch"] != false {
		t.Fatalf("persistent_switch = %#v, want false", r.AuthorityCutover["persistent_switch"])
	}
	if r.AuthorityCutover["post_cutover_replay"] != true || r.AuthorityCutover["rollback_available"] != true {
		t.Fatalf("authority replay missing replay/rollback evidence: %#v", r.AuthorityCutover)
	}
	if r.SafetyFlags["authority_switch"] || r.SafetyFlags["go_default_switch"] {
		t.Fatal("authority cutover replay must not persist authority/default switch")
	}
}

func TestRunWithAuthorityCutoverRequiresProductReadProof(t *testing.T) {
	lookup := &fakeLookup{paths: map[string]string{"mariadbd": "/usr/bin/mariadbd"}}
	r := runWithOptions("/db.sqlite", "", "http://127.0.0.1:9000", true, false, "sess-authority-missing", false, false, false, true, false, false, false, "", 0, "/bin/archive-center-go", "", lookup)
	if r.Status != "failed" {
		t.Fatalf("status = %q, want failed", r.Status)
	}
	if len(r.Errors) == 0 || !strings.Contains(r.Errors[0], "authority-cutover-replay requires -product-read-proof") {
		t.Fatalf("expected authority-cutover product-read-proof requirement error, got %v", r.Errors)
	}
}
