package eviction

import (
	"os"

	"br4zz4/thepurge/config"
	"br4zz4/thepurge/internal/fsutil"
)

// EngineOptions controls one eviction pass. All fields are optional; the zero
// value selects safe defaults (dry-run, real disk usage, user home).
type EngineOptions struct {
	Apply      bool                                    // when true, delete for real (default dry-run).
	HomeDir    string                                  // base for "~" expansion (default: os.UserHomeDir).
	DiskUsage  func(path string) (int64, int64, error) // free, total (default: fsutil.DiskUsage).
	MaxBytes   int64                                   // cap bytes freed per pass (default: cfg.Storage.MaxBytesPerRun).
	MinAgeDays int                                     // prioritization floor (default: cfg.Retention.MinAgeDays).
}

// RunResult reports what one eviction pass did (or would do).
type RunResult struct {
	UnderThreshold bool        // free space stayed above threshold -> no eviction needed.
	Candidates     []Candidate // files deleted (apply) or queued (dry-run), in eviction order.
	FreedBytes     int64
	Skipped        []string
}

// Run performs one eviction pass: it measures free space on the configured
// volume and, only when it is below the threshold, resolves targets, scans for
// eviction candidates and deletes them (or previews them in dry-run) until free
// space reaches threshold + safety margin or the per-pass byte cap is hit.
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

	minAge := opts.MinAgeDays
	if minAge == 0 {
		minAge = cfg.Retention.MinAgeDays
	}
	cands, err := ScanTargets(items, minAge)
	if err != nil {
		return res, err
	}

	maxBytes := opts.MaxBytes
	if maxBytes == 0 && cfg.Storage.MaxBytesPerRun != "" {
		maxBytes, err = fsutil.ParseBytes(cfg.Storage.MaxBytesPerRun)
		if err != nil {
			return res, err
		}
	}

	del, err := DeleteItems(cands, DeleteOptions{DryRun: !opts.Apply, MaxBytes: maxBytes})
	if err != nil {
		return res, err
	}

	res.Candidates = cands
	res.FreedBytes = del.FreedBytes
	res.Skipped = del.Skipped
	return res, nil
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
		// ThresholdValue is stored as float64; treat whole values as bytes.
		threshold = int64(cfg.Storage.ThresholdValue)
	default:
		return 0, config.ErrInvalidThreshold
	}

	margin := int64(float64(total) * float64(cfg.Storage.SafetyMarginPercent) / 100)
	return threshold + margin, nil
}
