package service

import (
	"fmt"
	"html"
	"os"
	"strconv"
	"strings"
)

// Status is the service state as seen from the unit definition on disk and from
// the service manager itself. Backend is "launchd" on macOS and "systemd" on
// Linux; the rest of the fields mean the same thing on both.
type Status struct {
	Backend       string // service manager that owns the definition
	UnitPath      string // plist or systemd unit path on disk
	Installed     bool   // the definition exists
	ProgramWanted string // binary the definition would launch
	ProgramExists bool   // ProgramWanted exists on disk
	Loaded        bool   // the manager has the job registered
	Running       bool   // a process is currently alive
	PID           int
	LastExit      int    // last exit status reported by the manager
	ProgramLoaded string // binary the manager holds (may lag behind the definition)
}

// Mismatch reports whether the manager holds a stale program path. That happens
// when the definition was rewritten without a reload, and the loaded job keeps
// running the old binary.
func (s Status) Mismatch() bool {
	return s.Loaded && s.ProgramLoaded != "" && s.ProgramWanted != "" && s.ProgramLoaded != s.ProgramWanted
}

// readUnit fills Installed/ProgramWanted/ProgramExists from the definition on
// disk, using parse to extract the program path.
func readUnit(st *Status, parse func(string) string) error {
	data, err := os.ReadFile(st.UnitPath)
	switch {
	case err == nil:
		st.Installed = true
		st.ProgramWanted = parse(string(data))
		if st.ProgramWanted != "" {
			if _, err := os.Stat(st.ProgramWanted); err == nil {
				st.ProgramExists = true
			} else if !os.IsNotExist(err) {
				return fmt.Errorf("service status: %v", err)
			}
		}
	case !os.IsNotExist(err):
		return fmt.Errorf("service status: read %s: %v", st.UnitPath, err)
	}
	return nil
}

// plistProgram extracts the first ProgramArguments entry from a rendered plist.
func plistProgram(xml string) string {
	_, after, ok := strings.Cut(xml, "<key>ProgramArguments</key>")
	if !ok {
		return ""
	}
	_, after, ok = strings.Cut(after, "<string>")
	if !ok {
		return ""
	}
	program, _, ok := strings.Cut(after, "</string>")
	if !ok {
		return ""
	}
	return html.UnescapeString(strings.TrimSpace(program))
}

// execStartPath extracts the binary from a systemd unit's ExecStart line,
// dropping the optional service-manager prefixes (-, @, +, !).
func execStartPath(unit string) string {
	for _, line := range strings.Split(unit, "\n") {
		rest, ok := strings.CutPrefix(strings.TrimSpace(line), "ExecStart=")
		if !ok {
			continue
		}
		fields := strings.Fields(rest)
		if len(fields) == 0 {
			return ""
		}
		return strings.TrimLeft(fields[0], "-@+!")
	}
	return ""
}

// execStartProperty extracts path= from `systemctl show -p ExecStart` output,
// which renders as `ExecStart={ path=/bin/x ; argv[]=/bin/x run ; ... }`.
func execStartProperty(out string) string {
	_, after, ok := strings.Cut(out, "path=")
	if !ok {
		return ""
	}
	if i := strings.IndexAny(after, "; \n"); i >= 0 {
		after = after[:i]
	}
	return strings.TrimSpace(after)
}

// property reads a `Key=value` line from `systemctl show` output.
func property(out, key string) string {
	for _, line := range strings.Split(out, "\n") {
		if v, ok := strings.CutPrefix(strings.TrimSpace(line), key+"="); ok {
			return strings.TrimSpace(v)
		}
	}
	return ""
}

func atoi(s string) int {
	n, err := strconv.Atoi(s)
	if err != nil {
		return 0
	}
	return n
}
