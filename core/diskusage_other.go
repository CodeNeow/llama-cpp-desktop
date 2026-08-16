//go:build !windows

package core

import (
	"errors"
	"syscall"
)

// diskUsageForPath returns filesystem usage for the given path on non-Windows
// platforms: uses syscall.Statfs to read block info, Free uses Bavail (blocks
// actually available to the normal user, excluding root reserved blocks),
// Used = Total - Free. Path is returned as-is. Returns error on failure;
// caller sampleDiskUsage sets nil to avoid blocking other sampling metrics.
func diskUsageForPath(path string) (*DiskUsage, error) {
	var st syscall.Statfs_t
	if err := syscall.Statfs(path, &st); err != nil {
		return nil, err
	}
	total := st.Blocks * uint64(st.Bsize)
	if total == 0 {
		return nil, errors.New("disk total blocks is zero")
	}
	free := uint64(st.Bavail) * uint64(st.Bsize)
	used := uint64(0)
	if free <= total {
		used = total - free
	}
	return &DiskUsage{Path: path, Used: used, Total: total}, nil
}
