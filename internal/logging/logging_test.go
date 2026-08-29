package logging

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestLoggerWritesTimestampedLine(t *testing.T) {
	path := filepath.Join(t.TempDir(), "thepurge.log")
	lg, err := New(path)
	if err != nil {
		t.Fatal(err)
	}
	defer lg.Close()

	lg.Info("purge applied: freed %s", "1.2 GB")

	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	line := strings.TrimSpace(string(b))
	if !strings.Contains(line, "INFO") {
		t.Errorf("line missing level: %q", line)
	}
	if !strings.Contains(line, "purge applied: freed 1.2 GB") {
		t.Errorf("line missing message: %q", line)
	}
	// Timestamp prefix: RFC3339 starts with 4-digit year.
	if !strings.HasPrefix(line, "20") {
		t.Errorf("line missing timestamp: %q", line)
	}
	if _, err := time.Parse(time.RFC3339, strings.Fields(line)[0]); err != nil {
		t.Errorf("first field is not RFC3339 timestamp: %q (%v)", line, err)
	}
}

func TestLoggerAppends(t *testing.T) {
	path := filepath.Join(t.TempDir(), "thepurge.log")
	lg, _ := New(path)
	lg.Info("first")
	lg.Warn("second")
	lg.Close()

	b, _ := os.ReadFile(path)
	lines := strings.Split(strings.TrimSpace(string(b)), "\n")
	if len(lines) != 2 {
		t.Fatalf("got %d lines, want 2", len(lines))
	}
	if !strings.Contains(lines[1], "WARN second") {
		t.Errorf("second line = %q, want WARN second", lines[1])
	}
}

func TestLoggerCreatesMissingDir(t *testing.T) {
	path := filepath.Join(t.TempDir(), "nested", "dir", "thepurge.log")
	lg, err := New(path)
	if err != nil {
		t.Fatalf("New should create parent dirs: %v", err)
	}
	defer lg.Close()
	lg.Info("hello")
	if _, err := os.Stat(path); err != nil {
		t.Errorf("log file not created: %v", err)
	}
}
