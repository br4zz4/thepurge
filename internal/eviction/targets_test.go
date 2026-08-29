package eviction

import (
	"os"
	"path/filepath"
	"testing"

	"br4zz4/thepurge/config"
)

// testTarget builds a Target with the given path and subfolders to keep tests terse.
func mkTarget(path string, subfolders ...string) config.Target {
	return config.Target{Path: path, Type: "cache", Subfolders: subfolders}
}

func TestResolveTargetsExpandsHome(t *testing.T) {
	home := t.TempDir()
	// Simulate ~/Library/Application Support/App/Cache via HOME override.
	if err := os.MkdirAll(filepath.Join(home, "Library/Application Support/App/Cache"), 0o755); err != nil {
		t.Fatal(err)
	}

	cfg := &config.Config{
		EvictionTargets: config.EvictionTargets{
			SafeCaches: []config.Target{mkTarget("~/Library/Application Support/App")},
		},
	}
	items, err := ResolveTargets(cfg, home)
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 1 {
		t.Fatalf("got %d items, want 1", len(items))
	}
	want := filepath.Join(home, "Library/Application Support/App")
	if items[0].Path != want {
		t.Errorf("resolved path = %q, want %q", items[0].Path, want)
	}
}

func TestResolveTargetsSubfolders(t *testing.T) {
	home := t.TempDir()
	base := filepath.Join(home, "App")
	if err := os.MkdirAll(filepath.Join(base, "Cache"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(base, "Other"), 0o755); err != nil {
		t.Fatal(err)
	}

	cfg := &config.Config{
		EvictionTargets: config.EvictionTargets{
			SafeCaches: []config.Target{mkTarget("~/App", "Cache")},
		},
	}
	items, err := ResolveTargets(cfg, home)
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 1 {
		t.Fatalf("got %d items, want 1", len(items))
	}
	want := filepath.Join(home, "App", "Cache")
	if items[0].Path != want {
		t.Errorf("item path = %q, want %q", items[0].Path, want)
	}
}

func TestResolveTargetsSubfolderMissing(t *testing.T) {
	home := t.TempDir()
	// App exists, but the "Cache" subfolder does not.
	if err := os.MkdirAll(filepath.Join(home, "App"), 0o755); err != nil {
		t.Fatal(err)
	}
	cfg := &config.Config{
		EvictionTargets: config.EvictionTargets{
			SafeCaches: []config.Target{mkTarget("~/App", "Cache")},
		},
	}
	items, err := ResolveTargets(cfg, home)
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 0 {
		t.Errorf("got %d items, want 0 (subfolder missing)", len(items))
	}
}

func TestResolveTargetsEnabledOrphans(t *testing.T) {
	home := t.TempDir()
	orphan := filepath.Join(home, "Application Support/Cursor")
	if err := os.MkdirAll(orphan, 0o755); err != nil {
		t.Fatal(err)
	}
	cfg := &config.Config{
		EvictionTargets: config.EvictionTargets{
			EnabledOrphanApps: []string{"~/Application Support/Cursor"},
		},
	}
	items, err := ResolveTargets(cfg, home)
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 1 {
		t.Fatalf("got %d items, want 1 orphan enabled", len(items))
	}
	if items[0].Orphan != true {
		t.Errorf("item Orphan = %v, want true; got %+v", items[0].Orphan, items[0])
	}
	// Whole-folder orphan is a single delete root; subfolders stay empty.
	if items[0].Subfolders != nil && len(items[0].Subfolders) != 0 {
		t.Errorf("orphan item should have empty subfolders, got %v", items[0].Subfolders)
	}
}

func TestResolveTargetsNone(t *testing.T) {
	cfg := &config.Config{EvictionTargets: config.EvictionTargets{}}
	items, err := ResolveTargets(cfg, t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 0 {
		t.Errorf("got %d items, want 0", len(items))
	}
}

func TestResolveTargetsVolatileApps(t *testing.T) {
	home := t.TempDir()
	base := filepath.Join(home, "Library/Application Support/TestApp")
	if err := os.MkdirAll(base, 0o755); err != nil {
		t.Fatal(err)
	}

	cfg := &config.Config{
		EvictionTargets: config.EvictionTargets{
			VolatileApps: []config.Target{
				{Path: "~/Library/Application Support/TestApp", Type: "cache", BundleID: "com.test.TestApp"},
			},
		},
	}
	items, err := ResolveTargets(cfg, home)
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 1 {
		t.Fatalf("got %d items, want 1", len(items))
	}
	if items[0].BundleID != "com.test.TestApp" {
		t.Errorf("BundleID = %q, want com.test.TestApp", items[0].BundleID)
	}
	if !items[0].RespectRunning {
		t.Error("RespectRunning should be true for volatile apps")
	}
}

func TestResolveTargetsVolatileAppsNoBundle(t *testing.T) {
	home := t.TempDir()
	base := filepath.Join(home, "Library/Application Support/Arc")
	if err := os.MkdirAll(base, 0o755); err != nil {
		t.Fatal(err)
	}

	cfg := &config.Config{
		EvictionTargets: config.EvictionTargets{
			VolatileApps: []config.Target{
				{Path: "~/Library/Application Support/Arc", Type: "cache"},
			},
		},
	}
	items, err := ResolveTargets(cfg, home)
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 1 {
		t.Fatalf("got %d items, want 1", len(items))
	}
	// Should derive process names from path basename.
	if len(items[0].ProcessNames) == 0 || items[0].ProcessNames[0] != "Arc" {
		t.Errorf("ProcessNames = %v, want [Arc]", items[0].ProcessNames)
	}
}

// TestItemFieldsAreExported guards the fields eviction logic reads, so a rename
// or unexport breaks the build instead of silently degrading.
func TestItemFieldsAreExported(t *testing.T) {
	_ = Item{
		Path: "p", Type: "cache", Subfolders: []string{"a"}, Orphan: false,
		BundleID: "b", ProcessNames: []string{"p"}, RespectRunning: true,
	}
}
