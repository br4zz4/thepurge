package eviction

import (
	"io/fs"
	"path/filepath"
	"sort"
	"time"
)

// Candidate is a single file (or empty directory) queued for eviction. Sorting
// is by mtime descending then size descending, so the oldest/heaviest go first.
type Candidate struct {
	Path    string
	Size    int64
	ModTime time.Time
	Type    string // "cache", "log" or "orphan"
}

// ScanTargets walks each item's directory (and its subfolders) collecting every
// regular file, ordered by mtime desc then size desc. minAgeDays > 0 keeps only
// files whose mtime is at least that many days old (a prioritization weight, not
// a hard floor — pass 0 to disable). Empty directories produce no candidates.
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
					// Permission or transient error: skip and keep walking siblings.
					return nil
				}
				if d.IsDir() {
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

	sort.Slice(cands, func(i, j int) bool {
		if !cands[i].ModTime.Equal(cands[j].ModTime) {
			return cands[i].ModTime.After(cands[j].ModTime)
		}
		return cands[i].Size > cands[j].Size
	})
	return cands, nil
}
