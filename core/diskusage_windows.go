//go:build windows

package core

import (
	"errors"

	"golang.org/x/sys/windows"
)

// diskUsageForPath 返回指定路径所在磁盘卷的用量：Path 为传入的卷根路径
// （如 "C:\"），Used = Total - Free。通过 golang.org/x/sys/windows 的
// GetDiskFreeSpaceEx 读取总字节与空闲字节（Windows 上 TotalNumberOfFreeBytes
// 为当前用户可用的空闲空间）。采样失败返回错误，由调用方 sampleDiskUsage
// 置 nil，不阻断其他采样指标。
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
