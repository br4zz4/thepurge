package service

import (
	"fmt"
	"os"
	"os/exec"
)

// Runner executes shell commands (used for launchctl/systemctl). It is
// injectable so tests can record calls instead of touching the real service
// manager.
type Runner interface {
	Run(name string, args ...string) ([]byte, error)
}

// execRunner runs commands via os/exec — the production default.
type execRunner struct{}

func (execRunner) Run(name string, args ...string) ([]byte, error) {
	return exec.Command(name, args...).CombinedOutput()
}

// InstallOptions configures Install/Uninstall/Current. Zero values select the
// defaults documented on each field.
type InstallOptions struct {
	BinPath         string // absolute path to the thepurge binary.
	IntervalSeconds int    // interval between runs in seconds (default: 1800).
	Apply           bool   // if true, the scheduled run deletes for real.
	UnitDir         string // dir holding the unit definition (default: per platform).
	Runner          Runner // command runner (default: execRunner).
}

// resolve resolves defaults for an InstallOptions.
func (o InstallOptions) resolve() (InstallOptions, error) {
	if o.IntervalSeconds == 0 {
		o.IntervalSeconds = 1800
	}
	if o.UnitDir == "" {
		d, err := defaultUnitDir()
		if err != nil {
			return o, err
		}
		o.UnitDir = d
	}
	if o.Runner == nil {
		o.Runner = execRunner{}
	}
	return o, nil
}

// Install writes the platform service definition and loads it so
// `thepurge run` fires every IntervalSeconds. A previously loaded service is
// replaced.
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
	return installDefinition(o)
}

// Uninstall stops the service and removes its definition. It is not an error if
// the service was never installed.
func Uninstall(opts InstallOptions) error {
	o, err := opts.resolve()
	if err != nil {
		return err
	}
	return removeDefinition(o)
}

// Current inspects the definition on disk and asks the service manager for the
// live state. On Linux that is systemd --user; on macOS it is launchd.
func Current(opts InstallOptions) (Status, error) {
	o, err := opts.resolve()
	if err != nil {
		return Status{}, err
	}
	return currentStatus(o)
}
