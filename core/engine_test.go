package core

import (
	"strings"
	"testing"
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
