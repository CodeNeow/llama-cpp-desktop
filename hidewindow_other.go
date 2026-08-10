//go:build !windows

package main

import "os/exec"

// hideWindow is a no-op on non-Windows platforms.
func hideWindow(cmd *exec.Cmd) {}
