package eviction

import (
	"os"

	"br4zz4/thepurge/internal/fsutil"
)

// DeleteOptions controls how DeleteItems removes candidates.
type DeleteOptions struct {
	DryRun   bool  // when true, enumerate what would be deleted without removing files.
	MaxBytes int64 // stop deleting once this many bytes have been freed (0 = unlimited).
}

// DeleteResult summarizes one DeleteItems pass.
type DeleteResult struct {
	Deleted    []string // paths removed (or would be removed in dry-run).
	FreedBytes int64
	Skipped    []string // paths skipped for safety reasons (immutable, missing).
}

// DeleteItems removes candidates in order, respecting safety rules: immutable
// files are skipped and logged, permission/stat errors are skipped without
// aborting, and a MaxBytes cap stops the pass early. In dry-run mode nothing is
// removed but Deleted is still populated so callers can preview.
func DeleteItems(cands []Candidate, opts DeleteOptions) (DeleteResult, error) {
	res := DeleteResult{}
	var freed int64

	for _, c := range cands {
		if opts.MaxBytes > 0 && freed >= opts.MaxBytes {
			break
		}
		if fsutil.IsImmutable(c.Path) {
			res.Skipped = append(res.Skipped, c.Path)
			continue
		}

		if opts.DryRun {
			res.Deleted = append(res.Deleted, c.Path)
			continue
		}

		if err := os.Remove(c.Path); err != nil {
			if os.IsNotExist(err) {
				// Already gone; treat as free space not claimed and move on.
				continue
			}
			res.Skipped = append(res.Skipped, c.Path)
			continue
		}
		res.Deleted = append(res.Deleted, c.Path)
		freed += c.Size
	}

	res.FreedBytes = freed
	return res, nil
}
