package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestResolveFixtureDirDefaultsToBenchmarkExport(t *testing.T) {
	benchmarkDir := filepath.Join("M:", "risulongmemory", "Archive Center 2.0", "benchmarks")
	got := resolveFixtureDir("", benchmarkDir)
	want := filepath.Join(benchmarkDir, defaultFixtureExportDirName)
	if got != want {
		t.Fatalf("resolveFixtureDir = %q, want %q", got, want)
	}
}

func TestGoBackendEnvUsesFixtureShadowStore(t *testing.T) {
	fixtureDir := filepath.Join("M:", "risulongmemory", "Archive Center 2.0", "benchmarks", defaultFixtureExportDirName)
	env := strings.Join(goBackendEnv(28293, fixtureDir), "\n")
	for _, want := range []string{
		"AC_BIND_ADDR=127.0.0.1:28293",
		"AC_STORE_MODE=fixture_shadow",
		"AC_STORE_FIXTURE_DIR=" + fixtureDir,
	} {
		if !strings.Contains(env, want) {
			t.Fatalf("goBackendEnv missing %q in:\n%s", want, env)
		}
	}
}

func TestGoBackendEnvCarriesReferenceEmbeddingModel(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "backend"), 0755); err != nil {
		t.Fatal(err)
	}
	envPath := filepath.Join(root, "backend", ".env")
	if err := os.WriteFile(envPath, []byte("PROJECT_EMBEDDING_MODEL=voyage-4-large\n"), 0644); err != nil {
		t.Fatal(err)
	}
	env := strings.Join(goBackendEnv(28293, "fixture", root), "\n")
	if !strings.Contains(env, "PROJECT_EMBEDDING_MODEL=voyage-4-large") {
		t.Fatalf("reference embedding model was not carried into Go env:\n%s", env)
	}
	wantPersist := "AC_CHROMA_SHADOW_PERSIST_DIR=" + filepath.Join(root, ".chroma_shadow")
	if !strings.Contains(env, wantPersist) {
		t.Fatalf("reference Chroma persist dir was not carried into Go env; missing %q in:\n%s", wantPersist, env)
	}
}

func TestCopy08ToTemp(t *testing.T) {
	src := t.TempDir()
	for rel, content := range map[string]string{
		filepath.Join("backend", "main.py"):              "print('fixture')\n",
		"Archive Center.js":                              "//@api 3.0\n",
		"memory.db":                                      "fixture",
		filepath.Join("backend", ".venv", "ignored.txt"): "must not be copied",
	} {
		path := filepath.Join(src, rel)
		if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(content), 0644); err != nil {
			t.Fatal(err)
		}
	}
	dst, err := os.MkdirTemp("", "fixture-live-test-*")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(dst)

	if err := copy08ToTemp(src, dst); err != nil {
		t.Fatalf("copy08ToTemp failed: %v", err)
	}

	for _, rel := range []string{"backend\\main.py", "Archive Center.js", "memory.db"} {
		p := filepath.Join(dst, rel)
		if _, err := os.Stat(p); os.IsNotExist(err) {
			t.Errorf("expected file not found: %s", p)
		}
	}

	venvPath := filepath.Join(dst, "backend", ".venv")
	if _, err := os.Stat(venvPath); !os.IsNotExist(err) {
		t.Errorf(".venv should be excluded from fixture copy, stat err=%v", err)
	}
}

func TestMakeJunction(t *testing.T) {
	src, err := os.MkdirTemp("", "junction-src-*")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(src)
	dst, err := os.MkdirTemp("", "junction-dst-*")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(dst)

	junctionDir := filepath.Join(dst, "link")
	if err := makeJunction(junctionDir, src); err != nil {
		t.Fatalf("makeJunction failed: %v", err)
	}

	info, err := os.Stat(junctionDir)
	if err != nil {
		t.Fatalf("stat junction failed: %v", err)
	}
	if !info.IsDir() {
		t.Error("junction should be a directory")
	}
}

func TestGoBackendEnvCarriesChromaShadowPersistDir(t *testing.T) {
	root := t.TempDir()
	env := strings.Join(goBackendEnv(28293, "fixture", root), "\n")
	want := "AC_CHROMA_SHADOW_PERSIST_DIR=" + filepath.Join(root, ".chroma_shadow")
	if !strings.Contains(env, want) {
		t.Fatalf("goBackendEnv missing %q in:\n%s", want, env)
	}
}
