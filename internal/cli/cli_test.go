package cli

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRunHelp(t *testing.T) {
	var out, errOut bytes.Buffer
	code := Run([]string{"--help"}, &out, &errOut)
	if code != 0 {
		t.Errorf("--help exit code = %d, want 0", code)
	}
	if !strings.Contains(out.String(), "Usage") {
		t.Errorf("help missing usage; got:\n%s", out.String())
	}
}

func TestRunUnknownCommand(t *testing.T) {
	var out, errOut bytes.Buffer
	code := Run([]string{"bogus"}, &out, &errOut)
	if code == 0 {
		t.Error("unknown command should exit non-zero")
	}
	if !strings.Contains(errOut.String(), "unknown command") {
		t.Errorf("expected unknown command error; got:\n%s", errOut.String())
	}
}

func TestRunNoArgsShowsHelp(t *testing.T) {
	var out, errOut bytes.Buffer
	code := Run(nil, &out, &errOut)
	if code != 0 {
		t.Errorf("no args exit code = %d, want 0 (help)", code)
	}
}

func TestRunStatusShowsSpace(t *testing.T) {
	cfgDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(cfgDir, "config.yml"), []byte("storage:\n  volume: \"/\"\n  threshold_type: \"percent\"\n  threshold_value: 15\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Setenv("THEPURGE_CONFIG", filepath.Join(cfgDir, "config.yml"))

	var out, errOut bytes.Buffer
	code := Run([]string{"status"}, &out, &errOut)
	if code != 0 {
		t.Errorf("status exit code = %d, want 0; err=%s", code, errOut.String())
	}
	if !strings.Contains(out.String(), "free") {
		t.Errorf("status should mention free space; got:\n%s", out.String())
	}
}
