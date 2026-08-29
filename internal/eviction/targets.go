// Package eviction resolves config targets into concrete on-disk items and
// runs the eviction cycle (scan, prioritise, delete).
package eviction

import (
	"os"
	"path/filepath"
	"strings"

	"br4zz4/thepurge/config"
)

// Item is a concrete, on-disk location that eviction may act on. It is resolved
// from a config.Target by expanding the home directory and (for caches/logs)
// narrowing to specific subfolders. Orphan items represent a whole folder that
// may be removed entirely once explicitly enabled.
type Item struct {
	Path            string
	Type            string   // "cache", "log" or "orphan"
	Subfolders      []string
	Orphan          bool
	BundleID        string   // bundle identifier for running-apps check
	ProcessNames    []string // alternative process names for running-apps check
	RespectRunning  bool     // if true, skip when the app is running (default: true for volatile)
}

// ResolveTargets converts the config’s eviction targets into concrete on-disk
// items. homeDir is injected so callers can point at a test sandbox; pass the
// real user home in production. Missing folders and subfolders are skipped
// silently (nothing to evict). Enabled orphan apps resolve to whole-folder items.
func ResolveTargets(cfg *config.Config, homeDir string) ([]Item, error) {
	var items []Item

	for _, t := range cfg.EvictionTargets.SafeCaches {
		items = append(items, resolveSubfolders(t, homeDir, t.Type, false)...)
	}
	for _, t := range cfg.EvictionTargets.Logs {
		items = append(items, resolveSubfolders(t, homeDir, t.Type, false)...)
	}
	for _, t := range cfg.EvictionTargets.VolatileApps {
		typ := t.Type
		if typ == "" {
			typ = "cache"
		}
		// Gather process names: derive from path basename and append any
		// explicit ProcessNames from the target (future config field).
		processNames := processNamesFromTarget(t)
		items = append(items, resolveSubfoldersWithProcess(t, homeDir, typ, t.BundleID, processNames)...)
	}
	for _, p := range cfg.EvictionTargets.EnabledOrphanApps {
		full := expandPath(p, homeDir)
		if _, err := os.Stat(full); err != nil {
			continue
		}
		items = append(items, Item{
			Path:           full,
			Type:           "orphan",
			Orphan:         true,
			RespectRunning: true,
		})
	}

	return items, nil
}

// processNamesFromTarget derives process names from a Target. It uses the path
// basename as fallback; in the future ProcessNames from config can be appended.
func processNamesFromTarget(t config.Target) []string {
	base := filepath.Base(t.Path)
	// Strip common suffixes and subdirs to guess the process name.
	name := strings.TrimSuffix(base, ".app")
	return []string{name}
}

// resolveSubfolders resolves a target into items with a given type.
func resolveSubfolders(t config.Target, homeDir, typ string, respectRunning bool) []Item {
	return resolveSubfoldersWithProcess(t, homeDir, typ, "", nil)
}

// resolveSubfoldersWithProcess resolves subfolders carrying bundle/process info.
func resolveSubfoldersWithProcess(t config.Target, homeDir, typ, bundleID string, processNames []string) []Item {
	var out []Item
	base := expandPath(t.Path, homeDir)

	if t.Pattern != "" {
		if _, err := os.Stat(base); err == nil {
			out = append(out, Item{
				Path:           base,
				Type:           typ,
				BundleID:       bundleID,
				ProcessNames:   processNames,
				RespectRunning: true,
			})
		}
		return out
	}

	if len(t.Subfolders) == 0 {
		if _, err := os.Stat(base); err == nil {
			out = append(out, Item{
				Path:           base,
				Type:           typ,
				BundleID:       bundleID,
				ProcessNames:   processNames,
				RespectRunning: true,
			})
		}
		return out
	}

	for _, sub := range t.Subfolders {
		p := filepath.Join(base, sub)
		if _, err := os.Stat(p); err == nil {
			out = append(out, Item{
				Path:           p,
				Type:           typ,
				BundleID:       bundleID,
				ProcessNames:   processNames,
				RespectRunning: true,
			})
		}
	}
	return out
}

// expandPath expands a leading "~/" to homeDir and cleans the result.
func expandPath(p, homeDir string) string {
	if p == "~" {
		return homeDir
	}
	if strings.HasPrefix(p, "~/") {
		return filepath.Join(homeDir, p[2:])
	}
	return filepath.ToSlash(p)
}
