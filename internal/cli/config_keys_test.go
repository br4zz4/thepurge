package cli

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func runCLI(args ...string) (int, string, string) {
	var out, errOut bytes.Buffer
	code := Run(args, &out, &errOut)
	return code, out.String(), errOut.String()
}

func TestConfigSetGetAllScalarKeys(t *testing.T) {
	cases := []struct{ key, value, want string }{
		{"storage.volume", "/tmp", "/tmp"},
		{"storage.threshold_type", "size", "size"},
		{"storage.threshold_value", "12.5", "12.5"},
		{"storage.safety_margin_percent", "7", "7"},
		{"storage.check_interval_minutes", "11", "11"},
		{"storage.poll_interval_seconds", "13", "13"},
		{"storage.max_bytes_per_run", "5GB", "5GB"},
		{"protection.respect_running_apps", "false", "false"},
		{"immutable.check_immutable", "false", "false"},
		{"logging.log_path", "/tmp/x.log", "/tmp/x.log"},
		{"logging.notify_on_failure", "true", "true"},
		{"retention.min_age_days", "3", "3"},
	}
	for _, tc := range cases {
		t.Run(tc.key, func(t *testing.T) {
			// arrange
			writeCfgFile(t, baseCfgYAML)

			// act
			setCode, _, setErr := runCLI("config", "set", tc.key, tc.value)
			getCode, getOut, getErr := runCLI("config", "get", tc.key)

			// assert
			if setCode != 0 {
				t.Fatalf("set exit = %d, err=%s", setCode, setErr)
			}
			if getCode != 0 {
				t.Fatalf("get exit = %d, err=%s", getCode, getErr)
			}
			if got := strings.TrimSpace(getOut); got != tc.key+": "+tc.want {
				t.Errorf("get = %q, want %q", got, tc.key+": "+tc.want)
			}
		})
	}
}

func TestConfigSetRejectsInvalidValues(t *testing.T) {
	cases := []struct{ key, value string }{
		{"storage.threshold_type", "bogus"},
		{"storage.threshold_value", "abc"},
		{"storage.safety_margin_percent", "abc"},
		{"storage.check_interval_minutes", "abc"},
		{"storage.poll_interval_seconds", "abc"},
		{"retention.min_age_days", "abc"},
		{"no.such_key", "1"},
	}
	for _, tc := range cases {
		t.Run(tc.key, func(t *testing.T) {
			// arrange
			writeCfgFile(t, baseCfgYAML)

			// act
			code, _, errOut := runCLI("config", "set", tc.key, tc.value)

			// assert
			if code == 0 {
				t.Error("exit = 0, want non-zero")
			}
			if !strings.Contains(errOut, "config set:") {
				t.Errorf("stderr = %q, want 'config set:' message", errOut)
			}
		})
	}
}

func TestConfigGetUnknownKeyFails(t *testing.T) {
	// arrange
	writeCfgFile(t, baseCfgYAML)

	// act
	code, _, errOut := runCLI("config", "get", "no.such_key")

	// assert
	if code == 0 || !strings.Contains(errOut, "unknown key") {
		t.Errorf("code=%d err=%q, want failure with unknown key", code, errOut)
	}
}

func TestConfigGetSetUsageErrors(t *testing.T) {
	// arrange
	writeCfgFile(t, baseCfgYAML)

	// act
	getCode, _, getErr := runCLI("config", "get")
	setCode, _, setErr := runCLI("config", "set", "storage.volume")

	// assert
	if getCode != 2 || !strings.Contains(getErr, "usage") {
		t.Errorf("get: code=%d err=%q", getCode, getErr)
	}
	if setCode != 2 || !strings.Contains(setErr, "usage") {
		t.Errorf("set: code=%d err=%q", setCode, setErr)
	}
}

func TestConfigSetMissingConfigFails(t *testing.T) {
	// arrange
	t.Setenv("THEPURGE_CONFIG", filepath.Join(t.TempDir(), "absent.yml"))

	// act
	code, _, errOut := runCLI("config", "set", "storage.volume", "/")

	// assert
	if code == 0 || !strings.Contains(errOut, "config not found") {
		t.Errorf("code=%d err=%q", code, errOut)
	}
}

func TestConfigGetMissingConfigFails(t *testing.T) {
	// arrange
	t.Setenv("THEPURGE_CONFIG", filepath.Join(t.TempDir(), "absent.yml"))

	// act
	code, _, errOut := runCLI("config", "get", "storage.volume")

	// assert
	if code == 0 || !strings.Contains(errOut, "config not found") {
		t.Errorf("code=%d err=%q", code, errOut)
	}
}

func TestConfigInitWritesDefaultWithUnlimitedCap(t *testing.T) {
	// arrange
	p := filepath.Join(t.TempDir(), "sub", "config.yml")
	t.Setenv("THEPURGE_CONFIG", p)

	// act
	code, out, errOut := runCLI("config", "init")

	// assert
	if code != 0 {
		t.Fatalf("init exit = %d, err=%s", code, errOut)
	}
	if !strings.Contains(out, "config written") {
		t.Errorf("out = %q", out)
	}
	if _, err := os.Stat(p); err != nil {
		t.Fatalf("config not written: %v", err)
	}
	getCode, getOut, _ := runCLI("config", "get", "storage.max_bytes_per_run")
	if getCode != 0 || strings.TrimSpace(getOut) != "storage.max_bytes_per_run:" {
		t.Errorf("max_bytes_per_run get = %q (code %d), want empty", getOut, getCode)
	}
}

func TestConfigInitRefusesToOverwrite(t *testing.T) {
	// arrange
	p := writeCfgFile(t, baseCfgYAML)

	// act
	code, _, errOut := runCLI("config", "init")

	// assert
	if code == 0 || !strings.Contains(errOut, "already exists") {
		t.Errorf("code=%d err=%q", code, errOut)
	}
	data, _ := os.ReadFile(p)
	if string(data) != baseCfgYAML {
		t.Error("existing config must be untouched")
	}
}

func TestConfigOrphansEnableDisableRoundTrip(t *testing.T) {
	// arrange
	p := writeCfgFile(t, baseCfgYAML)
	orphan := "~/Library/Application Support/Cursor"

	// act
	enableCode, _, _ := runCLI("config", "orphans.enable", "--path", orphan)
	runCLI("config", "orphans.enable", "--path", orphan)
	enabled, _ := os.ReadFile(p)
	disableCode, out, _ := runCLI("config", "orphans.disable", "--path", orphan)
	disabled, _ := os.ReadFile(p)

	// assert
	if enableCode != 0 || disableCode != 0 {
		t.Fatalf("exit codes enable=%d disable=%d", enableCode, disableCode)
	}
	if strings.Count(string(enabled), orphan) != 1 {
		t.Errorf("orphan should appear exactly once after double enable:\n%s", enabled)
	}
	if strings.Contains(string(disabled), orphan) {
		t.Errorf("orphan should be removed after disable:\n%s", disabled)
	}
	if !strings.Contains(out, "orphan disabled") {
		t.Errorf("out = %q", out)
	}
}

func TestConfigOrphansRequiresPathFlag(t *testing.T) {
	// arrange
	writeCfgFile(t, baseCfgYAML)

	// act
	code, _, errOut := runCLI("config", "orphans.enable")

	// assert
	if code != 2 || !strings.Contains(errOut, "usage") {
		t.Errorf("code=%d err=%q", code, errOut)
	}
}

func TestConfigOrphansMissingConfigFails(t *testing.T) {
	// arrange
	t.Setenv("THEPURGE_CONFIG", filepath.Join(t.TempDir(), "absent.yml"))

	// act
	code, _, errOut := runCLI("config", "orphans.enable", "--path", "x")

	// assert
	if code == 0 || !strings.Contains(errOut, "config not found") {
		t.Errorf("code=%d err=%q", code, errOut)
	}
}

func TestConfigValidateReportsTargets(t *testing.T) {
	// arrange
	writeCfgFile(t, baseCfgYAML)

	// act
	code, out, errOut := runCLI("config", "validate")

	// assert
	if code != 0 {
		t.Fatalf("exit = %d, err=%s", code, errOut)
	}
	if !strings.Contains(out, "config valid") || !strings.Contains(out, "targets resolved: 1") {
		t.Errorf("out = %q", out)
	}
}

func TestConfigUnknownSubcommand(t *testing.T) {
	// act
	code, _, errOut := runCLI("config", "frobnicate")

	// assert
	if code != 2 || !strings.Contains(errOut, "unknown config subcommand") {
		t.Errorf("code=%d err=%q", code, errOut)
	}
}

func TestConfigFileMissingFails(t *testing.T) {
	// arrange
	t.Setenv("THEPURGE_CONFIG", filepath.Join(t.TempDir(), "absent.yml"))

	// act
	code, _, errOut := runCLI("config", "--file")

	// assert
	if code == 0 || !strings.Contains(errOut, "config not found") {
		t.Errorf("code=%d err=%q", code, errOut)
	}
}

func TestAppendUniqueAndRemoveStr(t *testing.T) {
	// act
	added := appendUnique([]string{"a"}, "b")
	dup := appendUnique(added, "a")
	removed := removeStr(dup, "a")
	none := removeStr(nil, "x")

	// assert
	if len(added) != 2 || len(dup) != 2 {
		t.Errorf("added=%v dup=%v", added, dup)
	}
	if len(removed) != 1 || removed[0] != "b" {
		t.Errorf("removed = %v", removed)
	}
	if len(none) != 0 {
		t.Errorf("none = %v", none)
	}
}

func TestRunCommandDryRunKeepsFilesApplyDeletes(t *testing.T) {
	// arrange
	home := t.TempDir()
	t.Setenv("HOME", home)
	cfgPath := writeTestConfig(t, home)
	t.Setenv("THEPURGE_CONFIG", cfgPath)
	target := filepath.Join(home, "App", "cache.bin")

	// act
	dryCode, dryOut, dryErr := runCLI("run")
	_, dryStat := os.Stat(target)
	applyCode, applyOut, applyErr := runCLI("run", "--apply")
	_, applyStat := os.Stat(target)

	// assert
	if dryCode != 0 {
		t.Fatalf("run exit = %d, err=%s", dryCode, dryErr)
	}
	if dryStat != nil {
		t.Error("run without --apply must not delete")
	}
	if !strings.Contains(dryOut, "would free") {
		t.Errorf("dry out = %q", dryOut)
	}
	if applyCode != 0 {
		t.Fatalf("run --apply exit = %d, err=%s", applyCode, applyErr)
	}
	if applyStat == nil {
		t.Error("run --apply must delete")
	}
	if !strings.Contains(applyOut, "freed:") {
		t.Errorf("apply out = %q", applyOut)
	}
}

func TestRunCommandMissingConfigFails(t *testing.T) {
	// arrange
	t.Setenv("THEPURGE_CONFIG", filepath.Join(t.TempDir(), "absent.yml"))

	// act
	code, _, errOut := runCLI("run")

	// assert
	if code == 0 || !strings.Contains(errOut, "config not found") {
		t.Errorf("code=%d err=%q", code, errOut)
	}
}

const aboveThresholdYAML = "storage:\n  volume: \"/\"\n  threshold_type: \"percent\"\n  threshold_value: 0\n  safety_margin_percent: 0\n" +
	"retention:\n  min_age_days: 0\n" +
	"docker:\n  enabled: true\n  dangling_images: true\n  builder_cache: true\n" +
	"eviction_targets:\n  safe_caches:\n    - path: \"~/App\"\n      type: cache\n"

const recordingDocker = `#!/bin/sh
echo "$*" >> "$FAKE_DOCKER_LOG"
case "$1" in
  version) echo 27.0.1 ;;
  system) printf 'Images\t0B\nBuild Cache\t2kB\n' ;;
  image) case "$2" in ls) printf 'abc\n' ;; prune) echo "Total reclaimed space: 1kB" ;; esac ;;
  builder) echo "Total reclaimed space: 2kB" ;;
esac
`

func aboveThresholdEnv(t *testing.T) (target, dockerLog string) {
	t.Helper()
	home := t.TempDir()
	t.Setenv("HOME", home)
	dir := filepath.Join(home, "App")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	target = filepath.Join(dir, "cache.bin")
	if err := os.WriteFile(target, make([]byte, 2048), 0o644); err != nil {
		t.Fatal(err)
	}
	writeCfgFile(t, aboveThresholdYAML)
	bin := t.TempDir()
	if err := os.WriteFile(filepath.Join(bin, "docker"), []byte(recordingDocker), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", bin)
	noExtraDockerPaths(t)
	dockerLog = filepath.Join(t.TempDir(), "docker.log")
	t.Setenv("FAKE_DOCKER_LOG", dockerLog)
	return target, dockerLog
}

func dockerCalls(t *testing.T, logPath string) string {
	t.Helper()
	b, err := os.ReadFile(logPath)
	if err != nil && !os.IsNotExist(err) {
		t.Fatal(err)
	}
	return string(b)
}

func TestPurgeApplyAboveThresholdDeletesAndPrunesDocker(t *testing.T) {
	// arrange
	target, logPath := aboveThresholdEnv(t)

	// act
	code, out, errOut := runCLI("purge", "--apply")

	// assert
	if code != 0 {
		t.Fatalf("exit = %d, err=%s", code, errOut)
	}
	if _, err := os.Stat(target); !os.IsNotExist(err) {
		t.Errorf("candidate must be deleted above threshold: %v", err)
	}
	calls := dockerCalls(t, logPath)
	if !strings.Contains(calls, "image prune -f") || !strings.Contains(calls, "builder prune -f") {
		t.Errorf("docker prunes missing; calls:\n%s", calls)
	}
	if strings.Contains(out, "above threshold") || !strings.Contains(out, "docker freed") {
		t.Errorf("out = %q", out)
	}
}

func TestPurgeDryRunAboveThresholdListsCandidatesWithoutDeleting(t *testing.T) {
	// arrange
	target, logPath := aboveThresholdEnv(t)

	// act
	code, out, errOut := runCLI("purge")

	// assert
	if code != 0 {
		t.Fatalf("exit = %d, err=%s", code, errOut)
	}
	if _, err := os.Stat(target); err != nil {
		t.Errorf("dry-run must not delete: %v", err)
	}
	if !strings.Contains(out, "would free") || !strings.Contains(out, target) {
		t.Errorf("dry-run should list the candidate; out:\n%s", out)
	}
	if strings.Contains(dockerCalls(t, logPath), "prune") {
		t.Errorf("dry-run must not prune; calls:\n%s", dockerCalls(t, logPath))
	}
}

func TestRunAboveThresholdDoesNothing(t *testing.T) {
	// arrange
	target, logPath := aboveThresholdEnv(t)

	// act
	code, out, errOut := runCLI("run", "--apply")

	// assert
	if code != 0 {
		t.Fatalf("exit = %d, err=%s", code, errOut)
	}
	if !strings.Contains(out, "above threshold — nothing to evict") {
		t.Errorf("out = %q", out)
	}
	if _, err := os.Stat(target); err != nil {
		t.Errorf("run must not delete above threshold: %v", err)
	}
	if calls := dockerCalls(t, logPath); calls != "" {
		t.Errorf("run must not touch docker above threshold; calls:\n%s", calls)
	}
}
