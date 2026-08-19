//go:build windows

package core

import (
	"fmt"
	"os"
	"os/exec"
	"runtime"
	"testing"
	"time"

	"golang.org/x/sys/windows"
)

// testMutexEnv carries the test mutex name from the parent test process to
// the helper child process (the child cannot derive it: the name embeds the
// parent's pid for uniqueness).
const testMutexEnv = "LLAMA_DESKTOP_TEST_MUTEX_NAME"

// testMutexName returns a per-process-unique mutex name, so the test never
// contends with the production name: a live app instance in the same login
// session (e.g. a developer's running `wails dev`) legitimately owns
// Local\llama-desktop-single-instance while `go test` executes, and the test
// would otherwise race a third party instead of exercising its own hold.
func testMutexName() string {
	return fmt.Sprintf(`Local\llama-desktop-single-instance-test-%d`, os.Getpid())
}

// TestAcquireSingleInstanceBlockedAndReleased verifies AcquireSingleInstance
// against a really held mutex, cross-process:
//   - while another process owns the named mutex, AcquireSingleInstance must
//     return false (no panic) after actually executing the bounded wait +
//     retry loop — the regression: x/sys CreateMutex returns a valid handle
//     together with ERROR_ALREADY_EXISTS, and treating that err as a hard
//     failure made the acquire give up instantly, so the mode-switch retry
//     window never worked;
//   - once the mutex is released, a fresh acquire must succeed.
//
// Cross-process on purpose: Win32 mutex ownership is per-OS-thread and Go
// goroutines migrate between threads, so a second in-process acquire could run
// on the owning thread, where WaitForSingleObject returns WAIT_OBJECT_0
// immediately (recursive ownership) and the result would be non-deterministic.
// A separate process always blocks against the parent's ownership.
//
// The parent holds the mutex via a raw CreateMutex (keeping the handle)
// because AcquireSingleInstance deliberately never stores a handle to release;
// the hold/release is wrapped in LockOSThread: CreateMutex(initialOwner=true)
// attributes ownership to the calling OS thread, so the matching ReleaseMutex
// must run on that same thread (without pinning, goroutine migration could
// hand it to a non-owner thread → ERROR_NOT_OWNER). Releasing — not merely
// closing the handle — is required before the free-helper check: closing the
// last handle of a HELD mutex leaves the object alive and owned until the
// owning thread releases or terminates, so the next acquirer keeps blocking.
func TestAcquireSingleInstanceBlockedAndReleased(t *testing.T) {
	origName := singleInstanceMutexName
	singleInstanceMutexName = testMutexName()
	t.Cleanup(func() { singleInstanceMutexName = origName })

	runtime.LockOSThread()
	// UnlockOSThread is deferred FIRST so it runs LAST (LIFO): the mutex
	// release below must still happen on the owning (locked) thread.
	defer runtime.UnlockOSThread()

	name, err := windows.UTF16PtrFromString(singleInstanceMutexName)
	if err != nil {
		t.Fatal(err)
	}
	handle, createErr := windows.CreateMutex(nil, true, name)
	if handle == 0 {
		t.Fatalf("failed to hold the test mutex: %v", createErr)
	}
	// Best-effort cleanup for failure paths (t.Fatalf between hold and the
	// inline release): release + close on the owning thread so repeated runs
	// (-count=2) start from a destroyed object instead of one still owned by
	// this process.
	defer func() {
		if handle == 0 {
			return
		}
		_ = windows.ReleaseMutex(handle)
		windows.CloseHandle(handle)
	}()

	runHelper := func(mode string) int {
		t.Helper()
		cmd := exec.Command(os.Args[0], "-test.run=^TestSingleInstanceHelperProcess$")
		cmd.Env = append(os.Environ(),
			"GO_WANT_SINGLE_INSTANCE_HELPER="+mode,
			testMutexEnv+"="+singleInstanceMutexName)
		hideWindow(cmd)
		if err := cmd.Run(); err != nil {
			if exitErr, ok := err.(*exec.ExitError); ok {
				return exitErr.ExitCode()
			}
			t.Fatalf("helper run failed (mode %s): %v", mode, err)
		}
		return 0
	}

	// Blocked: the helper must report "false after a real wait" (exit 3).
	// Exit 1 = unexpectedly acquired; exit 5 = returned false without ever
	// waiting (the regression this test guards against); exit 2/4 = helper
	// misuse.
	if code := runHelper("blocked"); code != 3 {
		t.Errorf("blocked helper exit code = %d, want 3 (false after waiting; 5 would mean the wait path was skipped)", code)
	}

	// Release ownership from the owning (pinned) thread, then drop the
	// handle: merely closing the handle leaves a held mutex owned and alive
	// (see the LockOSThread rationale above).
	if err := windows.ReleaseMutex(handle); err != nil {
		t.Fatalf("failed to release the test mutex: %v", err)
	}
	windows.CloseHandle(handle)
	handle = 0 // cleanup defer already handled

	// Free: a fresh helper must now acquire it (exit 0).
	if code := runHelper("free"); code != 0 {
		t.Errorf("free helper exit code = %d, want 0 (acquire after release)", code)
	}
}

// TestSingleInstanceHelperProcess is the child side of
// TestAcquireSingleInstanceBlockedAndReleased (same pattern as
// TestHelperProcess in server_test.go): when GO_WANT_SINGLE_INSTANCE_HELPER is
// set it adopts the test mutex name from the environment, shortens the retry
// parameters, calls AcquireSingleInstance and reports the outcome via the
// process exit code; without the env var it is a no-op so the parent's normal
// test run never triggers it.
func TestSingleInstanceHelperProcess(t *testing.T) {
	mode := os.Getenv("GO_WANT_SINGLE_INSTANCE_HELPER")
	if mode == "" {
		return
	}
	if n := os.Getenv(testMutexEnv); n != "" {
		singleInstanceMutexName = n
	}
	singleInstanceRetryCount = 2
	singleInstanceRetryInterval = 100 * time.Millisecond

	switch mode {
	case "blocked":
		start := time.Now()
		if AcquireSingleInstance() {
			os.Exit(1) // parent holds the mutex: must not acquire
		}
		// The retry loop must really have waited: with 2 × 100 ms the first
		// WaitForSingleObject blocks the full 100 ms against the parent's
		// ownership. A near-zero elapsed time means CreateMutex's
		// ERROR_ALREADY_EXISTS was (again) misread as a hard failure and the
		// wait was skipped — the regression under test. 90 ms leaves clock
		// granularity headroom below the 100 ms wait.
		if elapsed := time.Since(start); elapsed < 90*time.Millisecond {
			os.Exit(5)
		}
		os.Exit(3) // expected: not acquired after a genuine bounded wait
	case "free":
		if !AcquireSingleInstance() {
			os.Exit(2) // mutex released by the parent: must acquire
		}
		os.Exit(0)
	}
	os.Exit(4) // unknown mode
}
