//go:build !windows && !linux

package core

import "time"

// processStartTime is a stub on platforms without a supported creation-time
// query: the handover start-time check is skipped there (fail-open; the port
// health probe still guards adoption). Headless mode itself is Windows-only,
// so this stub mainly keeps the package compiling on every platform.
func processStartTime(pid int) (time.Time, bool) {
	return time.Time{}, false
}
