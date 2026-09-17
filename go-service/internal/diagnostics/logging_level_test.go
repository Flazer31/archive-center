package diagnostics

import (
	"bytes"
	"log/slog"
	"strings"
	"sync"
	"testing"
)

func TestLoggingLevelChangesFilterWithoutRestart(t *testing.T) {
	var console bytes.Buffer
	w := &Writer{Dir: t.TempDir(), Console: &console}
	log := slog.New(slog.NewJSONHandler(w, &slog.HandlerOptions{Level: w}))
	log.Debug("hidden-before")
	log.Info("basic-event")
	if err := w.SetLevel(" DEBUG "); err != nil {
		t.Fatal(err)
	}
	log.Debug("visible-detail")
	before := w.LoggingState()
	if before.Level != "debug" || before.ChangedAt.IsZero() || before.Scope != "backend_process" {
		t.Fatal(before)
	}
	if err := w.SetLevel("trace"); err == nil {
		t.Fatal("invalid level accepted")
	}
	if w.LoggingState() != before {
		t.Fatal("invalid value changed active level")
	}
	if err := w.SetLevel("info"); err != nil {
		t.Fatal(err)
	}
	log.Debug("hidden-after")
	log.Error("always-error")
	for _, value := range []string{"basic-event", "visible-detail", "always-error"} {
		if !strings.Contains(console.String(), value) {
			t.Fatal(value)
		}
	}
	if strings.Contains(console.String(), "hidden-") {
		t.Fatal(console.String())
	}
	// UI control is process-local: opening a new writer does not read an old mode.
	if (&Writer{Dir: w.Dir}).Level() != slog.LevelInfo {
		t.Fatal("mode persisted")
	}
}

func TestLoggingConcurrentToggleKeepsBoundedRedactedFiles(t *testing.T) {
	w := &Writer{Dir: t.TempDir(), Limit: 2048}
	log := slog.New(slog.NewJSONHandler(w, &slog.HandlerOptions{Level: w}))
	var wg sync.WaitGroup
	for i := 0; i < 4; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < 30; j++ {
				_ = w.SetLevel("debug")
				log.Debug("detail", "error", "api_key=private-value", "padding", strings.Repeat("x", 80))
				_ = w.SetLevel("info")
				_ = w.LoggingState()
			}
		}()
	}
	wg.Wait()
	report := Collect(w.Dir, "fixture")
	if len(report.Files) != 4 {
		t.Fatalf("rotation: %d", len(report.Files))
	}
	for _, file := range report.Files {
		if len(file.Text) > 2048 || strings.Contains(file.Text, "private-value") {
			t.Fatal("unbounded or unredacted log")
		}
	}
}
