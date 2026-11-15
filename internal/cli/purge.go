package cli

import (
	"fmt"
	"io"

	"br4zz4/thepurge/config"
	"br4zz4/thepurge/internal/eviction"
	"br4zz4/thepurge/internal/fsutil"
	"br4zz4/thepurge/internal/logging"
)

// cmdPurge runs one eviction pass manually: dry-run by default, --apply deletes.
// --force bypasses running-apps protection (requires dry-run first).
func cmdPurge(args []string, out, errOut io.Writer) (int, error) {
	cfg, _, err := loadConfig()
	if err != nil {
		return 1, err
	}
	flags, _ := splitFlags(args)
	apply := flags["--apply"]
	force := flags["--force"]

	return runEviction(cfg, eviction.EngineOptions{Apply: apply, Force: force}, out)
}

// cmdRun is the service entrypoint: one automated pass. Defaults to dry-run;
// --apply deletes for real (used by thepurge service install --apply).
func cmdRun(args []string, out, errOut io.Writer) (int, error) {
	cfg, _, err := loadConfig()
	if err != nil {
		return 1, err
	}
	flags, _ := splitFlags(args)
	return runEviction(cfg, eviction.EngineOptions{Apply: flags["--apply"]}, out)
}

// runEviction shares purge/run logic: hooks around the pass (apply only),
// the eviction itself and the rendering of its result to out.
func runEviction(cfg *config.Config, opts eviction.EngineOptions, out io.Writer) (int, error) {
	if err := config.Validate(cfg); err != nil {
		return 1, err
	}
	log, err := logging.New(logPathFor(cfg))
	if err != nil {
		return 1, fmt.Errorf("opening log: %v", err)
	}
	defer log.Close()

	h := hookRunner{cfg: cfg, log: log, out: out, apply: opts.Apply}
	if !opts.Apply {
		h.listDryRun()
	} else if err := h.run("pre_eviction", cfg.Hooks.PreEviction); err != nil {
		h.notify("pre_eviction hook failed; eviction aborted")
		h.run("on_failure", cfg.Hooks.OnFailure)
		return 1, fmt.Errorf("pre_eviction hook failed, eviction aborted: %v", err)
	}

	res, err := eviction.Run(cfg, opts)
	if err != nil {
		h.run("on_failure", cfg.Hooks.OnFailure)
		h.post()
		return 1, err
	}

	renderEviction(cfg, opts, res, out, log)

	if opts.Apply && !res.UnderThreshold && res.FreedBytes+res.Docker.FreedBytes == 0 {
		h.run("on_failure", cfg.Hooks.OnFailure)
	}
	h.post()
	return 0, nil
}

// renderEviction prints and logs the outcome of one pass.
func renderEviction(cfg *config.Config, opts eviction.EngineOptions, res eviction.RunResult, out io.Writer, log *logging.Logger) {
	fmt.Fprintf(out, "free space status: %s\n", spaceStatus(cfg))
	if res.UnderThreshold {
		fmt.Fprintln(out, "above threshold — nothing to evict")
		log.Info("check: above threshold, no eviction (%s)", spaceStatus(cfg))
		return
	}

	for _, s := range res.AppSkipped {
		fmt.Fprintf(out, "  app running, skipped: %s\n", s)
		log.Warn("app running, skipped: %s", s)
	}

	if opts.Apply && cfg.Logging.NotifyOnFailure {
		if len(res.Candidates) == 0 && len(res.AppSkipped) > 0 {
			fsutil.Notify("The Purge", "All targets skipped: apps are running. Try --force or close the apps.")
		} else if res.FreedBytes == 0 && len(res.Skipped) > 0 {
			fsutil.Notify("The Purge", "Eviction ran but freed 0 bytes. Check config and Full Disk Access.")
		}
	}
	if opts.Apply {
		log.Info("eviction applied: freed %s (%d files), %d skipped",
			fsutil.FormatBytes(res.FreedBytes), len(res.Candidates), len(res.Skipped))
		fmt.Fprintf(out, "freed:   %s (%d files)\n", fsutil.FormatBytes(res.FreedBytes), len(res.Candidates))
		for _, c := range res.Candidates {
			log.Info("deleted %s %s", fsutil.FormatBytes(c.Size), c.Path)
			fmt.Fprintf(out, "  %-12s %-14s %s\n", "deleted", fsutil.FormatBytes(c.Size), c.Path)
		}
		for _, s := range res.Skipped {
			log.Warn("skipped %s", s)
			fmt.Fprintf(out, "  %-12s %s\n", "skipped", s)
		}
		renderDocker(res.Docker, true, out, log)
		return
	}

	summary := renderSummary(res.Candidates, res.Skipped, 10)
	summary += fmt.Sprintf("service mode: %s\n", modeLabel(opts.Apply))
	if opts.Force {
		summary += "force mode: running-apps protection bypassed\n"
	}
	fmt.Fprint(out, summary)
	log.Info("eviction dry-run: would free %s (%d files), %d skipped",
		fsutil.FormatBytes(candidateBytes(res.Candidates)), len(res.Candidates), len(res.Skipped))
	renderDocker(res.Docker, false, out, log)
}

// candidateBytes sums the sizes of all candidates (the total that would be freed).
func candidateBytes(cands []eviction.Candidate) int64 {
	var total int64
	for _, c := range cands {
		total += c.Size
	}
	return total
}

// modeLabel renders whether this pass deletes or previews.
func modeLabel(apply bool) string {
	if apply {
		return "apply"
	}
	return "dry-run"
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
