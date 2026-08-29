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
	Path       string
	Type       string // "cache", "log" or "orphan"
	Subfolders []string
	Orphan     bool
}

// ResolveTargets converts the config's eviction targets into concrete on-disk
// items. homeDir is injected so callers can point at a test sandbox; pass the
// real user home in production. Missing folders and subfolders are skipped
// silently (nothing to evict). Enabled orphan apps resolve to whole-folder items.
func ResolveTargets(cfg *config.Config, homeDir string) ([]Item, error) {
	var items []Item

	for _, t := range cfg.EvictionTargets.SafeCaches {
		items = append(items, resolveSubfolders(t, homeDir, t.Type)...)
	}
	for _, t := range cfg.EvictionTargets.Logs {
		items = append(items, resolveSubfolders(t, homeDir, t.Type)...)
	}
	for _, t := range cfg.EvictionTargets.VolatileApps {
		// Volatile apps reuse the same cache/log mechanics; their type is whatever
		// the target declares, defaulting to "cache".
		typ := t.Type
		if typ == "" {
			typ = "cache"
		}
		items = append(items, resolveSubfolders(t, homeDir, typ)...)
	}
	for _, p := range cfg.EvictionTargets.EnabledOrphanApps {
		full := expandPath(p, homeDir)
		if _, err := os.Stat(full); err != nil {
			continue // not present; nothing to evict
		}
		items = append(items, Item{Path: full, Type: "orphan", Orphan: true})
	}

	return items, nil
}

// resolveSubfolders expands a target and emits one Item per existing subfolder.
// Targets with a Pattern are handled as a single item if the resolved path
// exists; plain subfolder lists yield one item per subfolder that exists.
func resolveSubfolders(t config.Target, homeDir, typ string) []Item {
	var out []Item
	base := expandPath(t.Path, homeDir)

	if t.Pattern != "" {
		if _, err := os.Stat(base); err == nil {
			out = append(out, Item{Path: base, Type: typ})
		}
		return out
	}

	// No subfolders and no pattern: the target path itself is the item.
	if len(t.Subfolders) == 0 {
		if _, err := os.Stat(base); err == nil {
			out = append(out, Item{Path: base, Type: typ})
		}
		return out
	}

	for _, sub := range t.Subfolders {
		p := filepath.Join(base, sub)
		if _, err := os.Stat(p); err == nil {
			out = append(out, Item{Path: p, Type: typ})
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
