package core

import (
	"os"
	"runtime"
	"testing"
	"time"
)

// TestProcessStartTimeSelf verifies the platform processStartTime query
// against the test process itself. On Windows/Linux (supported platforms) the
// returned creation time must be plausible: non-zero and within the last hour
// (the go test process started moments ago). On other platforms the stub must
// report unavailable (zero time, ok=false) so the handover check fails open.
func TestProcessStartTimeSelf(t *testing.T) {
	start, ok := processStartTime(os.Getpid())
	switch runtime.GOOS {
	case "windows", "linux":
		if !ok {
			t.Fatalf("processStartTime should query the creation time of pid %d (GOOS %s)", os.Getpid(), runtime.GOOS)
		}
		if start.IsZero() {
			t.Fatal("processStartTime should return a non-zero creation time")
		}
		if age := time.Since(start); age < 0 || age > time.Hour {
			t.Errorf("creation time %v implies age %v, want within [0, 1h]", start, age)
		}
	default:
		if ok {
			t.Error("stub platform must report the creation time as unavailable")
		}
		if !start.IsZero() {
			t.Error("stub platform must return a zero time")
		}
	}
}

// TestProcessStartTimeInvalidPid verifies processStartTime reports unavailable
// for a pid that cannot exist (checked as unavailable, zero time).
func TestProcessStartTimeInvalidPid(t *testing.T) {
	// The pid guard rejects non-positive pids on every platform.
	for _, pid := range []int{0, -1} {
		if _, ok := processStartTime(pid); ok {
			t.Errorf("processStartTime(%d) should report unavailable", pid)
		}
	}

	if runtime.GOOS != "windows" && runtime.GOOS != "linux" {
		t.Skip("start-time query is stubbed on this platform")
	}

	// A pid no process can have: Linux pid_max tops out at 2^22, and Windows
	// pids are always multiples of 4, so this odd, out-of-range value cannot
	// exist. If it somehow did (unforeseen platform quirk), skip rather than
	// flake — the guard below uses processAlive for exactly that.
	const impossiblePid = 999999999
	if processAlive(impossiblePid) {
		t.Skip("pid unexpectedly alive on this platform; skipping to avoid a flaky assertion")
	}
	start, ok := processStartTime(impossiblePid)
	if ok {
		t.Errorf("processStartTime(%d) should report unavailable, got %v", impossiblePid, start)
	}
	if !start.IsZero() {
		t.Errorf("processStartTime(%d) should return a zero time on failure, got %v", impossiblePid, start)
	}
}
