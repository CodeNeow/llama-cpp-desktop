//go:build windows

package core

import (
	"errors"

	"golang.org/x/sys/windows"
)

// diskUsageForPath returns disk usage for the volume containing the given path:
// Path is the volume root (e.g. "C:\"), Used = Total - Free. Uses
// golang.org/x/sys/windows GetDiskFreeSpaceEx to read total and free bytes
// (TotalNumberOfFreeBytes is the current user's available space on Windows).
// Returns error on failure; caller sampleDiskUsage sets nil to avoid blocking
// other sampling metrics.
func diskUsageForPath(path string) (*DiskUsage, error) {
	var free, total, avail uint64
	if err := windows.GetDiskFreeSpaceEx(windows.StringToUTF16Ptr(path), &avail, &total, &free); err != nil {
		return nil, err
	}
	if total == 0 {
		return nil, errors.New("disk total bytes is zero")
	}
	used := uint64(0)
	if free <= total {
		used = total - free
	}
	return &DiskUsage{Path: path, Used: used, Total: total}, nil
}
