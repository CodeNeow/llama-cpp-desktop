//go:build !windows

package core

// defaultHeadlessServerAlert is a no-op on non-Windows platforms: headless
// mode itself is Windows-only (shouldRunHeadless always returns false
// elsewhere), so the alert path is unreachable there.
func defaultHeadlessServerAlert(err error) {}
