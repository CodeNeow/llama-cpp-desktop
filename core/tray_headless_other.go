//go:build !windows

package core

// InitHeadlessTray is a no-op stub on non-Windows platforms: headless
// (API-route) mode is Windows-only (ShouldRunHeadless always returns false
// elsewhere), so this never gets called; kept for cross-platform build
// signature consistency with the fyne implementation in
// tray_headless_windows.go. Tagged !windows (not the tray_other.go
// complement) because linux/darwin builds need this stub too — the fyne
// headless tray exists only on Windows.
func InitHeadlessTray(_ []byte, _, _ func()) {}
