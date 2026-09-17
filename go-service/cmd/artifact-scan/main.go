//go:build ignore

// Artifact-scan is a workspace hygiene gate for Archive Center 2.0.
// It walks the workspace root and reports any risky artifact that
// must not be included in a release manifest.
//
// Usage:
//
//	go run ./cmd/artifact-scan/main.go -root ".."
//
// Exit code 0 when no risky artifacts are found, 1 otherwise.
package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// exclusionRules mirror the patterns in .gitignore and the H-4e gate.
// Every rule has a name and a matcher function.
type exclusionRule struct {
	name    string
	pattern string
	isDir   bool
}

var rules = []exclusionRule{
	{name: "env_file", pattern: ".env", isDir: false},
	{name: "env_file_dot", pattern: ".env.", isDir: false},
	{name: "db_file", pattern: ".db", isDir: false},
	{name: "db_wal", pattern: ".db-wal", isDir: false},
	{name: "db_shm", pattern: ".db-shm", isDir: false},
	{name: "sqlite_file", pattern: ".sqlite", isDir: false},
	{name: "sqlite3_file", pattern: ".sqlite3", isDir: false},
	{name: "pycache_dir", pattern: "__pycache__", isDir: true},
	{name: "pytest_cache_dir", pattern: ".pytest_cache", isDir: true},
	{name: "pyc_file", pattern: ".pyc", isDir: false},
	{name: "venv_dir", pattern: ".venv", isDir: true},
	{name: "log_file", pattern: ".log", isDir: false},
	{name: "bak_file", pattern: ".bak", isDir: false},
	{name: "backup_file", pattern: ".backup", isDir: false},
	{name: "backup_file2", pattern: ".backup_", isDir: false},
	{name: "tmp_file", pattern: "_tmp_", isDir: false},
	{name: "tmp_dir", pattern: "tmp", isDir: true},
	{name: "temp_dir", pattern: "temp", isDir: true},
	{name: "cache_file", pattern: ".cache", isDir: false},
	{name: "cache_dir", pattern: "cache", isDir: true},
	{name: "dotcache_dir", pattern: ".cache", isDir: true},
	{name: "caches_dir", pattern: "caches", isDir: true},
	{name: "runtime_dir", pattern: ".runtime", isDir: true},
	{name: "runtime_cache_dir", pattern: ".runtime-cache", isDir: true},
	{name: "windows_binary", pattern: ".exe", isDir: false},
	{name: "windows_dll", pattern: ".dll", isDir: false},
	{name: "shared_object", pattern: ".so", isDir: false},
	{name: "dylib_file", pattern: ".dylib", isDir: false},
	// .git is intentionally omitted; it is expected in any cloned workspace.
	{name: "chroma_shadow_dir", pattern: ".chroma_shadow", isDir: true},
	{name: "chroma_data_dir", pattern: "chroma_data", isDir: true},
	{name: "milvus_data_dir", pattern: "milvus_data", isDir: true},
	{name: "milvus_db_file", pattern: "milvus.db", isDir: false},
	{name: "gocache_dir", pattern: ".gocache", isDir: true},
	{name: "ds_store", pattern: ".DS_Store", isDir: false},
	{name: "thumbs_db", pattern: "Thumbs.db", isDir: false},
}

func allowedFileName(name string, info os.FileInfo) bool {
	if info.IsDir() {
		return false
	}
	return strings.EqualFold(name, ".env.example")
}

func scratchRiskName(rel string, name string, info os.FileInfo) string {
	if info.IsDir() {
		return ""
	}
	parts := strings.Split(filepath.ToSlash(rel), "/")
	if len(parts) > 0 && parts[0] == "tools" {
		return ""
	}
	lower := strings.ToLower(name)
	ext := strings.ToLower(filepath.Ext(lower))
	if ext != ".py" && ext != ".txt" {
		return ""
	}
	switch {
	case strings.HasPrefix(lower, "tmp_"):
		return "scratch_tmp_file"
	case strings.HasPrefix(lower, "repair"):
		return "scratch_repair_file"
	case strings.HasPrefix(lower, "patch_"):
		return "scratch_patch_file"
	case strings.HasPrefix(lower, "debug"):
		return "scratch_debug_file"
	case strings.HasPrefix(lower, "check_"):
		return "scratch_check_file"
	case strings.HasPrefix(lower, "find_"):
		return "scratch_find_file"
	}
	return ""
}

func matches(name string, info os.FileInfo, r exclusionRule) bool {
	if r.isDir && !info.IsDir() {
		return false
	}
	if !r.isDir && info.IsDir() {
		return false
	}
	// Exact match for dir names, suffix match for file extensions / prefixes.
	if r.isDir {
		return name == r.pattern
	}
	// File matching
	switch {
	case strings.HasSuffix(name, r.pattern):
		return true
	case strings.HasPrefix(name, r.pattern):
		return true
	case name == r.pattern:
		return true
	}
	return false
}

func main() {
	root := flag.String("root", ".", "workspace root to scan")
	flag.Parse()

	absRoot, err := filepath.Abs(*root)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error resolving root: %v\n", err)
		os.Exit(1)
	}

	var risks []string

	filepath.Walk(absRoot, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil // skip unreadable
		}
		if path == absRoot {
			return nil
		}
		name := filepath.Base(path)
		rel, _ := filepath.Rel(absRoot, path)
		if allowedFileName(name, info) {
			return nil
		}

		// Check if any rule matches
		for _, r := range rules {
			if matches(name, info, r) {
				risks = append(risks, fmt.Sprintf("[%s] %s", r.name, rel))
				// If it's a directory, skip descending to keep scan fast
				if info.IsDir() {
					return filepath.SkipDir
				}
				break
			}
		}
		if riskName := scratchRiskName(rel, name, info); riskName != "" {
			risks = append(risks, fmt.Sprintf("[%s] %s", riskName, rel))
		}
		return nil
	})

	if len(risks) == 0 {
		fmt.Println("PASS: no risky artifacts found")
		os.Exit(0)
	}

	fmt.Fprintf(os.Stderr, "FAIL: %d risky artifact(s) found\n", len(risks))
	for _, r := range risks {
		fmt.Println(r)
	}
	os.Exit(1)
}
