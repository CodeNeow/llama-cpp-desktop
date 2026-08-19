//go:build !windows

package core

import "context"

// TrayIcon holds the tray icon bytes; unused on non-Windows platforms, kept so
// main.go compiles on all platforms (main.go assigns it unconditionally before
// wails.Run).
var TrayIcon []byte

// InitTray is a no-op stub on non-Windows platforms: macOS / Linux have no
// system tray requirement (the close button exits the app directly); this
// function exists only to keep cross-platform build signatures consistent.
func InitTray(_ context.Context, _ []byte) {}

// QuitTray is a no-op stub on non-Windows platforms.
func QuitTray() {}

// InitHeadlessTray is a no-op stub on non-Windows platforms: headless
// (API-route) mode is Windows-only (ShouldRunHeadless always returns false
// elsewhere), so this never gets called; kept for cross-platform build
// signature consistency.
func InitHeadlessTray(_ []byte, _, _ func()) {}
