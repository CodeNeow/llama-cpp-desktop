//go:build windows

package core

import (
	"errors"
	"log"
	"time"

	"golang.org/x/sys/windows"
)

// ─── Single-instance named mutex ──────────────────────────────────
//
// A session-local named mutex (Local\ prefix, per login session) serializes
// MyLlama processes: the mode-switch flow (GUI ⇄ headless) relaunches
// the app before the current process exits, so a new process briefly has to
// wait for the old one to release the mutex — hence the bounded retry loop.
// The mutex is held for the whole process lifetime and released by the OS on
// exit (including crashes, where the next waiter sees WAIT_ABANDONED and
// still acquires ownership).

// singleInstanceMutexName is the named-mutex name (session-local). A
// package-level var (not a const) so tests can substitute an isolated unique
// name: the production name is shared per login session, and a live app
// instance (e.g. a developer's running `wails dev`) legitimately owns it while
// `go test` executes, which would make the test contend with a third party.
var singleInstanceMutexName = `Local\llama-desktop-single-instance`

// singleInstanceRetryCount/Interval bound the acquire retry loop
// (10 × 500 ms = up to 5 s, covering the mode-switch relaunch window).
// Package-level vars instead of consts so tests can shorten them (same
// injection-point style as llamaVersionProbeTimeout / cmdTimeout).
var (
	singleInstanceRetryCount    = 10
	singleInstanceRetryInterval = 500 * time.Millisecond
)

// AcquireSingleInstance acquires the single-instance named mutex: it retries
// up to singleInstanceRetryCount times while another instance holds the mutex
// (the mode-switch relaunch window), and reports whether this process owns it.
// Only a zero handle from CreateMutex is a hard failure (logged, not acquired)
// — the caller decides whether to exit.
func AcquireSingleInstance() bool {
	name, err := windows.UTF16PtrFromString(singleInstanceMutexName)
	if err != nil {
		log.Printf("[WARN] failed to encode mutex name %q: %v", singleInstanceMutexName, err)
		return false
	}
	for attempt := 0; attempt < singleInstanceRetryCount; attempt++ {
		if attempt > 0 {
			time.Sleep(singleInstanceRetryInterval)
		}
		// Request ownership up front: when the mutex does not exist yet this
		// call creates and owns it; when another process owns it we get a
		// valid handle without ownership and wait below.
		//
		// Windows CreateMutex semantics: when the named mutex already exists
		// the call returns a VALID handle together with ERROR_ALREADY_EXISTS
		// (x/sys mirrors this: err is set when handle==0 OR e1 ==
		// ERROR_ALREADY_EXISTS). "Already exists" is not a creation failure —
		// it is exactly the retry situation (another instance is still alive,
		// e.g. mid-shutdown during a mode switch), and initialOwner=true does
		// NOT grant us ownership for a pre-existing mutex, so the wait below
		// blocks until the owner releases or the timeout lapses. Treat any
		// non-zero handle as usable; only handle==0 is a real failure.
		handle, err := windows.CreateMutex(nil, true, name)
		if handle == 0 {
			log.Printf("[WARN] CreateMutex failed: %v", err)
			return false
		}
		// A zero timeout would succeed immediately for a mutex we own; the
		// bounded wait doubles as one retry interval, so the loop takes at most
		// retryCount × 2 × interval in total (per-attempt wait timeout plus the
		// inter-attempt sleep).
		state, err := windows.WaitForSingleObject(handle, uint32(singleInstanceRetryInterval.Milliseconds()))
		if err == nil && (state == windows.WAIT_OBJECT_0 || state == windows.WAIT_ABANDONED) {
			// Acquired. The handle is deliberately neither closed nor stored
			// anywhere: closing it would release the mutex, and a plain
			// windows.Handle needs no Go reference to stay valid — the OS
			// releases the mutex when the process exits (crashes included;
			// the next waiter sees WAIT_ABANDONED and still acquires).
			return true
		}
		// Timeout (another instance still running) or wait failure: close the
		// handle and retry until the bound is exhausted.
		windows.CloseHandle(handle)
	}
	return false
}

// ─── Process liveness (used by the handover health check) ─────────

// processAlive reports whether the pid maps to a running process. On Windows
// os.FindProcess always succeeds (no validation), so OpenProcess is used:
// ERROR_INVALID_PARAMETER means the pid does not exist; other errors such as
// ERROR_ACCESS_DENIED mean the process exists but is protected — alive.
func processAlive(pid int) bool {
	if pid <= 0 {
		return false
	}
	handle, err := windows.OpenProcess(windows.PROCESS_QUERY_LIMITED_INFORMATION, false, uint32(pid))
	if err == nil {
		windows.CloseHandle(handle)
		return true
	}
	if errors.Is(err, windows.ERROR_INVALID_PARAMETER) {
		return false
	}
	// Exists but not queryable (protected/system process): count as alive.
	return true
}
