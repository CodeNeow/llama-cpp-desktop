//go:build (windows || linux || darwin) && !android && !ios

package core

import (
	"context"
	_ "embed"
	"log"
	"runtime"
	"sync"

	"github.com/wailsapp/wails/v3/pkg/application"
)

// TrayIcon holds the tray icon bytes (injected by main.go after embedding
// build/windows/icon.ico). Used by core when starting/stopping the tray at
// startup or via SetTrayEnabled (the latter cannot access main's embed at
// runtime). The GUI tray uses these bytes as-is on Windows; linux/macOS render
// the embedded appicon.png instead (see trayIconPNG).
var TrayIcon []byte

// trayIconPNG is a copy of build/appicon.png embedded next to this file. The
// go:embed directive cannot reach outside the package directory, and main.go's
// embed is the .ico only — which the linux DBus tray (png.Decode) and the
// macOS NSStatusItem (NSImage) do not render reliably — so desktop
// non-Windows builds use this PNG. If build/appicon.png changes, refresh this
// copy (`cp build/appicon.png core/appicon.png`).
//
//go:embed appicon.png
var trayIconPNG []byte

// trayMu guards concurrent access to tray startup state, following the
// project's explicit mutex convention (configMu / serverLogsMu / etc.) so
// InitTray / QuitTray are idempotent and concurrency-safe.
var trayMu sync.Mutex

// trayStarted marks whether a tray (GUI or headless) has been started in this
// process; QuitTray returns immediately when not started (idempotent).
//
// Semantics note: the previous fyne.io/systray tray could not be restarted
// after Quit (package-level quitOnce); the v3 SystemTray could be re-created
// in principle, but the one-shot-per-process semantics are deliberately kept
// so SetTrayEnabled's "restart required after disable" behavior is unchanged.
var trayStarted bool

// trayStop quits whichever tray is currently running: the GUI tray registers a
// Destroy hook (v3 SystemTray), the headless tray registers a fyne
// systray.Quit hook (tray_headless_windows.go). A process is either GUI or
// headless, never both, so a single hook suffices; nil when no tray runs.
var trayStop func()

// trayMenuLabels returns the two tray menu item labels (show main window /
// quit), generated via tr() in the active language (zh/en, auto follows
// system). Pure function; core/tray_test.go asserts the bilingual results
// directly.
func trayMenuLabels() (show, quit string) {
	return tr("显示主窗口", "Show Main Window"), tr("退出", "Quit")
}

// InitTray starts the GUI system tray on the Wails v3 built-in SystemTray
// (Windows / Linux / macOS): sets the tray icon and tooltip ("Llama
// Desktop"), with menu items "Show Main Window" and "Quit" (separator in
// between). Menu click callbacks drive the Wails v3 runtime via the global
// application handle (application.Get) — the named main window is
// shown/restored for "Show Main Window"; quit ends the app for real (App.Quit
// runs the full shutdown sequence → App.ServiceShutdown cleanup). The ctx
// parameter is retained for call-site compatibility (it is the
// ServiceStartup ctx from core/app.go) but the v3 runtime is ctx-free.
//
// Icon selection: on Windows the .ico bytes passed by main.go are used
// (w32.CreateSmallHIconFromImage accepts PNG and ICO); on Linux (DBus
// StatusNotifierItem, png.Decode) and macOS (NSImage) the embedded PNG is
// used because ICO bytes render unreliably there.
//
// Lifecycle: SystemTray.New during ServiceStartup lands in the application's
// pendingRun queue (a.running is not yet true there) and the tray is realized
// right after startup completes — SetIcon/SetMenu/OnClick store their state
// until then, so configuration order is irrelevant. Left and right click both
// open the menu (matching the previous fyne tray's Windows behavior).
// Repeated calls after startup return immediately (idempotent), and the call
// is a no-op when no application is running (headless mode / unit tests).
func InitTray(_ context.Context, icon []byte) {
	// The v3 tray hangs off the global application; without one (headless
	// mode, unit tests) there is nothing to attach a tray to.
	wa := application.Get()
	if wa == nil {
		log.Println("[WARN] system tray unavailable: application not running")
		return
	}

	trayMu.Lock()
	if trayStarted {
		trayMu.Unlock()
		return
	}
	trayStarted = true
	trayMu.Unlock()

	trayIconBytes := icon
	if runtime.GOOS != "windows" {
		trayIconBytes = trayIconPNG
	}

	showLabel, quitLabel := trayMenuLabels()
	menu := wa.NewMenu()
	menu.Add(showLabel).OnClick(func(*application.Context) {
		// Show may not restore when minimized; also unminimize. Re-resolve
		// the application/window at click time; guard against teardown.
		if wa := application.Get(); wa != nil {
			if win, ok := wa.Window.GetByName(MainWindowName); ok && win != nil {
				win.Show()
				win.UnMinimise()
			}
		}
	})
	menu.AddSeparator()
	menu.Add(quitLabel).OnClick(func(*application.Context) {
		// Quit runs the full shutdown sequence (tray removal + service
		// cleanup via App.ServiceShutdown), then exits.
		if wa := application.Get(); wa != nil {
			wa.Quit()
		}
	})

	tray := wa.SystemTray.New()
	tray.SetIcon(trayIconBytes)
	tray.SetTooltip("Llama Desktop")
	tray.SetMenu(menu)
	// Open the menu on left click too: the previous fyne tray did this on
	// Windows (systrayLeftClick falls back to showMenu), so keep the UX.
	// Right click opens it via the explicit handler (the v3 Windows default
	// would do the same once a menu is set; pinning it keeps every platform
	// on one code path). ShowMenu is v3's exported open-menu helper.
	tray.OnClick(tray.ShowMenu)
	tray.OnRightClick(tray.ShowMenu)

	trayMu.Lock()
	trayStop = func() { tray.Destroy() }
	trayMu.Unlock()

	log.Println("[INFO] system tray started")
}

// QuitTray stops the system tray and removes the tray icon; idempotent (no-op
// when no tray was started), concurrency-safe (trayMu guards). Called by
// ServiceShutdown (remove tray before app cleanup), SetTrayEnabled(false)
// (disable tray) and the headless exit path (quit the fyne headless tray via
// its registered hook). After removal the tray does not restart in this
// process (see trayStarted comment).
func QuitTray() {
	trayMu.Lock()
	started := trayStarted
	stop := trayStop
	trayMu.Unlock()
	if !started || stop == nil {
		return
	}
	stop()
}
