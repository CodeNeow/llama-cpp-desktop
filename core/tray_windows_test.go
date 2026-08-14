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

// TestQuitTrayIdempotentWhenNotStarted 验证托盘未启动时 QuitTray 幂等返回：
// 直接走 trayStarted=false 分支、不触发 systray.Quit，且不会 panic。这是
// 动态启停（SetTrayEnabled 禁用路径）依赖的纯逻辑前提——重复禁用不得重复
// 摘除。不真正启动 systray（Run 会拉起 GetMessage 消息循环，测试进程无窗口
// 环境不可靠），因此只断言未启动分支的幂等行为。
//
// 关于「复位」逻辑：第 0 步源码结论（fyne.io/systray v1.12.2）确认同进程
// 内 systray.Quit() 结束后无法再次 Run（包级 quitOnce sync.Once 阻断、窗口
// 类注册与 initialized 状态不复位），故本项目未实现 QuitTray 后复位 trayStarted
// 的动态重启，禁用托盘为一次性操作——无可测的二次启动纯逻辑，仅保留上述幂等
// 断言。
func TestQuitTrayIdempotentWhenNotStarted(t *testing.T) {
	// 记录并恢复全局 trayStarted（包级状态，避免污染其他测试）
	trayMu.Lock()
	orig := trayStarted
	trayStarted = false
	trayMu.Unlock()
	defer func() {
		trayMu.Lock()
		trayStarted = orig
		trayMu.Unlock()
	}()

	// 未启动时调用：幂等返回、不 panic
	QuitTray()
	// 状态仍应为未启动（QuitTray 不应改写 trayStarted——它只发退出信号）
	trayMu.Lock()
	still := trayStarted
	trayMu.Unlock()
	if still {
		t.Error("QuitTray 不应把 trayStarted 置为 true")
	}
}
