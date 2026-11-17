package cli

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestCommandsFailWhenConfigMissing(t *testing.T) {
	for _, cmd := range []string{"status", "doctor", "purge", "config"} {
		t.Run(cmd, func(t *testing.T) {
			// arrange
			t.Setenv("THEPURGE_CONFIG", filepath.Join(t.TempDir(), "absent.yml"))

			// act
			code, _, errOut := runCLI(cmd)

			// assert
			if code == 0 || !strings.Contains(errOut, "config not found") {
				t.Errorf("code=%d err=%q", code, errOut)
			}
		})
	}
}

func TestStatusUnreadableVolumeFails(t *testing.T) {
	// arrange
	writeCfgFile(t, "storage:\n  volume: \"/no/such/volume/xyz\"\n  threshold_type: \"percent\"\n  threshold_value: 15\n")

	// act
	code, _, errOut := runCLI("status")

	// assert
	if code == 0 || !strings.Contains(errOut, "reading disk usage") {
		t.Errorf("code=%d err=%q", code, errOut)
	}
}

func TestDoctorNoTargetsConfiguredReportsProblem(t *testing.T) {
	// arrange
	writeCfgFile(t, "storage:\n  volume: \"/\"\n  threshold_type: \"percent\"\n  threshold_value: 15\n")

	// act
	code, out, _ := runCLI("doctor")

	// assert
	if code == 0 || !strings.Contains(out, "none configured") {
		t.Errorf("code=%d out=%q", code, out)
	}
}
