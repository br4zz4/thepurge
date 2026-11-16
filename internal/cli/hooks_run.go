package cli

import (
	"context"
	"errors"
	"fmt"
	"io"

	"br4zz4/thepurge/config"
	"br4zz4/thepurge/internal/eviction"
	"br4zz4/thepurge/internal/fsutil"
	"br4zz4/thepurge/internal/hooks"
	"br4zz4/thepurge/internal/logging"
)

// hookRunner runs the configured hooks of one pass, logging each event.
type hookRunner struct {
	cfg   *config.Config
	log   *logging.Logger
	out   io.Writer
	apply bool
}

func (h hookRunner) mode() string { return modeLabel(h.apply) }

func (h hookRunner) run(event string, list []config.Hook) error {
	if len(list) == 0 {
		return nil
	}
	var errs []error
	for _, hk := range list {
		h.log.Info("hook %s: start %s", event, hooks.Label(hk))
		if err := (hooks.Runner{}).Run(context.Background(), event, h.mode(), []config.Hook{hk}); err != nil {
			h.log.Error("hook %s failed: %v", event, err)
			errs = append(errs, err)
			continue
		}
		h.log.Info("hook %s: ok %s", event, hooks.Label(hk))
	}
	return errors.Join(errs...)
}

func (h hookRunner) post() {
	if err := h.run("post_eviction", h.cfg.Hooks.PostEviction); err != nil {
		h.notify("post_eviction hook failed")
	}
}

func (h hookRunner) notify(msg string) {
	if h.cfg.Logging.NotifyOnFailure {
		fsutil.Notify("The Purge", msg)
	}
}

func (h hookRunner) listDryRun() {
	events := []struct {
		name string
		list []config.Hook
	}{
		{"pre_eviction", h.cfg.Hooks.PreEviction},
		{"post_eviction", h.cfg.Hooks.PostEviction},
		{"on_failure", h.cfg.Hooks.OnFailure},
	}
	for _, ev := range events {
		for _, hk := range ev.list {
			fmt.Fprintf(h.out, "hook %s: would run %s\n", ev.name, hooks.Label(hk))
		}
	}
}

func renderDocker(d eviction.DockerResult, apply bool, out io.Writer, log *logging.Logger) {
	if d.SkippedReason != "" {
		fmt.Fprintf(out, "docker: skipped (%s)\n", d.SkippedReason)
		log.Warn("docker skipped: %s", d.SkippedReason)
	}
	for _, e := range d.Errors {
		fmt.Fprintf(out, "docker: %s\n", e)
		log.Warn("%s", e)
	}
	if !d.Ran {
		return
	}
	if apply {
		fmt.Fprintf(out, "docker freed: %s\n", fsutil.FormatBytes(d.FreedBytes))
		log.Info("docker freed %s (%v)", fsutil.FormatBytes(d.FreedBytes), d.Prunes)
		return
	}
	fmt.Fprintf(out, "docker: reclaimable ~%s (images) + ~%s (build cache), %d dangling image(s)\n",
		fsutil.FormatBytes(d.ImagesBytes), fsutil.FormatBytes(d.BuildCacheBytes), d.DanglingImageCount)
	for _, id := range d.DanglingImageIDs {
		fmt.Fprintf(out, "  dangling %s\n", id)
	}
	log.Info("docker dry-run: reclaimable ~%s", fsutil.FormatBytes(d.ReclaimableBytes))
}
