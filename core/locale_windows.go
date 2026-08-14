//go:build windows

package core

import (
	"syscall"
	"unsafe"
)

// readSystemLocale 读取 Windows 用户默认区域设置（如 "zh-CN" / "en-US"）。
// golang.org/x/sys/windows 未导出 GetUserDefaultLocaleName（截至 v0.47 全版本
// 均无），这里直接经标准库 syscall 调用 kernel32.GetUserDefaultLocaleName：
// 该 API 将区域名（如 "zh-CN"）写入传入的 UTF-16 缓冲并返回长度，失败返回 0。
// 调用失败返回空串，由 detectSystemLanguage 兜底 en。
func readSystemLocale() string {
	proc := syscall.NewLazyDLL("kernel32.dll").NewProc("GetUserDefaultLocaleName")
	// LOCALE_NAME_MAX_LENGTH 为 85（含结尾空字符），64 足够容纳常见区域名。
	buf := make([]uint16, 64)
	r, _, _ := proc.Call(uintptr(unsafe.Pointer(&buf[0])), uintptr(len(buf)))
	if r == 0 {
		return ""
	}
	return syscall.UTF16ToString(buf)
}
