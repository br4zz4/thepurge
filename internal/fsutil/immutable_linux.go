//go:build linux

package fsutil

import (
	"os"

	"golang.org/x/sys/unix"
)

// Linux inode flags that make a file undeletable or unsafe to truncate.
const (
	fsImmutableFlag = 0x00000010 // FS_IMMUTABLE_FL
	fsAppendFlag    = 0x00000020 // FS_APPEND_FL
)

// IsImmutable reports whether path carries the immutable or append-only inode
// flag (set with chattr +i / +a). Reading the flags needs a real file
// descriptor, so unreadable paths are reported as mutable.
func IsImmutable(path string) bool {
	f, err := os.Open(path)
	if err != nil {
		return false
	}
	defer f.Close()

	flags, err := unix.IoctlGetInt(int(f.Fd()), unix.FS_IOC_GETFLAGS)
	if err != nil {
		return false
	}
	return flags&(fsImmutableFlag|fsAppendFlag) != 0
}
