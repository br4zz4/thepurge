package cli

import (
	"fmt"
	"io"

	"br4zz4/thepurge/config"
	"br4zz4/thepurge/internal/docker"
	"br4zz4/thepurge/internal/eviction"
	"br4zz4/thepurge/internal/fsutil"
)

// cmdStatus prints real free space, the configured threshold and the run mode.
func cmdStatus(args []string, out, errOut io.Writer) (int, error) {
	cfg, _, err := loadConfig()
	if err != nil {
		return 1, err
	}

	volume := cfg.Storage.Volume
	if volume == "" {
		volume = "/"
	}
	free, total, err := fsutil.DiskUsage(volume)
	if err != nil {
		return 1, fmt.Errorf("reading disk usage of %s: %v", volume, err)
	}

	fmt.Fprintf(out, "volume:       %s\n", volume)
	fmt.Fprintf(out, "total:        %s\n", fsutil.FormatBytes(total))
	fmt.Fprintf(out, "free:         %s\n", fsutil.FormatBytes(free))
	pct := float64(0)
	if total > 0 {
		pct = float64(free) / float64(total) * 100
	}
	fmt.Fprintf(out, "free percent: %.1f%%\n", pct)
	fmt.Fprintf(out, "threshold:    %v %s\n", cfg.Storage.ThresholdValue, cfg.Storage.ThresholdType)
	renderPotential(cfg, out)
	return 0, nil
}

// statusDocker is the docker backend used by status; nil selects the real CLI.
var statusDocker docker.Ops

// renderPotential previews what a pass could free regardless of the threshold
// and of max_bytes_per_run. It never deletes anything.
func renderPotential(cfg *config.Config, out io.Writer) {
	probe := *cfg
	probe.Storage.MaxBytesPerRun = ""
	res, err := eviction.Run(&probe, eviction.EngineOptions{IgnoreThreshold: true, Docker: statusDocker})
	if err != nil {
		fmt.Fprintf(out, "potential:    unavailable (%v)\n", err)
		return
	}
	files := candidateBytes(res.Candidates)
	fmt.Fprintf(out, "potential:    %s total (does not depend on threshold)\n",
		fsutil.FormatBytes(files+res.Docker.ReclaimableBytes))
	fmt.Fprintf(out, "  files:      %s (%d files)\n", fsutil.FormatBytes(files), len(res.Candidates))
	if res.Docker.Ran {
		fmt.Fprintf(out, "  docker:     %s (images %s + build cache %s)\n",
			fsutil.FormatBytes(res.Docker.ReclaimableBytes),
			fsutil.FormatBytes(res.Docker.ImagesBytes), fsutil.FormatBytes(res.Docker.BuildCacheBytes))
	} else if res.Docker.SkippedReason != "" {
		fmt.Fprintf(out, "  docker:     skipped (%s)\n", res.Docker.SkippedReason)
	}
}
