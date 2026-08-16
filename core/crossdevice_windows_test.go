//go:build windows

package core

import (
	"errors"
	"os"
	"path/filepath"
	"syscall"
	"testing"
)

// TestMoveFileWindowsCrossVolumeRealError is a cross-device save-failure regression test.
// On Windows, os.Rename across volumes wraps ERROR_NOT_SAME_DEVICE (winerror.h 17),
// while Go defines syscall.EXDEV as a synthetic constant (536871040) — the real error
// never matches, so moveFile's old errors.Is(err, syscall.EXDEV) check never fired.
// Assertion: when injected with the real error shape, moveFile falls back to copy + delete source.
func TestMoveFileWindowsCrossVolumeRealError(t *testing.T) {
	src := filepath.Join(t.TempDir(), "src.exe")
	dst := filepath.Join(t.TempDir(), "dst.exe")
	payload := []byte("update exe payload")
	if err := os.WriteFile(src, payload, 0644); err != nil {
		t.Fatal(err)
	}

	origRename := renameFile
	renameFile = func(oldpath, newpath string) error {
		return &os.LinkError{Op: "rename", Old: oldpath, New: newpath, Err: syscall.Errno(17)}
	}
	defer func() { renameFile = origRename }()

	if err := moveFile(src, dst); err != nil {
		t.Fatalf("Windows real cross-volume error should trigger copy fallback, not return error: %v", err)
	}
	got, err := os.ReadFile(dst)
	if err != nil {
		t.Fatalf("failed to read destination: %v", err)
	}
	if string(got) != string(payload) {
		t.Errorf("destination content = %q, want %q", got, payload)
	}
	if _, err := os.Stat(src); !errors.Is(err, os.ErrNotExist) {
		t.Errorf("source should be deleted, stat err = %v", err)
	}
}
