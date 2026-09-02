//go:build windows

package core

import (
	"log"
	"runtime"

	"fyne.io/systray"
)

// headlessTrayLabels returns the headless (API-route) tray menu labels.
// Intentionally the same two entries as the GUI tray (the headless menu is a
// subset workflow: return to the GUI or quit), kept as a dedicated function so
// the headless menu can diverge later without touching the GUI tray.
// Pure function; core/tray_headless_windows_test.go asserts the bilingual
// results.
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
// LockOSThread rationale identical to the GUI tray's old fyne implementation
// (Win32 message routing). Headless mode stays on fyne.io/systray (Windows
// only) while the GUI tray moved to the v3 built-in SystemTray: headless never
// has a Wails application to hang a v3 tray on.
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
			systray.SetTooltip("MyLlama")

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

	// Route QuitTray to systray.Quit: the shared trayStop hook (see tray.go)
	// makes the headless exit path keep removing the fyne icon exactly as
	// before the GUI tray moved to the v3 SystemTray.
	trayMu.Lock()
	trayStop = func() { systray.Quit() }
	trayMu.Unlock()
}
