//go:build windows

package core

import (
	"context"
	"log"
	"sync"

	"fyne.io/systray"
	wailsRuntime "github.com/wailsapp/wails/v2/pkg/runtime"
)

// TrayIcon holds the tray icon bytes (injected by main.go after embedding
// build/windows/icon.ico). Used by core when starting/stopping the tray at
// startup or via SetTrayEnabled (the latter cannot access main's embed at
// runtime).
var TrayIcon []byte

// trayMu guards concurrent access to tray startup state, following the
// project's explicit mutex convention (configMu / serverLogsMu / etc.) so
// InitTray / QuitTray are idempotent and concurrency-safe.
var trayMu sync.Mutex

// trayStarted marks whether the tray has been started; QuitTray returns
// immediately when not started (idempotent).
//
// Note: fyne.io/systray has a package-level quitOnce (sync.Once, see
// systray.go) that guarantees systray.Quit takes effect only once, and its
// internal window class / message loop state does not reset after Run exits —
// once Run returns in a process, it cannot be called again. Therefore:
//   - startup paths (OnStartup or SetTrayEnabled(true)) may only call
//     InitTray once; repeated calls after startup return immediately;
//   - QuitTray (OnShutdown or SetTrayEnabled(false)) removes the icon, after
//     which the tray cannot be restarted in this process (see trayStarted
//     comment and systray's quitOnce source). Re-enabling requires an app
//     restart.
var trayStarted bool

// trayMenuLabels returns the two tray menu item labels (show main window /
// quit), generated via tr() in the active language (zh/en, auto follows
// system). Pure function; core/tray_windows_test.go asserts the bilingual
// results directly.
func trayMenuLabels() (show, quit string) {
	return tr("显示主窗口", "Show Main Window"), tr("退出", "Quit")
}

// InitTray starts the system tray in a dedicated goroutine (Windows only): sets
// the tray icon and tooltip ("Llama Desktop"), with menu items "Show Main
// Window" and "Quit" (separator in between). Menu click callbacks use the
// passed ctx to drive Wails runtime window operations: showing the main window
// also restores it from minimized state; quit ends the app for real
// (runtime.Quit triggers Wails OnShutdown → app.Shutdown cleanup). The ctx
// source matches App.Startup's ctx in core/app.go (injected by Wails
// OnStartup).
//
// systray.Run on Windows carries its own GetMessage loop and can be called
// from a non-main goroutine alongside the Wails main-thread message loop
// (the fyne.io/systray README also states methods may be called from any
// goroutine). Repeated calls after startup return immediately (idempotent).
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
			systray.SetTooltip("Llama Desktop")

			showLabel, quitLabel := trayMenuLabels()
			showItem := systray.AddMenuItem(showLabel, showLabel)
			systray.AddSeparator()
			quitItem := systray.AddMenuItem(quitLabel, quitLabel)

			go func() {
				for range showItem.ClickedCh {
					// WindowShow may not restore when minimized; also unminimize.
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
			// Tray has exited (quit menu or QuitTray triggered); no extra
			// cleanup needed.
		})
		log.Println("[INFO] systray exited")
	}()
}

// QuitTray stops the system tray and removes the tray icon; idempotent (no-op
// when not initialized), concurrency-safe (trayMu guards). Called by
// main.go OnShutdown (remove tray before app cleanup) and
// SetTrayEnabled(false) (disable tray). After removal, systray quitOnce
// prevents restarting the tray in this process (see trayStarted comment).
func QuitTray() {
	trayMu.Lock()
	started := trayStarted
	trayMu.Unlock()
	if !started {
		return
	}
	systray.Quit()
}
