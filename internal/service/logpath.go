package service

import (
	"os"
	"path/filepath"
)

// defaultLogPath returns the standard log location for The Purge on macOS.
func defaultLogPath() string {
	home, err := os.UserHomeDir()
	if err != nil {
		// Fall back to a temp location if the home dir is unavailable.
		return filepath.Join(os.TempDir(), "thepurge.log")
	}
	return filepath.Join(home, "Library", "Logs", "thepurge.log")
}
