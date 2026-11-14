package service

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

func TestDefaultLogPath(t *testing.T) {
	// arrange
	home, err := os.UserHomeDir()
	if err != nil {
		t.Skipf("no home dir: %v", err)
	}
	want := filepath.Join(home, "Library", "Logs", "thepurge.log")
	if runtime.GOOS == "linux" {
		want = filepath.Join(home, ".local", "state", "thepurge", "thepurge.log")
	}

	// act
	got := defaultLogPath()

	// assert
	if got != want {
		t.Errorf("defaultLogPath() = %q, want %q", got, want)
	}
	if !filepath.IsAbs(got) {
		t.Errorf("defaultLogPath() = %q, want an absolute path", got)
	}
}

func TestPlistForUsesDefaultLogPath(t *testing.T) {
	// arrange
	logPath := defaultLogPath()

	// act
	pl := PlistFor("/usr/local/bin/thepurge", 300, false)

	// assert
	if pl.LogPath != logPath {
		t.Errorf("PlistFor().LogPath = %q, want %q", pl.LogPath, logPath)
	}
	if pl.StandardOutPath != logPath {
		t.Errorf("PlistFor().StandardOutPath = %q, want %q", pl.StandardOutPath, logPath)
	}
	if pl.StandardErrorPath != logPath {
		t.Errorf("PlistFor().StandardErrorPath = %q, want %q", pl.StandardErrorPath, logPath)
	}
}
