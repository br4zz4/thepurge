//go:build linux

package fsutil

import (
	"path/filepath"
	"testing"
)

func TestDiskUsageLinuxRoot(t *testing.T) {
	// act
	free, total, err := DiskUsage("/")

	// assert
	if err != nil {
		t.Fatalf("DiskUsage(/) error = %v", err)
	}
	if total <= 0 {
		t.Errorf("total = %d, want > 0", total)
	}
	if free <= 0 {
		t.Errorf("free = %d, want > 0", free)
	}
	if free > total {
		t.Errorf("free = %d > total = %d", free, total)
	}
}

func TestDiskUsageLinuxTempDir(t *testing.T) {
	// arrange
	dir := t.TempDir()

	// act
	free, total, err := DiskUsage(dir)

	// assert
	if err != nil {
		t.Fatalf("DiskUsage(%q) error = %v", dir, err)
	}
	if total <= 0 || free <= 0 {
		t.Errorf("free = %d, total = %d, want both > 0", free, total)
	}
	if free > total {
		t.Errorf("free = %d > total = %d", free, total)
	}
}

func TestDiskUsageLinuxMissingPath(t *testing.T) {
	// arrange
	p := filepath.Join(t.TempDir(), "gone", "deeper")

	// act
	_, _, err := DiskUsage(p)

	// assert
	if err == nil {
		t.Fatalf("DiskUsage(%q) error = nil, want error", p)
	}
}

func TestFreeSpaceLinux(t *testing.T) {
	// arrange
	dir := t.TempDir()
	wantFree, _, err := DiskUsage(dir)
	if err != nil {
		t.Fatal(err)
	}

	// act
	free, err := FreeSpace(dir)

	// assert
	if err != nil {
		t.Fatalf("FreeSpace(%q) error = %v", dir, err)
	}
	if free != wantFree {
		t.Errorf("FreeSpace() = %d, want %d", free, wantFree)
	}
	if free <= 0 {
		t.Errorf("FreeSpace() = %d, want > 0", free)
	}
}

func TestFreeSpaceLinuxMissingPath(t *testing.T) {
	// arrange
	p := filepath.Join(t.TempDir(), "gone")

	// act
	_, err := FreeSpace(p)

	// assert
	if err == nil {
		t.Fatalf("FreeSpace(%q) error = nil, want error", p)
	}
}
