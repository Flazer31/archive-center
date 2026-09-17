package main

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/risulongmemory/archive-center-go/internal/config"
	"github.com/risulongmemory/archive-center-go/internal/httpapi"
)

func TestManagedUpdateLauncherAuthorizationRequiresAndConsumesSession(t *testing.T) {
	staging := t.TempDir()
	cfg := config.Default()
	cfg.UpdateStagingDir = staging
	t.Setenv("AC_UPDATE_APPLY_MODE", httpapi.UpdateApplyManagedLauncherMode)
	t.Setenv("AC_UPDATE_LAUNCHER_TOKEN", "0123456789abcdef0123456789abcdef") // gitleaks:allow -- deterministic test fixture
	path := filepath.Join(staging, "launcher-session.json")
	if err := os.WriteFile(path, []byte(`{"contract_version":"archive-center.update-launcher-session.v1","token":"0123456789abcdef0123456789abcdef"}`), 0o600); err != nil { // gitleaks:allow -- deterministic test fixture
		t.Fatal(err)
	}
	if !managedUpdateLauncherAuthorized(cfg) {
		t.Fatal("valid managed launcher session was rejected")
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatalf("launcher session was not consumed: %v", err)
	}
	if managedUpdateLauncherAuthorized(cfg) {
		t.Fatal("consumed launcher session was replayed")
	}
}

func TestManagedUpdateLauncherAuthorizationRejectsEnvironmentOnly(t *testing.T) {
	cfg := config.Default()
	cfg.UpdateStagingDir = t.TempDir()
	t.Setenv("AC_UPDATE_APPLY_MODE", httpapi.UpdateApplyManagedLauncherMode)
	t.Setenv("AC_UPDATE_LAUNCHER_TOKEN", "0123456789abcdef0123456789abcdef") // gitleaks:allow -- deterministic test fixture
	if managedUpdateLauncherAuthorized(cfg) {
		t.Fatal("environment-only launcher claim was accepted")
	}
}
