package eviction

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"br4zz4/thepurge/config"
)

// fakeDiskUsage returns a fixed free/total pair regardless of path.
func fakeDiskUsage(free, total int64) func(string) (int64, int64, error) {
	return func(string) (int64, int64, error) {
		return free, total, nil
	}
}

func TestRunAboveThresholdDoesNothing(t *testing.T) {
	home := t.TempDir()
	dir := filepath.Join(home, "App")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	p := writeFileIn(t, dir, "f", 100, time.Now())

	cfg := &config.Config{
		Storage:   config.Storage{Volume: "/", ThresholdType: "percent", ThresholdValue: 15, SafetyMarginPercent: 5},
		Retention: config.Retention{MinAgeDays: 0},
		EvictionTargets: config.EvictionTargets{
			SafeCaches: []config.Target{{Path: "~/App", Type: "cache"}},
		},
	}
	opts := EngineOptions{
		Apply:     true,
		HomeDir:   home,
		DiskUsage: fakeDiskUsage(500, 1000), // free=500 >= threshold 150
	}
	res, err := Run(cfg, opts)
	if err != nil {
		t.Fatal(err)
	}
	if res.FreedBytes != 0 {
		t.Errorf("FreedBytes = %d, want 0 (above threshold)", res.FreedBytes)
	}
	if len(res.Candidates) != 0 {
		t.Errorf("actioned %d candidates, want 0", len(res.Candidates))
	}
	if _, err := os.Stat(p); err != nil {
		t.Errorf("file should not be touched when above threshold: %v", err)
	}
}

func TestRunBelowThresholdApplies(t *testing.T) {
	home := t.TempDir()
	dir := filepath.Join(home, "App")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	p := writeFileIn(t, dir, "f", 100, time.Now())

	cfg := &config.Config{
		Storage:   config.Storage{Volume: "/", ThresholdType: "percent", ThresholdValue: 15, SafetyMarginPercent: 5},
		Retention: config.Retention{MinAgeDays: 0},
		EvictionTargets: config.EvictionTargets{
			SafeCaches: []config.Target{{Path: "~/App", Type: "cache"}},
		},
	}
	opts := EngineOptions{
		Apply:     true,
		HomeDir:   home,
		DiskUsage: fakeDiskUsage(90, 1000), // free=90 < threshold 150 -> evict
	}
	res, err := Run(cfg, opts)
	if err != nil {
		t.Fatal(err)
	}
	if res.FreedBytes != 100 {
		t.Errorf("FreedBytes = %d, want 100", res.FreedBytes)
	}
	if len(res.Candidates) != 1 {
		t.Errorf("actioned %d candidates, want 1", len(res.Candidates))
	}
	if _, err := os.Stat(p); err == nil {
		t.Error("file should be deleted when below threshold and apply")
	}
}

func TestRunBelowThresholdDryRunDoesNotDelete(t *testing.T) {
	home := t.TempDir()
	dir := filepath.Join(home, "App")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	p := writeFileIn(t, dir, "f", 100, time.Now())

	cfg := &config.Config{
		Storage:   config.Storage{Volume: "/", ThresholdType: "percent", ThresholdValue: 15, SafetyMarginPercent: 5},
		Retention: config.Retention{MinAgeDays: 0},
		EvictionTargets: config.EvictionTargets{
			SafeCaches: []config.Target{{Path: "~/App", Type: "cache"}},
		},
	}
	opts := EngineOptions{
		Apply:     false, // dry-run
		HomeDir:   home,
		DiskUsage: fakeDiskUsage(90, 1000),
	}
	res, err := Run(cfg, opts)
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Candidates) != 1 {
		t.Errorf("actioned %d candidates, want 1 (previewed)", len(res.Candidates))
	}
	if _, err := os.Stat(p); err != nil {
		t.Errorf("dry-run must not delete: %v", err)
	}
}

func TestRunSkipsRunningApp(t *testing.T) {
	home := t.TempDir()
	dir := filepath.Join(home, "App")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	p := writeFileIn(t, dir, "f", 100, time.Now())

	cfg := &config.Config{
		Storage:   config.Storage{Volume: "/", ThresholdType: "percent", ThresholdValue: 15, SafetyMarginPercent: 5},
		Retention: config.Retention{MinAgeDays: 0},
		EvictionTargets: config.EvictionTargets{
			VolatileApps: []config.Target{{Path: "~/App", Type: "cache", BundleID: "com.test.App"}},
		},
	}
	// Simulate that the app is running.
	opts := EngineOptions{
		Apply:            true,
		HomeDir:          home,
		DiskUsage:        fakeDiskUsage(90, 1000),
		IsProcessRunning: func(bundleID string, processName string) bool { return true },
	}
	res, err := Run(cfg, opts)
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Candidates) != 0 {
		t.Errorf("got %d candidates, want 0 (app running)", len(res.Candidates))
	}
	if len(res.AppSkipped) != 1 {
		t.Errorf("AppSkipped len = %d, want 1", len(res.AppSkipped))
	}
	if _, err := os.Stat(p); err != nil {
		t.Errorf("file should remain when app is running: %v", err)
	}
}

func TestRunForceOverridesRunningApp(t *testing.T) {
	home := t.TempDir()
	dir := filepath.Join(home, "App")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	p := writeFileIn(t, dir, "f", 100, time.Now())

	cfg := &config.Config{
		Storage:   config.Storage{Volume: "/", ThresholdType: "percent", ThresholdValue: 15, SafetyMarginPercent: 5},
		Retention: config.Retention{MinAgeDays: 0},
		EvictionTargets: config.EvictionTargets{
			VolatileApps: []config.Target{{Path: "~/App", Type: "cache", BundleID: "com.test.App"}},
		},
	}
	opts := EngineOptions{
		Apply:            true,
		Force:            true,
		HomeDir:          home,
		DiskUsage:        fakeDiskUsage(90, 1000),
		IsProcessRunning: func(bundleID string, processName string) bool { return true },
	}
	res, err := Run(cfg, opts)
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Candidates) != 1 {
		t.Errorf("got %d candidates, want 1 (Force overrides running check)", len(res.Candidates))
	}
	if _, err := os.Stat(p); err == nil {
		t.Error("file should be deleted when Force is true")
	}
}

func TestRunSizeThreshold(t *testing.T) {
	home := t.TempDir()
	dir := filepath.Join(home, "App")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	p := writeFileIn(t, dir, "f", 100, time.Now())

	cfg := &config.Config{
		Storage:   config.Storage{Volume: "/", ThresholdType: "size", ThresholdValue: 1_000, SafetyMarginPercent: 0},
		Retention: config.Retention{MinAgeDays: 0},
		EvictionTargets: config.EvictionTargets{
			SafeCaches: []config.Target{{Path: "~/App", Type: "cache"}},
		},
	}
	// free=900 < threshold 1000 -> evict
	opts := EngineOptions{Apply: true, HomeDir: home, DiskUsage: fakeDiskUsage(900, 100_000)}
	res, err := Run(cfg, opts)
	if err != nil {
		t.Fatal(err)
	}
	if res.FreedBytes != 100 {
		t.Errorf("FreedBytes = %d, want 100", res.FreedBytes)
	}
	if _, err := os.Stat(p); err == nil {
		t.Error("file should be deleted (below size threshold)")
	}
}

func TestRunDoesNotDeleteOwnLog(t *testing.T) {
	home := t.TempDir()
	dir := filepath.Join(home, "Logs")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	logPath := filepath.Join(dir, "thepurge.log")
	if err := os.WriteFile(logPath, []byte("line1\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	cache := filepath.Join(home, "App")
	if err := os.MkdirAll(cache, 0o755); err != nil {
		t.Fatal(err)
	}
	cacheFile := writeFileIn(t, cache, "f", 100, time.Now())

	cfg := &config.Config{
		Storage:   config.Storage{Volume: "/", ThresholdType: "percent", ThresholdValue: 15, SafetyMarginPercent: 5},
		Retention: config.Retention{MinAgeDays: 0},
		Logging:   config.Logging{LogPath: logPath},
		EvictionTargets: config.EvictionTargets{
			SafeCaches: []config.Target{{Path: "~/App", Type: "cache"}},
			Logs:       []config.Target{{Path: "~/Logs", Type: "log"}},
		},
	}
	opts := EngineOptions{Apply: true, HomeDir: home, DiskUsage: fakeDiskUsage(90, 1000)}
	res, err := Run(cfg, opts)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(logPath); err != nil {
		t.Errorf("thepurge own log must NOT be deleted: %v", err)
	}
	if _, err := os.Stat(cacheFile); err == nil {
		t.Error("cache file should be deleted")
	}
	if res.FreedBytes != 100 {
		t.Errorf("FreedBytes = %d, want 100", res.FreedBytes)
	}
}
