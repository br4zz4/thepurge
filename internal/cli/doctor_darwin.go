//go:build darwin

package cli

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
)

// checkHostAccess probes Full Disk Access, the macOS gate that blocks eviction
// for most targets. It returns false when the probe fails in a way the user
// must fix.
func checkHostAccess(out io.Writer) bool {
	home := cfgHome()
	if home == "" {
		return true
	}
	probe := filepath.Join(home, "Library", "Application Support")
	if _, err := os.ReadDir(probe); err != nil {
		switch {
		case os.IsPermission(err):
			fmt.Fprintf(out, "  fda: cannot read %s: %v\n", probe, err)
			fmt.Fprintln(out, "  fda: grant Full Disk Access to thepurge (System Settings > Privacy & Security > Full Disk Access)")
			return false
		case os.IsNotExist(err):
			fmt.Fprintf(out, "  fda: %s does not exist (not a TCC failure)\n", probe)
		default:
			fmt.Fprintf(out, "  fda: cannot read %s: %v\n", probe, err)
		}
		return true
	}
	fmt.Fprintln(out, "  fda: OK (can read Library/Application Support)")
	return true
}
