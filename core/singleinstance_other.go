//go:build !windows

package core

import (
	"os"
	"syscall"
)

// AcquireSingleInstance is a no-op success on non-Windows platforms: the
// API-route (headless) mode is Windows-only, and no cross-platform single
// instance mechanism is currently required (macOS/Linux close exits the app
// directly). Kept so main.go compiles identically on all platforms.
func AcquireSingleInstance() bool {
	return true
}

// processAlive reports whether the pid maps to a running process: on Unix
// os.FindProcess validates the pid and the null signal (0) checks liveness
// without actually signalling the process. Used by the handover health check
// (cross-platform code path; headless mode itself is Windows-only).
func processAlive(pid int) bool {
	if pid <= 0 {
		return false
	}
	proc, err := os.FindProcess(pid)
	if err != nil {
		return false
	}
	return proc.Signal(syscall.Signal(0)) == nil
}
