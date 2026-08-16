//go:build windows

package core

import (
	"os/exec"
	"testing"
)

// TestHideWindowWindows verifies hideWindow sets the HideWindow flag on Windows,
// preventing console windows from flashing when a GUI app spawns child processes.
func TestHideWindowWindows(t *testing.T) {
	cmd := exec.Command("go", "version")
	hideWindow(cmd)
	if cmd.SysProcAttr == nil || !cmd.SysProcAttr.HideWindow {
		t.Fatal("hideWindow did not set SysProcAttr.HideWindow on Windows")
	}
}
