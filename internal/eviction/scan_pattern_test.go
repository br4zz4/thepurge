package eviction

import (
	"os"
	"path/filepath"
	"testing"

	"br4zz4/thepurge/config"
)

func TestMatchesPattern(t *testing.T) {
	root := filepath.Join("/", "home", "test", "App")

	tests := []struct {
		name    string
		pattern string
		path    string
		want    bool
	}{
		{
			name: "empty pattern accepts deep file",
			path: filepath.Join(root, "Cookies", "x.db"),
			want: true,
		},
		{
			name: "empty pattern accepts root itself",
			path: root,
			want: true,
		},
		{
			name:    "star accepts direct child",
			pattern: "*",
			path:    filepath.Join(root, "a.bin"),
			want:    true,
		},
		{
			name:    "star accepts deep child",
			pattern: "*",
			path:    filepath.Join(root, "a", "b.bin"),
			want:    true,
		},
		{
			name:    "cache dir component matches",
			pattern: "*Cache*",
			path:    filepath.Join(root, "Cache", "x.bin"),
			want:    true,
		},
		{
			name:    "cookies component does not match",
			pattern: "*Cache*",
			path:    filepath.Join(root, "Cookies", "x.db"),
			want:    false,
		},
		{
			name:    "matching is case-sensitive",
			pattern: "*cache*",
			path:    filepath.Join(root, "Cache", "x.bin"),
			want:    false,
		},
		{
			name:    "case-sensitive glob for file extension",
			pattern: "*.LOG",
			path:    filepath.Join(root, "Logs", "a.log"),
			want:    false,
		},
		{
			name:    "nested cache component matches",
			pattern: "*Cache*",
			path:    filepath.Join(root, "Default", "GPUCache", "a"),
			want:    true,
		},
		{
			name:    "root itself never matches",
			pattern: "*Cache*",
			path:    root,
			want:    false,
		},
		{
			name:    "log file matches",
			pattern: "*.log",
			path:    filepath.Join(root, "Logs", "a.log"),
			want:    true,
		},
		{
			name:    "non-log file does not match",
			pattern: "*.log",
			path:    filepath.Join(root, "Logs", "a.txt"),
			want:    false,
		},
		{
			name:    "malformed pattern matches nothing",
			pattern: "[",
			path:    filepath.Join(root, "x"),
			want:    false,
		},
		{
			name:    "path outside root does not match a file glob",
			pattern: "*.log",
			path:    filepath.Join(root, "..", "x.txt"),
			want:    false,
		},
		{
			name:    "star does not match outside root",
			pattern: "*",
			path:    filepath.Join(root, "..", "elsewhere", "x.bin"),
			want:    false,
		},
		{
			name:    "star does not match the parent dir",
			pattern: "*",
			path:    filepath.Join(root, ".."),
			want:    false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// act
			got := matchesPattern(tt.pattern, root, tt.path)

			// assert
			if got != tt.want {
				t.Errorf("matchesPattern(%q, %q, %q) = %v, want %v", tt.pattern, root, tt.path, got, tt.want)
			}
		})
	}
}

func TestMatchesPatternOutsideRootIsRejected(t *testing.T) {
	root := filepath.Join("/", "home", "test", "App")

	tests := []struct {
		name    string
		pattern string
		path    string
	}{
		{name: "star", pattern: "*", path: filepath.Join(root, "..", "elsewhere", "x.bin")},
		{name: "parent dir", pattern: "*", path: filepath.Join(root, "..")},
		{name: "file glob", pattern: "*.log", path: filepath.Join(root, "..", "x.txt")},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// act
			got := matchesPattern(tt.pattern, root, tt.path)

			// assert
			if got {
				t.Errorf("matchesPattern(%q, %q) = true, want false for a path outside root", tt.pattern, tt.path)
			}
		})
	}
}

func TestSplitPath(t *testing.T) {
	tests := []struct {
		name string
		rel  string
		want []string
	}{
		{name: "empty", rel: "", want: nil},
		{name: "dot", rel: ".", want: nil},
		{name: "single", rel: "a", want: []string{"a"}},
		{name: "two levels", rel: filepath.Join("a", "b"), want: []string{"b", "a"}},
		{name: "three levels", rel: filepath.Join("a", "b", "c"), want: []string{"c", "b", "a"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// act
			got := splitPath(tt.rel)

			// assert
			if len(got) != len(tt.want) {
				t.Fatalf("splitPath(%q) = %v, want %v", tt.rel, got, tt.want)
			}
			for i := range tt.want {
				if got[i] != tt.want[i] {
					t.Fatalf("splitPath(%q) = %v, want %v", tt.rel, got, tt.want)
				}
			}
		})
	}
}

func patternTree(t *testing.T) string {
	t.Helper()
	app := t.TempDir()
	for _, dir := range []string{"Cache", "Cookies", filepath.Join("Default", "GPUCache")} {
		if err := os.MkdirAll(filepath.Join(app, dir), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	for _, f := range []string{
		filepath.Join("Cache", "x.bin"),
		filepath.Join("Cookies", "x.db"),
		filepath.Join("Default", "GPUCache", "a"),
		"top.txt",
	} {
		if err := os.WriteFile(filepath.Join(app, f), []byte("data"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return app
}

func scanPaths(t *testing.T, items []Item) map[string]bool {
	t.Helper()
	cands, err := ScanTargets(items, 0)
	if err != nil {
		t.Fatalf("ScanTargets() error = %v", err)
	}
	got := make(map[string]bool, len(cands))
	for _, c := range cands {
		got[c.Path] = true
	}
	return got
}

func TestScanPatternFiltersByComponent(t *testing.T) {
	// arrange
	app := patternTree(t)
	cfg := &config.Config{EvictionTargets: config.EvictionTargets{
		SafeCaches: []config.Target{{Path: app, Type: "cache", Pattern: "*Cache*"}},
	}}
	items, err := ResolveTargets(cfg, t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 1 {
		t.Fatalf("ResolveTargets() returned %d items, want 1", len(items))
	}
	if items[0].Pattern != "*Cache*" {
		t.Fatalf("item.Pattern = %q, want %q", items[0].Pattern, "*Cache*")
	}

	// act
	got := scanPaths(t, items)

	// assert
	if len(got) != 2 {
		t.Fatalf("candidates = %v, want exactly the two cache files", got)
	}
	for _, want := range []string{
		filepath.Join(app, "Cache", "x.bin"),
		filepath.Join(app, "Default", "GPUCache", "a"),
	} {
		if !got[want] {
			t.Errorf("missing candidate %q; got %v", want, got)
		}
	}
}

func TestScanWithoutPatternCollectsAllFiles(t *testing.T) {
	// arrange
	app := patternTree(t)

	// act
	got := scanPaths(t, []Item{{Path: app, Type: "cache"}})

	// assert
	if len(got) != 4 {
		t.Fatalf("candidates = %v, want all 4 files", got)
	}
	for _, want := range []string{
		filepath.Join(app, "Cache", "x.bin"),
		filepath.Join(app, "Cookies", "x.db"),
		filepath.Join(app, "Default", "GPUCache", "a"),
		filepath.Join(app, "top.txt"),
	} {
		if !got[want] {
			t.Errorf("missing candidate %q; got %v", want, got)
		}
	}
}

func TestResolveTargetsPatternWithoutSubfolders(t *testing.T) {
	// arrange
	app := t.TempDir()
	cfg := &config.Config{EvictionTargets: config.EvictionTargets{
		SafeCaches: []config.Target{{
			Path:       app,
			Type:       "cache",
			Pattern:    "*.log",
			Subfolders: []string{"ignored"},
		}},
	}}

	// act
	items, err := ResolveTargets(cfg, t.TempDir())

	// assert
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 1 {
		t.Fatalf("items = %d, want 1", len(items))
	}
	if items[0].Pattern != "*.log" {
		t.Errorf("Pattern = %q, want %q", items[0].Pattern, "*.log")
	}
	if len(items[0].Subfolders) != 0 {
		t.Errorf("Subfolders = %v, want none when a pattern is set", items[0].Subfolders)
	}
	if items[0].Path != app {
		t.Errorf("Path = %q, want %q", items[0].Path, app)
	}
}

func TestResolveTargetsPatternMissingBase(t *testing.T) {
	// arrange
	cfg := &config.Config{EvictionTargets: config.EvictionTargets{
		SafeCaches: []config.Target{{
			Path:    filepath.Join(t.TempDir(), "gone"),
			Type:    "cache",
			Pattern: "*Cache*",
		}},
	}}

	// act
	items, err := ResolveTargets(cfg, t.TempDir())

	// assert
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 0 {
		t.Errorf("items = %v, want none for a missing base dir", items)
	}
}
