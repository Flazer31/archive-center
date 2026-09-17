package main

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestWindowsLegacyDataImportSmoke(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("PowerShell legacy data import smoke is Windows-specific")
	}
	powerShell, err := exec.LookPath("powershell.exe")
	if err != nil {
		t.Skip("powershell.exe is unavailable")
	}
	script, err := filepath.Abs(filepath.Join("..", "..", "..", "ops", "legacy-data-import-smoke.ps1"))
	if err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command(powerShell, "-NoProfile", "-ExecutionPolicy", "Bypass", "-File", script)
	// Go inherits PowerShell 7's PSModulePath in the Codex/build shell. Put the
	// Windows PowerShell built-in module directory first so this production
	// script smoke exercises Get-FileHash under the same module set as a normal
	// powershell.exe package launch.
	windowsModulePath := filepath.Join(filepath.Dir(powerShell), "Modules")
	modulePath := windowsModulePath
	if inherited := strings.TrimSpace(os.Getenv("PSModulePath")); inherited != "" {
		modulePath += string(os.PathListSeparator) + inherited
	}
	env := os.Environ()
	modulePathReplaced := false
	for i, entry := range env {
		key, _, _ := strings.Cut(entry, "=")
		if strings.EqualFold(key, "PSModulePath") {
			env[i] = "PSModulePath=" + modulePath
			modulePathReplaced = true
		}
	}
	if !modulePathReplaced {
		env = append(env, "PSModulePath="+modulePath)
	}
	cmd.Env = env
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("legacy data import smoke failed: %v\n%s", err, output)
	}
	text := string(output)
	jsonStart := strings.LastIndex(text, "{")
	if jsonStart < 0 {
		t.Fatalf("legacy data import smoke returned no JSON:\n%s", text)
	}
	var report struct {
		Contract                                string `json:"contract"`
		Status                                  string `json:"status"`
		LauncherLivePortRejection               string `json:"launcher_live_port_rejection"`
		LauncherIPv6LivePortRejection           string `json:"launcher_ipv6_live_port_rejection"`
		LauncherConfiguredLivePortRejection     string `json:"launcher_configured_live_port_rejection"`
		LauncherConfiguredIPv6LivePortRejection string `json:"launcher_configured_ipv6_live_port_rejection"`
		ExternalInstallerLivePortRejection      string `json:"external_installer_live_port_rejection"`
		ExternalInstallerIPv6LivePortRejection  string `json:"external_installer_ipv6_live_port_rejection"`
		ExternalConfiguredLivePortRejection     string `json:"external_installer_configured_live_port_rejection"`
		ExternalConfiguredIPv6LivePortRejection string `json:"external_installer_configured_ipv6_live_port_rejection"`
		LauncherOfflineAtomicPromotion          string `json:"launcher_offline_atomic_promotion"`
		ExternalInstallerOfflineAtomicPromotion string `json:"external_installer_offline_atomic_promotion"`
		SourcePreserved                         bool   `json:"source_preserved"`
	}
	if err := json.Unmarshal([]byte(text[jsonStart:]), &report); err != nil {
		t.Fatalf("legacy data import smoke JSON: %v\n%s", err, text)
	}
	if report.Contract != "archive-center.legacy-data-import-smoke.v1" ||
		report.Status != "ok" ||
		report.LauncherLivePortRejection != "passed" ||
		report.LauncherIPv6LivePortRejection != "passed" ||
		report.LauncherConfiguredLivePortRejection != "passed" ||
		report.LauncherConfiguredIPv6LivePortRejection != "passed" ||
		report.ExternalInstallerLivePortRejection != "passed" ||
		report.ExternalInstallerIPv6LivePortRejection != "passed" ||
		report.ExternalConfiguredLivePortRejection != "passed" ||
		report.ExternalConfiguredIPv6LivePortRejection != "passed" ||
		report.LauncherOfflineAtomicPromotion != "passed" ||
		report.ExternalInstallerOfflineAtomicPromotion != "passed" ||
		!report.SourcePreserved {
		t.Fatalf("unexpected legacy data import smoke report: %+v", report)
	}
}
