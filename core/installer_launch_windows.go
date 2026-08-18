//go:build windows

package core

import "golang.org/x/sys/windows"

// launchInstaller starts the downloaded setup installer with elevation
// requested (UAC prompt). NSIS installers writing outside the user profile
// require administrator rights, and a plain fork/exec inherits the app's
// non-elevated token, failing with ERROR_ELEVATION_REQUIRED ("The requested
// operation requires elevation"). ShellExecute with the runas verb asks
// Windows to elevate; it returns immediately after spawning the elevated
// process, so the launch stays detached like the previous exec-based one.
func launchInstaller(path string) error {
	verb, err := windows.UTF16PtrFromString("runas")
	if err != nil {
		return err
	}
	file, err := windows.UTF16PtrFromString(path)
	if err != nil {
		return err
	}
	return windows.ShellExecute(0, verb, file, nil, nil, windows.SW_SHOWNORMAL)
}
