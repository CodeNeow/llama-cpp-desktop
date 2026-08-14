//go:build !windows

package core

import "syscall"

// crossDeviceRenameErr 是非 Windows 平台 os.Rename 跨设备（跨挂载点）
// 失败的真实错误 EXDEV，供 moveFile 判断是否回退为复制 + 删除。
var crossDeviceRenameErr error = syscall.EXDEV
