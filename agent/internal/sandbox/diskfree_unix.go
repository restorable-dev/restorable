//go:build darwin || linux

package sandbox

import "syscall"

// freeBytes reports the disk space available to unprivileged users on the
// filesystem containing path.
func freeBytes(path string) (uint64, error) {
	var st syscall.Statfs_t
	if err := syscall.Statfs(path, &st); err != nil {
		return 0, err
	}
	// Bsize is uint32 on darwin and int64 on linux; the conversion is
	// required on at least one of the two.
	return st.Bavail * uint64(st.Bsize), nil
}
