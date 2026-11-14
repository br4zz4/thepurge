//go:build linux

package eviction

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// checkApplications verifies whether an app is installed on a Linux host, where
// there is no single app bundle. It matches a binary on PATH or a .desktop entry
// in the XDG application dirs. It returns true if found, along with the name.
func checkApplications(itemPath string) (bool, string) {
	base := filepath.Base(itemPath)
	appName := strings.TrimSuffix(base, ".app")
	lower := strings.ToLower(appName)

	if _, err := exec.LookPath(lower); err == nil {
		return true, appName
	}

	dirs := []string{"/usr/share/applications", "/usr/local/share/applications"}
	if home := os.Getenv("HOME"); home != "" {
		dirs = append(dirs, filepath.Join(home, ".local", "share", "applications"))
	}
	for _, dir := range dirs {
		entries, err := os.ReadDir(dir)
		if err != nil {
			continue
		}
		for _, e := range entries {
			if !strings.HasSuffix(e.Name(), ".desktop") {
				continue
			}
			stem := strings.TrimSuffix(e.Name(), ".desktop")
			if strings.EqualFold(stem, appName) || strings.HasPrefix(strings.ToLower(stem), lower) {
				return true, stem
			}
		}
	}

	if _, err := os.Stat(filepath.Join("/opt", appName)); err == nil {
		return true, appName
	}

	return false, appName
}
