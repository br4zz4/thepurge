package cli

import (
	"fmt"
	"io"

	"br4zz4/thepurge/config"
	"br4zz4/thepurge/internal/eviction"
	"br4zz4/thepurge/internal/fsutil"
)

// cmdPurge runs one eviction pass manually: dry-run by default, --apply deletes.
func cmdPurge(args []string, out, errOut io.Writer) (int, error) {
	cfg, _, err := loadConfig()
	if err != nil {
		return 1, err
	}
	flags, _ := splitFlags(args)
	apply := flags["--apply"]
	force := flags["--force"]

	return runEviction(cfg, eviction.EngineOptions{Apply: apply}, force, out)
}

// cmdRun is the service entrypoint: one automated pass honouring service.apply.
func cmdRun(args []string, out, errOut io.Writer) (int, error) {
	cfg, _, err := loadConfig()
	if err != nil {
		return 1, err
	}
	return runEviction(cfg, eviction.EngineOptions{Apply: cfg.Service.Apply}, false, out)
}

// runEviction shares purge/run logic and renders candidates to out.
func runEviction(cfg *config.Config, opts eviction.EngineOptions, force bool, out io.Writer) (int, error) {
	res, err := eviction.Run(cfg, opts)
	if err != nil {
		return 1, err
	}

	fmt.Fprintf(out, "free space status: %s\n", spaceStatus(cfg))
	if res.UnderThreshold {
		fmt.Fprintln(out, "above threshold — nothing to evict")
		return 0, nil
	}

	var freed int64
	if opts.Apply {
		freed = res.FreedBytes
	} else {
		// Walk candidates to compute total size that would be freed in dry-run.
		for _, c := range res.Candidates {
			if isSkipped(c.Path, res.Skipped) {
				continue
			}
			freed += c.Size
		}
	}

	fmt.Fprintf(out, "freeing:       %s\n", fsutil.FormatBytes(freed))
	for _, c := range res.Candidates {
		marker := "would delete"
		if opts.Apply {
			marker = "deleted"
		}
		fmt.Fprintf(out, "  %-12s %-14s %s\n", marker, fsutil.FormatBytes(c.Size), c.Path)
	}
	for _, s := range res.Skipped {
		fmt.Fprintf(out, "  %-12s %s\n", "skipped", s)
	}
	return 0, nil
}

func isSkipped(path string, skipped []string) bool {
	for _, s := range skipped {
		if s == path {
			return true
		}
	}
	return false
}

func spaceStatus(cfg *config.Config) string {
	free, total, err := fsutil.DiskUsage(cfg.Storage.Volume)
	if err != nil {
		return "unknown"
	}
	pct := float64(0)
	if total > 0 {
		pct = float64(free) / float64(total) * 100
	}
	return fmt.Sprintf("%.1f%% free (%.1f GB of %.1f GB)", pct, float64(free)/1e9, float64(total)/1e9)
}
