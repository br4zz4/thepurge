//go:build darwin

package fsutil

import (
	"testing"
)

func TestDiskUsage(t *testing.T) {
	dir := t.TempDir()
	free, total, err := DiskUsage(dir)
	if err != nil {
		t.Fatalf("DiskUsage(%q) error: %v", dir, err)
	}
	if total <= 0 {
		t.Errorf("DiskUsage total = %d, want > 0", total)
	}
	if free < 0 || free > total {
		t.Errorf("DiskUsage free = %d, want in [0,total=%d]", free, total)
	}
}
