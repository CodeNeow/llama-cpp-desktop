//go:build windows

package core

import "testing"

// TestHeadlessTrayLabels verifies the headlessTrayLabels() pure function for the
// headless (API-route) tray menu labels: zh returns the Chinese labels, en the
// English ones — intentionally the same entries as the GUI tray (the headless
// menu is a subset workflow: return to GUI / quit).
func TestHeadlessTrayLabels(t *testing.T) {
	setLanguageForTest(t, "zh")
	show, quit := headlessTrayLabels()
	if show != "显示主窗口" {
		t.Errorf("zh show label should be 显示主窗口, got %q", show)
	}
	if quit != "退出" {
		t.Errorf("zh quit label should be 退出, got %q", quit)
	}

	setLanguageForTest(t, "en")
	show, quit = headlessTrayLabels()
	if show != "Show Main Window" {
		t.Errorf("en show label should be Show Main Window, got %q", show)
	}
	if quit != "Quit" {
		t.Errorf("en quit label should be Quit, got %q", quit)
	}
}
