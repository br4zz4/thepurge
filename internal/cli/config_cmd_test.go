package cli

import (
	"bytes"
	"strings"
	"testing"
)

func TestRunConfigShowsFormattedSummary(t *testing.T) {
	home := t.TempDir()
	cfgPath := writeTestConfig(t, home)
	t.Setenv("THEPURGE_CONFIG", cfgPath)

	var out, errOut bytes.Buffer
	code := Run([]string{"config"}, &out, &errOut)
	if code != 0 {
		t.Fatalf("config exit = %d, err=%s", code, errOut.String())
	}
	if !strings.Contains(out.String(), "threshold") {
		t.Errorf("config should show formatted summary; got:\n%s", out.String())
	}
	if strings.Contains(out.String(), "usage:") {
		t.Errorf("config without args should NOT show usage; got:\n%s", out.String())
	}
}

func TestRunConfigHelp(t *testing.T) {
	var out, errOut bytes.Buffer
	code := Run([]string{"config", "--help"}, &out, &errOut)
	if code != 0 {
		t.Fatalf("config --help exit = %d, err=%s", code, errOut.String())
	}
	if !strings.Contains(out.String(), "usage:") {
		t.Errorf("config --help should show usage; got:\n%s", out.String())
	}
}

func TestRunConfigFileColoredYAML(t *testing.T) {
	home := t.TempDir()
	cfgPath := writeTestConfig(t, home)
	t.Setenv("THEPURGE_CONFIG", cfgPath)

	var out, errOut bytes.Buffer
	code := Run([]string{"config", "--file"}, &out, &errOut)
	if code != 0 {
		t.Fatalf("config --file exit = %d, err=%s", code, errOut.String())
	}
	if !strings.Contains(out.String(), "\x1b[") {
		t.Errorf("config --file should output colored YAML; got:\n%q", out.String())
	}
	if !strings.Contains(out.String(), "storage:") {
		t.Errorf("config --file should contain YAML; got:\n%q", out.String())
	}
}
