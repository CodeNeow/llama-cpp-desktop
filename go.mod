module github.com/CodeNeow/llama-cpp-desktop

go 1.25.0

require (
	// fyne.io/systray — Windows system tray (core/tray_windows.go).
	// Floor = last verified v1.12.2; ceiling = v2 (next major, different API).
	// What breaks if it moves: our tray lifecycle is designed around this
	// version's package-level quitOnce and its non-resettable window-class /
	// message-loop state — systray.Run can never be re-called in one process.
	// The trayStarted/QuitTray idempotency and the "re-enabling the tray
	// requires an app restart" rule in tray_windows.go assume exactly that
	// behavior; a version that resets internal state (or changes Run/Quit
	// semantics) would turn the guard into a silent no-op or a crash.
	fyne.io/systray v1.12.2

	// github.com/wailsapp/wails/v2 — app shell (window, bindings, assets).
	// The /v2 module path pins the major (v3 is a separate module and a
	// rewrite, not an upgrade); within v2 this is the last-verified minor.
	// What breaks if it moves: the v2 runtime injects the window.go bridge
	// consumed by frontend/src/wails.ts (window.go.core.App.*) and regenerates
	// frontend/wailsjs on build, and main.go wires the go:embed frontend/dist
	// asset server plus the Frameless window and OnStartup/OnShutdown
	// lifecycle against v2 behavior — bump only together with a frontend
	// rebuild and the full quality gates.
	github.com/wailsapp/wails/v2 v2.14.0

	// golang.org/x/sys — direct Windows syscalls: single-instance named mutex
	// (CreateMutex/WaitForSingleObject/OpenProcess, core/singleinstance_windows.go),
	// GetDiskFreeSpaceEx (core/diskusage_windows.go), ShellExecute "runas"
	// elevation (core/installer_launch_windows.go), native MessageBoxW alert
	// via NewLazySystemDLL + UTF16PtrFromString (core/headlessalert_windows.go),
	// and UTF-16 helpers for the locale probe. A floor, not a ceiling: x/sys
	// keeps backward compatibility within v0, so bump freely — never let it
	// drop below what wails/v2 requires (MVS picks the max anyway).
	golang.org/x/sys v0.46.0
)

require (
	git.sr.ht/~jackmordaunt/go-toast/v2 v2.0.3 // indirect
	github.com/bep/debounce v1.2.1 // indirect
	github.com/go-ole/go-ole v1.3.0 // indirect
	github.com/godbus/dbus/v5 v5.1.0 // indirect
	github.com/google/uuid v1.6.0 // indirect
	github.com/gorilla/websocket v1.5.3 // indirect
	github.com/jchv/go-winloader v0.0.0-20210711035445-715c2860da7e // indirect
	github.com/labstack/echo/v4 v4.13.3 // indirect
	github.com/labstack/gommon v0.4.2 // indirect
	github.com/leaanthony/go-ansi-parser v1.6.1 // indirect
	github.com/leaanthony/gosod v1.0.4 // indirect
	github.com/leaanthony/slicer v1.6.0 // indirect
	github.com/leaanthony/u v1.1.1 // indirect
	github.com/mattn/go-colorable v0.1.13 // indirect
	github.com/mattn/go-isatty v0.0.20 // indirect
	github.com/pkg/browser v0.0.0-20240102092130-5ac0b6a4141c // indirect
	github.com/pkg/errors v0.9.1 // indirect
	github.com/rivo/uniseg v0.4.7 // indirect
	github.com/samber/lo v1.49.1 // indirect
	github.com/tkrajina/go-reflector v0.5.8 // indirect
	github.com/valyala/bytebufferpool v1.0.0 // indirect
	github.com/valyala/fasttemplate v1.2.2 // indirect
	github.com/wailsapp/go-webview2 v1.0.22 // indirect
	github.com/wailsapp/mimetype v1.4.1 // indirect
	golang.org/x/crypto v0.53.0 // indirect
	golang.org/x/net v0.56.0 // indirect
	golang.org/x/text v0.39.0 // indirect
)
