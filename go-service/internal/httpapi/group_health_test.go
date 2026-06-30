package httpapi

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/risulongmemory/archive-center-go/internal/config"
)

func TestHandlePromptsListDefaultPromptCatalog(t *testing.T) {
	mux := http.NewServeMux()
	srv := setupTestServer()
	srv.RegisterRoutes(mux)

	req := httptest.NewRequest(http.MethodGet, "/prompts", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, rec.Code)
	}

	var resp map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if resp["status"] != "ok" {
		t.Errorf("status = %v, want ok", resp["status"])
	}
	if resp["source"] != "prompt_dir_live" {
		t.Errorf("source = %v, want prompt_dir_live", resp["source"])
	}
	if resp["prompt_source"] != "not_configured" {
		t.Errorf("prompt_source = %v, want not_configured", resp["prompt_source"])
	}
	if int(resp["count"].(float64)) != len(promptCatalog()) {
		t.Errorf("count = %v, want %d", resp["count"], len(promptCatalog()))
	}

	items := resp["items"].([]any)
	if len(items) != len(promptCatalog()) {
		t.Fatalf("items length = %d, want %d", len(items), len(promptCatalog()))
	}
	first := items[0].(map[string]any)
	if first["write_enabled"] != false {
		t.Errorf("write_enabled = %v, want false", first["write_enabled"])
	}
	if _, ok := first["content"]; ok {
		t.Error("list response must not include prompt content")
	}
}

func TestHandlePromptGetReadsConfiguredPromptEvidence(t *testing.T) {
	dir := t.TempDir()
	content := "Supervisor authority fixture\n"
	if err := os.WriteFile(filepath.Join(dir, "supervisor_system.txt"), []byte(content), 0o600); err != nil {
		t.Fatalf("failed to write prompt fixture: %v", err)
	}

	cfg := config.Default()
	cfg.PromptDir = dir
	mux := http.NewServeMux()
	srv := NewServer(cfg)
	srv.RegisterRoutes(mux)

	req := httptest.NewRequest(http.MethodGet, "/prompts/supervisor_system.txt", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, rec.Code)
	}

	var resp map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if resp["prompt_source"] != "configured" {
		t.Errorf("prompt_source = %v, want configured", resp["prompt_source"])
	}
	prompt := resp["prompt"].(map[string]any)
	if prompt["name"] != "supervisor_system" {
		t.Errorf("name = %v, want supervisor_system", prompt["name"])
	}
	if prompt["filename"] != "supervisor_system.txt" {
		t.Errorf("filename = %v, want supervisor_system.txt", prompt["filename"])
	}
	if prompt["available"] != true {
		t.Errorf("available = %v, want true", prompt["available"])
	}
	if prompt["content"] != content {
		t.Errorf("content = %q, want %q", prompt["content"], content)
	}
	if prompt["write_enabled"] != true {
		t.Errorf("write_enabled = %v, want true", prompt["write_enabled"])
	}
	if got := prompt["sha256"].(string); len(got) != 64 {
		t.Errorf("sha256 length = %d, want 64", len(got))
	}
}

func TestHandlePromptGetAccepts08PromptKey(t *testing.T) {
	dir := t.TempDir()
	content := "0.8 compatible prompt key\n"
	if err := os.WriteFile(filepath.Join(dir, "supervisor_system.txt"), []byte(content), 0o600); err != nil {
		t.Fatalf("failed to write prompt fixture: %v", err)
	}

	cfg := config.Default()
	cfg.PromptDir = dir
	mux := http.NewServeMux()
	srv := NewServer(cfg)
	srv.RegisterRoutes(mux)

	req := httptest.NewRequest(http.MethodGet, "/prompts/supervisor_system", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, rec.Code)
	}

	var resp map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	prompt := resp["prompt"].(map[string]any)
	if prompt["name"] != "supervisor_system" {
		t.Errorf("name = %v, want supervisor_system", prompt["name"])
	}
	if prompt["filename"] != "supervisor_system.txt" {
		t.Errorf("filename = %v, want supervisor_system.txt", prompt["filename"])
	}
	if prompt["content"] != content {
		t.Errorf("content = %q, want %q", prompt["content"], content)
	}
}

func TestHandlePromptPutUpdatesConfiguredPromptFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "critic_system.txt")
	if err := os.WriteFile(path, []byte("old critic prompt\n"), 0o600); err != nil {
		t.Fatalf("failed to write prompt fixture: %v", err)
	}

	cfg := config.Default()
	cfg.PromptDir = dir
	mux := http.NewServeMux()
	srv := NewServer(cfg)
	srv.RegisterRoutes(mux)

	req := httptest.NewRequest(http.MethodPut, "/prompts/critic_system", strings.NewReader(`{"content":"new critic prompt\r\nline two"}`))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d: %s", http.StatusOK, rec.Code, rec.Body.String())
	}

	gotBytes, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("failed to read prompt fixture: %v", err)
	}
	if got := string(gotBytes); got != "new critic prompt\nline two" {
		t.Fatalf("file content = %q, want normalized prompt", got)
	}

	var resp map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if resp["source"] != "prompt_dir_live" {
		t.Errorf("source = %v, want prompt_dir_live", resp["source"])
	}
	prompt := resp["prompt"].(map[string]any)
	if prompt["name"] != "critic_system" {
		t.Errorf("name = %v, want critic_system", prompt["name"])
	}
	if prompt["content"] != "new critic prompt\nline two" {
		t.Errorf("content = %q, want normalized content", prompt["content"])
	}
	if prompt["write_enabled"] != true {
		t.Errorf("write_enabled = %v, want true", prompt["write_enabled"])
	}
}

func TestHandlePromptPutRejectsEmptyPromptContent(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "supervisor_system.txt"), []byte("existing\n"), 0o600); err != nil {
		t.Fatalf("failed to write prompt fixture: %v", err)
	}

	cfg := config.Default()
	cfg.PromptDir = dir
	mux := http.NewServeMux()
	srv := NewServer(cfg)
	srv.RegisterRoutes(mux)

	req := httptest.NewRequest(http.MethodPut, "/prompts/supervisor_system", strings.NewReader(`{"content":"   "}`))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected status %d, got %d", http.StatusBadRequest, rec.Code)
	}
}

func TestHandlePromptGetRejectsUnknownPromptName(t *testing.T) {
	mux := http.NewServeMux()
	srv := setupTestServer()
	srv.RegisterRoutes(mux)

	req := httptest.NewRequest(http.MethodGet, "/prompts/unknown.txt", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected status %d, got %d", http.StatusNotFound, rec.Code)
	}
}

func TestPromptNameCatalogGuardRejectsPathTraversal(t *testing.T) {
	for _, name := range []string{"../critic_system.txt", `..\critic_system.txt`, "nested/critic_system.txt", ""} {
		if isKnownPromptName(name) {
			t.Errorf("isKnownPromptName(%q) = true, want false", name)
		}
	}
}

// TestSeq13P196PromptEditorEndpointContract verifies P196 backend contracts:
// PUT rejects unknown prompt names, PUT rejects path traversal, PUT with .txt works.
func TestSeq13P196PromptEditorEndpointContract(t *testing.T) {
	t.Run("put_rejects_unknown_prompt_name", func(t *testing.T) {
		mux := http.NewServeMux()
		srv := setupTestServer()
		srv.RegisterRoutes(mux)

		req := httptest.NewRequest(http.MethodPut, "/prompts/unknown_system.txt", strings.NewReader(`{"content":"test"}`))
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()
		mux.ServeHTTP(rec, req)

		if rec.Code != http.StatusNotFound {
			t.Fatalf("expected status %d for unknown prompt PUT, got %d", http.StatusNotFound, rec.Code)
		}
	})

	t.Run("put_rejects_path_traversal", func(t *testing.T) {
		dir := t.TempDir()
		if err := os.WriteFile(filepath.Join(dir, "supervisor_system.txt"), []byte("safe\n"), 0o600); err != nil {
			t.Fatalf("failed to write prompt fixture: %v", err)
		}

		cfg := config.Default()
		cfg.PromptDir = dir
		mux := http.NewServeMux()
		srv := NewServer(cfg)
		srv.RegisterRoutes(mux)

		// Path with ".." is rejected by the router (307 redirect), so test a matched path that fails catalog guard.
		req := httptest.NewRequest(http.MethodPut, "/prompts/nested/critic_system.txt", strings.NewReader(`{"content":"evil"}`))
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()
		mux.ServeHTTP(rec, req)

		if rec.Code != http.StatusNotFound {
			t.Fatalf("expected status %d for path traversal PUT, got %d", http.StatusNotFound, rec.Code)
		}
	})

	t.Run("put_with_txt_extension_updates_file", func(t *testing.T) {
		dir := t.TempDir()
		path := filepath.Join(dir, "critic_system.txt")
		if err := os.WriteFile(path, []byte("old\n"), 0o600); err != nil {
			t.Fatalf("failed to write prompt fixture: %v", err)
		}

		cfg := config.Default()
		cfg.PromptDir = dir
		mux := http.NewServeMux()
		srv := NewServer(cfg)
		srv.RegisterRoutes(mux)

		req := httptest.NewRequest(http.MethodPut, "/prompts/critic_system.txt", strings.NewReader(`{"content":"new content"}`))
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()
		mux.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("expected status %d, got %d: %s", http.StatusOK, rec.Code, rec.Body.String())
		}

		gotBytes, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("failed to read updated file: %v", err)
		}
		if string(gotBytes) != "new content" {
			t.Fatalf("file content = %q, want %q", string(gotBytes), "new content")
		}
	})
}
