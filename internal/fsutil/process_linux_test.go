//go:build linux

package fsutil

import (
	"os/exec"
	"testing"
	"time"
)

func startSleep(t *testing.T, name string) *exec.Cmd {
	t.Helper()
	cmd := exec.Command(name, "30")
	if err := cmd.Start(); err != nil {
		t.Skipf("cannot start %q: %v", name, err)
	}
	t.Cleanup(func() {
		cmd.Process.Kill()
		cmd.Wait()
	})
	return cmd
}

func startArgv0(t *testing.T, argv0 string) *exec.Cmd {
	t.Helper()
	if _, err := exec.LookPath("bash"); err != nil {
		t.Skip("bash not available")
	}
	cmd := exec.Command("bash", "-c", "exec -a "+argv0+" sleep 30")
	if err := cmd.Start(); err != nil {
		t.Skipf("cannot start bash: %v", err)
	}
	t.Cleanup(func() {
		cmd.Process.Kill()
		cmd.Wait()
	})
	return cmd
}

func eventually(t *testing.T, want bool, check func() bool) bool {
	t.Helper()
	deadline := time.Now().Add(3 * time.Second)
	for {
		got := check()
		if got == want {
			return got
		}
		if time.Now().After(deadline) {
			return got
		}
		time.Sleep(20 * time.Millisecond)
	}
}

func TestIsProcessRunningLinuxFindsRunningProcess(t *testing.T) {
	// arrange
	startSleep(t, "sleep")

	// act
	got := eventually(t, true, func() bool { return IsProcessRunning("", "sleep") })

	// assert
	if !got {
		t.Error("IsProcessRunning(\"\", \"sleep\") = false, want true")
	}
}

func TestIsProcessRunningLinuxIsCaseInsensitive(t *testing.T) {
	// arrange
	startSleep(t, "sleep")

	// act
	got := eventually(t, true, func() bool { return IsProcessRunning("", "SLEEP") })

	// assert
	if !got {
		t.Error("IsProcessRunning(\"\", \"SLEEP\") = false, want true")
	}
}

func TestIsProcessRunningLinuxMatchesArgv0(t *testing.T) {
	// arrange
	const argv0 = "zzpurgetestargv0"
	startArgv0(t, argv0)

	// act
	got := eventually(t, true, func() bool { return IsProcessRunning("", argv0) })

	// assert
	if !got {
		t.Errorf("IsProcessRunning(\"\", %q) = false, want true via argv[0]", argv0)
	}
}

func TestIsProcessRunningLinuxIgnoresBundleID(t *testing.T) {
	// arrange
	startSleep(t, "sleep")

	// act
	got := eventually(t, true, func() bool { return IsProcessRunning("com.example.sleep", "sleep") })

	// assert
	if !got {
		t.Error("IsProcessRunning() = false, want true; bundleID must be ignored")
	}
}

func TestIsProcessRunningLinuxUnknownProcess(t *testing.T) {
	// act
	got := IsProcessRunning("", "zzpurgetestnotrunning")

	// assert
	if got {
		t.Error("IsProcessRunning() = true, want false for an absent process")
	}
}

func TestIsProcessRunningLinuxEmptyProcessName(t *testing.T) {
	// arrange
	startSleep(t, "sleep")

	// act
	got := IsProcessRunning("com.example.sleep", "")

	// assert
	if got {
		t.Error("IsProcessRunning() = true, want false without a process name")
	}
}
