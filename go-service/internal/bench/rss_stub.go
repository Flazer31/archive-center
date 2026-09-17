//go:build !windows

package bench

import "fmt"

// GetProcessRSS returns an error on non-Windows platforms.
func GetProcessRSS(pid int) (int64, error) {
	return 0, fmt.Errorf("GetProcessRSS is only implemented on Windows")
}
