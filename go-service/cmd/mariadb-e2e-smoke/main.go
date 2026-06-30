// mariadb-e2e-smoke chains the existing MariaDB migration executors as a guarded
// manual shadow E2E runner. It does not switch authority or enable live retrieval.
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

type stepResult struct {
	Name       string `json:"name"`
	Status     string `json:"status"`
	ExitCode   int    `json:"exit_code"`
	ReportPath string `json:"report_path,omitempty"`
	Error      string `json:"error,omitempty"`
}

type e2eReport struct {
	Status      string       `json:"status"`
	SessionID   string       `json:"session_id,omitempty"`
	SourceMode  string       `json:"source_mode"`
	ExportDir   string       `json:"export_dir,omitempty"`
	SQLiteDB    string       `json:"sqlite_db,omitempty"`
	Note        string       `json:"note"`
	GeneratedAt string       `json:"generated_at"`
	Steps       []stepResult `json:"steps"`
	Error       string       `json:"error,omitempty"`
}

type stepRunner func(binary string, args []string) (exitCode int, err error)

func main() {
	dsn := flag.String("dsn", os.Getenv("AC_MARIADB_DSN"), "MariaDB DSN. Defaults to AC_MARIADB_DSN.")
	exportDir := flag.String("export-dir", "", "Path to export directory containing manifest.json and canonical NDJSON files.")
	sqliteDB := flag.String("sqlite-db", "", "Path to SQLite database. Mutually exclusive with -export-dir.")
	execute := flag.Bool("execute", false, "Actually run the E2E migration steps against MariaDB.")
	outPath := flag.String("out", "", "Path to write JSON report. Defaults to stdout.")
	sessionID := flag.String("session-id", fmt.Sprintf("mariadb-e2e-smoke-%d", time.Now().UTC().UnixNano()), "Smoke session id.")
	flag.Parse()

	report := buildReport(*execute, *dsn, *exportDir, *sqliteDB, *sessionID)
	if report != nil {
		writeReport(report, *outPath)
		if report.Status == "failed" {
			os.Exit(2)
		}
		os.Exit(2) // guarded exits 2 to match sibling commands
	}

	runner := defaultStepRunner()
	report = runE2E(*dsn, *exportDir, *sqliteDB, *sessionID, *outPath, runner)
	writeReport(report, *outPath)
	if report.Status == "failed" {
		os.Exit(1)
	}
}

func buildReport(execute bool, dsn, exportDir, sqliteDB, sessionID string) *e2eReport {
	now := time.Now().UTC().Format(time.RFC3339)

	sourceMode := sourceMode(exportDir, sqliteDB)
	note := fmt.Sprintf("Manual shadow E2E evidence runner; source=%s; not a MariaDB authority/cutover.", sourceMode)

	hasExportDir := strings.TrimSpace(exportDir) != ""
	hasSQLiteDB := strings.TrimSpace(sqliteDB) != ""

	if hasExportDir && hasSQLiteDB {
		return &e2eReport{
			Status:      "failed",
			SessionID:   sessionID,
			SourceMode:  sourceMode,
			ExportDir:   exportDir,
			SQLiteDB:    sqliteDB,
			Note:        note,
			GeneratedAt: now,
			Steps:       []stepResult{},
			Error:       "ambiguous source: provide only one of --export-dir or --sqlite-db",
		}
	}

	if !execute {
		return &e2eReport{
			Status:      "guarded",
			SessionID:   sessionID,
			SourceMode:  sourceMode,
			ExportDir:   exportDir,
			SQLiteDB:    sqliteDB,
			Note:        note,
			GeneratedAt: now,
			Steps:       []stepResult{},
		}
	}

	if strings.TrimSpace(dsn) == "" {
		return &e2eReport{
			Status:      "failed",
			SessionID:   sessionID,
			SourceMode:  sourceMode,
			ExportDir:   exportDir,
			SQLiteDB:    sqliteDB,
			Note:        note,
			GeneratedAt: now,
			Steps:       []stepResult{},
			Error:       "missing DSN: provide --dsn or AC_MARIADB_DSN",
		}
	}

	if !hasExportDir && !hasSQLiteDB {
		return &e2eReport{
			Status:      "failed",
			SessionID:   sessionID,
			SourceMode:  sourceMode,
			Note:        note,
			GeneratedAt: now,
			Steps:       []stepResult{},
			Error:       "missing source: provide --export-dir or --sqlite-db",
		}
	}

	return nil
}

func runE2E(dsn, exportDir, sqliteDB, sessionID, outPath string, runner stepRunner) *e2eReport {
	sourceMode := sourceMode(exportDir, sqliteDB)
	report := &e2eReport{
		Status:      "ok",
		SessionID:   sessionID,
		SourceMode:  sourceMode,
		ExportDir:   exportDir,
		SQLiteDB:    sqliteDB,
		Note:        "Manual shadow E2E evidence runner; not a MariaDB authority/cutover.",
		GeneratedAt: time.Now().UTC().Format(time.RFC3339),
		Steps:       []stepResult{},
	}

	effectiveExportDir := exportDir

	if strings.TrimSpace(sqliteDB) != "" {
		tempDir, err := os.MkdirTemp("", "mariadb-e2e-smoke-sqlite-*")
		if err != nil {
			report.Status = "failed"
			report.Error = fmt.Sprintf("creating temp dir for sqlite-export: %v", err)
			report.Steps = append(report.Steps, stepResult{
				Name:   "sqlite-export",
				Status: "failed",
				Error:  fmt.Sprintf("mkdir temp: %v", err),
			})
			return report
		}
		report.ExportDir = tempDir

		stepOutPath := filepath.Join(tempDir, "manifest.json")
		args := buildStepArgs("sqlite-export", dsn, tempDir, sqliteDB, sessionID, stepOutPath)

		exitCode, err := runner("sqlite-export", args)
		sr := stepResult{
			Name:       "sqlite-export",
			ExitCode:   exitCode,
			ReportPath: stepOutPath,
		}
		if err != nil {
			sr.Status = "failed"
			sr.Error = err.Error()
			report.Status = "failed"
			report.Error = fmt.Sprintf("step sqlite-export failed: %v", err)
			report.Steps = append(report.Steps, sr)
			return report
		}
		if exitCode != 0 {
			sr.Status = "failed"
			sr.Error = fmt.Sprintf("exit code %d", exitCode)
			report.Status = "failed"
			report.Error = fmt.Sprintf("step sqlite-export exited with code %d", exitCode)
			report.Steps = append(report.Steps, sr)
			return report
		}
		sr.Status = "ok"
		report.Steps = append(report.Steps, sr)
		effectiveExportDir = tempDir
	}

	steps := []struct {
		name   string
		binary string
	}{
		{name: "schema-apply", binary: "mariadb-schema"},
		{name: "import-canonical", binary: "mariadb-import"},
		{name: "mariadb-compare", binary: "mariadb-compare"},
		{name: "canonical-route-smoke", binary: "canonical-route-smoke"},
	}

	for _, step := range steps {
		stepOutPath := deriveStepOutPath(outPath, step.name)
		args := buildStepArgs(step.binary, dsn, effectiveExportDir, sqliteDB, sessionID, stepOutPath)

		exitCode, err := runner(step.binary, args)
		sr := stepResult{
			Name:       step.name,
			ExitCode:   exitCode,
			ReportPath: stepOutPath,
		}
		if err != nil {
			sr.Status = "failed"
			sr.Error = err.Error()
			report.Status = "failed"
			report.Error = fmt.Sprintf("step %d (%s) failed: %v", len(report.Steps)+1, step.name, err)
			report.Steps = append(report.Steps, sr)
			break
		}
		if exitCode != 0 {
			sr.Status = "failed"
			sr.Error = fmt.Sprintf("exit code %d", exitCode)
			report.Status = "failed"
			report.Error = fmt.Sprintf("step %d (%s) exited with code %d", len(report.Steps)+1, step.name, exitCode)
			report.Steps = append(report.Steps, sr)
			break
		}
		sr.Status = "ok"
		report.Steps = append(report.Steps, sr)
	}

	return report
}

func buildStepArgs(binary, dsn, exportDir, sqliteDB, sessionID, outPath string) []string {
	switch binary {
	case "sqlite-export":
		return []string{"-db", sqliteDB, "-out", exportDir, "-all"}
	case "mariadb-schema":
		args := []string{"-dsn", dsn, "-execute"}
		if outPath != "" {
			args = append(args, "-out", outPath)
		}
		return args
	case "mariadb-import":
		args := []string{"-dsn", dsn, "-export-dir", exportDir, "-execute"}
		if outPath != "" {
			args = append(args, "-out", outPath)
		}
		return args
	case "mariadb-compare":
		args := []string{"-dsn", dsn, "-export-dir", exportDir}
		if outPath != "" {
			args = append(args, "-out", outPath)
		}
		return args
	case "canonical-route-smoke":
		args := []string{"-dsn", dsn, "-execute", "-session-id", sessionID}
		if outPath != "" {
			args = append(args, "-out", outPath)
		}
		return args
	default:
		return nil
	}
}

func sourceMode(exportDir, sqliteDB string) string {
	hasExportDir := strings.TrimSpace(exportDir) != ""
	hasSQLiteDB := strings.TrimSpace(sqliteDB) != ""
	switch {
	case hasExportDir && hasSQLiteDB:
		return "ambiguous"
	case hasSQLiteDB:
		return "sqlite-db"
	case hasExportDir:
		return "export-dir"
	default:
		return "none"
	}
}

func deriveStepOutPath(baseOut, stepName string) string {
	if baseOut == "" {
		return ""
	}
	dir := filepath.Dir(baseOut)
	base := filepath.Base(baseOut)
	ext := filepath.Ext(base)
	stem := strings.TrimSuffix(base, ext)
	return filepath.Join(dir, fmt.Sprintf("%s-%s%s", stem, stepName, ext))
}

func defaultStepRunner() stepRunner {
	return func(name string, args []string) (int, error) {
		binPath, err := findBinary(name)
		var cmd *exec.Cmd
		if err == nil {
			cmd = exec.Command(binPath, args...)
		} else {
			sourcePath, sourceErr := findCommandSource(name)
			if sourceErr != nil {
				return 1, err
			}
			goArgs := append([]string{"run", "-buildvcs=false", sourcePath}, args...)
			cmd = exec.Command("go", goArgs...)
		}
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		if err := cmd.Run(); err != nil {
			if exitErr, ok := err.(*exec.ExitError); ok {
				return exitErr.ExitCode(), nil
			}
			return 1, err
		}
		return 0, nil
	}
}

func findBinary(name string) (string, error) {
	execPath, err := os.Executable()
	if err == nil {
		execDir := filepath.Dir(execPath)
		candidates := []string{
			filepath.Join(execDir, name),
			filepath.Join(execDir, name+".exe"),
		}
		for _, c := range candidates {
			if info, err := os.Stat(c); err == nil && !info.IsDir() {
				return c, nil
			}
		}
	}
	if path, err := exec.LookPath(name); err == nil {
		return path, nil
	}
	return "", fmt.Errorf("binary %q not found in executable directory or PATH", name)
}

func findCommandSource(name string) (string, error) {
	wd, err := os.Getwd()
	if err != nil {
		return "", err
	}
	candidates := []string{
		filepath.Join(wd, "cmd", name),
		filepath.Join(wd, name),
	}
	for _, c := range candidates {
		if info, err := os.Stat(filepath.Join(c, "main.go")); err == nil && !info.IsDir() {
			if rel, err := filepath.Rel(wd, c); err == nil && !strings.HasPrefix(rel, "..") {
				return "." + string(os.PathSeparator) + rel, nil
			}
			return c, nil
		}
	}
	return "", fmt.Errorf("command source %q not found from %s", name, wd)
}

func writeReport(report *e2eReport, outPath string) {
	data, err := json.MarshalIndent(report, "", "  ")
	if err != nil {
		fmt.Fprintf(os.Stderr, "marshal report: %v\n", err)
		return
	}
	data = append(data, '\n')
	if outPath == "" {
		_, _ = os.Stdout.Write(data)
		return
	}
	if err := os.WriteFile(outPath, data, 0644); err != nil {
		fmt.Fprintf(os.Stderr, "write report: %v\n", err)
	}
}
