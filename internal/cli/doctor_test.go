package cli

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"br4zz4/thepurge/internal/version"
)

func writeDoctorConfig(t *testing.T, home, yml string) {
	t.Helper()
	cfgPath := filepath.Join(t.TempDir(), "config.yml")
	if err := os.WriteFile(cfgPath, []byte(yml), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Setenv("THEPURGE_CONFIG", cfgPath)
	t.Setenv("HOME", home)
}

func TestCmdDoctorReportsVersion(t *testing.T) {
	// arrange
	stubCurrentVersion(t, version.Info{
		Version: "v0.2.0",
		Commit:  "abcdef1234",
		Go:      "go1.27.0",
	})
	writeDoctorConfig(t, t.TempDir(), "storage:\n  volume: \"/\"\n  threshold_type: \"percent\"\n  threshold_value: 15\n")
	var out, errOut bytes.Buffer

	// act
	_, _ = cmdDoctor(nil, &out, &errOut)

	// assert
	if !strings.Contains(out.String(), "  version: v0.2.0 (abcdef1) go1.27.0\n") {
		t.Errorf("doctor output missing version line; got:\n%s", out.String())
	}
}

func TestCmdDoctorNoneResolvedMentionsHostAccess(t *testing.T) {
	// arrange
	home := t.TempDir()
	writeDoctorConfig(t, home, "storage:\n  volume: \"/\"\n"+
		"eviction_targets:\n  safe_caches:\n    - path: \"~/Missing\"\n      type: cache\n")
	var out, errOut bytes.Buffer

	// act
	_, _ = cmdDoctor(nil, &out, &errOut)

	// assert
	if !strings.Contains(out.String(), "  targets: none resolved (check paths and host access)\n") {
		t.Errorf("doctor output missing host access hint; got:\n%s", out.String())
	}
	if strings.Contains(out.String(), "Full Disk Access") && !strings.Contains(out.String(), "fda:") {
		t.Errorf("unexpected Full Disk Access guidance; got:\n%s", out.String())
	}
}

func TestCmdDoctorNoneConfiguredFails(t *testing.T) {
	// arrange
	writeDoctorConfig(t, t.TempDir(), "storage:\n  volume: \"/\"\n")
	var out, errOut bytes.Buffer

	// act
	code, err := cmdDoctor(nil, &out, &errOut)

	// assert
	if err != nil {
		t.Fatalf("cmdDoctor() error = %v", err)
	}
	if code != 1 {
		t.Errorf("exit code = %d, want 1", code)
	}
	if !strings.Contains(out.String(), "  targets: none configured") {
		t.Errorf("doctor output missing none configured; got:\n%s", out.String())
	}
	if !strings.Contains(out.String(), "doctor: PROBLEMS FOUND") {
		t.Errorf("doctor output missing problem summary; got:\n%s", out.String())
	}
}

func TestCmdDoctorOKWithReachableTarget(t *testing.T) {
	// arrange
	home := t.TempDir()
	target := filepath.Join(home, "App")
	if err := os.MkdirAll(target, 0o755); err != nil {
		t.Fatal(err)
	}
	writeDoctorConfig(t, home, "storage:\n  volume: \"/\"\n"+
		"eviction_targets:\n  safe_caches:\n    - path: \"~/App\"\n      type: cache\n")
	var out, errOut bytes.Buffer

	// act
	code, err := cmdDoctor(nil, &out, &errOut)

	// assert
	if err != nil {
		t.Fatalf("cmdDoctor() error = %v", err)
	}
	if code != 0 {
		t.Errorf("exit code = %d, want 0; output:\n%s", code, out.String())
	}
	if !strings.Contains(out.String(), "  targets: 1 resolved, 1 reachable") {
		t.Errorf("doctor output missing target summary; got:\n%s", out.String())
	}
	if !strings.Contains(out.String(), "doctor: OK") {
		t.Errorf("doctor output missing OK; got:\n%s", out.String())
	}
}
