//go:build !windows

package core

import (
	"os/exec"
	"testing"
)

// TestHideWindowOther 验证非 Windows 平台 hideWindow 为 no-op，不修改 SysProcAttr。
func TestHideWindowOther(t *testing.T) {
	cmd := exec.Command("go", "version")
	hideWindow(cmd)
	if cmd.SysProcAttr != nil {
		t.Fatal("非 Windows 平台 hideWindow 不应设置 SysProcAttr")
	}
}
