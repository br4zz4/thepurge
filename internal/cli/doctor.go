package cli

import (
	"context"
	"fmt"
	"io"
	"os"

	"br4zz4/thepurge/config"
	"br4zz4/thepurge/internal/docker"
	"br4zz4/thepurge/internal/eviction"
	"br4zz4/thepurge/internal/fsutil"
	"br4zz4/thepurge/internal/hooks"
)

// cmdDoctor is a pre-flight check: config parse, disk space, target existence and
// access (FDA/TCC). It reports problems without failing the process.
func cmdDoctor(args []string, out, errOut io.Writer) (int, error) {
	cfg, path, err := loadConfig()
	if err != nil {
		return 1, err
	}

	fail := false
	fmt.Fprintf(out, "config: %s\n", path)
	fmt.Fprintf(out, "  %s\n", currentVersion().Report())

	vol := cfg.Storage.Volume
	if vol == "" {
		vol = "/"
	}
	free, total, err := fsutil.DiskUsage(vol)
	if err != nil {
		fmt.Fprintf(out, "  disk: ERROR reading %s: %v\n", vol, err)
		fail = true
	} else {
		fmt.Fprintf(out, "  disk: %s free of %s on %s\n", fsutil.FormatBytes(free), fsutil.FormatBytes(total), vol)
	}

	// Platform gate: Full Disk Access on macOS, a systemd user session on Linux.
	if !checkHostAccess(out) {
		fail = true
	}

	items, err := eviction.ResolveTargets(cfg, cfgHome())
	if err != nil {
		fmt.Fprintf(out, "  targets: ERROR %v\n", err)
		fail = true
	}
	if len(items) == 0 {
		if countTargets(cfg) == 0 {
			fmt.Fprintln(out, "  targets: none configured (run `thepurge config init` or edit config.yml)")
			fail = true
		} else {
			fmt.Fprintln(out, "  targets: none resolved (check paths and host access)")
		}
	} else {
		unreachable := 0
		for _, it := range items {
			if _, err := os.Stat(it.Path); err != nil {
				unreachable++
				fmt.Fprintf(out, "  target: %s (unreachable: %v)\n", it.Path, err)
			}
		}
		if unreachable == len(items) {
			fmt.Fprintln(out, "  targets: none reachable — check paths and host access")
			fail = true
		} else {
			fmt.Fprintf(out, "  targets: %d resolved, %d reachable\n", len(items), len(items)-unreachable)
		}
	}

	checkDocker(cfg, out)
	checkHooks(cfg, out)

	if fail {
		fmt.Fprintln(out, "doctor: PROBLEMS FOUND — resolve before relying on the service")
		return 1, nil
	}
	fmt.Fprintln(out, "doctor: OK")
	return 0, nil
}

// cfgHome resolves the home dir used for target expansion (same as engine default).
func cfgHome() string {
	h, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	return h
}

// doctorDocker is the docker backend probed by doctor; tests may replace it.
var doctorDocker docker.Ops = docker.NewClient()

// checkDocker reports docker availability. It only warns, never fails doctor.
func checkDocker(cfg *config.Config, out io.Writer) {
	if !cfg.Docker.Enabled {
		return
	}
	status, _ := doctorDocker.Probe(context.Background())
	switch status {
	case docker.StatusOK:
		fmt.Fprintln(out, "  docker: ok")
	case docker.StatusBinaryMissing:
		fmt.Fprintln(out, "  docker: not installed — cleanup disabled")
	default:
		fmt.Fprintln(out, "  docker: daemon not running")
	}
}

// checkHooks reports hook counts and warns about invalid or missing hooks.
func checkHooks(cfg *config.Config, out io.Writer) {
	events := []struct {
		name string
		list []config.Hook
	}{
		{"pre_eviction", cfg.Hooks.PreEviction},
		{"post_eviction", cfg.Hooks.PostEviction},
		{"on_failure", cfg.Hooks.OnFailure},
	}
	for _, ev := range events {
		if len(ev.list) == 0 {
			continue
		}
		fmt.Fprintf(out, "  hooks %s: %d\n", ev.name, len(ev.list))
		for _, h := range ev.list {
			switch {
			case h.Path != "" && h.Command != "", h.Path == "" && h.Command == "":
				fmt.Fprintf(out, "  hooks %s: WARNING invalid hook (need exactly one of path/command)\n", ev.name)
			case h.Path != "":
				if _, err := os.Stat(hooks.ExpandHome(h.Path)); err != nil {
					fmt.Fprintf(out, "  hooks %s: WARNING %s not found\n", ev.name, h.Path)
				}
			}
		}
	}
}
