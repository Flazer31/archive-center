package main

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

type fakeRunner struct {
	calls []fakeCall
	fail  map[string]error
}

type fakeCall struct {
	tool string
	args []string
}

func (f *fakeRunner) Tool(name string) (string, error) {
	return name, nil
}

func (f *fakeRunner) Run(tool string, args ...string) commandResult {
	f.calls = append(f.calls, fakeCall{tool: tool, args: append([]string(nil), args...)})
	if err := f.fail[tool]; err != nil {
		return commandResult{Stderr: err.Error(), Err: err}
	}
	if tool == "compare-dry-run" {
		return commandResult{Stdout: `{"status":"ok"}` + "\n"}
	}
	return commandResult{Stdout: tool + " ok\n"}
}

func TestDryRunPipelineSkipsMariaDBImport(t *testing.T) {
	sourceDB := writeTempSourceDB(t)
	workDir := t.TempDir()
	runner := &fakeRunner{fail: map[string]error{}}

	report := runMigration(config{
		SQLiteDB:      sourceDB,
		WorkDir:       workDir,
		CanonicalOnly: true,
	}, runner)

	if report.Status != "ok" {
		t.Fatalf("status = %q, errors=%v", report.Status, report.Errors)
	}
	if len(runner.calls) != 4 {
		t.Fatalf("calls = %d, want 4: %#v", len(runner.calls), runner.calls)
	}
	for _, call := range runner.calls {
		if call.tool == "mariadb-import" {
			t.Fatalf("dry-run must not call mariadb-import: %#v", runner.calls)
		}
	}
	if _, err := os.Stat(filepath.Join(workDir, "compare-report.json")); err != nil {
		t.Fatalf("compare report was not written: %v", err)
	}
}

func TestExecuteRequiresDSNBeforeRunningTools(t *testing.T) {
	sourceDB := writeTempSourceDB(t)
	runner := &fakeRunner{fail: map[string]error{}}

	report := runMigration(config{
		SQLiteDB: sourceDB,
		WorkDir:  t.TempDir(),
		Execute:  true,
	}, runner)

	if report.Status != "failed" {
		t.Fatalf("status = %q, want failed", report.Status)
	}
	if len(runner.calls) != 0 {
		t.Fatalf("execute without DSN should not run tools: %#v", runner.calls)
	}
}

func TestExecuteCallsImportAndRedactsDSN(t *testing.T) {
	sourceDB := writeTempSourceDB(t)
	runner := &fakeRunner{fail: map[string]error{}}

	report := runMigration(config{
		SQLiteDB: sourceDB,
		WorkDir:  t.TempDir(),
		Execute:  true,
		DSN:      "user:secret@tcp(127.0.0.1:3307)/archive_center?parseTime=true",
	}, runner)

	if report.Status != "ok" {
		t.Fatalf("status = %q, errors=%v", report.Status, report.Errors)
	}
	if runner.calls[len(runner.calls)-1].tool != "mariadb-import" {
		t.Fatalf("last call = %#v, want mariadb-import", runner.calls[len(runner.calls)-1])
	}
	if strings.Contains(report.Config.DSN, "secret") {
		t.Fatalf("report config leaked DSN: %q", report.Config.DSN)
	}
	for _, step := range report.Steps {
		for _, arg := range step.Args {
			if strings.Contains(arg, "secret") {
				t.Fatalf("step arg leaked DSN: %#v", step.Args)
			}
		}
	}
}

func TestToolFailureStopsPipeline(t *testing.T) {
	sourceDB := writeTempSourceDB(t)
	runner := &fakeRunner{fail: map[string]error{
		"dry-run-validator": errors.New("validator failed"),
	}}

	report := runMigration(config{
		SQLiteDB: sourceDB,
		WorkDir:  t.TempDir(),
	}, runner)

	if report.Status != "failed" {
		t.Fatalf("status = %q, want failed", report.Status)
	}
	if len(runner.calls) != 2 {
		t.Fatalf("calls = %d, want stop after validator: %#v", len(runner.calls), runner.calls)
	}
}

func writeTempSourceDB(t *testing.T) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "memory.db")
	if err := os.WriteFile(path, []byte("sqlite placeholder"), 0644); err != nil {
		t.Fatal(err)
	}
	return path
}
