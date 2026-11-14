package cli

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"br4zz4/thepurge/internal/version"
)

func TestCmdDoctorReportsVersion(t *testing.T) {
	// arrange
	stubCurrentVersion(t, version.Info{
		Version: "v0.2.0",
		Commit:  "abcdef1234",
		Go:      "go1.27.0",
	})
	cfgDir := t.TempDir()
	cfgPath := filepath.Join(cfgDir, "config.yml")
	yml := "storage:\n  volume: \"/\"\n  threshold_type: \"percent\"\n  threshold_value: 15\n"
	if err := os.WriteFile(cfgPath, []byte(yml), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Setenv("THEPURGE_CONFIG", cfgPath)
	t.Setenv("HOME", t.TempDir())
	var out, errOut bytes.Buffer

	// act
	_, _ = cmdDoctor(nil, &out, &errOut)

	// assert
	if !strings.Contains(out.String(), "  version: v0.2.0 (abcdef1) go1.27.0\n") {
		t.Errorf("doctor output missing version line; got:\n%s", out.String())
	}
}
