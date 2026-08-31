//go:build android

package core

import (
	"os"
	"strings"
)

// readProcFile reads a kernel pseudo-file (/proc, /sys, /system) directly
// with os.ReadFile. The Android app sandbox (the Wails v3 c-shared target)
// cannot spawn child processes, so os/exec is unusable there — the same
// reason gpuProbesUnsupported short-circuits the nvidia-smi probes — and
// procfs must be read in-process instead. Returns "" on any read failure;
// callers' parsers treat that as "unavailable" exactly like an empty exec
// output.
func readProcFile(path string) string {
	data, err := os.ReadFile(path)
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(data))
}
