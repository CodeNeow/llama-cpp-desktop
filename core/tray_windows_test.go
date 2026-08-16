//go:build windows

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
// has not been started: directly takes the trayStarted=false branch, does not trigger
// systray.Quit, and does not panic. This is the pure-logic prerequisite relied on by
// dynamic start/stop (SetTrayEnabled disable path) — repeated disable must not detach
// repeatedly. systray is not actually started (Run would pull up a GetMessage message loop,
// unreliable in test processes without a window environment); only the not-started branch
// idempotency is asserted.
//
// Regarding "reset" logic: step-0 source-code conclusion (fyne.io/systray v1.12.2) confirms
// that systray.Quit() cannot be Run again within the same process (package-level quitOnce
// sync.Once blocks, window-class registration and initialized state are not reset), so this
// project does not implement trayStarted reset after QuitTray for dynamic restart —
// disabling the tray is a one-time operation. There is no testable second-start pure logic,
// only the above idempotency assertions are retained.
func TestQuitTrayIdempotentWhenNotStarted(t *testing.T) {
	// snapshot and restore global trayStarted (package-level state, avoid polluting other tests)
	trayMu.Lock()
	orig := trayStarted
	trayStarted = false
	trayMu.Unlock()
	defer func() {
		trayMu.Lock()
		trayStarted = orig
		trayMu.Unlock()
	}()

	// call when not started: idempotent return, no panic
	QuitTray()
	// state should still be not-started (QuitTray must not set trayStarted to true — it only sends the quit signal)
	trayMu.Lock()
	still := trayStarted
	trayMu.Unlock()
	if still {
		t.Error("QuitTray must not set trayStarted to true")
	}
}
