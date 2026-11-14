package cli

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"br4zz4/thepurge/config"
)

func TestParseLogFlags(t *testing.T) {
	n, tail := parseLogFlags([]string{"-n", "10"})
	if n != 10 || tail {
		t.Errorf("parseLogFlags(-n 10) = (%d, %v), want (10, false)", n, tail)
	}

	n, tail = parseLogFlags([]string{"--tail"})
	if tail != true || n != defaultLogLines {
		t.Errorf("parseLogFlags(--tail) = (%d, %v), want (default, true)", n, tail)
	}

	n, tail = parseLogFlags([]string{"-t"})
	if tail != true {
		t.Errorf("parseLogFlags(-t) tail = %v, want true", tail)
	}

	n, _ = parseLogFlags([]string{"--n=25"})
	if n != 25 {
		t.Errorf("parseLogFlags(--n=25) n = %d, want 25", n)
	}
}

func TestReadTailReturnsLastN(t *testing.T) {
	path := filepath.Join(t.TempDir(), "thepurge.log")
	var b strings.Builder
	for i := 1; i <= 100; i++ {
		fmt.Fprintf(&b, "2026-08-29T00:00:00Z INFO event %03d\n", i)
	}
	if err := os.WriteFile(path, []byte(b.String()), 0o644); err != nil {
		t.Fatal(err)
	}

	got, err := readTail(path, 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 10 {
		t.Fatalf("readTail returned %d lines, want 10", len(got))
	}
	if !strings.Contains(got[len(got)-1], "event 100") {
		t.Errorf("last line = %q, want event 100", got[len(got)-1])
	}
	if !strings.Contains(got[0], "event 091") {
		t.Errorf("first of last-10 = %q, want event 091", got[0])
	}
}

func TestReadTailMissingFile(t *testing.T) {
	got, err := readTail(filepath.Join(t.TempDir(), "nope.log"), 10)
	if err != nil {
		t.Fatalf("readTail on missing file should not error: %v", err)
	}
	if len(got) != 0 {
		t.Errorf("readTail missing file = %d lines, want 0", len(got))
	}
}

func TestLogPathForUsesConfigPath(t *testing.T) {
	// arrange
	cfg := &config.Config{Logging: config.Logging{LogPath: "/var/log/custom.log"}}

	// act
	got := logPathFor(cfg)

	// assert
	if got != "/var/log/custom.log" {
		t.Errorf("logPathFor() = %q, want %q", got, "/var/log/custom.log")
	}
}

func TestLogPathForFallsBackToPlatformDefault(t *testing.T) {
	// arrange
	home := t.TempDir()
	t.Setenv("HOME", home)

	// act
	got := logPathFor(nil)

	// assert
	want := expandHome(config.DefaultFor(runtime.GOOS).Logging.LogPath)
	if got != want {
		t.Errorf("logPathFor() = %q, want %q", got, want)
	}
	if !strings.HasPrefix(got, home) {
		t.Errorf("logPathFor() = %q, want path under %q", got, home)
	}
}

func TestLogPathForExpandsHome(t *testing.T) {
	// arrange
	home := t.TempDir()
	t.Setenv("HOME", home)
	cfg := &config.Config{Logging: config.Logging{LogPath: "~/state/thepurge.log"}}

	// act
	got := logPathFor(cfg)

	// assert
	if want := filepath.Join(home, "state", "thepurge.log"); got != want {
		t.Errorf("logPathFor() = %q, want %q", got, want)
	}
}
