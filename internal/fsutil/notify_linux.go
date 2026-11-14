//go:build linux

package fsutil

import "os/exec"

// Notify sends a desktop notification via libnotify's notify-send. It is a
// no-op when the binary is not installed (headless hosts, minimal containers).
func Notify(title, message string) {
	if _, err := exec.LookPath("notify-send"); err != nil {
		return
	}
	exec.Command("notify-send", title, message).Run()
}
