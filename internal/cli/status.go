package cli

import (
	"fmt"
	"io"

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
	return 0, nil
}
