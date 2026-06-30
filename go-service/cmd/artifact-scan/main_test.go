package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestArtifactScanFlagsBuildArtifacts(t *testing.T) {
	tmp := t.TempDir()
	if err := os.WriteFile(filepath.Join(tmp, "archive-center.exe"), []byte("binary"), 0o600); err != nil {
		t.Fatalf("write exe: %v", err)
	}

	cmd := exec.Command("go", "run", "-buildvcs=false", "./main.go", "-root", tmp)
	out, err := cmd.CombinedOutput()
	if err == nil {
		t.Fatalf("artifact scan unexpectedly passed with exe artifact: %s", string(out))
	}
	if !strings.Contains(string(out), "[windows_binary]") {
		t.Fatalf("artifact scan output missing windows_binary risk: %s", string(out))
	}
}

func TestArtifactScanFlagsRuntimeCacheAndScratchFiles(t *testing.T) {
	tmp := t.TempDir()
	if err := os.Mkdir(filepath.Join(tmp, ".runtime"), 0o700); err != nil {
		t.Fatalf("mkdir runtime: %v", err)
	}
	for _, name := range []string{"repair_seq.py", "patch_route.py", "debug_notes.txt", "tmp_gen.py", "check_lines.py", "find_ellipsis.py"} {
		if err := os.WriteFile(filepath.Join(tmp, name), []byte("scratch"), 0o600); err != nil {
			t.Fatalf("write %s: %v", name, err)
		}
	}

	cmd := exec.Command("go", "run", "-buildvcs=false", "./main.go", "-root", tmp)
	out, err := cmd.CombinedOutput()
	if err == nil {
		t.Fatalf("artifact scan unexpectedly passed with runtime/scratch artifacts: %s", string(out))
	}
	for _, want := range []string{
		"[runtime_dir]",
		"[scratch_repair_file]",
		"[scratch_patch_file]",
		"[scratch_debug_file]",
		"[scratch_tmp_file]",
		"[scratch_check_file]",
		"[scratch_find_file]",
	} {
		if !strings.Contains(string(out), want) {
			t.Fatalf("artifact scan output missing %s risk: %s", want, string(out))
		}
	}
}

func TestArtifactScanPassesCleanTempRoot(t *testing.T) {
	tmp := t.TempDir()
	if err := os.WriteFile(filepath.Join(tmp, "README.md"), []byte("ok"), 0o600); err != nil {
		t.Fatalf("write readme: %v", err)
	}
	if err := os.WriteFile(filepath.Join(tmp, ".env.example"), []byte("AC_EXAMPLE=1"), 0o600); err != nil {
		t.Fatalf("write env example: %v", err)
	}
	toolsDir := filepath.Join(tmp, "tools")
	if err := os.Mkdir(toolsDir, 0o700); err != nil {
		t.Fatalf("mkdir tools: %v", err)
	}
	if err := os.WriteFile(filepath.Join(toolsDir, "check_runtime.py"), []byte("print('ok')"), 0o600); err != nil {
		t.Fatalf("write tools check: %v", err)
	}

	cmd := exec.Command("go", "run", "-buildvcs=false", "./main.go", "-root", tmp)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("artifact scan failed on clean root: %v\n%s", err, string(out))
	}
	if !strings.Contains(string(out), "PASS") {
		t.Fatalf("artifact scan output missing PASS: %s", string(out))
	}
}
