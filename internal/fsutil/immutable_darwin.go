//go:build darwin

package fsutil

import (
	"golang.org/x/sys/unix"
)

// immutableFlags are the file flags that mark a path as undeletable/unmodifiable
// by the owner. UF_APPEND is included because append-only files refuse deletion
// as well.
const immutableFlags = unix.UF_IMMUTABLE | unix.SF_IMMUTABLE | unix.UF_APPEND | unix.SF_APPEND

// IsImmutable reports whether path carries immutable/append-only flags that make
// deleting it unsafe (it can break the app or the disk). Missing paths and stat
// errors are reported as non-immutable so callers can log and continue.
func IsImmutable(path string) bool {
	var st unix.Stat_t
	if err := unix.Lstat(path, &st); err != nil {
		return false
	}
	return st.Flags&immutableFlags != 0
}
