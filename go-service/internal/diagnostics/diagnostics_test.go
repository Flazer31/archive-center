package diagnostics

import (
	"bytes"
	"encoding/json"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"testing"
)

func TestCrashReportCapturesGoroutinePanic(t *testing.T) {
	if dir := os.Getenv("AC_DIAGNOSTIC_CRASH_FIXTURE"); dir != "" {
		if err := EnableCrashLog(dir); err != nil {
			panic(err)
		}
		go func() { panic("fixture goroutine crash cause") }()
		select {}
	}
	dir := t.TempDir()
	cmd := exec.Command(os.Args[0], "-test.run=^TestCrashReportCapturesGoroutinePanic$")
	cmd.Env = append(os.Environ(), "AC_DIAGNOSTIC_CRASH_FIXTURE="+dir)
	if err := cmd.Run(); err == nil {
		t.Fatal("fixture should crash")
	}
	report := Collect(dir, "fixture")
	data, _ := json.Marshal(report)
	if !strings.Contains(string(data), "fixture goroutine crash cause") || !strings.Contains(string(data), "diagnostics_test.go") {
		t.Fatalf("panic cause or stack absent: %s", data)
	}
}

func TestDurableReportRotationAndRedaction(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "어드민 package", "logs")
	w := &Writer{Dir: dir, Limit: 2048, Secrets: []string{"synthetic-key-123"}}
	logger := slog.New(slog.NewJSONHandler(w, nil))
	var workers sync.WaitGroup
	for i := 0; i < 8; i++ {
		workers.Add(1)
		go func() {
			defer workers.Done()
			for j := 0; j < 30; j++ {
				logger.Error("fixture error", "error", "password=private-value Bearer synthetic-key-123", "padding", strings.Repeat("x", 80))
			}
		}()
	}
	workers.Wait()
	logger.Error("last durable failure", "error", "api_key=synthetic-key-123 user:dbpassword@tcp(127.0.0.1:3307)/db")
	// A fresh reader, with no reference to the logger, models backend restart/offline export.
	report := Collect(dir, "fixture")
	if len(report.Files) != 4 {
		t.Fatalf("rotation: got %d files", len(report.Files))
	}
	all := ""
	for _, f := range report.Files {
		if f.Error != "" {
			t.Fatal(f.Error)
		}
		if len(f.Text) > 2048 {
			t.Fatalf("unbounded log: %d", len(f.Text))
		}
		for _, line := range strings.Split(strings.TrimSpace(f.Text), "\n") {
			if !json.Valid([]byte(line)) {
				t.Fatalf("invalid log JSON: %s", line)
			}
		}
		all += f.Text
	}
	if !strings.Contains(all, "last durable failure") {
		t.Fatal("last error lost")
	}
	for _, secret := range []string{"synthetic-key-123", "private-value", "dbpassword"} {
		if strings.Contains(all, secret) {
			t.Fatalf("secret leaked: %s", secret)
		}
	}
	if err := os.WriteFile(filepath.Join(dir, "private.env"), []byte("do not export"), 0600); err != nil {
		t.Fatal(err)
	}
	if len(Collect(dir, "fixture").Files) != 4 {
		t.Fatal("unrelated file exported")
	}
}

func TestLogDiskFailureAndBoundedRead(t *testing.T) {
	dir := t.TempDir()
	block := filepath.Join(dir, "not-a-directory")
	if err := os.WriteFile(block, []byte("file"), 0600); err != nil {
		t.Fatal(err)
	}
	var console bytes.Buffer
	w := &Writer{Dir: block, Console: &console}
	if n, err := w.Write([]byte("original startup failure\n")); err != nil || n == 0 {
		t.Fatal("logging changed caller result")
	}
	if w.Error() == "" || !strings.Contains(console.String(), "diagnostic log write failed") || !strings.Contains(console.String(), "original startup failure") {
		t.Fatal(console.String())
	}
	if err := os.WriteFile(filepath.Join(dir, "launcher.err.log"), []byte(strings.Repeat("line\n", 20000)+"FINAL CAUSE\n"), 0600); err != nil {
		t.Fatal(err)
	}
	report := Collect(dir, "fixture")
	if len(report.Files) != 1 || !report.Files[0].Truncated || len(report.Files[0].Text) > TailBytes || !strings.Contains(report.Files[0].Text, "FINAL CAUSE") {
		t.Fatalf("bounded tail missing: %+v", report)
	}
}
