package cli

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// writeTestConfig writes a minimal config whose only target is a fake app dir
// under home, and returns the config path.
func writeTestConfig(t *testing.T, home string) string {
	t.Helper()
	app := filepath.Join(home, "App")
	if err := os.MkdirAll(app, 0o755); err != nil {
		t.Fatal(err)
	}
	// seed a cache file
	if err := os.WriteFile(filepath.Join(app, "cache.bin"), make([]byte, 2048), 0o644); err != nil {
		t.Fatal(err)
	}

	yml := "storage:\n  volume: \"/\"\n  threshold_type: \"percent\"\n  threshold_value: 99\n  safety_margin_percent: 0\n" +
		"eviction_targets:\n  safe_caches:\n    - path: \"~/App\"\n      type: cache\n"
	cfgPath := filepath.Join(t.TempDir(), "config.yml")
	if err := os.WriteFile(cfgPath, []byte(yml), 0o644); err != nil {
		t.Fatal(err)
	}
	return cfgPath
}

func TestRunPurgeDryRunDoesNotDelete(t *testing.T) {
	home := t.TempDir()
	cfgPath := writeTestConfig(t, home)
	t.Setenv("THEPURGE_CONFIG", cfgPath)
	t.Setenv("HOME", home) // so ~ expands to the fake home for targets

	targetFile := filepath.Join(home, "App", "cache.bin")
	if _, err := os.Stat(targetFile); err != nil {
		t.Fatal("precondition: target file missing")
	}

	var out, errOut bytes.Buffer
	code := Run([]string{"purge"}, &out, &errOut)
	if code != 0 {
		t.Fatalf("purge exit = %d, err=%s", code, errOut.String())
	}
	if _, err := os.Stat(targetFile); err != nil {
		t.Errorf("dry-run must not delete file: %v", err)
	}
	if !strings.Contains(out.String(), "would free") {
		t.Errorf("dry-run should show 'would free' summary; got:\n%s", out.String())
	}
}

func TestRunPurgeApplyDeletes(t *testing.T) {
	home := t.TempDir()
	cfgPath := writeTestConfig(t, home)
	t.Setenv("THEPURGE_CONFIG", cfgPath)
	t.Setenv("HOME", home)

	targetFile := filepath.Join(home, "App", "cache.bin")

	var out, errOut bytes.Buffer
	code := Run([]string{"purge", "--apply"}, &out, &errOut)
	if code != 0 {
		t.Fatalf("purge --apply exit = %d, err=%s", code, errOut.String())
	}
	if _, err := os.Stat(targetFile); err == nil {
		t.Error("apply should delete the file; still present")
	}
}

func TestRunConfigOrphansEnable(t *testing.T) {
	home := t.TempDir()
	cfgPath := writeTestConfig(t, home)
	t.Setenv("THEPURGE_CONFIG", cfgPath)

	orphan := "~/Application Support/Cursor"
	var out, errOut bytes.Buffer
	code := Run([]string{"config", "orphans.enable", "--path", orphan}, &out, &errOut)
	if code != 0 {
		t.Fatalf("orphans.enable exit = %d, err=%s", code, errOut.String())
	}
	data, err := os.ReadFile(cfgPath)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), orphan) {
		t.Errorf("enabled orphan not persisted in config:\n%s", string(data))
	}
}
