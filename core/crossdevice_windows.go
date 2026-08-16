//go:build windows

package core

import "syscall"

// crossDeviceRenameErr is the real Windows error for os.Rename across drives:
// ERROR_NOT_SAME_DEVICE (winerror.h 17). Go's syscall.EXDEV on Windows is a
// fabricated constant (536871040) that never equals the real code, so rename
// fallback must use this constant instead of syscall.EXDEV.
var crossDeviceRenameErr error = syscall.Errno(17)
