package main

import (
	"embed"

	"llama-gui/core"

	"github.com/wailsapp/wails/v2"
	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/options/assetserver"
)

//go:embed frontend/dist
var assets embed.FS

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
		Frameless:        true,
		OnStartup:        app.Startup,
		OnShutdown:       app.Shutdown,
		Bind:             []interface{}{app},
		BackgroundColour: &options.RGBA{R: 248, G: 250, B: 252, A: 1},
	})

	if err != nil {
		println("Error:", err.Error())
	}
}
