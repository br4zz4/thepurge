package eviction

import (
	"os"
	"path/filepath"
	"sort"
	"testing"
	"time"
)

// writeFileIn creates a file at dir/name with size bytes and sets its mtime.
func writeFileIn(t *testing.T, dir, name string, size int64, mtime time.Time) string {
	t.Helper()
	p := filepath.Join(dir, name)
	data := make([]byte, size)
	if err := os.WriteFile(p, data, 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Chtimes(p, mtime, mtime); err != nil {
		t.Fatal(err)
	}
	return p
}

func TestScanOrdersByMtimeAscThenSizeDesc(t *testing.T) {
	dir := t.TempDir()
	now := time.Now()
	old := writeFileIn(t, dir, "old_small", 10, now.Add(-48*time.Hour))
	recent := writeFileIn(t, dir, "recent_big", 100, now.Add(-1*time.Hour))
	mid := writeFileIn(t, dir, "mid", 50, now.Add(-24*time.Hour))

	items := []Item{{Path: dir, Type: "cache", Subfolders: []string{}}}
	cands, err := ScanTargets(items, 0)
	if err != nil {
		t.Fatal(err)
	}

	// Expect order: old (oldest) -> mid -> recent (newest) — oldest first per regra 3.
	wantOrder := []string{old, mid, recent}
	if len(cands) != len(wantOrder) {
		t.Fatalf("got %d candidates, want %d", len(cands), len(wantOrder))
	}
	for i, w := range wantOrder {
		if cands[i].Path != w {
			t.Errorf("cands[%d].Path = %q, want %q", i, cands[i].Path, w)
		}
	}
}

// TestScanOrdersLogsBeforeCaches verifies the priority ordering: logs (type="log")
// must appear before caches (type="cache"), regardless of mtime.
func TestScanOrdersLogsBeforeCaches(t *testing.T) {
	logDir := t.TempDir()
	cacheDir := t.TempDir()
	now := time.Now()
	// Recent log file in log dir (should come first).
	recentLog := writeFileIn(t, logDir, "recent_log", 100, now.Add(-1*time.Hour))
	// Old cache file in cache dir (should come second).
	oldCache := writeFileIn(t, cacheDir, "old_cache", 10, now.Add(-72*time.Hour))

	items := []Item{
		{Path: logDir, Type: "log", Subfolders: []string{}},
		{Path: cacheDir, Type: "cache", Subfolders: []string{}},
	}
	cands, err := ScanTargets(items, 0)
	if err != nil {
		t.Fatal(err)
	}
	if len(cands) != 2 {
		t.Fatalf("got %d candidates, want 2", len(cands))
	}
	// Log first (even though recent_log is newer, logs have priority).
	if cands[0].Path != recentLog {
		t.Errorf("first candidate = %q, want %q (log first)", cands[0].Path, recentLog)
	}
	// Cache second.
	if cands[1].Path != oldCache {
		t.Errorf("second candidate = %q, want %q (cache second)", cands[1].Path, oldCache)
	}
}

func TestScanSortingInvariant(t *testing.T) {
	dir := t.TempDir()
	now := time.Now()
	writeFileIn(t, dir, "a", 10, now.Add(-3*time.Hour))
	writeFileIn(t, dir, "b", 20, now.Add(-3*time.Hour)) // same mtime, bigger size
	writeFileIn(t, dir, "c", 5, now.Add(-1*time.Hour))
	items := []Item{{Path: dir, Type: "cache"}}
	cands, err := ScanTargets(items, 0)
	if err != nil {
		t.Fatal(err)
	}
	if !sort.SliceIsSorted(cands, func(i, j int) bool {
		// Logs before caches.
		if cands[i].Type != cands[j].Type {
			return priorityOrder[cands[i].Type] < priorityOrder[cands[j].Type]
		}
		// Oldest first.
		if !cands[i].ModTime.Equal(cands[j].ModTime) {
			return cands[i].ModTime.Before(cands[j].ModTime)
		}
		// Heaviest first.
		return cands[i].Size > cands[j].Size
	}) {
		t.Errorf("candidates not sorted by priority, mtime ASC, then size DESC: %+v", cands)
	}
}

func TestScanFiltersByMinAge(t *testing.T) {
	dir := t.TempDir()
	now := time.Now()
	old := writeFileIn(t, dir, "old", 10, now.Add(-10*24*time.Hour))
	writeFileIn(t, dir, "fresh", 100, now.Add(-1*time.Hour))

	items := []Item{{Path: dir, Type: "cache", Subfolders: []string{}}}
	cands, err := ScanTargets(items, 7)
	if err != nil {
		t.Fatal(err)
	}
	if len(cands) != 1 {
		t.Fatalf("got %d candidates, want 1", len(cands))
	}
	if cands[0].Path != old {
		t.Errorf("candidate = %q, want %q (the old file)", cands[0].Path, old)
	}
}

func TestScanSkipsEmptyDirs(t *testing.T) {
	dir := t.TempDir()
	items := []Item{{Path: dir, Type: "cache", Subfolders: []string{}}}
	cands, err := ScanTargets(items, 0)
	if err != nil {
		t.Fatal(err)
	}
	if len(cands) != 0 {
		t.Errorf("got %d candidates, want 0 for empty dir", len(cands))
	}
}
