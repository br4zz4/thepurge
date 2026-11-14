//go:build darwin

package eviction

import (
	"os"
	"path/filepath"
	"testing"
)

func writeAppBundle(t *testing.T, home, name string) string {
	t.Helper()
	dir := filepath.Join(home, "Applications", name)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	return dir
}

func TestCheckApplicationsDarwinFindsHomeBundle(t *testing.T) {
	// arrange
	home := t.TempDir()
	t.Setenv("HOME", home)
	writeAppBundle(t, home, "Zzpurgetest.app")

	// act
	ok, name := checkApplications("/somewhere/Zzpurgetest.app")

	// assert
	if !ok {
		t.Fatal("checkApplications() = false, want true for a bundle in ~/Applications")
	}
	if name != "Zzpurgetest" {
		t.Errorf("name = %q, want %q", name, "Zzpurgetest")
	}
}

func TestCheckApplicationsDarwinPrefixedBundle(t *testing.T) {
	// arrange
	home := t.TempDir()
	t.Setenv("HOME", home)
	writeAppBundle(t, home, "Zzpurgetestsuite.app")

	// act
	ok, name := checkApplications("/somewhere/Zzpurgetest.app")

	// assert
	if !ok {
		t.Fatal("checkApplications() = false, want true for a prefixed bundle")
	}
	if name != "Zzpurgetestsuite.app" {
		t.Errorf("name = %q, want %q", name, "Zzpurgetestsuite.app")
	}
}

func TestCheckApplicationsDarwinNotFound(t *testing.T) {
	// arrange
	t.Setenv("HOME", t.TempDir())

	// act
	ok, name := checkApplications("/somewhere/Zzpurgetest.app")

	// assert
	if ok {
		t.Error("checkApplications() = true, want false")
	}
	if name != "Zzpurgetest" {
		t.Errorf("name = %q, want %q", name, "Zzpurgetest")
	}
}
