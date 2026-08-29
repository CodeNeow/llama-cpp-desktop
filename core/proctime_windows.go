//go:build windows

package core

import (
	"time"

	"golang.org/x/sys/windows"
)

// processStartTime queries the OS for a process's real creation time. Used by
// the handover start-time check to detect pid reuse: Windows recycles pids
// quickly, so a record whose pid is alive may point at an unrelated process —
// a creation-time mismatch identifies the recycled pid. ok is false on any
// failure (pid invalid, process gone, protected process, OS error); callers
// treat that as "check unavailable" and fail open (see evaluateHandover).
func processStartTime(pid int) (time.Time, bool) {
	if pid <= 0 {
		return time.Time{}, false
	}
	// PROCESS_QUERY_LIMITED_INFORMATION is enough for GetProcessTimes and
	// works for same-user processes (an adopted llama-server belongs to the
	// previous, exited app process but runs as the same user).
	handle, err := windows.OpenProcess(windows.PROCESS_QUERY_LIMITED_INFORMATION, false, uint32(pid))
	if err != nil {
		return time.Time{}, false
	}
	defer windows.CloseHandle(handle)
	var creation, exit, kernel, user windows.Filetime
	if err := windows.GetProcessTimes(handle, &creation, &exit, &kernel, &user); err != nil {
		return time.Time{}, false
	}
	// Filetime.Nanoseconds converts from the Windows 1601 epoch to Unix
	// nanoseconds; the creation time is absolute UTC.
	return time.Unix(0, creation.Nanoseconds()).UTC(), true
}
