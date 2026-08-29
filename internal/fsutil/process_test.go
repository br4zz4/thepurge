//go:build darwin

package fsutil

import (
	"os/exec"
	"strings"
	"testing"
)

func TestIsProcessRunningByNameCurrentProcess(t *testing.T) {
	// pgrep -i -q should find the running test binary (matched by "go" or similar).
	running := IsProcessRunning("", "go")
	if !running {
		// The test runner may not be named "go"; try the test binary name.
		cmd := exec.Command("ps", "-o", "comm=", "-p", "$$")
		out, err := cmd.Output()
		if err != nil {
			t.Skipf("cannot determine current process name: %v", err)
		}
		running = IsProcessRunning("", strings.TrimSpace(string(out)))
	}
	if !running {
		t.Error("IsProcessRunning('', current process name) = false, want true")
	}
}

func TestIsProcessRunningByNameNonexistent(t *testing.T) {
	running := IsProcessRunning("", "NONEXISTENT_PROCESS_ZZZ_999")
	if running {
		t.Error("IsProcessRunning('', nonexistent) = true, want false")
	}
}

func TestIsProcessRunningByBundleNonexistent(t *testing.T) {
	running := IsProcessRunning("com.nonexistent.bundle.zzz", "")
	if running {
		t.Error("IsProcessRunning(nonexistent bundle, '') = true, want false")
	}
}

func TestIsProcessRunningEmpty(t *testing.T) {
	running := IsProcessRunning("", "")
	if running {
		t.Error("IsProcessRunning('', '') = true, want false")
	}
}

func TestIsProcessRunningByNameSleep(t *testing.T) {
	// sleep is a reliable process to test
	cmd := exec.Command("sleep", "30")
	if err := cmd.Start(); err != nil {
		t.Skipf("cannot start sleep: %v", err)
	}
	defer cmd.Process.Kill()

	running := IsProcessRunning("", "sleep")
	if !running {
		t.Error("IsProcessRunning('', 'sleep') = false while sleep is running, want true")
	}
}
