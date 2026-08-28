package core

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"
)

// ─── Shared small helpers ───────────────────────────────────────
// Cross-domain process/file/format helpers shared by the domain files:
// runCmd, copyFile, moveFile, formatBytes, countString.

// cmdTimeout is the upper bound for runCmd child-process execution. System
// collection queries (WMI/CIM via powershell, nvidia-smi, sysctl, ...) are
// expected to return in well under a second; a hung query (e.g. the WMI
// service stalling) must not freeze the whole info fetch, so the child is
// killed and runCmd returns "" (callers fall back to their defaults). It is a
// package-level var so tests can shorten it (same style as
// llamaVersionProbeTimeout).
var cmdTimeout = 8 * time.Second

func runCmd(name string, args ...string) string {
	ctx, cancel := context.WithTimeout(context.Background(), cmdTimeout)
	defer cancel()
	cmd := exec.CommandContext(ctx, name, args...)
	hideWindow(cmd)
	var out, errOut bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &errOut
	if err := cmd.Run(); err != nil {
		if ctx.Err() == context.DeadlineExceeded {
			log.Printf("[WARN] runCmd timeout after %v: %s %v", cmdTimeout, name, args)
			return "" // timed-out output may be truncated; treat as unavailable
		}
		if errOut.Len() > 0 {
			log.Printf("[CMD] %s %v stderr: %s", name, args, strings.TrimSpace(errOut.String()))
		}
	}
	return strings.TrimSpace(out.String())
}

func countString(out, substr string) int {
	count := 0
	for _, line := range strings.Split(out, "\n") {
		if strings.Contains(line, substr) {
			count++
		}
	}
	return count
}

func formatBytes(bytes int64) string {
	const unit = 1024
	if bytes < unit {
		return fmt.Sprintf("%d B", bytes)
	}
	div, exp := int64(unit), 0
	for n := bytes / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %cB", float64(bytes)/float64(div), "KMGTPE"[exp])
}

// copyFile copies src to dst: creates dst with src's FileMode and explicitly
// chmods, preserving the executable permission (updating the exe on Linux
// needs +x), independent of umask.
func copyFile(src, dst string) error {
	srcF, err := os.Open(src)
	if err != nil {
		return fmt.Errorf(tr("打开源文件失败: %w", "failed to open source file: %w"), err)
	}
	defer srcF.Close()

	fi, err := srcF.Stat()
	if err != nil {
		return fmt.Errorf(tr("读取源文件信息失败: %w", "failed to read source file info: %w"), err)
	}

	dstF, err := os.OpenFile(dst, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, fi.Mode())
	if err != nil {
		return fmt.Errorf(tr("创建目标文件失败: %w", "failed to create destination file: %w"), err)
	}
	_, copyErr := io.Copy(dstF, srcF)
	closeErr := dstF.Close()
	if copyErr != nil {
		return fmt.Errorf(tr("复制文件内容失败: %w", "failed to copy file contents: %w"), copyErr)
	}
	if closeErr != nil {
		return fmt.Errorf(tr("关闭目标文件失败: %w", "failed to close destination file: %w"), closeErr)
	}
	// Explicit chmod guarantees destination permissions exactly match the
	// source (independent of umask)
	if err := os.Chmod(dst, fi.Mode()); err != nil {
		return fmt.Errorf(tr("设置目标文件权限失败: %w", "failed to set destination file permissions: %w"), err)
	}
	return nil
}

// moveFile moves src to dst: prefers renameFile (a package-level injection
// point so tests can simulate failure); across devices (Windows cross-drive
// ERROR_NOT_SAME_DEVICE / Unix cross-mount EXDEV) os.Rename always fails, so
// it falls back to copyFile + os.Remove(src), preserving source permissions.
// Cross-device detection uses the platform constant crossDeviceRenameErr: on
// Windows, syscall.EXDEV is an invented Go constant that never equals the real
// error code, so it must not be used for the check.
// Other failures (e.g. destination already exists) keep the original
// semantics: delete dst and retry renameFile once.
// Critical ordering: the cross-device check must run before the delete-old-
// and-retry path, to avoid deleting an existing old file in cross-device cases.
func moveFile(src, dst string) error {
	err := renameFile(src, dst)
	if err == nil {
		return nil
	}
	if errors.Is(err, crossDeviceRenameErr) {
		// Rename across devices is impossible: copy to the destination
		// (overwriting an existing same-name file), then remove the source
		if copyErr := copyFile(src, dst); copyErr != nil {
			return copyErr
		}
		if removeErr := os.Remove(src); removeErr != nil {
			return fmt.Errorf(tr("删除源文件失败: %w", "failed to remove source file: %w"), removeErr)
		}
		return nil
	}
	// Other failures such as destination-exists: delete the old destination
	// and retry once, consistent with the original update logic
	if removeErr := os.Remove(dst); removeErr == nil {
		return renameFile(src, dst)
	}
	return err
}

// ─── Crash-safe atomic file write ────────────────────────────────
// atomicWriteFile persists critical JSON files (app config, handover record)
// without ever exposing torn content on crash or power loss.

// atomicRename is the atomic-swap step of atomicWriteFile, declared as a
// package-level var so tests can force the swap to fail and exercise the
// cleanup path (same injection-point style as renameFile / killProcessByPid /
// updateLauncher; deliberately a separate var from moveFile's renameFile so
// one test's injection never leaks into the other's path).
var atomicRename = os.Rename

// atomicWriteFile writes data to path crash-safely. The config file (which
// also carries the persisted download-task queue) and the handover record are
// the app's critical persistence points; a crash or power loss in the middle
// of a plain os.WriteFile can tear them (truncated/partial JSON), which
// loadConfig only half-defends against and which silently drops persisted
// download tasks or breaks the GUI ↔ headless handover. The temp-then-rename
// discipline guarantees the target path only ever contains a fully written,
// flushed file:
//  1. write to a unique temp file in the SAME directory (unique suffix, so
//     concurrent writers never clobber each other's temp; same directory, so
//     the rename below is an atomic same-filesystem swap),
//  2. chmod the temp file to the exact requested perm (os.CreateTemp always
//     creates 0600; the explicit chmod is umask-independent, same as copyFile),
//  3. fsync the temp file before renaming (payload durability),
//  4. os.Rename it over the target — an atomic swap (MoveFileEx with
//     REPLACE_EXISTING on Windows, rename(2) on POSIX),
//  5. best-effort directory fsync where the platform supports it (skipped
//     silently on Windows, matching FreeToken's _fsync_dir pattern).
//
// The temp file is removed on ANY failure — never leave temp litter.
func atomicWriteFile(path string, data []byte, perm os.FileMode) error {
	dir := filepath.Dir(path)
	tmp, err := os.CreateTemp(dir, "."+filepath.Base(path)+".tmp-*")
	if err != nil {
		return fmt.Errorf("create temp file for %s: %w", path, err)
	}
	tmpName := tmp.Name()
	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		os.Remove(tmpName)
		return fmt.Errorf("write temp file for %s: %w", path, err)
	}
	if err := tmp.Chmod(perm); err != nil {
		tmp.Close()
		os.Remove(tmpName)
		return fmt.Errorf("set temp file permissions for %s: %w", path, err)
	}
	// fsync before rename: without it a crash right after the swap can leave
	// the target with empty/partial content on some filesystems.
	if err := tmp.Sync(); err != nil {
		tmp.Close()
		os.Remove(tmpName)
		return fmt.Errorf("sync temp file for %s: %w", path, err)
	}
	if err := tmp.Close(); err != nil {
		os.Remove(tmpName)
		return fmt.Errorf("close temp file for %s: %w", path, err)
	}
	if err := atomicRename(tmpName, path); err != nil {
		os.Remove(tmpName)
		return fmt.Errorf("replace %s with temp file: %w", path, err)
	}
	fsyncDir(dir)
	return nil
}

// fsyncDir flushes a directory entry so the just-renamed file's swap itself
// survives a crash (a payload fsync alone is not enough on Linux: without a
// directory fsync the renamed-in file can vanish after power loss).
// Best-effort by design: errors are swallowed — the payload is already
// fsynced and the swap is atomic — and Windows cannot flush directory handles
// at all, so the syscall is skipped there entirely.
func fsyncDir(dir string) {
	if runtime.GOOS == "windows" {
		return
	}
	f, err := os.Open(dir)
	if err != nil {
		return
	}
	defer f.Close()
	_ = f.Sync()
}
