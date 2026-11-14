//go:build darwin

package eviction

import (
	"os"
	"path/filepath"
	"strings"
)

// checkApplications verifies whether an app is installed in any standard macOS
// location. It returns true if found, along with the app name.
func checkApplications(itemPath string) (bool, string) {
	base := filepath.Base(itemPath)
	appName := strings.TrimSuffix(base, ".app")

	locations := []string{
		filepath.Join("/Applications", appName+".app"),
		filepath.Join("/Applications/Utilities", appName+".app"),
		filepath.Join(os.Getenv("HOME"), "Applications", appName+".app"),
	}
	for _, loc := range locations {
		if _, err := os.Stat(loc); err == nil {
			return true, appName
		}
	}

	for _, loc := range []string{"/Applications", filepath.Join(os.Getenv("HOME"), "Applications")} {
		entries, err := os.ReadDir(loc)
		if err != nil {
			continue
		}
		for _, e := range entries {
			if strings.HasPrefix(strings.ToLower(e.Name()), strings.ToLower(appName)) {
				return true, e.Name()
			}
		}
	}

	return false, appName
}
