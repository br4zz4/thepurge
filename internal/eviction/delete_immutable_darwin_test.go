//go:build darwin

package eviction

import (
	"os/exec"
	"testing"
)

func lockImmutable(t *testing.T, path string) {
	t.Helper()
	if _, err := exec.LookPath("chflags"); err != nil {
		t.Skip("chflags not available")
	}
	if err := exec.Command("chflags", "uchg", path).Run(); err != nil {
		t.Skipf("chflags uchg unsupported here: %v", err)
	}
	t.Cleanup(func() { exec.Command("chflags", "nouchg", path).Run() })
}
