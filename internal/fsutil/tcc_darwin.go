//go:build darwin

package fsutil

import (
	"os"
	"path/filepath"
	"strings"
)

// tccProtectedDirs are macOS locations guarded by their own TCC service. Reading
// them raises a permission prompt that a disk cleaner has no use for, and macOS
// asks again every time the binary changes, so they are pruned from the walk.
// Media libraries and private databases only — Desktop, Documents and Downloads
// stay reachable because pointing a target there is an explicit choice.
var tccProtectedDirs = []string{
	"Music",
	"Movies",
	"Pictures",
	"Library/Accounts",
	"Library/Calendars",
	"Library/Messages",
	"Library/PersonalizationPortrait",
	"Library/Reminders",
	"Library/Safari",
	"Library/Application Support/AddressBook",
	"Library/Application Support/com.apple.Music",
	"Library/Application Support/com.apple.Photos",
	"Library/Caches/com.apple.AppleMediaServices",
	"Library/Caches/com.apple.Music",
	"Library/Caches/com.apple.TV",
	"Library/Caches/com.apple.podcasts",
	"Library/Group Containers/group.com.apple.calendar",
	"Library/Group Containers/group.com.apple.music",
	"Library/Group Containers/group.com.apple.photolibrary",
	"Library/Group Containers/group.com.apple.reminders",
}

// IsTCCProtected reports whether path is, or lives inside, a TCC-guarded
// location that eviction must not read. Always false off macOS.
func IsTCCProtected(path string) bool {
	home, err := os.UserHomeDir()
	if err != nil {
		return false
	}
	clean := filepath.Clean(path)
	for _, rel := range tccProtectedDirs {
		dir := filepath.Join(home, rel)
		if clean == dir || strings.HasPrefix(clean, dir+string(filepath.Separator)) {
			return true
		}
	}
	return false
}
