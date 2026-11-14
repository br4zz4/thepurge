//go:build darwin

package service

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func launchdRunner(list, printOut string, err error) Runner {
	return funcRunner(func(_ string, args ...string) ([]byte, error) {
		if err != nil {
			return nil, err
		}
		if len(args) > 0 && args[0] == "list" {
			return []byte(list), nil
		}
		if len(args) > 1 && args[0] == "print" {
			return []byte(printOut), nil
		}
		return nil, nil
	})
}

func writeAgentPlist(t *testing.T, dir, bin string) {
	t.Helper()
	xml, err := RenderPlist(PlistFor(bin, 300, false))
	if err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(unitPath(dir), []byte(xml), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestLaunchdState(t *testing.T) {
	tests := []struct {
		name     string
		runner   Runner
		loaded   bool
		running  bool
		pid      int
		lastExit int
	}{
		{
			name:     "job running",
			runner:   launchdRunner("PID\tStatus\tLabel\n123\t0\t"+Label+"\n", "", nil),
			loaded:   true,
			running:  true,
			pid:      123,
			lastExit: 0,
		},
		{
			name:     "job idle",
			runner:   launchdRunner("PID\tStatus\tLabel\n-\t78\t"+Label+"\n", "", nil),
			loaded:   true,
			running:  false,
			pid:      0,
			lastExit: 78,
		},
		{
			name: "job running among other labels",
			runner: launchdRunner(
				"PID\tStatus\tLabel\n-\t0\tcom.apple.alpha\n900\t0\tcom.apple.beta\n55\t0\t"+Label+"\n",
				"", nil,
			),
			loaded:   true,
			running:  true,
			pid:      55,
			lastExit: 0,
		},
		{
			name:   "label absent",
			runner: launchdRunner("PID\tStatus\tLabel\n-\t0\tcom.apple.alpha\n", "", nil),
		},
		{
			name:   "empty output",
			runner: launchdRunner("", "", nil),
		},
		{
			name:   "malformed rows",
			runner: launchdRunner("123\t"+Label+"\n\t\t"+Label+"\n", "", nil),
		},
		{
			name:   "runner error",
			runner: launchdRunner("", "", errors.New("launchctl failed")),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// act
			loaded, running, pid, lastExit := launchdState(tt.runner)

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
		})
	}
}

func TestLaunchdProgram(t *testing.T) {
	realistic := "gui/501/" + Label + " = {\n" +
		"\tactive count = 0\n" +
		"\tpath = /Users/test/Library/LaunchAgents/" + Label + ".plist\n" +
		"\ttype = LaunchAgent\n" +
		"\tstate = not running\n" +
		"\n" +
		"\tprogram = /usr/local/bin/thepurge\n" +
		"\targuments = {\n" +
		"\t\t/usr/local/bin/thepurge\n" +
		"\t\trun\n" +
		"\t}\n" +
		"}\n"

	tests := []struct {
		name   string
		runner Runner
		want   string
	}{
		{
			name:   "program present",
			runner: launchdRunner("", realistic, nil),
			want:   "/usr/local/bin/thepurge",
		},
		{
			name:   "program with padded value",
			runner: launchdRunner("", "\t\tprogram =    /opt/bin/thepurge   \n", nil),
			want:   "/opt/bin/thepurge",
		},
		{
			name:   "program line absent",
			runner: launchdRunner("", "\tstate = not running\n\targuments = {\n\t}\n", nil),
			want:   "",
		},
		{
			name:   "runner error",
			runner: launchdRunner("", "", errors.New("could not find service")),
			want:   "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// act
			got := launchdProgram(tt.runner)

			// assert
			if got != tt.want {
				t.Errorf("launchdProgram() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestCurrentNotInstalled(t *testing.T) {
	// arrange
	dir := t.TempDir()
	runner := launchdRunner("PID\tStatus\tLabel\n-\t0\tcom.apple.alpha\n", "", nil)

	// act
	st, err := Current(InstallOptions{UnitDir: dir, Runner: runner})

	// assert
	if err != nil {
		t.Fatalf("Current() error = %v", err)
	}
	if st.Backend != "launchd" {
		t.Errorf("Backend = %q, want launchd", st.Backend)
	}
	if st.Installed {
		t.Error("Installed = true, want false")
	}
	if st.UnitPath != unitPath(dir) {
		t.Errorf("UnitPath = %q, want %q", st.UnitPath, unitPath(dir))
	}
	if st.ProgramWanted != "" {
		t.Errorf("ProgramWanted = %q, want empty", st.ProgramWanted)
	}
	if st.ProgramExists {
		t.Error("ProgramExists = true, want false")
	}
	if st.Loaded {
		t.Error("Loaded = true, want false")
	}
}

func TestCurrentInstalledAndLoaded(t *testing.T) {
	// arrange
	dir := t.TempDir()
	bin := writeBinary(t, filepath.Join(t.TempDir(), "thepurge"))
	writeAgentPlist(t, dir, bin)
	runner := launchdRunner(
		"PID\tStatus\tLabel\n4242\t0\t"+Label+"\n",
		"\tprogram = "+bin+"\n",
		nil,
	)

	// act
	st, err := Current(InstallOptions{UnitDir: dir, Runner: runner})

	// assert
	if err != nil {
		t.Fatalf("Current() error = %v", err)
	}
	if !st.Installed {
		t.Error("Installed = false, want true")
	}
	if st.UnitPath != unitPath(dir) {
		t.Errorf("UnitPath = %q, want %q", st.UnitPath, unitPath(dir))
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

func TestCurrentInstalledIdle(t *testing.T) {
	// arrange
	dir := t.TempDir()
	bin := writeBinary(t, filepath.Join(t.TempDir(), "thepurge"))
	writeAgentPlist(t, dir, bin)
	runner := launchdRunner(
		"PID\tStatus\tLabel\n-\t1\t"+Label+"\n",
		"\tprogram = "+bin+"\n",
		nil,
	)

	// act
	st, err := Current(InstallOptions{UnitDir: dir, Runner: runner})

	// assert
	if err != nil {
		t.Fatalf("Current() error = %v", err)
	}
	if !st.Loaded {
		t.Error("Loaded = false, want true")
	}
	if st.Running {
		t.Error("Running = true, want false")
	}
	if st.PID != 0 {
		t.Errorf("PID = %d, want 0", st.PID)
	}
	if st.LastExit != 1 {
		t.Errorf("LastExit = %d, want 1", st.LastExit)
	}
}

func TestCurrentNotLoaded(t *testing.T) {
	// arrange
	dir := t.TempDir()
	bin := writeBinary(t, filepath.Join(t.TempDir(), "thepurge"))
	writeAgentPlist(t, dir, bin)
	runner := launchdRunner("PID\tStatus\tLabel\n-\t0\tcom.apple.alpha\n", "\tprogram = /somewhere/else\n", nil)

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
	if st.ProgramLoaded != "" {
		t.Errorf("ProgramLoaded = %q, want empty when not loaded", st.ProgramLoaded)
	}
	if st.Mismatch() {
		t.Error("Mismatch() = true, want false when not loaded")
	}
}

func TestCurrentMissingBinary(t *testing.T) {
	// arrange
	dir := t.TempDir()
	bin := filepath.Join(t.TempDir(), "gone", "thepurge")
	writeAgentPlist(t, dir, bin)
	runner := launchdRunner("PID\tStatus\tLabel\n-\t0\t"+Label+"\n", "\tprogram = "+bin+"\n", nil)

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
	if st.ProgramExists {
		t.Error("ProgramExists = true, want false")
	}
}

func TestCurrentMismatch(t *testing.T) {
	// arrange
	dir := t.TempDir()
	bin := writeBinary(t, filepath.Join(t.TempDir(), "thepurge"))
	writeAgentPlist(t, dir, bin)
	runner := launchdRunner(
		"PID\tStatus\tLabel\n777\t0\t"+Label+"\n",
		"\tprogram = /stale/path/thepurge\n",
		nil,
	)

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
}

func TestCurrentUnreadableUnit(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("root bypasses file permissions")
	}
	// arrange
	dir := t.TempDir()
	if err := os.WriteFile(unitPath(dir), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(dir, 0o000); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.Chmod(dir, 0o755) })

	// act
	_, err := Current(InstallOptions{UnitDir: dir, Runner: launchdRunner("", "", nil)})

	// assert
	if err == nil {
		t.Fatal("Current() error = nil, want read error")
	}
}
