package cli

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRunLogsShowsEntries(t *testing.T) {
	logPath := filepath.Join(t.TempDir(), "thepurge.log")
	if err := os.WriteFile(logPath, []byte("2026-08-29T00:00:00Z INFO first\n2026-08-29T00:00:01Z INFO second\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	// Point logs at the temp log via a config with logging.log_path.
	cfgPath := filepath.Join(t.TempDir(), "config.yml")
	if err := os.WriteFile(cfgPath, []byte("logging:\n  log_path: \""+logPath+"\"\nstorage:\n  volume: \"/\"\n  threshold_type: \"percent\"\n  threshold_value: 15\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Setenv("THEPURGE_CONFIG", cfgPath)

	var out, errOut bytes.Buffer
	code := Run([]string{"logs"}, &out, &errOut)
	if code != 0 {
		t.Fatalf("logs exit = %d, err=%s", code, errOut.String())
	}
	if !strings.Contains(out.String(), "first") || !strings.Contains(out.String(), "second") {
		t.Errorf("logs output missing entries; got:\n%s", out.String())
	}
}

func TestRunLogsN(t *testing.T) {
	logPath := filepath.Join(t.TempDir(), "thepurge.log")
	var b strings.Builder
	for i := 1; i <= 5; i++ {
		b.WriteString("2026-08-29T00:00:00Z INFO line ")
		b.WriteString(string(rune('0' + i)))
		b.WriteString("\n")
	}
	if err := os.WriteFile(logPath, []byte(b.String()), 0o644); err != nil {
		t.Fatal(err)
	}
	cfgPath := filepath.Join(t.TempDir(), "config.yml")
	if err := os.WriteFile(cfgPath, []byte("logging:\n  log_path: \""+logPath+"\"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Setenv("THEPURGE_CONFIG", cfgPath)

	var out, errOut bytes.Buffer
	code := Run([]string{"logs", "-n", "2"}, &out, &errOut)
	if code != 0 {
		t.Fatalf("logs -n exit = %d, err=%s", code, errOut.String())
	}
	if strings.Contains(out.String(), "line 1") || strings.Contains(out.String(), "line 2") || strings.Contains(out.String(), "line 3") {
		t.Errorf("-n 2 should only show last 2 lines; got:\n%s", out.String())
	}
	if !strings.Contains(out.String(), "line 4") || !strings.Contains(out.String(), "line 5") {
		t.Errorf("-n 2 should show lines 4,5; got:\n%s", out.String())
	}
}
