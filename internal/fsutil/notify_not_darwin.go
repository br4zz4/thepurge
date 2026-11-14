//go:build !darwin && !linux

package fsutil

// Notify is a no-op stub on platforms without a native implementation.
func Notify(title, message string) {}