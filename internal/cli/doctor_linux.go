//go:build linux

package cli

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"
)

// checkHostAccess reports whether the systemd user session is reachable, which
// `thepurge service install` needs. Eviction itself works without it, so an
// unreachable session is reported but never fails the doctor.
func checkHostAccess(out io.Writer) bool {
	if os.Getenv("XDG_RUNTIME_DIR") == "" {
		fmt.Fprintln(out, "  systemd: no XDG_RUNTIME_DIR; user services unavailable (use a graphical login or loginctl enable-linger)")
		return true
	}
	state, err := exec.Command("systemctl", "--user", "is-system-running").Output()
	status := strings.TrimSpace(string(state))
	if err != nil && status == "" {
		fmt.Fprintln(out, "  systemd: user session not reachable; `thepurge service install` will fail")
		return true
	}
	fmt.Fprintf(out, "  systemd: user session %s\n", status)
	return true
}
