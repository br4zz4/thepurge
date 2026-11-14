//go:build linux

package service

import (
	"fmt"
	"os"
	"path/filepath"
)

// backend names the Linux service manager.
const backend = "systemd"

// unitName is the systemd unit basename; the timer drives the oneshot service.
const unitName = "thepurge"

const serviceTemplate = `[Unit]
Description=thepurge eviction pass
Documentation=https://github.com/br4zz4/thepurge

[Service]
Type=oneshot
ExecStart=%s run%s
`

const timerTemplate = `[Unit]
Description=thepurge periodic eviction

[Timer]
OnBootSec=2min
OnUnitActiveSec=%ds
Persistent=true

[Install]
WantedBy=timers.target
`

func defaultUnitDir() (string, error) {
	dir, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "systemd", "user"), nil
}

func servicePath(dir string) string { return filepath.Join(dir, unitName+".service") }
func timerPath(dir string) string   { return filepath.Join(dir, unitName+".timer") }

func installDefinition(o InstallOptions) error {
	if err := os.MkdirAll(o.UnitDir, 0o755); err != nil {
		return err
	}

	apply := ""
	if o.Apply {
		apply = " --apply"
	}
	service := fmt.Sprintf(serviceTemplate, o.BinPath, apply)
	timer := fmt.Sprintf(timerTemplate, o.IntervalSeconds)

	if err := os.WriteFile(servicePath(o.UnitDir), []byte(service), 0o644); err != nil {
		return err
	}
	if err := os.WriteFile(timerPath(o.UnitDir), []byte(timer), 0o644); err != nil {
		return err
	}

	if out, err := o.Runner.Run("systemctl", "--user", "daemon-reload"); err != nil {
		return fmt.Errorf("service install: systemctl daemon-reload: %v: %s", err, out)
	}
	if out, err := o.Runner.Run("systemctl", "--user", "enable", "--now", unitName+".timer"); err != nil {
		return fmt.Errorf("service install: systemctl enable: %v: %s", err, out)
	}
	return nil
}

func removeDefinition(o InstallOptions) error {
	o.Runner.Run("systemctl", "--user", "disable", "--now", unitName+".timer") // best effort

	for _, p := range []string{timerPath(o.UnitDir), servicePath(o.UnitDir)} {
		if err := os.Remove(p); err != nil && !os.IsNotExist(err) {
			return err
		}
	}
	o.Runner.Run("systemctl", "--user", "daemon-reload") // best effort
	return nil
}

func currentStatus(o InstallOptions) (Status, error) {
	st := Status{Backend: backend, UnitPath: servicePath(o.UnitDir)}
	if err := readUnit(&st, execStartPath); err != nil {
		return Status{}, err
	}

	if st.Installed {
		if _, err := os.Stat(timerPath(o.UnitDir)); err != nil {
			if !os.IsNotExist(err) {
				return Status{}, fmt.Errorf("service status: %v", err)
			}
			st.Installed = false
		}
	}

	st.Loaded, st.Running, st.PID, st.LastExit, st.ProgramLoaded = systemdState(o.Runner)
	return st, nil
}

// systemdState reads the timer activation and the service's last run.
func systemdState(r Runner) (loaded, running bool, pid, lastExit int, program string) {
	if out, err := r.Run("systemctl", "--user", "show", unitName+".timer", "-p", "ActiveState"); err == nil {
		loaded = property(string(out), "ActiveState") == "active"
	}
	out, err := r.Run("systemctl", "--user", "show", unitName+".service", "-p", "MainPID", "-p", "ExecMainStatus", "-p", "ExecStart")
	if err != nil {
		return loaded, false, 0, 0, ""
	}
	pid = atoi(property(string(out), "MainPID"))
	lastExit = atoi(property(string(out), "ExecMainStatus"))
	return loaded, pid > 0, pid, lastExit, execStartProperty(string(out))
}
