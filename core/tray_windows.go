//go:build windows

package core

import (
	"context"
	"log"
	"runtime"
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

// headlessTrayLabels returns the headless (API-route) tray menu labels.
// Intentionally the same two entries as the GUI tray (the headless menu is a
// subset workflow: return to the GUI or quit), kept as a dedicated function so
// the headless menu can diverge later without touching the GUI tray.
// Pure function; core/tray_windows_test.go asserts the bilingual results.
func headlessTrayLabels() (show, quit string) {
	return tr("显示主窗口", "Show Main Window"), tr("退出", "Quit")
}

// InitHeadlessTray starts the headless-mode system tray: same icon/tooltip and
// menu shape as the GUI tray, but menu clicks invoke the injected callbacks
// instead of the Wails runtime — in headless mode no window/ctx exists. The
// callbacks run on the tray click goroutine and must not block (the headless
// module's handlers only shuffle state, spawn a process and signal exit).
// Shares the trayStarted/trayMu idempotency rules with InitTray: a process is
// either GUI or headless, never both, so a single started flag suffices.
// LockOSThread rationale identical to InitTray's (Win32 message routing).
func InitHeadlessTray(icon []byte, onShow func(), onQuit func()) {
	trayMu.Lock()
	if trayStarted {
		trayMu.Unlock()
		return
	}
	trayStarted = true
	trayMu.Unlock()

	go func() {
		runtime.LockOSThread()
		defer runtime.UnlockOSThread()
		systray.Run(func() {
			systray.SetIcon(icon)
			systray.SetTooltip("Llama Desktop")

			showLabel, quitLabel := headlessTrayLabels()
			showItem := systray.AddMenuItem(showLabel, showLabel)
			systray.AddSeparator()
			quitItem := systray.AddMenuItem(quitLabel, quitLabel)

			go func() {
				for range showItem.ClickedCh {
					onShow()
				}
			}()
			go func() {
				for range quitItem.ClickedCh {
					onQuit()
				}
			}()
		}, func() {
			// Tray has exited (quit menu or QuitTray triggered); no extra
			// cleanup needed.
		})
		log.Println("[INFO] headless systray exited")
	}()
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
		// Pin the tray to one OS thread for its whole lifetime: systray
		// creates the tray window on this goroutine's current thread and then
		// blocks in GetMessage on the same goroutine, and Win32 delivers
		// window messages (right-click, menu selection) to the queue of the
		// thread that created the window. Without locking, the Go scheduler
		// can resume this goroutine on a different thread after a blocking
		// GetMessage returns (systray's package-level LockOSThread in init
		// only pins the main goroutine); messages would then queue on the
		// old thread while the loop blocks on the new one, leaving the tray
		// icon permanently unresponsive — typically after the app has been
		// running a while and scheduling pressure grows.
		runtime.LockOSThread()
		defer runtime.UnlockOSThread()
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
