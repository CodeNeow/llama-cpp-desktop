//go:build (!windows && !linux && !darwin) || android || ios

package core

import "context"

// This file is the exact build-tag complement of core/tray.go (the desktop
// GUI tray): it covers mobile (android / ios — where the GOOS linux/darwin
// tags would otherwise also match) and every other non-desktop platform, so
// the tray symbols below always exist exactly once.

// TrayIcon holds the tray icon bytes; unused on non-desktop platforms, kept so
// main.go compiles on all platforms (main.go assigns it unconditionally before
// the application runs).
var TrayIcon []byte

// InitTray is a no-op stub on non-desktop platforms: this function exists only
// to keep cross-platform build signatures consistent.
func InitTray(_ context.Context, _ []byte) {}

// QuitTray is a no-op stub on non-desktop platforms.
func QuitTray() {}

// InitHeadlessTray is a no-op stub on non-desktop platforms: headless
// (API-route) mode is Windows-only (ShouldRunHeadless always returns false
// elsewhere), so this never gets called; kept for cross-platform build
// signature consistency.
func InitHeadlessTray(_ []byte, _, _ func()) {}
