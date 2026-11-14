package service

import (
	"os"
	"strconv"
	"strings"
)

// Status is the launchd agent state as seen from disk and from launchd itself.
type Status struct {
	PlistPath     string // path of the plist on disk
	Installed     bool   // the plist exists
	PlistProgram  string // binary the plist would launch
	ProgramExists bool   // PlistProgram exists on disk
	Loaded        bool   // the job is registered in the gui domain
	Running       bool   // the job has a live process
	PID           int
	LastExit      int    // last exit status reported by launchd
	Program       string // binary launchd has loaded (may lag behind the plist)
}

// Mismatch reports whether launchd holds a stale program path for the label.
// That happens when the plist was rewritten without a bootout, and the loaded
// job keeps running the old binary.
func (s Status) Mismatch() bool {
	return s.Loaded && s.Program != "" && s.PlistProgram != "" && s.Program != s.PlistProgram
}

// Current inspects the plist and asks launchd for the live job state.
func Current(opts InstallOptions) (Status, error) {
	o, err := opts.resolve()
	if err != nil {
		return Status{}, err
	}

	st := Status{PlistPath: AgentPlistPath(o.LaunchAgentsDir)}
	if data, err := os.ReadFile(st.PlistPath); err == nil {
		st.Installed = true
		st.PlistProgram = plistProgram(string(data))
		if st.PlistProgram != "" {
			_, err := os.Stat(st.PlistProgram)
			st.ProgramExists = err == nil
		}
	}

	st.Loaded, st.Running, st.PID, st.LastExit = loadedState(o.Runner)
	if st.Loaded {
		st.Program = loadedProgram(o.Runner)
	}
	return st, nil
}

// loadedState reads `launchctl list` and finds the label's row.
func loadedState(r Runner) (loaded, running bool, pid, lastExit int) {
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

// loadedProgram reads the program path launchd has for the label.
func loadedProgram(r Runner) string {
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
	return strings.TrimSpace(program)
}

func atoi(s string) int {
	n, err := strconv.Atoi(s)
	if err != nil {
		return 0
	}
	return n
}
