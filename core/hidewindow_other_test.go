//go:build !windows

package core

import (
	"os/exec"
	"testing"
)

// TestHideWindowOther verifies hideWindow is a no-op on non-Windows platforms
// and does not modify SysProcAttr.
func TestHideWindowOther(t *testing.T) {
	cmd := exec.Command("go", "version")
	hideWindow(cmd)
	if cmd.SysProcAttr != nil {
		t.Fatal("hideWindow must not set SysProcAttr on non-Windows platforms")
	}
}
