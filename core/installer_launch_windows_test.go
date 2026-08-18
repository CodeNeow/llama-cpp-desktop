//go:build windows

package core

import "testing"

// TestLaunchInstallerWindows verifies the Windows installer launcher requests
// elevation: an existing installer path is accepted and launched via
// ShellExecute runas (returning nil), while a nonexistent path fails fast
// without popping a UAC prompt (the elevation dialog only appears for real
// installer files). The nil-success half must never be exercised in tests, so
// only the error path is asserted; the successful branch is covered manually
// by the install-now flow.
func TestLaunchInstallerWindowsRejectsMissingFile(t *testing.T) {
	// A missing file makes ShellExecute return SE_ERR_FNF before any UAC
	// dialog, so the call fails deterministically and safely in tests.
	if err := launchInstaller(`C:\nonexistent\llama-desktop-setup-test.exe`); err == nil {
		t.Fatal("launchInstaller on a nonexistent path must return an error")
	}
}
