package main

import (
	"context"
	"embed"

	"llama-gui/core"

	"github.com/wailsapp/wails/v2"
	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/options/assetserver"
)

//go:embed frontend/dist
var assets embed.FS

//go:embed build/windows/icon.ico
var trayIcon []byte

func main() {
	app := core.NewApp()

	// 托盘图标在 Wails 启动前注入（SetTrayEnabled 运行时按配置启停托盘时需要），
	// main 包仅声明 embed，图标字节经 core.TrayIcon 供 core 包使用。
	core.TrayIcon = trayIcon

	err := wails.Run(&options.App{
		Title:     "Llama GUI",
		Width:     1200,
		Height:    800,
		MinWidth:  900,
		MinHeight: 600,
		AssetServer: &assetserver.Options{
			Assets: assets,
		},
		Frameless: true,
		OnStartup: func(ctx context.Context) {
			app.Startup(ctx)
			// 按持久化配置决定是否启动系统托盘（4aacac2 起为无条件启动；
			// 设置页可关闭）。loadConfig 已把旧配置缺字段兜底为 true。
			// 默认 true 时应用启动即带托盘，用户关闭设置项需重启应用生效。
			if core.TrayEnabled() {
				core.InitTray(ctx, trayIcon)
			}
		},
		OnShutdown: func(ctx context.Context) {
			// 先摘托盘图标，再做应用清理（停止服务/持久化配置等）
			core.QuitTray()
			app.Shutdown(ctx)
		},
		Bind:             []interface{}{app},
		BackgroundColour: &options.RGBA{R: 248, G: 250, B: 252, A: 1},
	})

	if err != nil {
		println("Error:", err.Error())
	}
}
