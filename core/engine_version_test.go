package core

import (
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"testing"
	"time"
)

// probeHelperStderrSrc is a probe-test normal stub: writes only the version line to
// stderr then exits (llama-server --version output all goes to stderr, stdout is empty),
// exit code 0.
const probeHelperStderrSrc = `package main

import "os"

func main() {
	os.Stderr.WriteString("version: b1234 (build 1234)\n")
}
`

// probeHelperHangSrc is a probe-test hang stub: infinite sleep, simulating an abnormal
// binary that treats -v as a version flag and starts a full HTTP server then never exits.
const probeHelperHangSrc = `package main

import "time"

func main() {
	time.Sleep(60 * time.Second)
}
`

// buildProbeHelper compiles a probe-test stub executable: src is a standalone main-package
// source; the build artifact is placed under t.TempDir() and automatically cleaned up
// after the test. Only by actually launching a child process can the two default
// implementation branches in probeLlamaVersion ("merge stdout+stderr" and "timeout kill
// child process") be covered; simply replacing the probeLlamaVersion variable cannot
// reach those branches.
func buildProbeHelper(t *testing.T, src string) string {
	t.Helper()
	dir := t.TempDir()
	srcFile := filepath.Join(dir, "main.go")
	if err := os.WriteFile(srcFile, []byte(src), 0o644); err != nil {
		t.Fatal(err)
	}
	helper := filepath.Join(dir, "probe-helper")
	if runtime.GOOS == "windows" {
		// go build on Windows produces probe-helper.exe; probe must exec with the extension
		// for CreateProcess to find it
		helper += ".exe"
	}
	if out, err := exec.Command("go", "build", "-o", helper, srcFile).CombinedOutput(); err != nil {
		t.Fatalf("go build probe helper failed: %v\n%s", err, out)
	}
	return helper
}

// TestFillLlamaCppVersionReadsVersionFromStderr verifies version detection merges stderr:
// llama-server --version output all goes to stderr (stdout is empty); previously runCmd
// only captured stdout, causing version loss and the home page always showing "not found".
// Uses a real executable stub (writes only to stderr and exits) through the full
// fillLlamaCppVersion chain; asserts the parsed version line is
// "version: b1234 (build 1234)". The old implementation (stdout empty → fallback -v
// infinite hang) would block forever in this scenario; this test case is the regression
// guard for that root cause.
func TestFillLlamaCppVersionReadsVersionFromStderr(t *testing.T) {
	helper := buildProbeHelper(t, probeHelperStderrSrc)

	info := LlamaCppInfo{}
	fillLlamaCppVersion(&info, helper)

	want := "version: b1234 (build 1234)"
	if info.Version != want {
		t.Errorf("Version = %q, want version line extracted from stderr merge %q", info.Version, want)
	}
}

// TestFillLlamaCppVersionProbesVersionOnce verifies version detection is called exactly
// once and does not fall back to `-v`: injects a recording probeLlamaVersion (same
// package-level var style as githubReleasesAPI / renameFile / updateRepoAPI), simulating
// the worst case where --version produces no output; asserts fillLlamaCppVersion triggers
// exactly one probe and Version stays empty, not affecting Installed.
// The old implementation would execute `-v` a second time when --version output was empty
// (new llama-server -v starts a full HTTP server and runs forever, getLlamaCppInfo never
// returns); this test case guards against that regression.
func TestFillLlamaCppVersionProbesVersionOnce(t *testing.T) {
	origProbe := probeLlamaVersion
	var calls int
	probeLlamaVersion = func(_ string) string {
		calls++
		return ""
	}
	defer func() { probeLlamaVersion = origProbe }()

	info := LlamaCppInfo{}
	fillLlamaCppVersion(&info, "/fake/llama-server")

	if calls != 1 {
		t.Fatalf("probe call count = %d, want 1 (must not fall back to -v second probe when --version has no output)", calls)
	}
	if info.Version != "" {
		t.Errorf("--version no output: Version = %q, want empty", info.Version)
	}
	if info.Installed {
		t.Error("version probe failure must not affect Installed judgment, should remain false here")
	}
}

// TestProbeLlamaVersionTimeoutReturnsEmpty verifies the timeout guard: injects an
// infinite-hang stub (simulating an abnormal binary), temporarily shortens
// llamaVersionProbeTimeout to 300ms, calls the default probeLlamaVersion implementation,
// asserts the timeout-killed child process returns an empty string and the total elapsed
// time is far less than the hang duration — the detection chain is not frozen by any
// abnormal binary. Test deterministically ends: the hang stub itself sleeps 60s; if the
// timeout guard regresses, this test case would fail only after 60s, directly exposing
// the problem instead of blocking forever.
func TestProbeLlamaVersionTimeoutReturnsEmpty(t *testing.T) {
	helper := buildProbeHelper(t, probeHelperHangSrc)

	origTimeout := llamaVersionProbeTimeout
	llamaVersionProbeTimeout = 300 * time.Millisecond
	defer func() { llamaVersionProbeTimeout = origTimeout }()

	start := time.Now()
	out := probeLlamaVersion(helper)
	elapsed := time.Since(start)

	if out != "" {
		t.Errorf("probe output after timeout kill = %q, want empty", out)
	}
	if elapsed >= 10*time.Second {
		t.Errorf("timeout probe elapsed = %v, should be quickly terminated by 300ms timeout (must not block indefinitely)", elapsed)
	}
}
