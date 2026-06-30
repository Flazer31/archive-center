// legacy10-migrate wraps the 1.0 SQLite -> 2.0 MariaDB migration pipeline.
//
// It is intentionally conservative:
//   - source SQLite is read-only
//   - dry-run is the default
//   - --execute plus a MariaDB DSN is required before writes happen
package main

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"
)

type config struct {
	SQLiteDB      string `json:"sqlite_db"`
	WorkDir       string `json:"work_dir"`
	DSN           string `json:"-"`
	Execute       bool   `json:"execute"`
	CanonicalOnly bool   `json:"canonical_only"`
	KeepWorkDir   bool   `json:"keep_work_dir"`
}

type stepReport struct {
	Name       string   `json:"name"`
	Status     string   `json:"status"`
	Tool       string   `json:"tool"`
	Args       []string `json:"args"`
	ReportPath string   `json:"report_path,omitempty"`
	Stdout     string   `json:"stdout,omitempty"`
	Stderr     string   `json:"stderr,omitempty"`
	Error      string   `json:"error,omitempty"`
	StartedAt  string   `json:"started_at"`
	FinishedAt string   `json:"finished_at"`
}

type migrationReport struct {
	Status      string       `json:"status"`
	GeneratedAt string       `json:"generated_at"`
	Config      config       `json:"config"`
	ExportDir   string       `json:"export_dir"`
	Steps       []stepReport `json:"steps"`
	Warnings    []string     `json:"warnings,omitempty"`
	Errors      []string     `json:"errors,omitempty"`
	NextActions []string     `json:"next_actions,omitempty"`
}

type commandResult struct {
	Stdout string
	Stderr string
	Err    error
}

type commandRunner interface {
	Run(tool string, args ...string) commandResult
	Tool(name string) (string, error)
}

type execRunner struct {
	binDir string
}

func main() {
	var cfg config
	outPath := flag.String("out", "", "Path to write JSON migration report. Defaults to stdout.")
	flag.StringVar(&cfg.SQLiteDB, "sqlite-db", "", "Path to Archive Center 1.0 memory.db or compatible SQLite DB.")
	flag.StringVar(&cfg.WorkDir, "work-dir", "", "Directory for intermediate export and reports.")
	flag.StringVar(&cfg.DSN, "dsn", os.Getenv("AC_MARIADB_DSN"), "MariaDB DSN. Defaults to AC_MARIADB_DSN.")
	flag.BoolVar(&cfg.Execute, "execute", false, "Actually import rows into MariaDB. Default is dry-run only.")
	flag.BoolVar(&cfg.CanonicalOnly, "canonical-only", true, "Export/import only 2.0-recognized canonical tables.")
	flag.BoolVar(&cfg.KeepWorkDir, "keep-work-dir", true, "Keep intermediate export/report files.")
	flag.Parse()

	if strings.TrimSpace(cfg.SQLiteDB) == "" {
		fmt.Fprintln(os.Stderr, "error: --sqlite-db is required")
		os.Exit(2)
	}

	runner, err := newExecRunner()
	if err != nil {
		writeReport(migrationReport{
			Status:      "failed",
			GeneratedAt: nowUTC(),
			Config:      redactedConfig(cfg),
			Errors:      []string{err.Error()},
		}, *outPath)
		os.Exit(1)
	}

	report := runMigration(cfg, runner)
	writeReport(report, *outPath)
	if report.Status != "ok" {
		os.Exit(1)
	}
}

func newExecRunner() (*execRunner, error) {
	exe, err := os.Executable()
	if err != nil {
		return nil, err
	}
	return &execRunner{binDir: filepath.Dir(exe)}, nil
}

func (r *execRunner) Tool(name string) (string, error) {
	candidates := []string{name}
	if runtime.GOOS == "windows" && !strings.HasSuffix(strings.ToLower(name), ".exe") {
		candidates = append([]string{name + ".exe"}, candidates...)
	}
	for _, candidate := range candidates {
		path := filepath.Join(r.binDir, candidate)
		if info, err := os.Stat(path); err == nil && !info.IsDir() {
			return path, nil
		}
	}
	return "", fmt.Errorf("missing required migration helper %q beside legacy10-migrate", name)
}

func (r *execRunner) Run(tool string, args ...string) commandResult {
	cmd := exec.Command(tool, args...)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err := cmd.Run()
	return commandResult{Stdout: stdout.String(), Stderr: stderr.String(), Err: err}
}

func runMigration(cfg config, runner commandRunner) migrationReport {
	cfg = normalizeConfig(cfg)
	report := migrationReport{
		Status:      "ok",
		GeneratedAt: nowUTC(),
		Config:      redactedConfig(cfg),
		ExportDir:   filepath.Join(cfg.WorkDir, "export"),
	}
	if cfg.Execute && strings.TrimSpace(cfg.DSN) == "" {
		report.Status = "failed"
		report.Errors = append(report.Errors, "--execute requires --dsn or AC_MARIADB_DSN")
		return report
	}
	if err := os.MkdirAll(report.ExportDir, 0755); err != nil {
		report.Status = "failed"
		report.Errors = append(report.Errors, fmt.Sprintf("creating work dir: %v", err))
		return report
	}

	sourceDB, err := filepath.Abs(cfg.SQLiteDB)
	if err != nil {
		report.Status = "failed"
		report.Errors = append(report.Errors, fmt.Sprintf("resolving sqlite db: %v", err))
		return report
	}
	if info, err := os.Stat(sourceDB); err != nil || info.IsDir() {
		report.Status = "failed"
		if err != nil {
			report.Errors = append(report.Errors, fmt.Sprintf("source sqlite db not found: %v", err))
		} else {
			report.Errors = append(report.Errors, "source sqlite path is a directory")
		}
		return report
	}

	if !cfg.Execute {
		report.NextActions = append(report.NextActions, "Review validation-report.json and import-plan.json, then re-run with --execute to write MariaDB rows.")
	}
	report.Warnings = append(report.Warnings, "import-plan.json is a conservative core-table plan; execute mode imports every 2.0-recognized table supported by mariadb-import.")
	report.NextActions = append(report.NextActions, "After import, run Archive Center 2.0 /admin/reindex or the UI reindex flow to rebuild ChromaDB vectors from MariaDB.")

	exportArgs := []string{"-db", sourceDB, "-out", report.ExportDir}
	if cfg.CanonicalOnly {
		exportArgs = append(exportArgs, "-canonical-only")
	} else {
		exportArgs = append(exportArgs, "-all")
		report.Warnings = append(report.Warnings, "--canonical-only=false exports all source tables, but only 2.0-recognized tables are imported into MariaDB.")
	}
	if !runStep(&report, runner, "export_sqlite", "sqlite-export", "", exportArgs...) {
		return failReport(report)
	}

	validationPath := filepath.Join(cfg.WorkDir, "validation-report.json")
	validateArgs := []string{"-export-dir", report.ExportDir, "-report", validationPath, "-strict-canonical", "-report-derived"}
	if !runStep(&report, runner, "validate_export", "dry-run-validator", validationPath, validateArgs...) {
		return failReport(report)
	}

	comparePath := filepath.Join(cfg.WorkDir, "compare-report.json")
	compareArgs := []string{"-sqlite-db", sourceDB, "-dry-run-report", validationPath, "-json"}
	if !runStepCaptureToFile(&report, runner, "compare_export_to_source", "compare-dry-run", comparePath, compareArgs...) {
		return failReport(report)
	}

	planPath := filepath.Join(cfg.WorkDir, "import-plan.json")
	planArgs := []string{"-export-dir", report.ExportDir, "-out", planPath}
	if !runStep(&report, runner, "plan_mariadb_import", "mariadb-dry-run-import", planPath, planArgs...) {
		return failReport(report)
	}

	if cfg.Execute {
		importPath := filepath.Join(cfg.WorkDir, "mariadb-import-report.json")
		importArgs := []string{"-export-dir", report.ExportDir, "-dsn", cfg.DSN, "-out", importPath, "-execute"}
		if !runStep(&report, runner, "execute_mariadb_import", "mariadb-import", importPath, importArgs...) {
			return failReport(report)
		}
		report.NextActions = append(report.NextActions, "Open Archive Center 2.0 and confirm sessions/timeline counts, then run vector reindex if ChromaDB search should use imported memories.")
	}

	return report
}

func normalizeConfig(cfg config) config {
	if cfg.WorkDir == "" {
		cfg.WorkDir = filepath.Join(".", "legacy10-migration-"+time.Now().UTC().Format("20060102-150405"))
	}
	return cfg
}

func redactedConfig(cfg config) config {
	cfg.DSN = redactDSN(cfg.DSN)
	return cfg
}

func redactDSN(dsn string) string {
	if strings.TrimSpace(dsn) == "" {
		return ""
	}
	at := strings.Index(dsn, "@")
	if at < 0 {
		return "***"
	}
	return "***" + dsn[at:]
}

func runStep(report *migrationReport, runner commandRunner, name, toolName, reportPath string, args ...string) bool {
	tool, err := runner.Tool(toolName)
	if err != nil {
		appendStep(report, name, "failed", toolName, args, reportPath, "", "", err)
		return false
	}
	result := runner.Run(tool, args...)
	status := "ok"
	if result.Err != nil {
		status = "failed"
	}
	appendStep(report, name, status, tool, args, reportPath, result.Stdout, result.Stderr, result.Err)
	return result.Err == nil
}

func runStepCaptureToFile(report *migrationReport, runner commandRunner, name, toolName, reportPath string, args ...string) bool {
	tool, err := runner.Tool(toolName)
	if err != nil {
		appendStep(report, name, "failed", toolName, args, reportPath, "", "", err)
		return false
	}
	result := runner.Run(tool, args...)
	if result.Err == nil {
		if err := os.WriteFile(reportPath, []byte(result.Stdout), 0644); err != nil {
			result.Err = err
		}
	}
	status := "ok"
	if result.Err != nil {
		status = "failed"
	}
	appendStep(report, name, status, tool, args, reportPath, result.Stdout, result.Stderr, result.Err)
	return result.Err == nil
}

func appendStep(report *migrationReport, name, status, tool string, args []string, reportPath, stdout, stderr string, err error) {
	step := stepReport{
		Name:       name,
		Status:     status,
		Tool:       tool,
		Args:       redactArgs(args),
		ReportPath: reportPath,
		Stdout:     truncate(stdout, 2000),
		Stderr:     truncate(stderr, 2000),
		StartedAt:  nowUTC(),
		FinishedAt: nowUTC(),
	}
	if err != nil {
		step.Error = err.Error()
		report.Errors = append(report.Errors, fmt.Sprintf("%s: %v", name, err))
	}
	report.Steps = append(report.Steps, step)
}

func redactArgs(args []string) []string {
	out := append([]string(nil), args...)
	for i := 0; i < len(out); i++ {
		if out[i] == "-dsn" && i+1 < len(out) {
			out[i+1] = redactDSN(out[i+1])
		}
	}
	return out
}

func failReport(report migrationReport) migrationReport {
	report.Status = "failed"
	if len(report.Errors) == 0 {
		report.Errors = append(report.Errors, "migration failed")
	}
	return report
}

func truncate(s string, limit int) string {
	if len(s) <= limit {
		return s
	}
	if limit < 20 {
		return s[:limit]
	}
	return s[:limit-15] + "...[truncated]"
}

func nowUTC() string {
	return time.Now().UTC().Format(time.RFC3339)
}

func writeReport(report migrationReport, outPath string) {
	data, err := json.MarshalIndent(report, "", "  ")
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: encoding report: %v\n", err)
		return
	}
	data = append(data, '\n')
	if outPath == "" {
		_, _ = os.Stdout.Write(data)
		return
	}
	if err := os.WriteFile(outPath, data, 0644); err != nil {
		fmt.Fprintf(os.Stderr, "error: writing report: %v\n", err)
	}
}
