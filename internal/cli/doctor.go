package cli

import (
	"fmt"
	"io"
	"os"
	"path/filepath"

	"br4zz4/thepurge/internal/eviction"
	"br4zz4/thepurge/internal/fsutil"
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

	// FDA/TCC probe: ~/Library/Application Support is TCC-protected; without
	// Full Disk Access we cannot even list it. This is a hard blocker, not a WARN.
	if home := cfgHome(); home != "" {
		probe := filepath.Join(home, "Library", "Application Support")
		if _, err := os.ReadDir(probe); err != nil {
			if os.IsPermission(err) {
				fmt.Fprintf(out, "  fda: cannot read %s: %v\n", probe, err)
				fmt.Fprintln(out, "  fda: grant Full Disk Access to thepurge (System Settings > Privacy & Security > Full Disk Access)")
				fail = true
			} else if os.IsNotExist(err) {
				fmt.Fprintf(out, "  fda: %s does not exist (not a TCC failure)\n", probe)
			} else {
				fmt.Fprintf(out, "  fda: cannot read %s: %v\n", probe, err)
			}
		} else {
			fmt.Fprintln(out, "  fda: OK (can read Library/Application Support)")
		}
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
			fmt.Fprintln(out, "  targets: none resolved (check paths and Full Disk Access)")
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
			fmt.Fprintln(out, "  targets: none reachable — likely missing Full Disk Access (TCC)")
			fail = true
		} else {
			fmt.Fprintf(out, "  targets: %d resolved, %d reachable\n", len(items), len(items)-unreachable)
		}
	}

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
