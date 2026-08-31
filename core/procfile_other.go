//go:build !android

package core

// readProcFile reads a kernel pseudo-file via `cat` on platforms that can
// spawn child processes (Windows / desktop Linux / macOS). Android overrides
// this with a direct os.ReadFile — its app sandbox cannot exec (see
// procfile_android.go).
func readProcFile(path string) string {
	return runCmd("cat", path)
}
