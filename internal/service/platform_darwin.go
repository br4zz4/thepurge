//go:build darwin

package service

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// backend names the macOS service manager.
const backend = "launchd"

// unitFileName is the launchd plist filename for the agent.
const unitFileName = Label + ".plist"

func defaultUnitDir() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, "Library", "LaunchAgents"), nil
}

func unitPath(dir string) string { return filepath.Join(dir, unitFileName) }

// uid returns the current user id as a string, used for the gui/ target.
func uid() string { return fmt.Sprintf("%d", os.Getuid()) }

func installDefinition(o InstallOptions) error {
	xml, err := RenderPlist(PlistFor(o.BinPath, o.IntervalSeconds, o.Apply))
	if err != nil {
		return err
	}
	if err := os.MkdirAll(o.UnitDir, 0o755); err != nil {
		return err
	}
	p := unitPath(o.UnitDir)
	if err := os.WriteFile(p, []byte(xml), 0o644); err != nil {
		return err
	}

	o.Runner.Run("launchctl", "bootout", "gui/"+uid()+"/"+Label) // best effort

	if out, err := o.Runner.Run("launchctl", "bootstrap", "gui/"+uid(), p); err != nil {
		return fmt.Errorf("service install: launchctl bootstrap: %v: %s", err, out)
	}
	return nil
}

func removeDefinition(o InstallOptions) error {
	p := unitPath(o.UnitDir)
	o.Runner.Run("launchctl", "bootout", "gui/"+uid(), p) // best effort
	if err := os.Remove(p); err != nil && !os.IsNotExist(err) {
		return err
	}
	return nil
}

func currentStatus(o InstallOptions) (Status, error) {
	st := Status{Backend: backend, UnitPath: unitPath(o.UnitDir)}
	if err := readUnit(&st, plistProgram); err != nil {
		return Status{}, err
	}
	st.Loaded, st.Running, st.PID, st.LastExit = launchdState(o.Runner)
	if st.Loaded {
		st.ProgramLoaded = launchdProgram(o.Runner)
	}
	return st, nil
}

// launchdState reads `launchctl list` and finds the label's row.
func launchdState(r Runner) (loaded, running bool, pid, lastExit int) {
	out, err := r.Run("launchctl", "list")
	if err != nil {
		return false, false, 0, 0
	}
	for _, line := range strings.Split(string(out), "\n") {
		fields := strings.Fields(line)
		if len(fields) != 3 || fields[2] != Label {
			continue
		}
		pid = atoi(fields[0])
		lastExit = atoi(fields[1])
		return true, pid > 0, pid, lastExit
	}
	return false, false, 0, 0
}

// launchdProgram reads the program path launchd holds for the label.
func launchdProgram(r Runner) string {
	out, err := r.Run("launchctl", "print", "gui/"+uid()+"/"+Label)
	if err != nil {
		return ""
	}
	for _, line := range strings.Split(string(out), "\n") {
		if p, ok := strings.CutPrefix(strings.TrimSpace(line), "program = "); ok {
			return strings.TrimSpace(p)
		}
	}
	return ""
}
