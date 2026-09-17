// Package diagnostics keeps bounded operator logs independently of the database.
package diagnostics

import (
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"runtime/debug"
	"sort"
	"strings"
	"sync"
	"time"
)

const MaxFileBytes = 4 << 20
const TailBytes = 64 << 10

var patterns = []*regexp.Regexp{
	regexp.MustCompile(`(?i)(bearer\s+)[^\s"',;]+`),
	regexp.MustCompile(`(?i)((?:api[_-]?key|access[_-]?token|authorization|password|passwd|secret|token)["']?\s*[:=]\s*["']?)[^\s"',;&}]+`),
	regexp.MustCompile(`(?i)(https?://)[^/\s:@]+:[^/\s@]+@`),
	regexp.MustCompile(`[^\s"':]+:[^\s"']+@(?:tcp|unix)\([^)]*\)`),
	regexp.MustCompile(`(?:sk-[A-Za-z0-9_-]{8,}|AIza[A-Za-z0-9_-]{20,})`),
}

func Redact(text string, secrets ...string) string {
	// Replace JSON-escaped as well as literal credential values.
	for _, secret := range secrets {
		if secret != "" {
			text = strings.ReplaceAll(text, secret, "[REDACTED]")
			encoded, _ := json.Marshal(secret)
			text = strings.ReplaceAll(text, string(encoded[1:len(encoded)-1]), "[REDACTED]")
		}
	}
	for i, re := range patterns {
		replacement := "${1}[REDACTED]"
		if i >= 3 {
			replacement = "[REDACTED]"
		}
		text = re.ReplaceAllString(text, replacement)
	}
	return strings.ToValidUTF8(text, "�")
}

func Directory() string {
	if dir := os.Getenv("AC_LOG_DIR"); dir != "" {
		return dir
	}
	if dir := os.Getenv("ARCHIVE_CENTER_DATA_DIR"); dir != "" {
		return filepath.Join(dir, "logs")
	}
	if runtime.GOOS == "windows" && os.Getenv("LOCALAPPDATA") != "" {
		return filepath.Join(os.Getenv("LOCALAPPDATA"), "ArchiveCenter", "data", "logs")
	}
	if home, err := os.UserHomeDir(); err == nil {
		return filepath.Join(home, ".archive-center", "logs")
	}
	return filepath.Join(os.TempDir(), "archive-center-logs")
}

// Writer opens and closes each append, including fatal errors followed by os.Exit.
// No request bodies or in-memory history are retained. Disk failure remains visible
// on stderr without turning a diagnostic failure into a chat failure.
type Writer struct {
	mu         sync.Mutex
	Dir        string
	Console    io.Writer
	Secrets    []string
	lastError  string
	Limit      int64
	level      slog.LevelVar
	levelSince time.Time
}

// LoggingState describes this process only; changing it never writes settings or DB.
type LoggingState struct {
	Level     string    `json:"level"`
	ChangedAt time.Time `json:"changed_at"`
	Scope     string    `json:"scope"`
}

func (w *Writer) Level() slog.Level { return w.level.Level() }

func (w *Writer) SetLevel(value string) error {
	var level slog.Level
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "", "info":
		level = slog.LevelInfo
	case "debug":
		level = slog.LevelDebug
	default:
		return fmt.Errorf("log level must be info or debug")
	}
	w.mu.Lock()
	defer w.mu.Unlock()
	w.level.Set(level)
	w.levelSince = time.Now().UTC()
	return nil
}

func (w *Writer) LoggingState() LoggingState {
	w.mu.Lock()
	defer w.mu.Unlock()
	return LoggingState{Level: strings.ToLower(w.level.Level().String()), ChangedAt: w.levelSince, Scope: "backend_process"}
}

func (w *Writer) Write(p []byte) (int, error) {
	w.mu.Lock()
	defer w.mu.Unlock()
	clean := []byte(Redact(string(p), w.Secrets...))
	if w.Console != nil {
		_, _ = w.Console.Write(clean)
	}
	limit := w.Limit
	if limit <= 0 {
		limit = MaxFileBytes
	}
	if int64(len(clean)) > limit {
		clean = clean[:limit]
		clean[len(clean)-1] = '\n'
	}
	err := os.MkdirAll(w.Dir, 0700)
	path := filepath.Join(w.Dir, "backend.log")
	if err == nil {
		if info, statErr := os.Stat(path); statErr == nil && info.Size()+int64(len(clean)) > limit {
			err = rotate(path)
		}
	}
	if err == nil {
		var f *os.File
		f, err = os.OpenFile(path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0600)
		if err == nil {
			_, err = f.Write(clean)
			if closeErr := f.Close(); err == nil {
				err = closeErr
			}
		}
	}
	if err != nil {
		w.lastError = Redact(err.Error(), w.Secrets...)
		if w.Console != nil {
			_, _ = fmt.Fprintf(w.Console, "Archive Center diagnostic log write failed: %s\n", w.lastError)
		}
	} else {
		w.lastError = ""
	}
	return len(p), nil
}

func rotate(path string) error {
	if err := os.Remove(path + ".3"); err != nil && !os.IsNotExist(err) {
		return err
	}
	for n := 2; n >= 0; n-- {
		old := path
		if n > 0 {
			old = fmt.Sprintf("%s.%d", path, n)
		}
		if err := os.Rename(old, fmt.Sprintf("%s.%d", path, n+1)); err != nil && !os.IsNotExist(err) {
			return err
		}
	}
	return nil
}

func (w *Writer) Error() string { w.mu.Lock(); defer w.mu.Unlock(); return w.lastError }

// The runtime duplicates the file descriptor; it captures unhandled goroutine
// panics as well as main-goroutine panics, independently of slog and the database.
func EnableCrashLog(dir string) error {
	if err := os.MkdirAll(dir, 0700); err != nil {
		return err
	}
	path := filepath.Join(dir, "backend-crash.log")
	if err := rotate(path); err != nil {
		return err
	}
	f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0600)
	if err != nil {
		return err
	}
	defer f.Close()
	return debug.SetCrashOutput(f, debug.CrashOptions{})
}

type LogFile struct {
	Name      string `json:"name"`
	Text      string `json:"text,omitempty"`
	Error     string `json:"error,omitempty"`
	Truncated bool   `json:"truncated,omitempty"`
}

type Report struct {
	Contract     string        `json:"contract_version"`
	GeneratedAt  time.Time     `json:"generated_at"`
	OS           string        `json:"os"`
	Arch         string        `json:"arch"`
	Version      string        `json:"version"`
	LogDirectory string        `json:"log_directory"`
	LogError     string        `json:"log_error,omitempty"`
	Files        []LogFile     `json:"files"`
	Logging      *LoggingState `json:"logging,omitempty"`
}

// Collect only known diagnostic files. Never enumerate data/config directories.
func Collect(dir, version string, secrets ...string) Report {
	report := Report{Contract: "archive-center.diagnostics.v1", GeneratedAt: time.Now().UTC(), OS: runtime.GOOS, Arch: runtime.GOARCH, Version: version, LogDirectory: dir, Files: []LogFile{}}
	entries, err := os.ReadDir(dir)
	if err != nil {
		report.LogError = Redact(err.Error(), secrets...)
	}
	names := []string{}
	for _, e := range entries {
		name := e.Name()
		base := strings.TrimSuffix(strings.TrimSuffix(strings.TrimSuffix(name, ".1"), ".2"), ".3")
		switch base {
		case "backend.log", "backend-crash.log", "launcher.log", "launcher.out.log", "launcher.err.log", "chromadb.out.log", "chromadb.err.log", "mariadb.log", "mariadb.out.log", "mariadb.err.log", "mariadb-init.log", "schema.log", "install.log", "update.log", "update-candidate.out.log", "update-candidate.err.log":
			if e.Type().IsRegular() {
				names = append(names, name)
			}
		}
	}
	sort.Strings(names)
	paths := map[string]string{}
	for _, name := range names {
		paths[name] = filepath.Join(dir, name)
	}
	// Fresh-install logging survives cleanup of an unsuccessful install root.
	bootstrapRoot, _ := os.UserHomeDir()
	if runtime.GOOS == "windows" {
		bootstrapRoot = os.Getenv("LOCALAPPDATA")
	}
	if bootstrapRoot != "" {
		bootstrap := filepath.Join(bootstrapRoot, "ArchiveCenter-install.log")
		if info, err := os.Lstat(bootstrap); err == nil && info.Mode().IsRegular() {
			names = append(names, "installer-bootstrap.log")
			paths["installer-bootstrap.log"] = bootstrap
		}
	}
	for _, name := range names {
		item := LogFile{Name: name}
		f, err := os.Open(paths[name])
		if err == nil {
			var info os.FileInfo
			info, err = f.Stat()
			if err == nil && info.Size() > TailBytes {
				item.Truncated = true
				_, err = f.Seek(-TailBytes, io.SeekEnd)
			}
			if err == nil {
				var data []byte
				data, err = io.ReadAll(io.LimitReader(f, TailBytes))
				if item.Truncated {
					if pos := strings.IndexByte(string(data), '\n'); pos >= 0 {
						data = data[pos+1:]
					}
				}
				item.Text = Redact(string(data), secrets...)
			}
			_ = f.Close()
		}
		if err != nil {
			item.Error = Redact(err.Error(), secrets...)
		}
		report.Files = append(report.Files, item)
	}
	return report
}
