package core

import (
	"fmt"
	"log"
	"os"
	"os/exec"
	"runtime"
	"sync"
)

// ─── API-route (headless) mode ────────────────────────────────────
//
// API-route mode restarts the app as a pure background process: the GUI
// (WebView2 process tree) is released, leaving the Go backend + system tray +
// llama-server serving the OpenAI API. Entry: settings toggle (GUI →
// headless). Exit: tray "Show Main Window" (headless → GUI) or tray "Quit"
// (stops the service). The llama-server process survives both switch paths
// uninterrupted via the handover file (core/handover.go).

// shouldRunHeadless is the pure decision core for headless startup:
//   - non-Windows never runs headless (v1 platform restriction);
//   - an explicit --headless flag wins;
//   - an explicit --gui flag overrides the persisted preference;
//   - dev builds (wails dev) never follow the persisted preference: the
//     relaunch-based switch escapes the wails dev supervisor and tears the
//     dev session down (locked binary), so only the explicit flag works;
//   - otherwise the persisted apiRouteMode preference decides.
func shouldRunHeadless(goos string, headlessFlag, guiFlag, modeEnabled, devBuild bool) bool {
	if goos != "windows" {
		return false
	}
	if headlessFlag {
		return true
	}
	if guiFlag {
		return false
	}
	if devBuild {
		return false
	}
	return modeEnabled
}

// ShouldRunHeadless reports whether this process should run in headless
// API-route mode per the flags and the persisted preference. It loads the
// config when the preference can influence the decision (the GUI startup path
// calls loadConfig again in App.Startup — re-reading the same file is
// side-effect-free).
func ShouldRunHeadless(headlessFlag, guiFlag bool) bool {
	if !headlessFlag && !guiFlag {
		loadConfig()
	}
	return shouldRunHeadless(runtime.GOOS, headlessFlag, guiFlag, ApiRouteMode(), isDevBuild)
}

// relaunchSelf is the test injection point starting a fresh copy of this
// executable with the given args as a detached child (hideWindow per project
// convention; same style as updateLauncher).
var relaunchSelf = func(args ...string) error {
	exe, err := os.Executable()
	if err != nil {
		return fmt.Errorf("resolve executable: %w", err)
	}
	cmd := exec.Command(exe, args...)
	hideWindow(cmd)
	return cmd.Start()
}

// headlessExit is closed (once) when the headless process should exit; the
// tray callbacks trigger it and RunHeadless's blocking wait returns.
var headlessExit = make(chan struct{})
var headlessExitOnce sync.Once

func requestHeadlessExit() {
	headlessExitOnce.Do(func() { close(headlessExit) })
}

// writeServerHandover writes the handover record for the currently running
// llama-server (child pid, or adopted pid when the server was itself adopted;
// no file is written when no server runs or its identity is unknown).
func writeServerHandover() error {
	serverMu.Lock()
	running := serverRunning
	port := serverPort
	cmd := serverCmd
	adopted := adoptedPid
	serverMu.Unlock()
	if !running {
		return nil
	}
	pid := 0
	if cmd != nil && cmd.Process != nil {
		pid = cmd.Process.Pid
	} else if adopted > 0 {
		pid = adopted
	}
	if pid == 0 || port == 0 {
		return nil
	}
	return writeHandover(pid, port)
}

// adoptOrCleanHandover applies the handover plan to a GUI startup: a healthy
// record is adopted (serverRunning=true, serverCmd=nil, adoptedPid set — the
// frontend immediately reports the service as running, and its server log is
// tailed when the record carries a log path), a stale record is deleted;
// unlike headless startup, GUI never auto-starts the service.
func adoptOrCleanHandover() {
	plan := evaluateHandover()
	switch {
	case plan.Adopt:
		adoptHandover(plan.PID, plan.Port, plan.LogPath, plan.Start)
	case plan.RemoveFile:
		if err := removeHandover(); err != nil {
			log.Printf("[WARN] %v", err)
		}
	}
}

// notifyHeadlessServerStartFailed surfaces a headless llama-server start
// failure to the user. Headless mode has no window and no console, so the
// default platform implementation (core/headlessalert_windows.go) shows a
// native alert dialog; other platforms default to a no-op (headless is
// Windows-only). Tests replace this variable to observe the failure.
var notifyHeadlessServerStartFailed = defaultHeadlessServerAlert

// startOrAdoptServer brings llama-server up in a headless startup: healthy
// handover record → adopt; stale record → delete + auto start; no record →
// auto start. A start failure logs and notifies the user (headless keeps
// running so the tray can still return to GUI mode).
func startOrAdoptServer() {
	plan := evaluateHandover()
	if plan.Adopt {
		adoptHandover(plan.PID, plan.Port, plan.LogPath, plan.Start)
		return
	}
	if plan.RemoveFile {
		if err := removeHandover(); err != nil {
			log.Printf("[WARN] %v", err)
		}
	}
	if err := startServerInternal(); err != nil {
		log.Printf("[WARN] headless llama-server start failed (use the tray to return to GUI): %v", err)
		notifyHeadlessServerStartFailed(err)
	}
}

// returnToGuiFromHeadless is the headless tray "Show Main Window" handler:
// persist apiRouteMode=false, hand the running llama-server over, relaunch the
// GUI (no args) and signal headless exit — the service is not stopped.
func returnToGuiFromHeadless() {
	configMu.Lock()
	apiRouteMode = false
	configMu.Unlock()
	saveConfig()

	if err := writeServerHandover(); err != nil {
		log.Printf("[WARN] %v", err)
	}
	if err := relaunchSelf(); err != nil {
		// Relaunch failed: stay headless (exiting would strand the user with
		// no way back); the log line surfaces the cause.
		log.Printf("[ERROR] failed to relaunch GUI, staying headless: %v", err)
		return
	}
	switchRestartPending.Store(true)
	log.Println("[INFO] returning to GUI mode; llama-server keeps running")
	requestHeadlessExit()
}

// RunHeadless runs the headless API-route lifecycle (Windows only; callers
// gate on ShouldRunHeadless): load config (also restores the persisted
// download queue), acquire the single-instance mutex (retrying through the
// switch-restart window), start the headless tray, start/adopt llama-server,
// then block until a tray action requests exit. On switch-restart exits the
// service stays alive; on tray-quit the (possibly adopted) service is stopped
// and the handover record removed.
func RunHeadless(icon []byte) {
	loadConfig()

	if !AcquireSingleInstance() {
		log.Println("[ERROR] another Llama Desktop instance holds the single-instance lock; headless start aborted")
		return
	}

	InitHeadlessTray(icon, returnToGuiFromHeadless, requestHeadlessExit)
	startOrAdoptServer()

	<-headlessExit
	QuitTray()

	if switchRestartPending.Load() {
		// Switching to the relaunched process: it adopts the service via the
		// handover file; downloads resume from the persisted queue.
		log.Println("[INFO] headless exiting for mode switch; llama-server keeps running")
		return
	}

	// Real quit (tray "Quit"): stop the adopted or owned service and clean up.
	if err := stopServerInternal(); err != nil {
		log.Printf("[WARN] stop llama-server on headless exit: %v", err)
	}
	log.Println("[INFO] headless exited")
}
