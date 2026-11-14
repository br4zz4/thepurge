//go:build !darwin && !linux

package fsutil

// IsProcessRunning is a no-op stub on platforms without a native implementation.
func IsProcessRunning(bundleID, processName string) bool {
	return false
}
