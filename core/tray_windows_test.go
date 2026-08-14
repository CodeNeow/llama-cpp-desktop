//go:build windows

package core

import "testing"

// TestTrayMenuLabels 验证托盘菜单标签纯函数 trayMenuLabels()：
//   - 返回两个非空标签（显示主窗口 / 退出）；
//   - zh 语言下返回中文，en 语言下返回英文（复用 i18n_test.go 的
//     setLanguageForTest 辅助函数，与 tr() 其余测试同一套互斥保护）。
func TestTrayMenuLabels(t *testing.T) {
	// 中文：返回「显示主窗口」与「退出」
	setLanguageForTest(t, "zh")
	show, quit := trayMenuLabels()
	if show != "显示主窗口" {
		t.Errorf("zh 显示标签应为 显示主窗口, 实际 %q", show)
	}
	if quit != "退出" {
		t.Errorf("zh 退出标签应为 退出, 实际 %q", quit)
	}
	if show == "" || quit == "" {
		t.Errorf("标签均不得为空, 实际 show=%q quit=%q", show, quit)
	}

	// 英文：返回 "Show Main Window" 与 "Quit"
	setLanguageForTest(t, "en")
	show, quit = trayMenuLabels()
	if show != "Show Main Window" {
		t.Errorf("en 显示标签应为 Show Main Window, 实际 %q", show)
	}
	if quit != "Quit" {
		t.Errorf("en 退出标签应为 Quit, 实际 %q", quit)
	}
}
