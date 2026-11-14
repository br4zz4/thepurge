package service

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

type fakeRunner struct {
	cmds [][]string
}

func (f *fakeRunner) Run(name string, args ...string) ([]byte, error) {
	f.cmds = append(f.cmds, append([]string{name}, args...))
	return nil, nil
}

func callIndex(cmds [][]string, want ...string) int {
	for i, c := range cmds {
		if len(c) != len(want) {
			continue
		}
		match := true
		for j := range want {
			if c[j] != want[j] {
				match = false
				break
			}
		}
		if match {
			return i
		}
	}
	return -1
}

func TestInstallRequiresBinPath(t *testing.T) {
	// arrange
	r := &fakeRunner{}

	// act
	err := Install(InstallOptions{UnitDir: t.TempDir(), Runner: r})

	// assert
	if err == nil {
		t.Fatal("Install() error = nil, want error")
	}
	if !strings.Contains(err.Error(), "BinPath is required") {
		t.Errorf("error = %v, want BinPath requirement", err)
	}
	if len(r.cmds) != 0 {
		t.Errorf("runner called despite invalid options: %v", r.cmds)
	}
}

func TestInstallMissingBinary(t *testing.T) {
	// arrange
	dir := t.TempDir()
	r := &fakeRunner{}

	// act
	err := Install(InstallOptions{
		BinPath: filepath.Join(dir, "gone", "thepurge"),
		UnitDir: dir,
		Runner:  r,
	})

	// assert
	if err == nil {
		t.Fatal("Install() error = nil, want error")
	}
	if !strings.Contains(err.Error(), "not found") {
		t.Errorf("error = %v, want not found", err)
	}
	if len(r.cmds) != 0 {
		t.Errorf("runner called despite missing binary: %v", r.cmds)
	}
}

func TestUninstallNotInstalledIsNoop(t *testing.T) {
	// arrange
	dir := t.TempDir()
	r := &fakeRunner{}

	// act
	err := Uninstall(InstallOptions{UnitDir: dir, Runner: r})

	// assert
	if err != nil {
		t.Fatalf("Uninstall() error = %v", err)
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 0 {
		t.Errorf("unit dir not clean after uninstall: %v", entries)
	}
}
