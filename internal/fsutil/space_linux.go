//go:build linux

package fsutil

import "golang.org/x/sys/unix"

// DiskUsage returns the free and total bytes available on the filesystem
// containing path. free is f_bavail (space a non-root user can actually claim);
// total is the full volume capacity.
func DiskUsage(path string) (free, total int64, err error) {
	var st unix.Statfs_t
	if err := unix.Statfs(path, &st); err != nil {
		return 0, 0, err
	}
	bsize := int64(st.Bsize)
	return int64(st.Bavail) * bsize, int64(st.Blocks) * bsize, nil
}

// FreeSpace returns the number of bytes available to a non-root user on the
// filesystem containing path. It uses f_bavail (not f_bfree) so the value is
// usable without root privileges, matching what the daemon can actually claim.
func FreeSpace(path string) (int64, error) {
	free, _, err := DiskUsage(path)
	return free, err
}
