//go:build darwin

package fsutil

import (
	"os/exec"
	"strings"
)

// IsProcessRunning checks whether a macOS process matching the given criteria
// is currently active. It tries bundle ID first (via osascript), then falls
// back to process name matching (via pgrep). An empty bundleID means skip
// bundle lookup; an empty processName means skip name lookup.
//
// On query failure it logs nothing and returns false — the caller may treat
// an uncertain check as "running" (skip deletion) for safety.
func IsProcessRunning(bundleID, processName string) bool {
	if bundleID != "" {
		running, err := isBundleRunning(bundleID)
		if err == nil && running {
			return true
		}
	}
	if processName != "" {
		running, err := isProcessNameRunning(processName)
		if err == nil && running {
			return true
		}
	}
	return false
}

// isBundleRunning checks if any process with the given bundle ID is active
// using AppleScript via osascript.
func isBundleRunning(bundleID string) (bool, error) {
	script := `tell application "System Events" to exists (processes where bundle identifier is "` + bundleID + `")`
	cmd := exec.Command("osascript", "-e", script)
	out, err := cmd.Output()
	if err != nil {
		return false, err
	}
	return strings.TrimSpace(string(out)) == "true", nil
}

// isProcessNameRunning checks if any process with the given name is active
// using pgrep -i -q (case-insensitive quiet match).
func isProcessNameRunning(name string) (bool, error) {
	cmd := exec.Command("pgrep", "-i", "-q", name)
	err := cmd.Run()
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok && exitErr.ExitCode() == 1 {
			return false, nil
		}
		return false, err
	}
	return true, nil
}
