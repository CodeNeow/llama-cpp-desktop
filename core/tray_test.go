//go:build (windows || linux || darwin) && !android && !ios

package core

import "testing"

// TestTrayMenuLabels verifies the trayMenuLabels() pure function for tray menu labels:
//   - returns two non-empty labels (Show Main Window / Quit);
//   - zh locale returns the Chinese "Show Main Window" and "Quit" labels (reuses setLanguageForTest
//     from i18n_test.go, sharing the same mutex protection as other tr() tests).
func TestTrayMenuLabels(t *testing.T) {
	// zh locale: returns the Chinese "Show Main Window" and "Quit" labels
	setLanguageForTest(t, "zh")
	show, quit := trayMenuLabels()
	if show != "显示主窗口" {
		t.Errorf("zh show label should be 显示主窗口, got %q", show)
	}
	if quit != "退出" {
		t.Errorf("zh quit label should be 退出, got %q", quit)
	}
	if show == "" || quit == "" {
		t.Errorf("labels must not be empty, show=%q quit=%q", show, quit)
	}

	// English: returns "Show Main Window" and "Quit"
	setLanguageForTest(t, "en")
	show, quit = trayMenuLabels()
	if show != "Show Main Window" {
		t.Errorf("en show label should be Show Main Window, got %q", show)
	}
	if quit != "Quit" {
		t.Errorf("en quit label should be Quit, got %q", quit)
	}
}

// TestQuitTrayIdempotentWhenNotStarted verifies QuitTray is idempotent when the tray
// has not been started: directly takes the trayStarted=false branch, does not invoke the
// tray stop hook, and does not panic. This is the pure-logic prerequisite relied on by
// dynamic start/stop (SetTrayEnabled disable path) — repeated disable must not destroy
// repeatedly. No tray is actually started in tests (the v3 SystemTray needs a running
// application); only the not-started branch idempotency is asserted.
//
// Regarding "reset" logic: the one-shot-per-process semantics of the previous
// fyne.io/systray tray are deliberately preserved on the v3 SystemTray (see the
// trayStarted comment in tray.go), so there is no trayStarted reset after QuitTray and
// no testable second-start pure logic — only the idempotency assertions are retained.
func TestQuitTrayIdempotentWhenNotStarted(t *testing.T) {
	// snapshot and restore global tray state (package-level, avoid polluting other tests)
	trayMu.Lock()
	origStarted := trayStarted
	origStop := trayStop
	trayStarted = false
	trayStop = nil
	trayMu.Unlock()
	defer func() {
		trayMu.Lock()
		trayStarted = origStarted
		trayStop = origStop
		trayMu.Unlock()
	}()

	// call when not started: idempotent return, no panic
	QuitTray()
	// state should still be not-started (QuitTray must not set trayStarted to true — it
	// only invokes the registered stop hook)
	trayMu.Lock()
	still := trayStarted
	trayMu.Unlock()
	if still {
		t.Error("QuitTray must not set trayStarted to true")
	}
}

// TestQuitTrayInvokesStopHook verifies QuitTray delegates to the registered trayStop
// hook exactly once when a tray is marked started: the hook abstraction is what lets the
// GUI (v3 SystemTray.Destroy) and headless (fyne systray.Quit) trays share one quit path.
func TestQuitTrayInvokesStopHook(t *testing.T) {
	trayMu.Lock()
	origStarted := trayStarted
	origStop := trayStop
	trayStarted = true
	calls := 0
	trayStop = func() { calls++ }
	trayMu.Unlock()
	defer func() {
		trayMu.Lock()
		trayStarted = origStarted
		trayStop = origStop
		trayMu.Unlock()
	}()

	QuitTray()
	if calls != 1 {
		t.Errorf("QuitTray should invoke the stop hook once, got %d calls", calls)
	}
}
