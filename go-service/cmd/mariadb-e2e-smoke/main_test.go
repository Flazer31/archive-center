package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

type fakeRunRecord struct {
	binary string
	args   []string
}

func TestBuildReportGuarded(t *testing.T) {
	r := buildReport(false, "", "", "", "sess-1")
	if r == nil {
		t.Fatal("expected non-nil report")
	}
	if r.Status != "guarded" {
		t.Fatalf("status = %q, want guarded", r.Status)
	}
	if !strings.Contains(r.Note, "source=none") {
		t.Fatalf("expected source=none in note, got %q", r.Note)
	}
	if r.SourceMode != "none" {
		t.Fatalf("source mode = %q, want none", r.SourceMode)
	}
	if len(r.Steps) != 0 {
		t.Fatalf("expected 0 steps, got %d", len(r.Steps))
	}
}

func TestBuildReportGuardedWithSQLite(t *testing.T) {
	r := buildReport(false, "", "", "/db.sqlite", "sess-1")
	if r == nil {
		t.Fatal("expected non-nil report")
	}
	if r.Status != "guarded" {
		t.Fatalf("status = %q, want guarded", r.Status)
	}
	if !strings.Contains(r.Note, "source=sqlite-db") {
		t.Fatalf("expected source=sqlite-db in note, got %q", r.Note)
	}
	if r.SourceMode != "sqlite-db" || r.SQLiteDB != "/db.sqlite" {
		t.Fatalf("unexpected sqlite source fields: %+v", r)
	}
	if len(r.Steps) != 0 {
		t.Fatalf("expected 0 steps, got %d", len(r.Steps))
	}
}

func TestBuildReportMissingDSN(t *testing.T) {
	r := buildReport(true, "", "/export", "", "sess-1")
	if r == nil {
		t.Fatal("expected non-nil report")
	}
	if r.Status != "failed" {
		t.Fatalf("status = %q, want failed", r.Status)
	}
	if !strings.Contains(r.Error, "missing DSN") {
		t.Fatalf("expected missing DSN error, got %q", r.Error)
	}
}

func TestBuildReportMissingSource(t *testing.T) {
	r := buildReport(true, "dsn", "", "", "sess-1")
	if r == nil {
		t.Fatal("expected non-nil report")
	}
	if r.Status != "failed" {
		t.Fatalf("status = %q, want failed", r.Status)
	}
	if !strings.Contains(r.Error, "missing source") {
		t.Fatalf("expected missing source error, got %q", r.Error)
	}
}

func TestBuildReportMutualExclusion(t *testing.T) {
	r := buildReport(true, "dsn", "/export", "/db.sqlite", "sess-1")
	if r == nil {
		t.Fatal("expected non-nil report")
	}
	if r.Status != "failed" {
		t.Fatalf("status = %q, want failed", r.Status)
	}
	if r.SourceMode != "ambiguous" {
		t.Fatalf("source mode = %q, want ambiguous", r.SourceMode)
	}
	if !strings.Contains(r.Error, "ambiguous source") {
		t.Fatalf("expected mutual exclusion error, got %q", r.Error)
	}
}

func TestBuildReportExecuteReady(t *testing.T) {
	r := buildReport(true, "dsn", "/export", "", "sess-1")
	if r != nil {
		t.Fatalf("expected nil report when ready, got %+v", r)
	}
}

func TestBuildReportExecuteReadyWithSQLite(t *testing.T) {
	r := buildReport(true, "dsn", "", "/db.sqlite", "sess-1")
	if r != nil {
		t.Fatalf("expected nil report when ready with sqlite, got %+v", r)
	}
}

func TestRunE2EAllStepsPass(t *testing.T) {
	var records []fakeRunRecord
	runner := func(binary string, args []string) (int, error) {
		records = append(records, fakeRunRecord{binary: binary, args: append([]string(nil), args...)})
		return 0, nil
	}

	outPath := filepath.Join(t.TempDir(), "report.json")
	report := runE2E("dsn", "/export", "", "sess-1", outPath, runner)

	if report.Status != "ok" {
		t.Fatalf("status = %q, want ok", report.Status)
	}
	if report.SourceMode != "export-dir" || report.ExportDir != "/export" {
		t.Fatalf("unexpected source fields: %+v", report)
	}
	if len(report.Steps) != 4 {
		t.Fatalf("expected 4 steps, got %d", len(report.Steps))
	}
	if len(records) != 4 {
		t.Fatalf("expected 4 runner calls, got %d", len(records))
	}

	expected := []struct {
		name   string
		binary string
	}{
		{"schema-apply", "mariadb-schema"},
		{"import-canonical", "mariadb-import"},
		{"mariadb-compare", "mariadb-compare"},
		{"canonical-route-smoke", "canonical-route-smoke"},
	}
	for i, exp := range expected {
		if report.Steps[i].Name != exp.name {
			t.Fatalf("step %d name = %q, want %q", i, report.Steps[i].Name, exp.name)
		}
		if report.Steps[i].Status != "ok" {
			t.Fatalf("step %d status = %q, want ok", i, report.Steps[i].Status)
		}
		if records[i].binary != exp.binary {
			t.Fatalf("record %d binary = %q, want %q", i, records[i].binary, exp.binary)
		}
	}

	// Verify schema step has -out with correct suffix
	schemaArgs := records[0].args
	outIdx := indexOf(schemaArgs, "-out")
	if outIdx == -1 || outIdx+1 >= len(schemaArgs) {
		t.Fatal("expected -out in schema args")
	}
	if !strings.HasSuffix(schemaArgs[outIdx+1], "-schema-apply.json") {
		t.Fatalf("unexpected schema out path: %q", schemaArgs[outIdx+1])
	}

	// Verify import step has -export-dir and -execute
	if !sliceContainsAll(records[1].args, []string{"-dsn", "dsn", "-export-dir", "/export", "-execute"}) {
		t.Fatalf("unexpected import args: %v", records[1].args)
	}

	// Verify compare step has no -execute
	for _, a := range records[2].args {
		if a == "-execute" {
			t.Fatal("mariadb-compare should not receive -execute")
		}
	}

	// Verify smoke step has -session-id
	if !sliceContainsAll(records[3].args, []string{"-dsn", "dsn", "-execute", "-session-id", "sess-1"}) {
		t.Fatalf("unexpected smoke args: %v", records[3].args)
	}
}

func TestRunE2EStopsOnRunnerError(t *testing.T) {
	var records []fakeRunRecord
	runner := func(binary string, args []string) (int, error) {
		records = append(records, fakeRunRecord{binary: binary, args: append([]string(nil), args...)})
		if binary == "mariadb-import" {
			return 1, fmt.Errorf("import crashed")
		}
		return 0, nil
	}

	report := runE2E("dsn", "/export", "", "sess-1", "", runner)

	if report.Status != "failed" {
		t.Fatalf("status = %q, want failed", report.Status)
	}
	if len(report.Steps) != 2 {
		t.Fatalf("expected 2 steps, got %d", len(report.Steps))
	}
	if len(records) != 2 {
		t.Fatalf("expected 2 runner calls, got %d", len(records))
	}
	if report.Steps[1].Status != "failed" {
		t.Fatalf("step 1 status = %q, want failed", report.Steps[1].Status)
	}
	if !strings.Contains(report.Error, "import-canonical") {
		t.Fatalf("expected error mentioning import-canonical, got %q", report.Error)
	}
	if report.Steps[1].Error != "import crashed" {
		t.Fatalf("step error = %q, want import crashed", report.Steps[1].Error)
	}
}

func TestRunE2EStopsOnNonZeroExit(t *testing.T) {
	runner := func(binary string, args []string) (int, error) {
		if binary == "mariadb-compare" {
			return 2, nil
		}
		return 0, nil
	}

	report := runE2E("dsn", "/export", "", "sess-1", "", runner)
	if report.Status != "failed" {
		t.Fatalf("status = %q, want failed", report.Status)
	}
	if len(report.Steps) != 3 {
		t.Fatalf("expected 3 steps, got %d", len(report.Steps))
	}
	if report.Steps[2].Status != "failed" {
		t.Fatalf("step 2 status = %q, want failed", report.Steps[2].Status)
	}
	if report.Steps[2].ExitCode != 2 {
		t.Fatalf("step 2 exit code = %d, want 2", report.Steps[2].ExitCode)
	}
	if !strings.Contains(report.Steps[2].Error, "exit code 2") {
		t.Fatalf("expected exit code error, got %q", report.Steps[2].Error)
	}
}

func TestRunE2ESQLiteExportSequence(t *testing.T) {
	var records []fakeRunRecord
	var sqliteExportOutDir string
	runner := func(binary string, args []string) (int, error) {
		records = append(records, fakeRunRecord{binary: binary, args: append([]string(nil), args...)})
		if binary == "sqlite-export" {
			outIdx := indexOf(args, "-out")
			if outIdx != -1 && outIdx+1 < len(args) {
				sqliteExportOutDir = args[outIdx+1]
			}
		}
		return 0, nil
	}

	outPath := filepath.Join(t.TempDir(), "report.json")
	report := runE2E("dsn", "", "/db.sqlite", "sess-1", outPath, runner)

	if report.Status != "ok" {
		t.Fatalf("status = %q, want ok", report.Status)
	}
	if report.SourceMode != "sqlite-db" || report.SQLiteDB != "/db.sqlite" {
		t.Fatalf("unexpected source fields: %+v", report)
	}
	if len(report.Steps) != 5 {
		t.Fatalf("expected 5 steps, got %d", len(report.Steps))
	}
	if len(records) != 5 {
		t.Fatalf("expected 5 runner calls, got %d", len(records))
	}
	if records[0].binary != "sqlite-export" {
		t.Fatalf("first binary = %q, want sqlite-export", records[0].binary)
	}
	if !sliceContainsAll(records[0].args, []string{"-db", "/db.sqlite", "-all"}) {
		t.Fatalf("unexpected sqlite-export args: %v", records[0].args)
	}
	if sqliteExportOutDir == "" {
		t.Fatal("expected sqlite-export -out dir to be captured")
	}
	if report.ExportDir != sqliteExportOutDir {
		t.Fatalf("report export dir = %q, want %q", report.ExportDir, sqliteExportOutDir)
	}

	// Verify subsequent steps use the temp dir as export-dir
	importArgs := records[2].args // 0: sqlite-export, 1: schema-apply, 2: import-canonical
	expDirIdx := indexOf(importArgs, "-export-dir")
	if expDirIdx == -1 || expDirIdx+1 >= len(importArgs) {
		t.Fatal("expected -export-dir in import args")
	}
	if importArgs[expDirIdx+1] != sqliteExportOutDir {
		t.Fatalf("import -export-dir = %q, want %q", importArgs[expDirIdx+1], sqliteExportOutDir)
	}

	compareArgs := records[3].args
	expDirIdx = indexOf(compareArgs, "-export-dir")
	if expDirIdx == -1 || expDirIdx+1 >= len(compareArgs) {
		t.Fatal("expected -export-dir in compare args")
	}
	if compareArgs[expDirIdx+1] != sqliteExportOutDir {
		t.Fatalf("compare -export-dir = %q, want %q", compareArgs[expDirIdx+1], sqliteExportOutDir)
	}

	// Verify sqlite-export step report path
	if report.Steps[0].Name != "sqlite-export" {
		t.Fatalf("step 0 name = %q, want sqlite-export", report.Steps[0].Name)
	}
	if report.Steps[0].Status != "ok" {
		t.Fatalf("step 0 status = %q, want ok", report.Steps[0].Status)
	}
	if report.Steps[0].ReportPath != filepath.Join(sqliteExportOutDir, "manifest.json") {
		t.Fatalf("unexpected sqlite-export manifest path: %q", report.Steps[0].ReportPath)
	}
}

func TestRunE2ESQLiteExportFails(t *testing.T) {
	runner := func(binary string, args []string) (int, error) {
		if binary == "sqlite-export" {
			return 1, fmt.Errorf("sqlite-export crashed")
		}
		return 0, nil
	}

	report := runE2E("dsn", "", "/db.sqlite", "sess-1", "", runner)
	if report.Status != "failed" {
		t.Fatalf("status = %q, want failed", report.Status)
	}
	if len(report.Steps) != 1 {
		t.Fatalf("expected 1 step, got %d", len(report.Steps))
	}
	if report.Steps[0].Name != "sqlite-export" {
		t.Fatalf("step name = %q, want sqlite-export", report.Steps[0].Name)
	}
	if report.Steps[0].Status != "failed" {
		t.Fatalf("step status = %q, want failed", report.Steps[0].Status)
	}
	if !strings.Contains(report.Steps[0].Error, "sqlite-export crashed") {
		t.Fatalf("expected sqlite-export error, got %q", report.Steps[0].Error)
	}
}

func TestRunE2EReportPathsDerived(t *testing.T) {
	runner := func(binary string, args []string) (int, error) {
		return 0, nil
	}

	outPath := filepath.Join(t.TempDir(), "e2e.json")
	report := runE2E("dsn", "/export", "", "sess-1", outPath, runner)

	expected := []string{
		"e2e-schema-apply.json",
		"e2e-import-canonical.json",
		"e2e-mariadb-compare.json",
		"e2e-canonical-route-smoke.json",
	}
	for i, exp := range expected {
		got := report.Steps[i].ReportPath
		if filepath.Base(got) != exp {
			t.Fatalf("step %d report path base = %q, want %q", i, filepath.Base(got), exp)
		}
	}
}

func TestRunE2ENoOutPathNoReportPaths(t *testing.T) {
	runner := func(binary string, args []string) (int, error) {
		return 0, nil
	}

	report := runE2E("dsn", "/export", "", "sess-1", "", runner)
	for i, step := range report.Steps {
		if step.ReportPath != "" {
			t.Fatalf("step %d report path = %q, want empty", i, step.ReportPath)
		}
	}
}

func TestBuildStepArgsExact(t *testing.T) {
	cases := []struct {
		binary   string
		wantArgs []string
	}{
		{
			binary:   "sqlite-export",
			wantArgs: []string{"-db", "/db.sqlite", "-out", "/tmp/sqlite-out", "-all"},
		},
		{
			binary:   "mariadb-schema",
			wantArgs: []string{"-dsn", "dsn", "-execute", "-out", "/tmp/schema.json"},
		},
		{
			binary:   "mariadb-import",
			wantArgs: []string{"-dsn", "dsn", "-export-dir", "/export", "-execute"},
		},
		{
			binary:   "mariadb-compare",
			wantArgs: []string{"-dsn", "dsn", "-export-dir", "/export", "-out", "/tmp/compare.json"},
		},
		{
			binary:   "canonical-route-smoke",
			wantArgs: []string{"-dsn", "dsn", "-execute", "-session-id", "sess-1"},
		},
	}

	for _, tc := range cases {
		var got []string
		switch tc.binary {
		case "sqlite-export":
			got = buildStepArgs(tc.binary, "", "/tmp/sqlite-out", "/db.sqlite", "sess-1", "")
		case "mariadb-schema":
			got = buildStepArgs(tc.binary, "dsn", "/export", "", "sess-1", "/tmp/schema.json")
		case "mariadb-import":
			got = buildStepArgs(tc.binary, "dsn", "/export", "", "sess-1", "")
		case "mariadb-compare":
			got = buildStepArgs(tc.binary, "dsn", "/export", "", "sess-1", "/tmp/compare.json")
		case "canonical-route-smoke":
			got = buildStepArgs(tc.binary, "dsn", "/export", "", "sess-1", "")
		}
		if !reflect.DeepEqual(got, tc.wantArgs) {
			t.Fatalf("binary %q: got %v, want %v", tc.binary, got, tc.wantArgs)
		}
	}
}

func TestFindCommandSourceFromGoServiceRoot(t *testing.T) {
	oldWD, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := os.Chdir(oldWD); err != nil {
			t.Fatalf("restore wd: %v", err)
		}
	})

	if err := os.Chdir(filepath.Join("..", "..")); err != nil {
		t.Fatal(err)
	}

	got, err := findCommandSource("mariadb-schema")
	if err != nil {
		t.Fatal(err)
	}
	wantSuffix := filepath.Join("cmd", "mariadb-schema")
	if !strings.HasSuffix(got, wantSuffix) {
		t.Fatalf("source path = %q, want suffix %q", got, wantSuffix)
	}
}

func TestFindCommandSourceSQLiteExportFallback(t *testing.T) {
	oldWD, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := os.Chdir(oldWD); err != nil {
			t.Fatalf("restore wd: %v", err)
		}
	})

	if err := os.Chdir(filepath.Join("..", "..")); err != nil {
		t.Fatal(err)
	}

	got, err := findCommandSource("sqlite-export")
	if err != nil {
		t.Fatal(err)
	}
	wantSuffix := filepath.Join("cmd", "sqlite-export")
	if !strings.HasSuffix(got, wantSuffix) {
		t.Fatalf("source path = %q, want suffix %q", got, wantSuffix)
	}
}

func TestWriteReportRoundTrip(t *testing.T) {
	report := &e2eReport{
		Status: "ok",
		Steps: []stepResult{
			{Name: "schema-apply", Status: "ok", ExitCode: 0},
		},
	}
	dir := t.TempDir()
	path := filepath.Join(dir, "rt.json")
	writeReport(report, path)

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var decoded e2eReport
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatal(err)
	}
	if decoded.Status != "ok" || len(decoded.Steps) != 1 {
		t.Fatalf("unexpected decoded report: %+v", decoded)
	}
}

func indexOf(slice []string, s string) int {
	for i, v := range slice {
		if v == s {
			return i
		}
	}
	return -1
}

func sliceContainsAll(haystack, needles []string) bool {
	needleSet := make(map[string]int, len(needles))
	for _, n := range needles {
		needleSet[n]++
	}
	for _, h := range haystack {
		if needleSet[h] > 0 {
			needleSet[h]--
		}
	}
	for _, v := range needleSet {
		if v != 0 {
			return false
		}
	}
	return true
}
