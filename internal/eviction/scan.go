package eviction

import (
	"io/fs"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"br4zz4/thepurge/internal/fsutil"
)

// Candidate is a single file (or empty directory) queued for eviction. Sorting
// is by priority type (logs before caches), then mtime descending, then size
// descending, so high-priority, oldest, heaviest go first.
type Candidate struct {
	Path    string
	Size    int64
	ModTime time.Time
	Type    string // "cache", "log" or "orphan"
}

// priorityOrder maps candidate types to their eviction priority (lower = first).
var priorityOrder = map[string]int{
	"log":    0,
	"cache":  1,
	"orphan": 2,
}

// ScanTargets walks each item’s directory (and its subfolders) collecting every
// regular file, ordered by priority (logs before caches), then mtime desc, then
// size desc. minAgeDays > 0 keeps only files whose mtime is at least that many
// days old (a prioritization weight, not a hard floor — pass 0 to disable).
// Item.Pattern narrows the walk to paths with a matching component. Directories
// guarded by their own TCC permission are pruned so macOS never prompts for
// access a disk cleaner has no use for. Empty directories produce no candidates.
func ScanTargets(items []Item, minAgeDays int) ([]Candidate, error) {
	var cands []Candidate
	cutoff := time.Time{}
	if minAgeDays > 0 {
		cutoff = time.Now().AddDate(0, 0, -minAgeDays)
	}

	for _, it := range items {
		roots := []string{it.Path}
		roots = append(roots, it.Subfolders...)

		for _, root := range roots {
			err := filepath.WalkDir(root, func(p string, d fs.DirEntry, err error) error {
				if err != nil {
					return nil
				}
				if d.IsDir() {
					if fsutil.IsTCCProtected(p) {
						return fs.SkipDir
					}
					return nil
				}
				if !matchesPattern(it.Pattern, root, p) {
					return nil
				}
				info, ierr := d.Info()
				if ierr != nil || !info.Mode().IsRegular() {
					return nil
				}
				if !cutoff.IsZero() && info.ModTime().After(cutoff) {
					return nil
				}
				cands = append(cands, Candidate{
					Path:    p,
					Size:    info.Size(),
					ModTime: info.ModTime(),
					Type:    it.Type,
				})
				return nil
			})
			if err != nil {
				return nil, err
			}
		}
	}

	// Sort: logs first (priority 0), then caches (1), then orphans (2).
	// Within same type: newest mtime first (so oldest files are processed
	// later when MaxBytes still has room — regra 3 says "oldest first",
	// which means we want OLDEST first when running out of space; but the
	// ordering is mtime DESC so recent files come up first and we process
	// newest→oldest, which means older files survive until they're the only
	// ones left. This matches "delete oldest first" under MaxBytes: as we
	// drain the list from the front, older files get their turn later when
	// we've already freed some space — which is fine. The spec says "delete
	// the oldest/heaviest first" (regra 3); to satisfy that strictly we should
	// sort mtime ASC (oldest first). Fixing: mtime ASC, then size DESC.
	sort.Slice(cands, func(i, j int) bool {
		pi := priorityOrder[cands[i].Type]
		pj := priorityOrder[cands[j].Type]
		if pi != pj {
			return pi < pj
		}
		// Oldest (smallest mtime) first — regra 3.
		if !cands[i].ModTime.Equal(cands[j].ModTime) {
			return cands[i].ModTime.Before(cands[j].ModTime)
		}
		// Heaviest first.
		return cands[i].Size > cands[j].Size
	})
	return cands, nil
}

// matchesPattern reports whether a file is in scope for an item's pattern. An
// empty pattern matches everything; otherwise every component of the path below
// root is tested, so a target on an app data dir with "*Cache*" only accepts
// files living inside a cache directory. Matching is case-sensitive, like
// filepath.Match. Paths outside root and malformed patterns match nothing.
func matchesPattern(pattern, root, path string) bool {
	if pattern == "" {
		return true
	}
	rel, err := filepath.Rel(root, path)
	if err != nil || rel == "." {
		return false
	}
	if rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return false
	}
	for _, part := range splitPath(rel) {
		if ok, err := filepath.Match(pattern, part); err == nil && ok {
			return true
		}
	}
	return false
}

func splitPath(rel string) []string {
	var parts []string
	for rel != "." && rel != string(filepath.Separator) && rel != "" {
		dir, file := filepath.Split(rel)
		if file != "" {
			parts = append(parts, file)
		}
		rel = filepath.Clean(dir)
	}
	return parts
}
