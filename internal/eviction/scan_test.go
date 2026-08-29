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

func TestScanOrdersByMtimeDescThenSizeDesc(t *testing.T) {
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

	// Expect order: recent (mtime newest) -> mid -> old.
	wantOrder := []string{recent, mid, old}
	if len(cands) != len(wantOrder) {
		t.Fatalf("got %d candidates, want %d", len(cands), len(wantOrder))
	}
	for i, w := range wantOrder {
		if cands[i].Path != w {
			t.Errorf("cands[%d].Path = %q, want %q", i, cands[i].Path, w)
		}
	}
}

func TestScanFiltersByMinAge(t *testing.T) {
	dir := t.TempDir()
	now := time.Now()
	old := writeFileIn(t, dir, "old", 10, now.Add(-10*24*time.Hour)) // 10 days
	writeFileIn(t, dir, "fresh", 100, now.Add(-1*time.Hour))         // 1 hour

	items := []Item{{Path: dir, Type: "cache", Subfolders: []string{}}}
	cands, err := ScanTargets(items, 7) // min_age_days = 7
	if err != nil {
		t.Fatal(err)
	}
	if len(cands) != 1 {
		t.Fatalf("got %d candidates, want 1 (only the 10-day-old file)", len(cands))
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

func TestCandidatesSorted(t *testing.T) {
	// Assert the slice is actually sorted (mtime desc, then size desc) as a
	// defensive invariant, not just first/last.
	dir := t.TempDir()
	now := time.Now()
	writeFileIn(t, dir, "a", 10, now.Add(-3*time.Hour))
	writeFileIn(t, dir, "b", 20, now.Add(-3*time.Hour)) // newer? same mtime, bigger size
	writeFileIn(t, dir, "c", 5, now.Add(-1*time.Hour))
	items := []Item{{Path: dir, Type: "cache"}}
	cands, err := ScanTargets(items, 0)
	if err != nil {
		t.Fatal(err)
	}
	if !sort.SliceIsSorted(cands, func(i, j int) bool {
		if cands[i].ModTime != cands[j].ModTime {
			return cands[i].ModTime.After(cands[j].ModTime)
		}
		return cands[i].Size > cands[j].Size
	}) {
		t.Errorf("candidates not sorted by mtime desc then size desc: %+v", cands)
	}
}
