//go:build darwin

package eviction

import (
	"os"
	"path/filepath"
	"testing"
)

func writeTreeFile(t *testing.T, path string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("data"), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestScanPrunesTCCProtectedDirs(t *testing.T) {
	// arrange
	home := t.TempDir()
	t.Setenv("HOME", home)
	locked := filepath.Join(home, "Library", "Caches", "com.apple.Music", "locked.bin")
	okFile := filepath.Join(home, "Library", "Caches", "other", "ok.bin")
	deepOK := filepath.Join(home, "Library", "Caches", "other", "Safe", "more.bin")
	writeTreeFile(t, locked)
	writeTreeFile(t, okFile)
	writeTreeFile(t, deepOK)
	writeTreeFile(t, filepath.Join(home, "Music", "song.bin"))

	// act
	got := scanPaths(t, []Item{{Path: filepath.Join(home, "Library"), Type: "cache"}})

	// assert
	if len(got) != 2 {
		t.Fatalf("candidates = %v, want only the two unguarded files", got)
	}
	for _, want := range []string{okFile, deepOK} {
		if !got[want] {
			t.Errorf("missing candidate %q; got %v", want, got)
		}
	}
	if got[locked] {
		t.Errorf("guarded file %q must not be a candidate", locked)
	}
}

func TestScanPrunesGuardedRoot(t *testing.T) {
	// arrange
	home := t.TempDir()
	t.Setenv("HOME", home)
	writeTreeFile(t, filepath.Join(home, "Music", "Media", "song.bin"))
	writeTreeFile(t, filepath.Join(home, "Movies", "clip.mov"))

	// act
	got := scanPaths(t, []Item{
		{Path: filepath.Join(home, "Music"), Type: "cache"},
		{Path: filepath.Join(home, "Movies"), Type: "cache"},
	})

	// assert
	if len(got) != 0 {
		t.Errorf("candidates = %v, want none when every root is TCC-guarded", got)
	}
}

func TestScanKeepsUnprotectedDirs(t *testing.T) {
	// arrange
	home := t.TempDir()
	t.Setenv("HOME", home)
	desktop := filepath.Join(home, "Desktop", "a.bin")
	logs := filepath.Join(home, "Library", "Logs", "b.log")
	writeTreeFile(t, desktop)
	writeTreeFile(t, logs)

	// act
	got := scanPaths(t, []Item{{Path: home, Type: "cache"}})

	// assert
	if len(got) != 2 {
		t.Fatalf("candidates = %v, want both unprotected files", got)
	}
	for _, want := range []string{desktop, logs} {
		if !got[want] {
			t.Errorf("missing candidate %q; got %v", want, got)
		}
	}
}
