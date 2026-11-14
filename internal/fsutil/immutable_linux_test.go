//go:build linux

package fsutil

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

func chattrFlag(t *testing.T, flag, path string) {
	t.Helper()
	if _, err := exec.LookPath("chattr"); err != nil {
		t.Skip("chattr not available (install e2fsprogs)")
	}
	if err := exec.Command("chattr", flag, path).Run(); err != nil {
		t.Skipf("chattr %s unsupported on this filesystem: %v", flag, err)
	}
	undo := "-i"
	if flag == "+a" {
		undo = "-a"
	}
	t.Cleanup(func() { exec.Command("chattr", undo, path).Run() })
}

func TestIsImmutableLinuxPlainFile(t *testing.T) {
	// arrange
	p := filepath.Join(t.TempDir(), "plain")
	if err := os.WriteFile(p, []byte("data"), 0o644); err != nil {
		t.Fatal(err)
	}

	// act
	got := IsImmutable(p)

	// assert
	if got {
		t.Errorf("IsImmutable(%q) = true, want false", p)
	}
}

func TestIsImmutableLinuxPlainDir(t *testing.T) {
	// arrange
	dir := t.TempDir()

	// act
	got := IsImmutable(dir)

	// assert
	if got {
		t.Errorf("IsImmutable(%q) = true, want false", dir)
	}
}

func TestIsImmutableLinuxMissingPath(t *testing.T) {
	// arrange
	p := filepath.Join(t.TempDir(), "gone")

	// act
	got := IsImmutable(p)

	// assert
	if got {
		t.Errorf("IsImmutable(%q) = true, want false", p)
	}
}

func TestIsImmutableLinuxUnopenableFile(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("root bypasses file permissions")
	}
	// arrange
	p := filepath.Join(t.TempDir(), "unreadable")
	if err := os.WriteFile(p, []byte("data"), 0o000); err != nil {
		t.Fatal(err)
	}

	// act
	got := IsImmutable(p)

	// assert
	if got {
		t.Errorf("IsImmutable(%q) = true, want false without a usable fd", p)
	}
}

func TestIsImmutableLinuxImmutableFlag(t *testing.T) {
	// arrange
	p := filepath.Join(t.TempDir(), "locked")
	if err := os.WriteFile(p, []byte("data"), 0o644); err != nil {
		t.Fatal(err)
	}
	chattrFlag(t, "+i", p)

	// act
	got := IsImmutable(p)

	// assert
	if !got {
		t.Errorf("IsImmutable(%q) = false, want true with FS_IMMUTABLE_FL", p)
	}
}

func TestIsImmutableLinuxAppendFlag(t *testing.T) {
	// arrange
	p := filepath.Join(t.TempDir(), "appendonly")
	if err := os.WriteFile(p, []byte("data"), 0o644); err != nil {
		t.Fatal(err)
	}
	chattrFlag(t, "+a", p)

	// act
	got := IsImmutable(p)

	// assert
	if !got {
		t.Errorf("IsImmutable(%q) = false, want true with FS_APPEND_FL", p)
	}
}
