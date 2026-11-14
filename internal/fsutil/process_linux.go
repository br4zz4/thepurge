//go:build linux

package fsutil

import (
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

// IsProcessRunning checks whether a process named processName is active. It
// scans /proc instead of shelling out to pgrep so detection keeps working on
// hosts without procps; failing to see a running app would let its cache be
// deleted. Bundle IDs are a macOS concept and are ignored.
func IsProcessRunning(bundleID, processName string) bool {
	if processName == "" {
		return false
	}
	entries, err := os.ReadDir("/proc")
	if err != nil {
		return false
	}
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		if _, err := strconv.Atoi(e.Name()); err != nil {
			continue
		}
		dir := filepath.Join("/proc", e.Name())
		if comm, err := os.ReadFile(filepath.Join(dir, "comm")); err == nil {
			if strings.EqualFold(strings.TrimSpace(string(comm)), processName) {
				return true
			}
		}
		if cmdline, err := os.ReadFile(filepath.Join(dir, "cmdline")); err == nil {
			if argv0, _, _ := strings.Cut(string(cmdline), "\x00"); strings.EqualFold(filepath.Base(argv0), processName) {
				return true
			}
		}
	}
	return false
}
