package eviction

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"br4zz4/thepurge/config"
	"br4zz4/thepurge/internal/docker"
	"br4zz4/thepurge/internal/fsutil"
)

// EngineOptions controls one eviction pass. All fields are optional; the zero
// value selects safe defaults (dry-run, real disk usage, user home).
type EngineOptions struct {
	Apply            bool                                           // when true, delete for real (default dry-run).
	HomeDir          string                                         // base for "~" expansion (default: os.UserHomeDir).
	DiskUsage        func(path string) (int64, int64, error)        // free, total (default: fsutil.DiskUsage).
	MaxBytes         int64                                          // cap bytes freed per pass (default: cfg.Storage.MaxBytesPerRun).
	MinAgeDays       int                                            // prioritization floor (default: cfg.Retention.MinAgeDays).
	Force            bool                                           // when true, ignore running-apps protection (default: false).
	IsProcessRunning func(bundleID string, processName string) bool // app-running check (default: fsutil.IsProcessRunning).
	IsAppInstalled   func(appPath string) (bool, string)            // app installation check for orphan detection (default: checkApplications).
	ExcludePaths     []string                                       // paths never to delete (e.g. the app own log).
	Docker           docker.Ops                                     // docker cleanup backend (default: real docker CLI).
}

// RunResult reports what one eviction pass did (or would do).
type RunResult struct {
	UnderThreshold bool        // free space stayed above threshold -> no eviction needed.
	Candidates     []Candidate // files deleted (apply) or queued (dry-run), in eviction order.
	FreedBytes     int64
	Skipped        []string
	AppSkipped     []string // paths skipped because the owning app was running.
	Docker         DockerResult
}

// Run performs one eviction pass: it measures free space and, when below
// threshold, resolves targets, filters out items whose apps are running,
// scans for candidates, prioritises logs over caches, and deletes until
// free space reaches threshold + safety margin or the per-pass byte cap.
func Run(cfg *config.Config, opts EngineOptions) (RunResult, error) {
	var res RunResult

	home := opts.HomeDir
	if home == "" {
		h, err := os.UserHomeDir()
		if err != nil {
			return res, err
		}
		home = h
	}

	disk := opts.DiskUsage
	if disk == nil {
		disk = fsutil.DiskUsage
	}
	free, total, err := disk(cfg.Storage.Volume)
	if err != nil {
		return res, err
	}

	threshold, err := computeThreshold(cfg, total)
	if err != nil {
		return res, err
	}
	if free >= threshold {
		res.UnderThreshold = true
		return res, nil
	}

	items, err := ResolveTargets(cfg, home)
	if err != nil {
		return res, err
	}

	// Filter out items whose apps are running (unless Force).
	processCheck := opts.IsProcessRunning
	if processCheck == nil {
		processCheck = fsutil.IsProcessRunning
	}
	appCheck := opts.IsAppInstalled
	if appCheck == nil {
		appCheck = checkApplications
	}

	var filtered []Item
	for _, it := range items {
		if it.Orphan {
			// Orphan detection: verify the app is truly uninstalled.
			if installed, name := appCheck(it.Path); installed {
				res.Skipped = append(res.Skipped,
					fmt.Sprintf("orphan %s: app %s still installed", it.Path, name))
				continue
			}
		}

		if !opts.Force && it.RespectRunning && (it.BundleID != "" || len(it.ProcessNames) > 0) {
			// Try bundle ID first, then each process name.
			running := false
			if it.BundleID != "" {
				running = processCheck(it.BundleID, "")
			}
			if !running {
				names := it.ProcessNames
				if len(names) == 0 {
					names = deriveProcessNames(it.Path)
				}
				for _, name := range names {
					if processCheck("", name) {
						running = true
						break
					}
				}
			}
			if running {
				res.AppSkipped = append(res.AppSkipped, it.Path)
				continue
			}
		}
		filtered = append(filtered, it)
	}

	// Prepare items for scanning: exclude subfolders from orphan items
	// (they are whole-folder removals).
	scanItems := make([]Item, 0, len(filtered))
	for _, it := range filtered {
		if it.Orphan {
			// Orphans are whole folders; we remove the directory itself.
			scanItems = append(scanItems, it)
		} else {
			scanItems = append(scanItems, it)
		}
	}

	minAge := opts.MinAgeDays
	if minAge == 0 && cfg.Retention.MinAgeDays > 0 {
		minAge = cfg.Retention.MinAgeDays
	}
	cands, err := ScanTargets(scanItems, minAge)
	if err != nil {
		return res, err
	}

	// Never evict the app own log (or any explicitly excluded path).
	excludes := append([]string{}, opts.ExcludePaths...)
	if cfg.Logging.LogPath != "" {
		excludes = append(excludes, expandPath(cfg.Logging.LogPath, home))
	}
	if len(excludes) > 0 {
		cands = filterExcluded(cands, excludes)
	}

	maxBytes := opts.MaxBytes
	if maxBytes == 0 && cfg.Storage.MaxBytesPerRun != "" {
		maxBytes, err = fsutil.ParseBytes(cfg.Storage.MaxBytesPerRun)
		if err != nil {
			return res, err
		}
	}

	del, err := DeleteItems(cands, DeleteOptions{
		DryRun:   !opts.Apply,
		MaxBytes: maxBytes,
		Force:    opts.Force,
	})
	if err != nil {
		return res, err
	}

	// Candidates must reflect what DeleteItems actually accepted: files skipped
	// as immutable (and anything past the byte cap) must not be reported as
	// freed in dry-run nor as deleted in apply.
	res.Candidates = selected(cands, del.Deleted)
	res.FreedBytes = del.FreedBytes
	res.Skipped = append(res.Skipped, del.Skipped...)

	res.Docker = runDockerPhase(cfg, opts, disk, threshold)
	return res, nil
}

// DockerResult reports the docker cleanup phase. Its bytes are kept apart from
// RunResult.FreedBytes and never count against max_bytes_per_run.
type DockerResult struct {
	Ran                bool
	Prunes             []string
	ReclaimableBytes   int64
	DanglingImageCount int
	FreedBytes         int64
	SkippedReason      string
}

func runDockerPhase(cfg *config.Config, opts EngineOptions, disk func(string) (int64, int64, error), threshold int64) DockerResult {
	var dr DockerResult
	d := cfg.Docker
	if !d.Enabled || !(d.DanglingImages || d.UnusedImages || d.BuilderCache) {
		return dr
	}
	if free, _, err := disk(cfg.Storage.Volume); err == nil && free >= threshold {
		return dr
	}

	ops := opts.Docker
	if ops == nil {
		ops = docker.NewClient()
	}
	ctx := context.Background()

	status, err := ops.Probe(ctx)
	if err != nil {
		dr.SkippedReason = fmt.Sprintf("docker probe failed: %v", err)
		return dr
	}
	switch status {
	case docker.StatusBinaryMissing:
		dr.SkippedReason = "docker not installed"
		return dr
	case docker.StatusDaemonDown:
		dr.SkippedReason = "docker daemon not running"
		return dr
	}
	dr.Ran = true

	if !opts.Apply {
		if rec, err := ops.Reclaimable(ctx); err != nil {
			dr.SkippedReason = fmt.Sprintf("docker df failed: %v", err)
		} else {
			if d.DanglingImages || d.UnusedImages {
				dr.ReclaimableBytes += rec.Images
			}
			if d.BuilderCache {
				dr.ReclaimableBytes += rec.BuildCache
			}
		}
		if d.DanglingImages || d.UnusedImages {
			if ids, err := ops.DanglingImageIDs(ctx); err == nil {
				dr.DanglingImageCount = len(ids)
			}
		}
		return dr
	}

	var kinds []docker.PruneKind
	switch {
	case d.UnusedImages:
		kinds = append(kinds, docker.PruneAllUnused)
	case d.DanglingImages:
		kinds = append(kinds, docker.PruneDangling)
	}
	if d.BuilderCache {
		kinds = append(kinds, docker.PruneBuilder)
	}
	for _, k := range kinds {
		pr, err := ops.Prune(ctx, k)
		if err != nil {
			dr.SkippedReason = fmt.Sprintf("docker %s failed: %v", k, err)
			continue
		}
		dr.Prunes = append(dr.Prunes, k.String())
		dr.FreedBytes += pr.FreedBytes
	}
	return dr
}

// selected keeps the candidates whose path appears in deleted, preserving order.
func selected(cands []Candidate, deleted []string) []Candidate {
	if len(deleted) == len(cands) {
		return cands
	}
	kept := make(map[string]bool, len(deleted))
	for _, p := range deleted {
		kept[p] = true
	}
	out := cands[:0]
	for _, c := range cands {
		if kept[c.Path] {
			out = append(out, c)
		}
	}
	return out
}

// computeThreshold translates the config's threshold into bytes, deriving the
// margin (safety_margin_percent) from the volume total.
func computeThreshold(cfg *config.Config, total int64) (int64, error) {
	var threshold int64
	switch cfg.Storage.ThresholdType {
	case "percent":
		if cfg.Storage.ThresholdValue < 0 || cfg.Storage.ThresholdValue > 100 {
			return 0, config.ErrInvalidThreshold
		}
		threshold = int64(float64(total) * cfg.Storage.ThresholdValue / 100)
	case "size":
		threshold = int64(cfg.Storage.ThresholdValue)
	default:
		return 0, config.ErrInvalidThreshold
	}

	margin := int64(float64(total) * float64(cfg.Storage.SafetyMarginPercent) / 100)
	return threshold + margin, nil
}

// deriveProcessNames extracts plausible macOS process names from a filesystem
// path (e.g. "~/Library/Application Support/Google/Chrome" -> ["Chrome"]).
func deriveProcessNames(path string) []string {
	// Walk the path looking for a plausible app name.
	parts := strings.Split(filepath.ToSlash(path), "/")
	for i := len(parts) - 1; i >= 0; i-- {
		p := parts[i]
		if p != "" && p != "Application Support" && p != "Library" &&
			p != "Containers" && p != "Data" && !strings.HasPrefix(p, "com.") &&
			!strings.HasPrefix(p, "~") {
			return []string{p}
		}
	}
	return nil
}

// filterExcluded drops candidates whose path equals any excluded path (exact match).
func filterExcluded(cands []Candidate, excludes []string) []Candidate {
	if len(excludes) == 0 {
		return cands
	}
	exSet := make(map[string]bool, len(excludes))
	for _, e := range excludes {
		if e != "" {
			exSet[e] = true
		}
	}
	out := cands[:0]
	for _, c := range cands {
		if !exSet[c.Path] {
			out = append(out, c)
		}
	}
	return out
}
