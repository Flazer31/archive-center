//go:build windows

package bench

import (
	"fmt"
	"os/exec"
	"strconv"
	"strings"
)

// GetProcessRSS returns the working set size (RSS) in bytes for a Windows process.
func GetProcessRSS(pid int) (int64, error) {
	out, err := exec.Command("wmic", "process", "where", fmt.Sprintf("ProcessId=%d", pid), "get", "WorkingSetSize").Output()
	if err == nil {
		if rss, parseErr := ParseWMICWorkingSet(string(out)); parseErr == nil {
			return rss, nil
		}
	}

	psOut, psErr := exec.Command(
		"powershell.exe",
		"-NoProfile",
		"-Command",
		fmt.Sprintf("(Get-Process -Id %d).WorkingSet64", pid),
	).Output()
	if psErr != nil {
		if err != nil {
			return 0, fmt.Errorf("wmic failed: %w; powershell failed: %w", err, psErr)
		}
		return 0, fmt.Errorf("wmic parse failed; powershell failed: %w", psErr)
	}
	return ParsePowerShellWorkingSet(string(psOut))
}

// ParseWMICWorkingSet extracts the WorkingSetSize value from wmic output.
func ParseWMICWorkingSet(output string) (int64, error) {
	lines := strings.Split(output, "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" || strings.EqualFold(line, "WorkingSetSize") {
			continue
		}
		val, err := strconv.ParseInt(line, 10, 64)
		if err != nil {
			return 0, fmt.Errorf("invalid WorkingSetSize %q: %w", line, err)
		}
		return val, nil
	}
	return 0, fmt.Errorf("WorkingSetSize not found in output")
}

// ParsePowerShellWorkingSet extracts a WorkingSet64 byte value from PowerShell output.
func ParsePowerShellWorkingSet(output string) (int64, error) {
	for _, line := range strings.Split(output, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		val, err := strconv.ParseInt(line, 10, 64)
		if err != nil {
			return 0, fmt.Errorf("invalid WorkingSet64 %q: %w", line, err)
		}
		return val, nil
	}
	return 0, fmt.Errorf("WorkingSet64 not found in output")
}
