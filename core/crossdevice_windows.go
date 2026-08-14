//go:build windows

package core

import "syscall"

// crossDeviceRenameErr 是 Windows 上 os.Rename 跨盘失败的真实错误
// ERROR_NOT_SAME_DEVICE（winerror.h 值 17）。Go 在 Windows 的 syscall.EXDEV
// 是发明常量（536871040），真实跨盘错误码与之永不相等，判断跨设备
// 必须用本常量而非 syscall.EXDEV（否则跨盘回退复制永不触发）。
var crossDeviceRenameErr error = syscall.Errno(17)
