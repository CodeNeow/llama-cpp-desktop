//go:build (!windows && !linux && !darwin) || android || ios

package core

import "context"

// This file is the exact build-tag complement of core/tray.go (the desktop
// GUI tray): it covers mobile (android / ios — where the GOOS linux/darwin
// tags would otherwise also match) and every other non-desktop platform, so
// the GUI tray symbols below always exist exactly once. The headless tray
// stub lives in tray_headless_other.go (tag !windows), whose coverage spans
// this file's platforms AND linux/darwin — so it must not repeat here.

// TrayIcon holds the tray icon bytes; unused on non-desktop platforms, kept so
// main.go compiles on all platforms (main.go assigns it unconditionally before
// the application runs).
var TrayIcon []byte

// InitTray is a no-op stub on non-desktop platforms: this function exists only
// to keep cross-platform build signatures consistent.
func InitTray(_ context.Context, _ []byte) {}

// QuitTray is a no-op stub on non-desktop platforms.
func QuitTray() {}
