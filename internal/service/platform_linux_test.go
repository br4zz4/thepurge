//go:build linux

package service

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

const wantServiceUnit = `[Unit]
Description=thepurge eviction pass
Documentation=https://github.com/br4zz4/thepurge

[Service]
Type=oneshot
ExecStart=/opt/bin/thepurge run
`

const wantServiceUnitApply = `[Unit]
Description=thepurge eviction pass
Documentation=https://github.com/br4zz4/thepurge

[Service]
Type=oneshot
ExecStart=/opt/bin/thepurge run --apply
`

const wantTimerUnit300 = `[Unit]
Description=thepurge periodic eviction

[Timer]
OnBootSec=2min
OnUnitActiveSec=300s
Persistent=true

[Install]
WantedBy=timers.target
`

type scriptedRunner struct {
	cmds   [][]string
	handle func(name string, args []string) ([]byte, error)
}

func (s *scriptedRunner) Run(name string, args ...string) ([]byte, error) {
	s.cmds = append(s.cmds, append([]string{name}, args...))
	if s.handle == nil {
		return nil, nil
	}
	return s.handle(name, args)
}

func systemdShow(timerOut, serviceOut string, timerErr, serviceErr error) Runner {
	return funcRunner(func(_ string, args ...string) ([]byte, error) {
		if len(args) < 3 {
			return nil, nil
		}
		switch args[2] {
		case unitName + ".timer":
			if timerErr != nil {
				return nil, timerErr
			}
			return []byte(timerOut), nil
		case unitName + ".service":
			if serviceErr != nil {
				return nil, serviceErr
			}
			return []byte(serviceOut), nil
		}
		return nil, nil
	})
}

func writeUnits(t *testing.T, dir, bin string, apply bool) {
	t.Helper()
	flag := ""
	if apply {
		flag = " --apply"
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(servicePath(dir), []byte(fmt.Sprintf(serviceTemplate, bin, flag)), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(timerPath(dir), []byte(fmt.Sprintf(timerTemplate, 300)), 0o644); err != nil {
		t.Fatal(err)
	}
}

func showOutput(bin string, pid, lastExit int) string {
	return fmt.Sprintf(
		"ExecStart={ path=%s ; argv[]=%s run ; ignore_errors=no ; start_time=[n/a] ; stop_time=[n/a] ; pid=%d ; code=(null) ; status=0/0 }\nExecMainStatus=%d\nMainPID=%d\n",
		bin, bin, pid, lastExit, pid,
	)
}

func TestBackendAndUnitPaths(t *testing.T) {
	t.Run("xdg config home", func(t *testing.T) {
		// arrange
		xdg := t.TempDir()
		t.Setenv("XDG_CONFIG_HOME", xdg)

		// act
		dir, err := defaultUnitDir()

		// assert
		if err != nil {
			t.Fatalf("defaultUnitDir() error = %v", err)
		}
		if want := filepath.Join(xdg, "systemd", "user"); dir != want {
			t.Errorf("defaultUnitDir() = %q, want %q", dir, want)
		}
	})

	t.Run("home fallback", func(t *testing.T) {
		// arrange
		home := t.TempDir()
		t.Setenv("HOME", home)
		t.Setenv("XDG_CONFIG_HOME", "")

		// act
		dir, err := defaultUnitDir()

		// assert
		if err != nil {
			t.Fatalf("defaultUnitDir() error = %v", err)
		}
		if want := filepath.Join(home, ".config", "systemd", "user"); dir != want {
			t.Errorf("defaultUnitDir() = %q, want %q", dir, want)
		}
	})

	t.Run("unit file names", func(t *testing.T) {
		// arrange
		dir := "/tmp/units"

		// act
		svc, tmr := servicePath(dir), timerPath(dir)

		// assert
		if backend != "systemd" {
			t.Errorf("backend = %q, want systemd", backend)
		}
		if want := filepath.Join(dir, "thepurge.service"); svc != want {
			t.Errorf("servicePath() = %q, want %q", svc, want)
		}
		if want := filepath.Join(dir, "thepurge.timer"); tmr != want {
			t.Errorf("timerPath() = %q, want %q", tmr, want)
		}
	})
}

func TestInstallWritesUnitFiles(t *testing.T) {
	tests := []struct {
		name    string
		apply   bool
		wantSvc string
	}{
		{name: "dry run", apply: false, wantSvc: wantServiceUnit},
		{name: "apply", apply: true, wantSvc: wantServiceUnitApply},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// arrange
			dir := t.TempDir()
			r := &fakeRunner{}

			// act
			err := installDefinition(InstallOptions{
				BinPath:         "/opt/bin/thepurge",
				IntervalSeconds: 300,
				Apply:           tt.apply,
				UnitDir:         dir,
				Runner:          r,
			})

			// assert
			if err != nil {
				t.Fatalf("installDefinition() error = %v", err)
			}
			svc, err := os.ReadFile(servicePath(dir))
			if err != nil {
				t.Fatalf("service unit not written: %v", err)
			}
			if string(svc) != tt.wantSvc {
				t.Errorf("service unit = %q, want %q", svc, tt.wantSvc)
			}
			tmr, err := os.ReadFile(timerPath(dir))
			if err != nil {
				t.Fatalf("timer unit not written: %v", err)
			}
			if string(tmr) != wantTimerUnit300 {
				t.Errorf("timer unit = %q, want %q", tmr, wantTimerUnit300)
			}
		})
	}
}

func TestInstallRunsDaemonReloadBeforeEnable(t *testing.T) {
	// arrange
	dir := t.TempDir()
	r := &fakeRunner{}

	// act
	err := installDefinition(InstallOptions{
		BinPath:         "/opt/bin/thepurge",
		IntervalSeconds: 300,
		UnitDir:         dir,
		Runner:          r,
	})

	// assert
	if err != nil {
		t.Fatalf("installDefinition() error = %v", err)
	}
	want := [][]string{
		{"systemctl", "--user", "daemon-reload"},
		{"systemctl", "--user", "enable", "--now", "thepurge.timer"},
	}
	if !reflect.DeepEqual(r.cmds, want) {
		t.Errorf("systemctl calls = %v, want %v", r.cmds, want)
	}
}

func TestInstallCreatesUnitDir(t *testing.T) {
	// arrange
	dir := filepath.Join(t.TempDir(), "nested", "systemd", "user")

	// act
	err := installDefinition(InstallOptions{
		BinPath:         "/opt/bin/thepurge",
		IntervalSeconds: 300,
		UnitDir:         dir,
		Runner:          &fakeRunner{},
	})

	// assert
	if err != nil {
		t.Fatalf("installDefinition() error = %v", err)
	}
	if _, err := os.Stat(timerPath(dir)); err != nil {
		t.Errorf("timer unit not written: %v", err)
	}
}

func TestInstallAppliesDefaults(t *testing.T) {
	// arrange
	dir := t.TempDir()
	bin := writeBinary(t, filepath.Join(t.TempDir(), "thepurge"))
	r := &fakeRunner{}

	// act
	err := Install(InstallOptions{BinPath: bin, UnitDir: dir, Runner: r})

	// assert
	if err != nil {
		t.Fatalf("Install() error = %v", err)
	}
	tmr, err := os.ReadFile(timerPath(dir))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(tmr), "OnUnitActiveSec=1800s") {
		t.Errorf("timer missing default interval; got:\n%s", tmr)
	}
	if callIndex(r.cmds, "systemctl", "--user", "enable", "--now", "thepurge.timer") < 0 {
		t.Errorf("missing systemctl enable; calls = %v", r.cmds)
	}
}

func TestInstallDaemonReloadError(t *testing.T) {
	// arrange
	dir := t.TempDir()
	r := &scriptedRunner{handle: func(_ string, args []string) ([]byte, error) {
		if len(args) > 1 && args[1] == "daemon-reload" {
			return []byte("Failed to reload daemon: Access denied"), errors.New("exit status 1")
		}
		return nil, nil
	}}

	// act
	err := installDefinition(InstallOptions{
		BinPath:         "/opt/bin/thepurge",
		IntervalSeconds: 300,
		UnitDir:         dir,
		Runner:          r,
	})

	// assert
	if err == nil {
		t.Fatal("installDefinition() error = nil, want daemon-reload failure")
	}
	if !strings.Contains(err.Error(), "service install: systemctl daemon-reload") {
		t.Errorf("error = %v, want daemon-reload context", err)
	}
	if len(r.cmds) != 1 {
		t.Errorf("enable must not run after daemon-reload fails; calls = %v", r.cmds)
	}
}

func TestInstallEnableError(t *testing.T) {
	// arrange
	dir := t.TempDir()
	r := &scriptedRunner{handle: func(_ string, args []string) ([]byte, error) {
		if len(args) > 1 && args[1] == "enable" {
			return []byte("Failed to enable unit"), errors.New("exit status 1")
		}
		return nil, nil
	}}

	// act
	err := installDefinition(InstallOptions{
		BinPath:         "/opt/bin/thepurge",
		IntervalSeconds: 300,
		UnitDir:         dir,
		Runner:          r,
	})

	// assert
	if err == nil {
		t.Fatal("installDefinition() error = nil, want enable failure")
	}
	if !strings.Contains(err.Error(), "service install: systemctl enable") {
		t.Errorf("error = %v, want enable context", err)
	}
}

func TestUninstallRemovesUnitsAndReloads(t *testing.T) {
	// arrange
	dir := t.TempDir()
	writeUnits(t, dir, "/opt/bin/thepurge", false)
	r := &fakeRunner{}

	// act
	err := Uninstall(InstallOptions{UnitDir: dir, Runner: r})

	// assert
	if err != nil {
		t.Fatalf("Uninstall() error = %v", err)
	}
	for _, p := range []string{timerPath(dir), servicePath(dir)} {
		if _, statErr := os.Stat(p); !os.IsNotExist(statErr) {
			t.Errorf("%s still present after uninstall: %v", p, statErr)
		}
	}
	disable := callIndex(r.cmds, "systemctl", "--user", "disable", "--now", "thepurge.timer")
	reload := callIndex(r.cmds, "systemctl", "--user", "daemon-reload")
	if disable < 0 {
		t.Errorf("missing systemctl disable; calls = %v", r.cmds)
	}
	if reload < 0 {
		t.Errorf("missing systemctl daemon-reload; calls = %v", r.cmds)
	}
	if disable >= 0 && reload >= 0 && disable > reload {
		t.Errorf("disable must precede daemon-reload; calls = %v", r.cmds)
	}
	if last := r.cmds[len(r.cmds)-1]; !reflect.DeepEqual(last, []string{"systemctl", "--user", "daemon-reload"}) {
		t.Errorf("daemon-reload must be the last call; calls = %v", r.cmds)
	}
}

func TestUninstallWithoutUnits(t *testing.T) {
	// arrange
	dir := t.TempDir()
	r := &fakeRunner{}

	// act
	err := Uninstall(InstallOptions{UnitDir: dir, Runner: r})

	// assert
	if err != nil {
		t.Fatalf("Uninstall() error = %v", err)
	}
	if callIndex(r.cmds, "systemctl", "--user", "disable", "--now", "thepurge.timer") < 0 {
		t.Errorf("missing systemctl disable; calls = %v", r.cmds)
	}
	if callIndex(r.cmds, "systemctl", "--user", "daemon-reload") < 0 {
		t.Errorf("missing systemctl daemon-reload; calls = %v", r.cmds)
	}
}

func TestUninstallRemoveError(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("root bypasses file permissions")
	}
	// arrange
	dir := t.TempDir()
	writeUnits(t, dir, "/opt/bin/thepurge", false)
	if err := os.Chmod(dir, 0o500); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.Chmod(dir, 0o755) })
	r := &fakeRunner{}

	// act
	err := Uninstall(InstallOptions{UnitDir: dir, Runner: r})

	// assert
	if err == nil {
		t.Fatal("Uninstall() error = nil, want removal error")
	}
	if callIndex(r.cmds, "systemctl", "--user", "daemon-reload") >= 0 {
		t.Errorf("daemon-reload must not run after a failed removal; calls = %v", r.cmds)
	}
}

func TestCurrentNotInstalled(t *testing.T) {
	// arrange
	dir := t.TempDir()
	runner := systemdShow("ActiveState=inactive\n", showOutput("/opt/bin/thepurge", 0, 0), nil, nil)

	// act
	st, err := Current(InstallOptions{UnitDir: dir, Runner: runner})

	// assert
	if err != nil {
		t.Fatalf("Current() error = %v", err)
	}
	if st.Backend != "systemd" {
		t.Errorf("Backend = %q, want systemd", st.Backend)
	}
	if st.UnitPath != servicePath(dir) {
		t.Errorf("UnitPath = %q, want %q", st.UnitPath, servicePath(dir))
	}
	if st.Installed {
		t.Error("Installed = true, want false")
	}
	if st.Loaded {
		t.Error("Loaded = true, want false")
	}
}

func TestCurrentInstalledAndRunning(t *testing.T) {
	// arrange
	dir := t.TempDir()
	bin := writeBinary(t, filepath.Join(t.TempDir(), "thepurge"))
	writeUnits(t, dir, bin, false)
	runner := systemdShow("ActiveState=active\n", showOutput(bin, 4242, 0), nil, nil)

	// act
	st, err := Current(InstallOptions{UnitDir: dir, Runner: runner})

	// assert
	if err != nil {
		t.Fatalf("Current() error = %v", err)
	}
	if !st.Installed {
		t.Error("Installed = false, want true")
	}
	if st.ProgramWanted != bin {
		t.Errorf("ProgramWanted = %q, want %q", st.ProgramWanted, bin)
	}
	if !st.ProgramExists {
		t.Error("ProgramExists = false, want true")
	}
	if !st.Loaded || !st.Running {
		t.Errorf("Loaded = %v, Running = %v, want both true", st.Loaded, st.Running)
	}
	if st.PID != 4242 {
		t.Errorf("PID = %d, want 4242", st.PID)
	}
	if st.ProgramLoaded != bin {
		t.Errorf("ProgramLoaded = %q, want %q", st.ProgramLoaded, bin)
	}
	if st.Mismatch() {
		t.Error("Mismatch() = true, want false")
	}
}

func TestCurrentTimerStopped(t *testing.T) {
	// arrange
	dir := t.TempDir()
	bin := writeBinary(t, filepath.Join(t.TempDir(), "thepurge"))
	writeUnits(t, dir, bin, false)
	runner := systemdShow("ActiveState=inactive\n", showOutput(bin, 0, 3), nil, nil)

	// act
	st, err := Current(InstallOptions{UnitDir: dir, Runner: runner})

	// assert
	if err != nil {
		t.Fatalf("Current() error = %v", err)
	}
	if !st.Installed {
		t.Error("Installed = false, want true")
	}
	if st.Loaded {
		t.Error("Loaded = true, want false")
	}
	if st.Running {
		t.Error("Running = true, want false")
	}
	if st.LastExit != 3 {
		t.Errorf("LastExit = %d, want 3", st.LastExit)
	}
}

func TestCurrentServiceWithoutTimer(t *testing.T) {
	// arrange
	dir := t.TempDir()
	bin := writeBinary(t, filepath.Join(t.TempDir(), "thepurge"))
	if err := os.WriteFile(servicePath(dir), []byte(fmt.Sprintf(serviceTemplate, bin, "")), 0o644); err != nil {
		t.Fatal(err)
	}
	runner := systemdShow("ActiveState=active\n", showOutput(bin, 0, 0), nil, nil)

	// act
	st, err := Current(InstallOptions{UnitDir: dir, Runner: runner})

	// assert
	if err != nil {
		t.Fatalf("Current() error = %v", err)
	}
	if st.Installed {
		t.Error("Installed = true, want false when the timer unit is missing")
	}
	if !st.Loaded {
		t.Error("Loaded = false, want true from a loaded timer")
	}
}

func TestCurrentMismatch(t *testing.T) {
	// arrange
	dir := t.TempDir()
	bin := writeBinary(t, filepath.Join(t.TempDir(), "thepurge"))
	writeUnits(t, dir, bin, false)
	runner := systemdShow("ActiveState=active\n", showOutput("/stale/path/thepurge", 77, 0), nil, nil)

	// act
	st, err := Current(InstallOptions{UnitDir: dir, Runner: runner})

	// assert
	if err != nil {
		t.Fatalf("Current() error = %v", err)
	}
	if !st.Mismatch() {
		t.Fatalf("Mismatch() = false, want true; status = %+v", st)
	}
	if st.ProgramLoaded != "/stale/path/thepurge" {
		t.Errorf("ProgramLoaded = %q, want %q", st.ProgramLoaded, "/stale/path/thepurge")
	}
	if st.ProgramWanted != bin {
		t.Errorf("ProgramWanted = %q, want %q", st.ProgramWanted, bin)
	}
}

func TestSystemdState(t *testing.T) {
	tests := []struct {
		name     string
		runner   Runner
		loaded   bool
		running  bool
		pid      int
		lastExit int
		program  string
	}{
		{
			name:     "active timer with running service",
			runner:   systemdShow("ActiveState=active\n", showOutput("/usr/local/bin/thepurge", 4242, 0), nil, nil),
			loaded:   true,
			running:  true,
			pid:      4242,
			lastExit: 0,
			program:  "/usr/local/bin/thepurge",
		},
		{
			name:     "active timer with idle oneshot",
			runner:   systemdShow("ActiveState=active\n", showOutput("/usr/local/bin/thepurge", 0, 1), nil, nil),
			loaded:   true,
			running:  false,
			pid:      0,
			lastExit: 1,
			program:  "/usr/local/bin/thepurge",
		},
		{
			name:    "inactive timer",
			runner:  systemdShow("ActiveState=inactive\n", showOutput("/usr/local/bin/thepurge", 0, 0), nil, nil),
			loaded:  false,
			program: "/usr/local/bin/thepurge",
		},
		{
			name:    "failed timer",
			runner:  systemdShow("ActiveState=failed\n", showOutput("/usr/local/bin/thepurge", 0, 0), nil, nil),
			loaded:  false,
			program: "/usr/local/bin/thepurge",
		},
		{
			name:    "timer query fails",
			runner:  systemdShow("", showOutput("/usr/local/bin/thepurge", 10, 0), errors.New("no such unit"), nil),
			loaded:  false,
			running: true,
			pid:     10,
			program: "/usr/local/bin/thepurge",
		},
		{
			name:   "service query fails",
			runner: systemdShow("ActiveState=active\n", "", nil, errors.New("no such unit")),
			loaded: true,
		},
		{
			name:   "both queries fail",
			runner: systemdShow("", "", errors.New("bus error"), errors.New("bus error")),
		},
		{
			name:   "garbage service output",
			runner: systemdShow("ActiveState=active\n", "not a property listing\n", nil, nil),
			loaded: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// act
			loaded, running, pid, lastExit, program := systemdState(tt.runner)

			// assert
			if loaded != tt.loaded {
				t.Errorf("loaded = %v, want %v", loaded, tt.loaded)
			}
			if running != tt.running {
				t.Errorf("running = %v, want %v", running, tt.running)
			}
			if pid != tt.pid {
				t.Errorf("pid = %d, want %d", pid, tt.pid)
			}
			if lastExit != tt.lastExit {
				t.Errorf("lastExit = %d, want %d", lastExit, tt.lastExit)
			}
			if program != tt.program {
				t.Errorf("program = %q, want %q", program, tt.program)
			}
		})
	}
}
