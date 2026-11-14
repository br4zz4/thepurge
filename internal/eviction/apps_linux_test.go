//go:build linux

package eviction

import (
	"os"
	"path/filepath"
	"testing"
)

func isolateAppDirs(t *testing.T) string {
	t.Helper()
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("PATH", t.TempDir())
	return home
}

func writeExecutable(t *testing.T, dir, name string) string {
	t.Helper()
	p := filepath.Join(dir, name)
	if err := os.WriteFile(p, []byte("#!/bin/sh\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	return p
}

func writeDesktopEntry(t *testing.T, home, name string) string {
	t.Helper()
	dir := filepath.Join(home, ".local", "share", "applications")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	p := filepath.Join(dir, name)
	if err := os.WriteFile(p, []byte("[Desktop Entry]\nName=Test\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	return p
}

func TestCheckApplicationsLinuxFindsBinaryOnPath(t *testing.T) {
	// arrange
	isolateAppDirs(t)
	writeExecutable(t, os.Getenv("PATH"), "zzpurgetestapp")

	// act
	ok, name := checkApplications("/somewhere/ZzpurgeTestApp.app")

	// assert
	if !ok {
		t.Fatalf("checkApplications() = false, want true for a binary on PATH")
	}
	if name != "ZzpurgeTestApp" {
		t.Errorf("name = %q, want %q", name, "ZzpurgeTestApp")
	}
}

func TestCheckApplicationsLinuxFindsDesktopEntry(t *testing.T) {
	// arrange
	home := isolateAppDirs(t)
	writeDesktopEntry(t, home, "zzpurgetestapp.desktop")

	// act
	ok, name := checkApplications("/somewhere/ZzpurgeTestApp.app")

	// assert
	if !ok {
		t.Fatalf("checkApplications() = false, want true for a .desktop entry")
	}
	if name != "zzpurgetestapp" {
		t.Errorf("name = %q, want %q", name, "zzpurgetestapp")
	}
}

func TestCheckApplicationsLinuxDesktopPrefixMatch(t *testing.T) {
	// arrange
	home := isolateAppDirs(t)
	writeDesktopEntry(t, home, "zzpurgetestapp-beta.desktop")

	// act
	ok, name := checkApplications("/somewhere/ZzpurgeTestApp.app")

	// assert
	if !ok {
		t.Fatalf("checkApplications() = false, want true for a prefixed .desktop entry")
	}
	if name != "zzpurgetestapp-beta" {
		t.Errorf("name = %q, want %q", name, "zzpurgetestapp-beta")
	}
}

func TestCheckApplicationsLinuxNonExecutableOnPath(t *testing.T) {
	// arrange
	isolateAppDirs(t)
	p := filepath.Join(os.Getenv("PATH"), "zzpurgetestapp")
	if err := os.WriteFile(p, []byte("not executable"), 0o644); err != nil {
		t.Fatal(err)
	}

	// act
	ok, name := checkApplications("/somewhere/ZzpurgeTestApp.app")

	// assert
	if ok {
		t.Errorf("checkApplications() = true, want false for a non-executable file")
	}
	if name != "ZzpurgeTestApp" {
		t.Errorf("name = %q, want %q", name, "ZzpurgeTestApp")
	}
}

func TestCheckApplicationsLinuxNotFound(t *testing.T) {
	// arrange
	isolateAppDirs(t)

	// act
	ok, name := checkApplications("/somewhere/ZzpurgeTestApp.app")

	// assert
	if ok {
		t.Error("checkApplications() = true, want false")
	}
	if name != "ZzpurgeTestApp" {
		t.Errorf("name = %q, want %q", name, "ZzpurgeTestApp")
	}
}

func TestCheckApplicationsLinuxWithoutHome(t *testing.T) {
	// arrange
	t.Setenv("HOME", "")
	t.Setenv("PATH", t.TempDir())

	// act
	ok, _ := checkApplications("/somewhere/ZzpurgeTestApp.app")

	// assert
	if ok {
		t.Error("checkApplications() = true, want false with no HOME")
	}
}

func TestCheckApplicationsLinuxOptDir(t *testing.T) {
	if os.Geteuid() != 0 {
		t.Skip("needs write access to /opt")
	}
	// arrange
	isolateAppDirs(t)
	dir := filepath.Join("/opt", "ZzpurgeTestApp")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Skipf("/opt not writable: %v", err)
	}
	t.Cleanup(func() { os.RemoveAll(dir) })

	// act
	ok, name := checkApplications("/somewhere/ZzpurgeTestApp.app")

	// assert
	if !ok {
		t.Fatalf("checkApplications() = false, want true for /opt/ZzpurgeTestApp")
	}
	if name != "ZzpurgeTestApp" {
		t.Errorf("name = %q, want %q", name, "ZzpurgeTestApp")
	}
}
