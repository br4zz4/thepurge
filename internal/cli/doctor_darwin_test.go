//go:build darwin

package cli

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestCheckHostAccessReadable(t *testing.T) {
	// arrange
	home := t.TempDir()
	if err := os.MkdirAll(filepath.Join(home, "Library", "Application Support"), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("HOME", home)
	var out bytes.Buffer

	// act
	ok := checkHostAccess(&out)

	// assert
	if !ok {
		t.Error("checkHostAccess() = false, want true")
	}
	if !strings.Contains(out.String(), "  fda: OK (can read Library/Application Support)\n") {
		t.Errorf("output missing OK line; got:\n%s", out.String())
	}
}

func TestCheckHostAccessProbeMissing(t *testing.T) {
	// arrange
	t.Setenv("HOME", t.TempDir())
	var out bytes.Buffer

	// act
	ok := checkHostAccess(&out)

	// assert
	if !ok {
		t.Error("checkHostAccess() = false, want true when the probe path does not exist")
	}
	if !strings.Contains(out.String(), "does not exist (not a TCC failure)") {
		t.Errorf("output missing missing-probe note; got:\n%s", out.String())
	}
	if strings.Contains(out.String(), "grant Full Disk Access") {
		t.Errorf("output must not ask for FDA on a missing path; got:\n%s", out.String())
	}
}

func TestCheckHostAccessDenied(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("root bypasses file permissions")
	}
	// arrange
	home := t.TempDir()
	library := filepath.Join(home, "Library")
	if err := os.MkdirAll(filepath.Join(library, "Application Support"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(library, 0o000); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.Chmod(library, 0o755) })
	t.Setenv("HOME", home)
	var out bytes.Buffer

	// act
	ok := checkHostAccess(&out)

	// assert
	if ok {
		t.Error("checkHostAccess() = true, want false on permission denied")
	}
	if !strings.Contains(out.String(), "  fda: grant Full Disk Access to thepurge") {
		t.Errorf("output missing FDA instructions; got:\n%s", out.String())
	}
}

func TestCheckHostAccessNoHome(t *testing.T) {
	// arrange
	t.Setenv("HOME", "")
	var out bytes.Buffer

	// act
	ok := checkHostAccess(&out)

	// assert
	if !ok {
		t.Error("checkHostAccess() = false, want true without a home dir")
	}
	if out.Len() != 0 {
		t.Errorf("unexpected output: %s", out.String())
	}
}
