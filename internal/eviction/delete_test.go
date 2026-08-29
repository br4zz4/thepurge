package eviction

import (
	"os"
	"os/exec"
	"testing"
	"time"

	"br4zz4/thepurge/internal/fsutil"
)

func TestDeleteItemsDryRunDoesNotDelete(t *testing.T) {
	dir := t.TempDir()
	p := writeFileIn(t, dir, "f", 100, time.Now())

	cands := []Candidate{{Path: p, Size: 100, ModTime: time.Now(), Type: "cache"}}
	res, err := DeleteItems(cands, DeleteOptions{DryRun: true})
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Deleted) != 1 {
		t.Fatalf("dry-run Deleted len = %d, want 1", len(res.Deleted))
	}
	if _, statErr := os.Stat(p); statErr != nil {
		t.Errorf("dry-run should not delete file, got error: %v", statErr)
	}
}

func TestDeleteItemsApplies(t *testing.T) {
	dir := t.TempDir()
	p := writeFileIn(t, dir, "f", 100, time.Now())

	cands := []Candidate{{Path: p, Size: 100, ModTime: time.Now(), Type: "cache"}}
	res, err := DeleteItems(cands, DeleteOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Deleted) != 1 {
		t.Fatalf("Deleted len = %d, want 1", len(res.Deleted))
	}
	if res.FreedBytes != 100 {
		t.Errorf("FreedBytes = %d, want 100", res.FreedBytes)
	}
	if _, statErr := os.Stat(p); statErr == nil {
		t.Error("file should be gone after apply")
	}
}

func TestDeleteItemsRespectsMaxBytes(t *testing.T) {
	dir := t.TempDir()
	a := writeFileIn(t, dir, "a", 100, time.Now())
	b := writeFileIn(t, dir, "b", 100, time.Now())

	cands := []Candidate{
		{Path: a, Size: 100, ModTime: time.Now(), Type: "cache"},
		{Path: b, Size: 100, ModTime: time.Now(), Type: "cache"},
	}
	res, err := DeleteItems(cands, DeleteOptions{MaxBytes: 100})
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Deleted) != 1 {
		t.Fatalf("Deleted len = %d, want 1 (stop at MaxBytes=100)", len(res.Deleted))
	}
	if res.FreedBytes != 100 {
		t.Errorf("FreedBytes = %d, want 100", res.FreedBytes)
	}
	if _, err := os.Stat(b); err != nil {
		t.Error("second file should remain (MaxBytes reached); got:", err)
	}
}

func TestDeleteItemsSkipsImmutable(t *testing.T) {
	dir := t.TempDir()
	p := writeFileIn(t, dir, "locked", 100, time.Now())
	if err := exec.Command("chflags", "uchg", p).Run(); err != nil {
		t.Fatalf("chflags uchg: %v", err)
	}
	defer exec.Command("chflags", "nouchg", p).Run()

	if !fsutil.IsImmutable(p) {
		t.Fatal("test precondition: file should be immutable")
	}

	cands := []Candidate{{Path: p, Size: 100, ModTime: time.Now(), Type: "cache"}}
	res, err := DeleteItems(cands, DeleteOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Deleted) != 0 {
		t.Errorf("Deleted len = %d, want 0 (immutable skipped)", len(res.Deleted))
	}
	if len(res.Skipped) != 1 {
		t.Errorf("Skipped len = %d, want 1", len(res.Skipped))
	}
	if _, statErr := os.Stat(p); statErr != nil {
		t.Errorf("immutable file should remain, got error: %v", statErr)
	}
}
