package hooks

import (
	"context"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"

	"br4zz4/thepurge/config"
)

func writeScript(t *testing.T, dir, name, body string, mode os.FileMode) string {
	t.Helper()
	p := filepath.Join(dir, name)
	if err := os.WriteFile(p, []byte("#!/bin/sh\n"+body+"\n"), mode); err != nil {
		t.Fatal(err)
	}
	return p
}

func readFile(t *testing.T, p string) string {
	t.Helper()
	b, err := os.ReadFile(p)
	if err != nil {
		t.Fatalf("read %s: %v", p, err)
	}
	return string(b)
}

func TestRunPathHookSuccess(t *testing.T) {
	// arrange
	dir := t.TempDir()
	marker := filepath.Join(dir, "marker")
	script := writeScript(t, dir, "ok.sh", `echo ran > "`+marker+`"`, 0o755)

	// act
	err := Runner{}.Run(context.Background(), "pre_eviction", "apply", []config.Hook{{Path: script}})

	// assert
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if strings.TrimSpace(readFile(t, marker)) != "ran" {
		t.Error("hook did not run")
	}
}

func TestRunPassesEnv(t *testing.T) {
	// arrange
	dir := t.TempDir()
	out := filepath.Join(dir, "env")
	script := writeScript(t, dir, "env.sh", `echo "$THEPURGE_MODE $THEPURGE_EVENT" > "`+out+`"`, 0o755)

	// act
	err := Runner{}.Run(context.Background(), "post_eviction", "apply", []config.Hook{{Path: script}})

	// assert
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if got := strings.TrimSpace(readFile(t, out)); got != "apply post_eviction" {
		t.Errorf("env = %q, want %q", got, "apply post_eviction")
	}
}

func TestRunCommandViaShell(t *testing.T) {
	// arrange
	dir := t.TempDir()
	out := filepath.Join(dir, "out")
	cmd := `echo "a" > "` + out + `" && echo "$THEPURGE_EVENT" >> "` + out + `"`

	// act
	err := Runner{}.Run(context.Background(), "on_failure", "apply", []config.Hook{{Command: cmd}})

	// assert
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if got := readFile(t, out); got != "a\non_failure\n" {
		t.Errorf("output = %q", got)
	}
}

func TestRunRunsHooksInOrder(t *testing.T) {
	// arrange
	dir := t.TempDir()
	out := filepath.Join(dir, "order")
	hooks := []config.Hook{
		{Command: `echo one >> "` + out + `"`},
		{Command: `echo two >> "` + out + `"`},
		{Command: `echo three >> "` + out + `"`},
	}

	// act
	err := Runner{}.Run(context.Background(), "pre_eviction", "apply", hooks)

	// assert
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if got := readFile(t, out); got != "one\ntwo\nthree\n" {
		t.Errorf("order = %q", got)
	}
}

func TestRunNonZeroExitIsFailure(t *testing.T) {
	// arrange
	dir := t.TempDir()
	script := writeScript(t, dir, "fail.sh", "exit 3", 0o755)

	// act
	err := Runner{}.Run(context.Background(), "pre_eviction", "apply", []config.Hook{{Path: script}})

	// assert
	if err == nil {
		t.Fatal("Run() = nil, want error for exit != 0")
	}
}

func TestRunFailureDoesNotStopOtherHooks(t *testing.T) {
	// arrange
	dir := t.TempDir()
	marker := filepath.Join(dir, "second")
	hooks := []config.Hook{
		{Command: "exit 1"},
		{Command: `echo ok > "` + marker + `"`},
	}

	// act
	err := Runner{}.Run(context.Background(), "post_eviction", "apply", hooks)

	// assert
	if err == nil {
		t.Fatal("Run() = nil, want aggregated error")
	}
	if _, statErr := os.Stat(marker); statErr != nil {
		t.Errorf("second hook should still run: %v", statErr)
	}
}

func TestRunAggregatesAllFailures(t *testing.T) {
	// arrange
	hooks := []config.Hook{{Command: "exit 1"}, {Command: "exit 2"}}

	// act
	err := Runner{}.Run(context.Background(), "pre_eviction", "apply", hooks)

	// assert
	if err == nil {
		t.Fatal("Run() = nil, want error")
	}
	if u, ok := err.(interface{ Unwrap() []error }); !ok || len(u.Unwrap()) != 2 {
		t.Errorf("error should join 2 failures, got %v", err)
	}
}

func TestRunPathNotExecutableFails(t *testing.T) {
	// arrange
	dir := t.TempDir()
	script := writeScript(t, dir, "noexec.sh", "exit 0", 0o644)

	// act
	err := Runner{}.Run(context.Background(), "pre_eviction", "apply", []config.Hook{{Path: script}})

	// assert
	if err == nil {
		t.Fatal("Run() = nil, want permission error")
	}
}

func TestRunMissingPathFails(t *testing.T) {
	// arrange
	missing := filepath.Join(t.TempDir(), "nope.sh")

	// act
	err := Runner{}.Run(context.Background(), "pre_eviction", "apply", []config.Hook{{Path: missing}})

	// assert
	if err == nil {
		t.Fatal("Run() = nil, want error for missing path")
	}
}

func TestRunTimeoutIsFailure(t *testing.T) {
	// arrange
	hooks := []config.Hook{{Command: "sleep 10", TimeoutSeconds: 1}}
	start := time.Now()

	// act
	err := Runner{}.Run(context.Background(), "pre_eviction", "apply", hooks)

	// assert
	if err == nil {
		t.Fatal("Run() = nil, want timeout error")
	}
	if elapsed := time.Since(start); elapsed > 8*time.Second {
		t.Errorf("hook was not killed on timeout; took %s", elapsed)
	}
}

func TestRunTimeoutKillsChildProcessGroup(t *testing.T) {
	// arrange
	dir := t.TempDir()
	pidFile := filepath.Join(dir, "pid")
	hooks := []config.Hook{{Command: `sleep 30 & echo $! > "` + pidFile + `"; wait`, TimeoutSeconds: 1}}

	// act
	err := Runner{}.Run(context.Background(), "pre_eviction", "apply", hooks)

	// assert
	if err == nil {
		t.Fatal("Run() = nil, want timeout error")
	}
	pid, convErr := strconv.Atoi(strings.TrimSpace(readFile(t, pidFile)))
	if convErr != nil {
		t.Fatal(convErr)
	}
	t.Cleanup(func() { _ = syscall.Kill(pid, syscall.SIGKILL) })
	var alive bool
	for i := 0; i < 20; i++ {
		alive = processRunning(pid)
		if !alive {
			break
		}
		time.Sleep(100 * time.Millisecond)
	}
	if alive {
		t.Errorf("child process %d survived the hook timeout", pid)
	}
}

func processRunning(pid int) bool {
	if syscall.Kill(pid, 0) != nil {
		return false
	}
	stat, err := os.ReadFile("/proc/" + strconv.Itoa(pid) + "/stat")
	if err != nil {
		return true
	}
	end := strings.LastIndex(string(stat), ")")
	if end < 0 || end+2 >= len(stat) {
		return true
	}
	return stat[end+2] != 'Z'
}

func TestRunDefaultTimeoutApplies(t *testing.T) {
	// arrange
	r := Runner{DefaultTimeout: 500 * time.Millisecond}
	start := time.Now()

	// act
	err := r.Run(context.Background(), "pre_eviction", "apply", []config.Hook{{Command: "sleep 10"}})

	// assert
	if err == nil {
		t.Fatal("Run() = nil, want timeout error from DefaultTimeout")
	}
	if elapsed := time.Since(start); elapsed > 8*time.Second {
		t.Errorf("hook was not killed on default timeout; took %s", elapsed)
	}
}

func TestRunNoHooksIsNoop(t *testing.T) {
	// arrange
	// act
	err := Runner{}.Run(context.Background(), "pre_eviction", "apply", nil)

	// assert
	if err != nil {
		t.Errorf("Run() error = %v, want nil", err)
	}
}
