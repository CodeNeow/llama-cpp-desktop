//go:build windows

package main

import (
	"os/exec"
	"syscall"
)

// hideWindow prevents a child process from flashing a console window
// when spawned from a GUI-subsystem application (Wails builds as GUI).
// Go maps HideWindow to CREATE_NO_WINDOW internally.
func hideWindow(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true}
}
