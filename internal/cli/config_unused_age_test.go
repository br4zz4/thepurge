package cli

import (
	"os"
	"strings"
	"testing"
)

const unusedAgeKey = "docker.unused_images_min_age_hours"

func TestConfigGetUnusedMinAgeHoursDefaultsToZero(t *testing.T) {
	// arrange
	writeCfgFile(t, baseCfgYAML)

	// act
	code, out, errOut := runCLI("config", "get", unusedAgeKey)

	// assert
	if code != 0 {
		t.Fatalf("get exit = %d, err=%s", code, errOut)
	}
	if got := strings.TrimSpace(out); got != unusedAgeKey+": 0" {
		t.Errorf("get = %q, want %q", got, unusedAgeKey+": 0")
	}
}

func TestConfigGetUnusedMinAgeHoursReadsFile(t *testing.T) {
	// arrange
	writeCfgFile(t, baseCfgYAML+"docker:\n  unused_images_min_age_hours: 72\n")

	// act
	code, out, errOut := runCLI("config", "get", unusedAgeKey)

	// assert
	if code != 0 {
		t.Fatalf("get exit = %d, err=%s", code, errOut)
	}
	if got := strings.TrimSpace(out); got != unusedAgeKey+": 72" {
		t.Errorf("get = %q, want %q", got, unusedAgeKey+": 72")
	}
}

func TestConfigSetUnusedMinAgeHoursPersists(t *testing.T) {
	// arrange
	p := writeCfgFile(t, baseCfgYAML)

	// act
	setCode, _, setErr := runCLI("config", "set", unusedAgeKey, "96")
	getCode, getOut, _ := runCLI("config", "get", unusedAgeKey)
	data, _ := os.ReadFile(p)

	// assert
	if setCode != 0 {
		t.Fatalf("set exit = %d, err=%s", setCode, setErr)
	}
	if getCode != 0 || strings.TrimSpace(getOut) != unusedAgeKey+": 96" {
		t.Errorf("get = %q (code %d), want 96", getOut, getCode)
	}
	if !strings.Contains(string(data), "unused_images_min_age_hours: 96") {
		t.Errorf("value not persisted in file:\n%s", data)
	}
}

func TestConfigSetUnusedMinAgeHoursZeroAccepted(t *testing.T) {
	// arrange
	writeCfgFile(t, baseCfgYAML)

	// act
	code, _, errOut := runCLI("config", "set", unusedAgeKey, "0")

	// assert
	if code != 0 {
		t.Errorf("set 0 exit = %d, err=%s", code, errOut)
	}
}

func TestConfigSetUnusedMinAgeHoursRejectsInvalid(t *testing.T) {
	cases := []struct{ name, value string }{
		{"negative", "-1"},
		{"not a number", "abc"},
		{"empty", ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			// arrange
			p := writeCfgFile(t, baseCfgYAML)

			// act
			code, _, errOut := runCLI("config", "set", unusedAgeKey, tc.value)
			data, _ := os.ReadFile(p)

			// assert
			if code == 0 {
				t.Error("exit = 0, want non-zero")
			}
			if !strings.Contains(errOut, "config set:") {
				t.Errorf("stderr = %q, want 'config set:' message", errOut)
			}
			if string(data) != baseCfgYAML {
				t.Errorf("config must be untouched on rejection:\n%s", data)
			}
		})
	}
}
