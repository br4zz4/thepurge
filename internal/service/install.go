package service

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
)

// Runner executes shell commands (used for launchctl). It is injectable so tests
// can record calls instead of touching the real launchd.
type Runner interface {
	Run(name string, args ...string) ([]byte, error)
}

// execRunner runs commands via os/exec — the production default.
type execRunner struct{}

func (execRunner) Run(name string, args ...string) ([]byte, error) {
	return exec.Command(name, args...).CombinedOutput()
}

// installFileName is the launchd plist filename for the agent.
const installFileName = Label + ".plist"

// InstallOptions configures Install/Uninstall. Zero values select defaults where
// documented.
type InstallOptions struct {
	BinPath         string // absolute path to the thepurge binary.
	IntervalSeconds int    // StartInterval in seconds (default: 1800).
	Apply           bool   // if true, the scheduled run deletes for real.
	LaunchAgentsDir string // dir for the agent plist (default: ~/Library/LaunchAgents).
	Runner          Runner // command runner (default: execRunner).
}

// AgentPlistPath returns where the agent plist lives (or would live).
func AgentPlistPath(dir string) string {
	return filepath.Join(dir, installFileName)
}

// defaultLaunchAgentsDir returns the per-user LaunchAgents directory.
func defaultLaunchAgentsDir() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, "Library", "LaunchAgents"), nil
}

// resolve resolves defaults for an InstallOptions.
func (o InstallOptions) resolve() (InstallOptions, error) {
	if o.IntervalSeconds == 0 {
		o.IntervalSeconds = 1800
	}
	if o.LaunchAgentsDir == "" {
		d, err := defaultLaunchAgentsDir()
		if err != nil {
			return o, err
		}
		o.LaunchAgentsDir = d
	}
	if o.Runner == nil {
		o.Runner = execRunner{}
	}
	return o, nil
}

// Install writes the launchd agent plist and bootstraps it into the user's GUI
// session so `thepurge run` fires every IntervalSeconds.
func Install(opts InstallOptions) error {
	o, err := opts.resolve()
	if err != nil {
		return err
	}
	if o.BinPath == "" {
		return fmt.Errorf("service install: BinPath is required")
	}
	if _, err := os.Stat(o.BinPath); err != nil {
		return fmt.Errorf("service install: binary %q not found: %v", o.BinPath, err)
	}

	pl := PlistFor(o.BinPath, o.IntervalSeconds, o.Apply)
	xml, err := RenderPlist(pl)
	if err != nil {
		return err
	}

	if err := os.MkdirAll(o.LaunchAgentsDir, 0o755); err != nil {
		return err
	}
	plistPath := AgentPlistPath(o.LaunchAgentsDir)
	if err := os.WriteFile(plistPath, []byte(xml), 0o644); err != nil {
		return err
	}

	_, err = o.Runner.Run("launchctl", "bootstrap", "gui/"+uid(), plistPath)
	return err
}

// Uninstall boots out the agent and removes its plist. It is not an error if the
// agent was never loaded.
func Uninstall(opts InstallOptions) error {
	o, err := opts.resolve()
	if err != nil {
		return err
	}
	plistPath := AgentPlistPath(o.LaunchAgentsDir)
	o.Runner.Run("launchctl", "bootout", "gui/"+uid(), plistPath) // best effort
	if err := os.Remove(plistPath); err != nil && !os.IsNotExist(err) {
		return err
	}
	return nil
}

// uid returns the current user id as a string, used for the gui/ target.
func uid() string {
	return fmt.Sprintf("%d", os.Getuid())
}
