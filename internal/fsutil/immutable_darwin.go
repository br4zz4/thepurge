//go:build darwin

package fsutil

import (
	"bytes"
	"golang.org/x/sys/unix"
)

// immutableFlags are the file flags that mark a path as undeletable/unmodifiable
// by the owner.
const immutableFlags = unix.UF_IMMUTABLE | unix.SF_IMMUTABLE | unix.UF_APPEND | unix.SF_APPEND

// appleImmutableXattrs lists com.apple.* xattrs that may indicate system-managed
// or undeletable files (e.g. com.apple.decmpfs for compression stubs,
// com.apple.rootless for SIP-protected, etc.).
var appleImmutableXattrs = [][]byte{
	[]byte("com.apple.decmpfs"),
	[]byte("com.apple.rootless"),
	[]byte("com.apple.system.Security"),
	[]byte("com.apple.system.recovery"),
}

// IsImmutable reports whether path carries immutable/append-only flags or
// relevant com.apple.* extended attributes that make deleting it unsafe.
func IsImmutable(path string) bool {
	var st unix.Stat_t
	if err := unix.Lstat(path, &st); err != nil {
		return false
	}
	if st.Flags&immutableFlags != 0 {
		return true
	}
	return hasAppleImmutableXattr(path)
}

// hasAppleImmutableXattr checks for specific com.apple.* xattrs that indicate
// immutability.
func hasAppleImmutableXattr(path string) bool {
	sz, err := unix.Listxattr(path, nil)
	if err != nil || sz <= 0 {
		return false
	}
	buf := make([]byte, sz)
	sz, err = unix.Listxattr(path, buf)
	if err != nil {
		return false
	}
	for _, maybe := range appleImmutableXattrs {
		if bytes.Contains(buf[:sz], maybe) {
			return true
		}
	}
	return false
}
