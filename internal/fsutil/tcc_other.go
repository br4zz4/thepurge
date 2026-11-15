//go:build !darwin

package fsutil

// IsTCCProtected reports whether path is guarded by a per-app privacy service.
// Only macOS has TCC, so nothing is protected elsewhere.
func IsTCCProtected(path string) bool { return false }
