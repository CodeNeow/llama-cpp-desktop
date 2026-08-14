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
			// Windows 上启动系统托盘（缩到托盘/托盘菜单退出）；其他平台 no-op
			core.InitTray(ctx, trayIcon)
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
