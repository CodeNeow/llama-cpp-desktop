package core

import (
	"runtime"
	"strings"
	"testing"
	"time"
)

// TestRunCmdTrimsOutput verifies runCmd captures child-process stdout and trims
// leading/trailing whitespace.
// Uses `go env GOHOSTOS`: cross-platform, no external binary or network needed.
func TestRunCmdTrimsOutput(t *testing.T) {
	out := runCmd("go", "env", "GOHOSTOS")
	if strings.TrimSpace(out) != out {
		t.Fatalf("runCmd returned untrimmed output: %q", out)
	}
	if out == "" {
		t.Fatal("runCmd(go env GOHOSTOS) returned empty output")
	}
}

// TestRunCmdMissingBinary verifies runCmd returns an empty string instead of panicking
// when the command does not exist.
func TestRunCmdMissingBinary(t *testing.T) {
	out := runCmd("llama-desktop-no-such-binary-xyz", "--version")
	if out != "" {
		t.Fatalf("non-existent command should return empty output, got: %q", out)
	}
}

// TestRunCmdTimeout verifies a hung child process (e.g. a stalled WMI query)
// is killed after cmdTimeout and runCmd returns "" instead of blocking
// forever. Uses a short injected timeout and a command that sleeps far beyond
// it; the elapsed time must stay close to the timeout, not the sleep length.
func TestRunCmdTimeout(t *testing.T) {
	orig := cmdTimeout
	cmdTimeout = 300 * time.Millisecond
	defer func() { cmdTimeout = orig }()

	// Sleep well beyond the timeout on every platform: sleep(1) is missing on
	// Windows, where ping -n keeps the child alive for ~seconds instead.
	var name string
	var args []string
	if runtime.GOOS == "windows" {
		name = "ping"
		args = []string{"-n", "30", "127.0.0.1"} // ~30s, far above the 300ms timeout
	} else {
		name = "sleep"
		args = []string{"30"}
	}
	start := time.Now()
	out := runCmd(name, args...)
	elapsed := time.Since(start)

	if out != "" {
		t.Fatalf("timed-out command should return empty output, got: %q", out)
	}
	if elapsed > 5*time.Second {
		t.Fatalf("runCmd blocked for %v, want close to cmdTimeout=%v", elapsed, cmdTimeout)
	}
}
