package cli

import (
	"fmt"
	"io"
	"os"
	"path/filepath"

	"br4zz4/thepurge/internal/service"
)

// binPath resolves the currently running thepurge binary for the agent.
func binPath() (string, error) {
	p, err := os.Executable()
	if err != nil {
		return "", err
	}
	abs, err := filepath.Abs(p)
	return abs, err
}

// service hooks are seams for tests, which must never touch the real launchd.
var (
	serviceInstall = service.Install
	serviceRemove  = service.Uninstall
	serviceCurrent = service.Current
)

// manager names the service manager in user-facing output.
func manager(st service.Status) string {
	if st.Backend == "" {
		return "service"
	}
	return st.Backend
}

// cmdService handles service install|uninstall|status.
func cmdService(args []string, out, errOut io.Writer) (int, error) {
	if len(args) == 0 {
		fmt.Fprintln(errOut, "usage: thepurge service install|uninstall|status")
		return 2, nil
	}
	switch args[0] {
	case "install":
		flags, _ := splitFlags(args[1:])
		apply := flags["--apply"]
		cfg, _, err := loadConfig()
		if err != nil {
			return 1, err
		}
		bin, err := binPath()
		if err != nil {
			return 1, err
		}
		interval := cfg.Storage.CheckIntervalMinutes * 60
		if err := serviceInstall(service.InstallOptions{BinPath: bin, IntervalSeconds: interval, Apply: apply}); err != nil {
			return 1, err
		}
		mode := "dry-run"
		if apply {
			mode = "apply"
		}
		fmt.Fprintf(out, "service installed (every %d min, %s): %s\n", cfg.Storage.CheckIntervalMinutes, mode, bin)
		return 0, nil

	case "uninstall":
		if err := serviceRemove(service.InstallOptions{}); err != nil {
			return 1, err
		}
		fmt.Fprintln(out, "service uninstalled")
		return 0, nil

	case "status":
		st, err := serviceCurrent(service.InstallOptions{})
		if err != nil {
			return 1, err
		}
		if !st.Installed {
			fmt.Fprintln(out, "service not installed")
			switch {
			case st.Loaded:
				fmt.Fprintf(out, "  WARNING: %s still has this job loaded; run `thepurge service uninstall`\n", manager(st))
			case st.ProgramWanted != "":
				fmt.Fprintln(out, "  WARNING: leftover unit definition found; run `thepurge service uninstall` to clean up")
			}
			return 0, nil
		}
		fmt.Fprintf(out, "unit:   %s\n", st.UnitPath)
		fmt.Fprintf(out, "binary: %s\n", st.ProgramWanted)
		if st.ProgramWanted != "" && !st.ProgramExists {
			fmt.Fprintln(out, "  WARNING: binary missing on disk; reinstall the service")
		}
		switch {
		case !st.Loaded:
			fmt.Fprintf(out, "%s: not loaded; run `thepurge service install`\n", manager(st))
		case st.Running:
			fmt.Fprintf(out, "%s: loaded, running (pid %d)\n", manager(st), st.PID)
		default:
			fmt.Fprintf(out, "%s: loaded, idle (last exit %d)\n", manager(st), st.LastExit)
		}
		if st.Mismatch() {
			fmt.Fprintf(out, "  WARNING: %s runs %s but the unit points to %s; reinstall the service\n", manager(st), st.ProgramLoaded, st.ProgramWanted)
		}
		return 0, nil

	default:
		fmt.Fprintf(errOut, "unknown service subcommand %q\n", args[0])
		return 2, nil
	}
}
