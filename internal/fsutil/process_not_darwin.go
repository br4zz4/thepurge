//go:build !darwin

package fsutil

// IsProcessRunning is a no-op stub on non-macOS platforms.
func IsProcessRunning(bundleID, processName string) bool {
	return false
}
