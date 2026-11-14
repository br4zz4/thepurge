package service

import (
	"os"
	"path/filepath"
	"runtime"
)

// defaultLogPath returns the file the service manager captures the agent's
// stdout into, under the platform log dir. The structured log written by the
// agent itself is configured separately via logging.log_path.
func defaultLogPath() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return filepath.Join(os.TempDir(), "thepurge.log")
	}
	if runtime.GOOS == "linux" {
		return filepath.Join(home, ".local", "state", "thepurge", "thepurge.log")
	}
	return filepath.Join(home, "Library", "Logs", "thepurge.log")
}
