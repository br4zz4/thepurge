//go:build !darwin

package fsutil

// Notify is a no-op stub on non-macOS platforms.
func Notify(title, message string) {}