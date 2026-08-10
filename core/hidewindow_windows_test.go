//go:build windows

package core

import (
	"os/exec"
	"testing"
)

// TestHideWindowWindows 验证 Windows 下 hideWindow 设置 HideWindow 标志，
// 防止 GUI 程序启动子进程时闪现控制台窗口。
func TestHideWindowWindows(t *testing.T) {
	cmd := exec.Command("go", "version")
	hideWindow(cmd)
	if cmd.SysProcAttr == nil || !cmd.SysProcAttr.HideWindow {
		t.Fatal("Windows 下 hideWindow 未设置 SysProcAttr.HideWindow")
	}
}
