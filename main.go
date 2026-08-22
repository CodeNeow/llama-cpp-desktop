package main

import (
	"context"
	"embed"
	"flag"
	"os"

	"github.com/CodeNeow/llama-cpp-desktop/core"
	"github.com/wailsapp/wails/v2"
	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/options/assetserver"
)

//go:embed frontend/dist
var assets embed.FS

//go:embed build/windows/icon.ico
var trayIcon []byte

func main() {
	// Mode flags for API-route (headless) mode: --headless forces headless
	// (used by the GUI → headless relaunch), --gui forces the GUI even when
	// the persisted apiRouteMode preference requests headless (recovery
	// escape hatch). ContinueOnError keeps unknown arguments from third-party
	// launchers harmless.
	flags := flag.NewFlagSet("llama-desktop", flag.ContinueOnError)
	headless := flags.Bool("headless", false, "run in headless API-route mode (tray + llama-server only, no GUI)")
	gui := flags.Bool("gui", false, "force GUI mode even when apiRouteMode is enabled in the config")
	_ = flags.Parse(os.Args[1:])

	app := core.NewApp()

	// Tray icon is injected before Wails starts (needed because SetTrayEnabled
	// may start/stop the tray at runtime); main only declares the embed, and
	// the bytes are exposed to core via core.TrayIcon.
	core.TrayIcon = trayIcon

	// Headless branch: skip wails.Run entirely (no WebView2 process tree) —
	// core.RunHeadless loads the config, takes the single-instance mutex,
	// starts the headless tray and starts/adopts llama-server.
	if core.ShouldRunHeadless(*headless, *gui) {
		core.RunHeadless(trayIcon)
		return
	}

	// GUI mode takes the same single-instance mutex (fixes double-launch);
	// the mutex retry window also covers the headless → GUI handover.
	if !core.AcquireSingleInstance() {
		println("Llama Desktop is already running.")
		return
	}

	err := wails.Run(&options.App{
		Title:     "Llama Desktop",
		Width:     1200,
		Height:    800,
		MinWidth:  900,
		MinHeight: 800,
		AssetServer: &assetserver.Options{
			Assets: assets,
		},
		Frameless: true,
		OnStartup: func(ctx context.Context) {
			app.Startup(ctx)
			// Start system tray based on persisted config (unconditionally since
			// 4aacac2; user can disable in settings). loadConfig defaults missing
			// legacy fields to true. When enabled, the app starts with a tray
			// icon; disabling the setting requires an app restart.
			if core.TrayEnabled() {
				core.InitTray(ctx, trayIcon)
			}
		},
		OnShutdown: func(ctx context.Context) {
			// Remove tray icon first, then clean up the app (stop server /
			// persist config, etc.)
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
