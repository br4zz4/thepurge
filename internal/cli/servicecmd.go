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
		if err := service.Install(service.InstallOptions{BinPath: bin, IntervalSeconds: interval, Apply: apply}); err != nil {
			return 1, err
		}
		mode := "dry-run"
		if apply {
			mode = "apply"
		}
		fmt.Fprintf(out, "service installed (every %d min, %s): %s\n", cfg.Storage.CheckIntervalMinutes, mode, bin)
		return 0, nil

	case "uninstall":
		if err := service.Uninstall(service.InstallOptions{}); err != nil {
			return 1, err
		}
		fmt.Fprintln(out, "service uninstalled")
		return 0, nil

	case "status":
		st, err := service.Current(service.InstallOptions{})
		if err != nil {
			return 1, err
		}
		if !st.Installed {
			fmt.Fprintln(out, "service not installed")
			return 0, nil
		}
		fmt.Fprintf(out, "plist:  %s\n", st.PlistPath)
		fmt.Fprintf(out, "binary: %s\n", st.PlistProgram)
		if !st.ProgramExists {
			fmt.Fprintln(out, "  WARNING: binary missing on disk; reinstall the service")
		}
		switch {
		case !st.Loaded:
			fmt.Fprintln(out, "launchd: not loaded; run `thepurge service install`")
		case st.Running:
			fmt.Fprintf(out, "launchd: loaded, running (pid %d)\n", st.PID)
		default:
			fmt.Fprintf(out, "launchd: loaded, idle (last exit %d)\n", st.LastExit)
		}
		if st.Mismatch() {
			fmt.Fprintf(out, "  WARNING: launchd runs %s but the plist points to %s; reinstall the service\n", st.Program, st.PlistProgram)
		}
		return 0, nil

	default:
		fmt.Fprintf(errOut, "unknown service subcommand %q\n", args[0])
		return 2, nil
	}
}
