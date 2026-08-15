//go:build !windows

package core

import (
	"errors"
	"syscall"
)

// diskUsageForPath 返回指定路径所在文件系统的用量：非 Windows 平台使用
// syscall.Statfs 读取块信息，Free 取 Bavail（普通用户实际可用的块数，排除
// root 保留块），Used = Total - Free。Path 原样返回传入路径。采样失败返回
// 错误，由调用方 sampleDiskUsage 置 nil，不阻断其他采样指标。
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
