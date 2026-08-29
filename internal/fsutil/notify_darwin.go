//go:build darwin

package fsutil

import (
	"os/exec"
	"strings"
)

// Notify sends a macOS notification using osascript.
func Notify(title, message string) {
	s := "display notification \"" + escapeForAppleScript(message) + "\"" +
		" with title \"" + escapeForAppleScript(title) + "\""
	exec.Command("osascript", "-e", s).Run()
}

func escapeForAppleScript(s string) string {
	var b strings.Builder
	for i := 0; i < len(s); i++ {
		switch s[i] {
		case 0x22:
			b.WriteString("\"")
		case 0x5c:
			b.WriteString("\\")
		default:
			b.WriteByte(s[i])
		}
	}
	return b.String()
}

