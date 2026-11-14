//go:build darwin

package service

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

type failingRunner struct {
	err  error
	cmds [][]string
}

func (f *failingRunner) Run(name string, args ...string) ([]byte, error) {
	f.cmds = append(f.cmds, append([]string{name}, args...))
	return []byte("boom"), f.err
}

func TestBackendAndUnitPaths(t *testing.T) {
	// arrange
	home := t.TempDir()
	t.Setenv("HOME", home)

	// act
	dir, err := defaultUnitDir()

	// assert
	if err != nil {
		t.Fatalf("defaultUnitDir() error = %v", err)
	}
	if backend != "launchd" {
		t.Errorf("backend = %q, want launchd", backend)
	}
	if want := filepath.Join(home, "Library", "LaunchAgents"); dir != want {
		t.Errorf("defaultUnitDir() = %q, want %q", dir, want)
	}
	if want := filepath.Join(dir, "br4zz4.thepurge.plist"); unitPath(dir) != want {
		t.Errorf("unitPath() = %q, want %q", unitPath(dir), want)
	}
}

func TestInstallWritesPlistAndBootstraps(t *testing.T) {
	// arrange
	agents := t.TempDir()
	bin := writeBinary(t, filepath.Join(t.TempDir(), "thepurge"))
	r := &fakeRunner{}
	wantXML, err := RenderPlist(PlistFor(bin, 300, false))
	if err != nil {
		t.Fatal(err)
	}

	// act
	err = Install(InstallOptions{BinPath: bin, IntervalSeconds: 300, UnitDir: agents, Runner: r})

	// assert
	if err != nil {
		t.Fatalf("Install() error = %v", err)
	}
	plist := unitPath(agents)
	data, err := os.ReadFile(plist)
	if err != nil {
		t.Fatalf("plist not written: %v", err)
	}
	if string(data) != wantXML {
		t.Errorf("plist content = %q, want %q", data, wantXML)
	}
	bootout := callIndex(r.cmds, "launchctl", "bootout", "gui/"+uid()+"/"+Label)
	bootstrap := callIndex(r.cmds, "launchctl", "bootstrap", "gui/"+uid(), plist)
	if bootout < 0 {
		t.Errorf("missing launchctl bootout; calls = %v", r.cmds)
	}
	if bootstrap < 0 {
		t.Errorf("missing launchctl bootstrap %s; calls = %v", plist, r.cmds)
	}
	if bootout >= 0 && bootstrap >= 0 && bootout > bootstrap {
		t.Errorf("bootout must precede bootstrap; calls = %v", r.cmds)
	}
}

func TestInstallApplyFlag(t *testing.T) {
	tests := []struct {
		name  string
		apply bool
		want  bool
	}{
		{name: "dry run", apply: false, want: false},
		{name: "apply", apply: true, want: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// arrange
			agents := t.TempDir()
			bin := writeBinary(t, filepath.Join(t.TempDir(), "thepurge"))
			r := &fakeRunner{}

			// act
			err := Install(InstallOptions{BinPath: bin, IntervalSeconds: 300, Apply: tt.apply, UnitDir: agents, Runner: r})

			// assert
			if err != nil {
				t.Fatalf("Install() error = %v", err)
			}
			data, err := os.ReadFile(unitPath(agents))
			if err != nil {
				t.Fatal(err)
			}
			if got := strings.Contains(string(data), "--apply"); got != tt.want {
				t.Errorf("plist contains --apply = %v, want %v; got:\n%s", got, tt.want, data)
			}
		})
	}
}

func TestInstallDefaultInterval(t *testing.T) {
	// arrange
	agents := t.TempDir()
	bin := writeBinary(t, filepath.Join(t.TempDir(), "thepurge"))

	// act
	err := Install(InstallOptions{BinPath: bin, UnitDir: agents, Runner: &fakeRunner{}})

	// assert
	if err != nil {
		t.Fatalf("Install() error = %v", err)
	}
	data, err := os.ReadFile(unitPath(agents))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), "<key>StartInterval</key>\n\t<integer>1800</integer>") {
		t.Errorf("plist missing default 1800s interval; got:\n%s", data)
	}
}

func TestInstallBootstrapError(t *testing.T) {
	// arrange
	agents := t.TempDir()
	bin := writeBinary(t, filepath.Join(t.TempDir(), "thepurge"))
	r := &failingRunner{err: errors.New("Bootstrap failed: 5: Input/output error")}

	// act
	err := Install(InstallOptions{BinPath: bin, IntervalSeconds: 300, UnitDir: agents, Runner: r})

	// assert
	if err == nil {
		t.Fatal("Install() error = nil, want bootstrap failure")
	}
	if !strings.Contains(err.Error(), "service install: launchctl bootstrap") {
		t.Errorf("error = %v, want bootstrap context", err)
	}
	if !strings.Contains(err.Error(), "boom") {
		t.Errorf("error = %v, want command output", err)
	}
	if _, statErr := os.Stat(unitPath(agents)); statErr != nil {
		t.Errorf("plist should be written before the load attempt: %v", statErr)
	}
}

func TestUninstallRemovesPlistAndBootouts(t *testing.T) {
	// arrange
	agents := t.TempDir()
	bin := writeBinary(t, filepath.Join(t.TempDir(), "thepurge"))
	xml, err := RenderPlist(PlistFor(bin, 300, false))
	if err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(agents, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(unitPath(agents), []byte(xml), 0o644); err != nil {
		t.Fatal(err)
	}
	r := &fakeRunner{}

	// act
	err = Uninstall(InstallOptions{UnitDir: agents, Runner: r})

	// assert
	if err != nil {
		t.Fatalf("Uninstall() error = %v", err)
	}
	if _, statErr := os.Stat(unitPath(agents)); !os.IsNotExist(statErr) {
		t.Errorf("plist still present after uninstall: %v", statErr)
	}
	if callIndex(r.cmds, "launchctl", "bootout", "gui/"+uid(), unitPath(agents)) < 0 {
		t.Errorf("missing launchctl bootout; calls = %v", r.cmds)
	}
}

func TestUninstallRemoveError(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("root bypasses file permissions")
	}
	// arrange
	agents := t.TempDir()
	if err := os.WriteFile(unitPath(agents), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(agents, 0o500); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.Chmod(agents, 0o755) })

	// act
	err := Uninstall(InstallOptions{UnitDir: agents, Runner: &fakeRunner{}})

	// assert
	if err == nil {
		t.Fatal("Uninstall() error = nil, want removal error")
	}
}
