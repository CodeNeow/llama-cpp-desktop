//go:build !windows

package core

import "syscall"

// crossDeviceRenameErr is the real cross-device rename error (EXDEV) on
// non-Windows platforms, used by moveFile to decide whether to fall back to
// copy + delete.
var crossDeviceRenameErr error = syscall.EXDEV
