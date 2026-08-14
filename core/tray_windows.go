//go:build windows

package core

import (
	"context"
	"log"
	"sync"

	"fyne.io/systray"
	wailsRuntime "github.com/wailsapp/wails/v2/pkg/runtime"
)

// trayMu 保护托盘启动状态的并发读写（项目显式互斥惯例：configMu /
// serverLogsMu 等同风格），保证 InitTray / QuitTray 幂等且并发安全。
var trayMu sync.Mutex

// trayStarted 标记托盘是否已启动，未启动时 QuitTray 直接返回（幂等）。
var trayStarted bool

// trayMenuLabels 返回托盘菜单两个条目的标签（显示主窗口 / 退出），
// 内部用 tr() 按当前生效语言（zh/en，auto 按系统检测）生成。
// 纯函数，供 core/tray_windows_test.go 直接断言中英文结果。
func trayMenuLabels() (show, quit string) {
	return tr("显示主窗口", "Show Main Window"), tr("退出", "Quit")
}

// InitTray 在独立 goroutine 中启动系统托盘（Windows 专用）：设置托盘图标
// 与 tooltip（"Llama GUI"），菜单为「显示主窗口」与「退出」两项（中间加
// 分隔线）。菜单点击回调使用传入的 ctx 调用 Wails runtime 操作窗口：
// 显示主窗口时同时恢复最小化状态，退出时真正结束应用（runtime.Quit 会
// 触发 Wails OnShutdown → app.Shutdown 完成清理）。ctx 来源与 core/app.go
// 中 App.Startup 的 ctx 一致（Wails OnStartup 注入）。
//
// systray.Run 在 Windows 上自带 GetMessage 消息循环，可从非主 goroutine
// 调用，与 Wails 主线程消息循环共存（fyne.io/systray 官方 README 亦声明
// 方法可从任意 goroutine 调用）。已启动时重复调用直接返回，幂等。
func InitTray(ctx context.Context, icon []byte) {
	trayMu.Lock()
	if trayStarted {
		trayMu.Unlock()
		return
	}
	trayStarted = true
	trayMu.Unlock()

	go func() {
		systray.Run(func() {
			systray.SetIcon(icon)
			systray.SetTooltip("Llama GUI")

			showLabel, quitLabel := trayMenuLabels()
			showItem := systray.AddMenuItem(showLabel, showLabel)
			systray.AddSeparator()
			quitItem := systray.AddMenuItem(quitLabel, quitLabel)

			go func() {
				for range showItem.ClickedCh {
					// 窗口最小化时 WindowShow 不一定恢复，需同时取消最小化
					wailsRuntime.WindowShow(ctx)
					wailsRuntime.WindowUnminimise(ctx)
				}
			}()
			go func() {
				for range quitItem.ClickedCh {
					wailsRuntime.Quit(ctx)
				}
			}()
		}, func() {
			// 托盘已退出（退出菜单或 QuitTray 触发），无需额外清理。
		})
		log.Println("[INFO] systray exited")
	}()
}

// QuitTray 结束系统托盘并移除托盘图标；幂等（未初始化时调用不 panic），
// 并发安全（trayMu 保护）。供 main.go 的 OnShutdown 在 app.Shutdown 之前
// 调用，先摘托盘再做应用清理。
func QuitTray() {
	trayMu.Lock()
	started := trayStarted
	trayMu.Unlock()
	if !started {
		return
	}
	systray.Quit()
}
