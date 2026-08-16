//go:build windows

package core

import (
	"syscall"
	"unsafe"
)

// readSystemLocale reads the Windows user default locale (e.g. "zh-CN" /
// "en-US"). golang.org/x/sys/windows does not export GetUserDefaultLocaleName
// (up to v0.47), so this calls kernel32.GetUserDefaultLocaleName directly via
// the standard library syscall: the API writes the locale name (e.g. "zh-CN")
// into the provided UTF-16 buffer and returns its length, or 0 on failure.
// Returns empty string on failure; detectSystemLanguage falls back to "en".
func readSystemLocale() string {
	proc := syscall.NewLazyDLL("kernel32.dll").NewProc("GetUserDefaultLocaleName")
	// LOCALE_NAME_MAX_LENGTH is 85 (including null terminator); 64 is enough
	// for common locale names.
	buf := make([]uint16, 64)
	r, _, _ := proc.Call(uintptr(unsafe.Pointer(&buf[0])), uintptr(len(buf)))
	if r == 0 {
		return ""
	}
	return syscall.UTF16ToString(buf)
}
