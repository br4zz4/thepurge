package service

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
)

// fakeRunner records launchctl commands instead of running them.
type fakeRunner struct {
	cmds [][]string
}

func (f *fakeRunner) Run(name string, args ...string) ([]byte, error) {
	f.cmds = append(f.cmds, append([]string{name}, args...))
	return nil, nil
}

func TestInstallWritesPlistAndBootstraps(t *testing.T) {
	agents := t.TempDir()
	bin := filepath.Join(t.TempDir(), "thepurge")
	if err := os.WriteFile(bin, []byte("fake"), 0o755); err != nil {
		t.Fatal(err)
	}
	r := &fakeRunner{}
	if err := Install(InstallOptions{
		BinPath:         bin,
		IntervalSeconds: 300,
		LaunchAgentsDir: agents,
		Runner:          r,
	}); err != nil {
		t.Fatal(err)
	}

	plistPath := filepath.Join(agents, "br4zz4.thepurge.plist")
	if _, err := os.Stat(plistPath); err != nil {
		t.Fatalf("plist not written: %v", err)
	}
	data, _ := os.ReadFile(plistPath)
	if !bytes.Contains(data, []byte("br4zz4.thepurge")) {
		t.Error("plist missing label")
	}

	foundBootstrap := false
	for _, c := range r.cmds {
		if c[0] == "launchctl" && c[1] == "bootstrap" {
			foundBootstrap = true
		}
	}
	if !foundBootstrap {
		t.Errorf("expected launchctl bootstrap, got %v", r.cmds)
	}
}

func TestUninstallRemovesPlistAndBootouts(t *testing.T) {
	agents := t.TempDir()
	r := &fakeRunner{}
	if err := Uninstall(InstallOptions{LaunchAgentsDir: agents, Runner: r}); err != nil {
		t.Fatal(err)
	}
	foundBootout := false
	for _, c := range r.cmds {
		if c[0] == "launchctl" && c[1] == "bootout" {
			foundBootout = true
		}
	}
	if !foundBootout {
		t.Errorf("expected launchctl bootout, got %v", r.cmds)
	}
}
