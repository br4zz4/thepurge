//go:build linux

package eviction

import (
	"os/exec"
	"testing"
)

func lockImmutable(t *testing.T, path string) {
	t.Helper()
	if _, err := exec.LookPath("chattr"); err != nil {
		t.Skip("chattr not available (install e2fsprogs)")
	}
	if err := exec.Command("chattr", "+i", path).Run(); err != nil {
		t.Skipf("chattr +i unsupported on this filesystem: %v", err)
	}
	t.Cleanup(func() { exec.Command("chattr", "-i", path).Run() })
}
