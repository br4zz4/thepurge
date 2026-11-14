package service

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

type funcRunner func(name string, args ...string) ([]byte, error)

func (f funcRunner) Run(name string, args ...string) ([]byte, error) {
	return f(name, args...)
}

func writeBinary(t *testing.T, path string) string {
	t.Helper()
	if err := os.WriteFile(path, []byte("binary"), 0o755); err != nil {
		t.Fatal(err)
	}
	return path
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
	if err := os.WriteFile(AgentPlistPath(dir), []byte(xml), 0o644); err != nil {
		t.Fatal(err)
	}
}

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

func TestPlistProgram(t *testing.T) {
	xml, err := RenderPlist(PlistFor("/usr/local/bin/thepurge", 300, false))
	if err != nil {
		t.Fatal(err)
	}

	tests := []struct {
		name string
		xml  string
		want string
	}{
		{
			name: "rendered plist",
			xml:  xml,
			want: "/usr/local/bin/thepurge",
		},
		{
			name: "program arguments key only",
			xml:  `<plist><dict><key>ProgramArguments</key><array></array></dict></plist>`,
			want: "",
		},
		{
			name: "no program arguments key",
			xml:  `<plist><dict><key>Label</key><string>br4zz4.thepurge</string></dict></plist>`,
			want: "",
		},
		{
			name: "no strings at all",
			xml:  `<plist><dict></dict></plist>`,
			want: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// act
			got := plistProgram(tt.xml)

			// assert
			if got != tt.want {
				t.Errorf("plistProgram() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestLoadedState(t *testing.T) {
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
			loaded, running, pid, lastExit := loadedState(tt.runner)

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

func TestLoadedProgram(t *testing.T) {
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
			got := loadedProgram(tt.runner)

			// assert
			if got != tt.want {
				t.Errorf("loadedProgram() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestStatusMismatch(t *testing.T) {
	tests := []struct {
		name   string
		status Status
		want   bool
	}{
		{
			name:   "loaded with different programs",
			status: Status{Loaded: true, Program: "/old/thepurge", PlistProgram: "/new/thepurge"},
			want:   true,
		},
		{
			name:   "loaded with same program",
			status: Status{Loaded: true, Program: "/bin/thepurge", PlistProgram: "/bin/thepurge"},
			want:   false,
		},
		{
			name:   "not loaded",
			status: Status{Loaded: false, Program: "/old/thepurge", PlistProgram: "/new/thepurge"},
			want:   false,
		},
		{
			name:   "loaded program unknown",
			status: Status{Loaded: true, Program: "", PlistProgram: "/new/thepurge"},
			want:   false,
		},
		{
			name:   "plist program unknown",
			status: Status{Loaded: true, Program: "/old/thepurge", PlistProgram: ""},
			want:   false,
		},
		{
			name:   "zero status",
			status: Status{},
			want:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// act
			got := tt.status.Mismatch()

			// assert
			if got != tt.want {
				t.Errorf("Mismatch() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestCurrentNotInstalled(t *testing.T) {
	dir := t.TempDir()
	runner := launchdRunner("PID\tStatus\tLabel\n-\t0\tcom.apple.alpha\n", "", nil)

	// act
	st, err := Current(InstallOptions{LaunchAgentsDir: dir, Runner: runner})

	// assert
	if err != nil {
		t.Fatalf("Current() error = %v", err)
	}
	if st.Installed {
		t.Error("Installed = true, want false")
	}
	if st.PlistPath != AgentPlistPath(dir) {
		t.Errorf("PlistPath = %q, want %q", st.PlistPath, AgentPlistPath(dir))
	}
	if st.PlistProgram != "" {
		t.Errorf("PlistProgram = %q, want empty", st.PlistProgram)
	}
	if st.ProgramExists {
		t.Error("ProgramExists = true, want false")
	}
	if st.Loaded {
		t.Error("Loaded = true, want false")
	}
}

func TestCurrentInstalledAndLoaded(t *testing.T) {
	dir := t.TempDir()
	bin := writeBinary(t, filepath.Join(t.TempDir(), "thepurge"))
	writeAgentPlist(t, dir, bin)
	runner := launchdRunner(
		"PID\tStatus\tLabel\n4242\t0\t"+Label+"\n",
		"\tprogram = "+bin+"\n",
		nil,
	)

	// act
	st, err := Current(InstallOptions{LaunchAgentsDir: dir, Runner: runner})

	// assert
	if err != nil {
		t.Fatalf("Current() error = %v", err)
	}
	if !st.Installed {
		t.Error("Installed = false, want true")
	}
	if st.PlistPath != AgentPlistPath(dir) {
		t.Errorf("PlistPath = %q, want %q", st.PlistPath, AgentPlistPath(dir))
	}
	if st.PlistProgram != bin {
		t.Errorf("PlistProgram = %q, want %q", st.PlistProgram, bin)
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
	if st.Program != bin {
		t.Errorf("Program = %q, want %q", st.Program, bin)
	}
	if st.Mismatch() {
		t.Error("Mismatch() = true, want false")
	}
}

func TestCurrentInstalledIdle(t *testing.T) {
	dir := t.TempDir()
	bin := writeBinary(t, filepath.Join(t.TempDir(), "thepurge"))
	writeAgentPlist(t, dir, bin)
	runner := launchdRunner(
		"PID\tStatus\tLabel\n-\t1\t"+Label+"\n",
		"\tprogram = "+bin+"\n",
		nil,
	)

	// act
	st, err := Current(InstallOptions{LaunchAgentsDir: dir, Runner: runner})

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
	dir := t.TempDir()
	bin := writeBinary(t, filepath.Join(t.TempDir(), "thepurge"))
	writeAgentPlist(t, dir, bin)
	runner := launchdRunner("PID\tStatus\tLabel\n-\t0\tcom.apple.alpha\n", "\tprogram = /somewhere/else\n", nil)

	// act
	st, err := Current(InstallOptions{LaunchAgentsDir: dir, Runner: runner})

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
	if st.Program != "" {
		t.Errorf("Program = %q, want empty when not loaded", st.Program)
	}
	if st.Mismatch() {
		t.Error("Mismatch() = true, want false when not loaded")
	}
}

func TestCurrentMissingBinary(t *testing.T) {
	dir := t.TempDir()
	bin := filepath.Join(t.TempDir(), "gone", "thepurge")
	writeAgentPlist(t, dir, bin)
	runner := launchdRunner("PID\tStatus\tLabel\n-\t0\t"+Label+"\n", "\tprogram = "+bin+"\n", nil)

	// act
	st, err := Current(InstallOptions{LaunchAgentsDir: dir, Runner: runner})

	// assert
	if err != nil {
		t.Fatalf("Current() error = %v", err)
	}
	if !st.Installed {
		t.Error("Installed = false, want true")
	}
	if st.PlistProgram != bin {
		t.Errorf("PlistProgram = %q, want %q", st.PlistProgram, bin)
	}
	if st.ProgramExists {
		t.Error("ProgramExists = true, want false")
	}
}

func TestCurrentMismatch(t *testing.T) {
	dir := t.TempDir()
	bin := writeBinary(t, filepath.Join(t.TempDir(), "thepurge"))
	writeAgentPlist(t, dir, bin)
	runner := launchdRunner(
		"PID\tStatus\tLabel\n777\t0\t"+Label+"\n",
		"\tprogram = /stale/path/thepurge\n",
		nil,
	)

	// act
	st, err := Current(InstallOptions{LaunchAgentsDir: dir, Runner: runner})

	// assert
	if err != nil {
		t.Fatalf("Current() error = %v", err)
	}
	if !st.Mismatch() {
		t.Fatalf("Mismatch() = false, want true; status = %+v", st)
	}
	if st.Program != "/stale/path/thepurge" {
		t.Errorf("Program = %q, want %q", st.Program, "/stale/path/thepurge")
	}
}

func TestAtoi(t *testing.T) {
	tests := []struct {
		in   string
		want int
	}{
		{"42", 42},
		{"0", 0},
		{"-9", -9},
		{"-", 0},
		{"", 0},
		{"abc", 0},
	}

	for _, tt := range tests {
		if got := atoi(tt.in); got != tt.want {
			t.Errorf("atoi(%q) = %d, want %d", tt.in, got, tt.want)
		}
	}
}

func TestPlistEscaping(t *testing.T) {
	bin := "/tmp/a & b/<the>purge"

	// act
	xml, err := RenderPlist(PlistFor(bin, 300, false))

	// assert
	if err != nil {
		t.Fatalf("RenderPlist() error = %v", err)
	}
	if !strings.Contains(xml, "/tmp/a &amp; b/&lt;the&gt;purge") {
		t.Errorf("plist does not escape ProgramPath; got:\n%s", xml)
	}
	if strings.Contains(xml, "<the>") {
		t.Errorf("plist contains raw angle brackets; got:\n%s", xml)
	}
	if got := plistProgram(xml); got != bin {
		t.Errorf("plistProgram() = %q, want %q", got, bin)
	}
}

func TestRenderPlistEscapedPathsValidateWithPlutil(t *testing.T) {
	if _, err := exec.LookPath("plutil"); err != nil {
		t.Skip("plutil not available")
	}
	xml, err := RenderPlist(PlistFor("/tmp/a & b/<the>purge", 300, false))
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(t.TempDir(), "agent.plist")
	if err := os.WriteFile(path, []byte(xml), 0o644); err != nil {
		t.Fatal(err)
	}

	// act
	out, err := exec.Command("plutil", "-lint", path).CombinedOutput()

	// assert
	if err != nil {
		t.Errorf("plutil -lint failed: %v\n%s", err, out)
	}
}

func TestCurrentPlistUnreadable(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("root bypasses file permissions")
	}
	dir := t.TempDir()
	writeAgentPlist(t, dir, "/usr/local/bin/thepurge")
	if err := os.Chmod(dir, 0o000); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.Chmod(dir, 0o755) })

	// act
	_, err := Current(InstallOptions{LaunchAgentsDir: dir, Runner: launchdRunner("", "", nil)})

	// assert
	if err == nil {
		t.Fatal("Current() error = nil, want read error")
	}
	if !strings.Contains(err.Error(), "service status") {
		t.Errorf("error = %v, want service status context", err)
	}
}

func TestCurrentStatError(t *testing.T) {
	dir := t.TempDir()
	blocker := filepath.Join(t.TempDir(), "blocker")
	if err := os.WriteFile(blocker, []byte("not a dir"), 0o644); err != nil {
		t.Fatal(err)
	}
	writeAgentPlist(t, dir, filepath.Join(blocker, "thepurge"))

	// act
	_, err := Current(InstallOptions{LaunchAgentsDir: dir, Runner: launchdRunner("", "", nil)})

	// assert
	if err == nil {
		t.Fatal("Current() error = nil, want stat error")
	}
	if !strings.Contains(err.Error(), "service status") {
		t.Errorf("error = %v, want service status context", err)
	}
}
