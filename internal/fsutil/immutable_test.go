//go:build darwin

package fsutil

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

// setImmutable marks path immutable via chflags uchg and returns a cleanup func.
func setImmutable(t *testing.T, path string) func() {
	t.Helper()
	if err := exec.Command("chflags", "uchg", path).Run(); err != nil {
		t.Fatalf("chflags uchg %s: %v", path, err)
	}
	return func() {
		if err := exec.Command("chflags", "nouchg", path).Run(); err != nil {
			t.Logf("cleanup chflags nouchg %s: %v", path, err)
		}
	}
}

func TestIsImmutableTrue(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "locked")
	if err := os.WriteFile(p, []byte("x"), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	cleanup := setImmutable(t, p)
	defer cleanup()

	if !IsImmutable(p) {
		t.Errorf("IsImmutable(%q) = false, want true (uchg set)", p)
	}
}

func TestIsImmutableFalse(t *testing.T) {
	p := filepath.Join(t.TempDir(), "plain")
	if err := os.WriteFile(p, []byte("x"), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	if IsImmutable(p) {
		t.Errorf("IsImmutable(%q) = true, want false (no flags)", p)
	}
}

func TestIsImmutableMissing(t *testing.T) {
	if IsImmutable("/no/such/file/thepurge") {
		t.Error("IsImmutable on missing file = true, want false")
	}
}
