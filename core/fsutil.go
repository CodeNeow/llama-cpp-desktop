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
	"sync"
	"syscall"
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

// atomicWriteMu serializes the swap step of atomicWriteFile process-wide (see
// atomicWriteFile's doc comment for the Windows same-destination rename-race
// rationale). Only the swap is covered — the preceding write/fsync stays
// concurrent on distinct temp files, so one writer's flush latency never
// chains into another's. Lock ordering: it is always the innermost lock; it
// is never held across the fsync, and atomicWriteFile never calls back into
// config/task code while holding it.
var atomicWriteMu sync.Mutex

// atomicRename is the raw atomic-swap step of atomicWriteFile (os.Rename),
// declared as a package-level var so tests can force the swap to fail and
// exercise the cleanup path (same injection-point style as renameFile /
// killProcessByPid / updateLauncher; deliberately a separate var from
// moveFile's renameFile so one test's injection never leaks into the other's
// path). atomicWriteFile routes through atomicRenameWithRetry, which calls
// this var — injected non-transient failures return without retries.
var atomicRename = os.Rename

// atomicRenameRetryDelays bounds the transient-error retry of the atomic
// swap: one initial attempt plus one retry per delay entry (5 attempts,
// ~30ms worst case). It is a package-level var so tests can tune or disable
// the backoff (same injection-point style as atomicRename / cmdTimeout).
var atomicRenameRetryDelays = []time.Duration{
	2 * time.Millisecond, 4 * time.Millisecond, 8 * time.Millisecond, 16 * time.Millisecond,
}

// transientRenameError reports whether err is a transient rename failure that
// a short retry can plausibly fix: on Windows, concurrent
// MoveFileEx(REPLACE_EXISTING) operations on the SAME destination file
// transiently fail with ERROR_ACCESS_DENIED (Errno 5) or
// ERROR_SHARING_VIOLATION (Errno 32) while the previous winner's handle on the
// destination is not yet fully released for DELETE (observed as "Access is
// denied" when concurrent saveConfig calls race the swap). Detected via
// numeric syscall.Errno so the code stays uniform across platforms (no build
// tags): on POSIX those Errno values mean EIO/EPIPE, which rename(2) does not
// produce in this pattern, and a bounded retry of a genuine failure is
// harmless. Same OS-error-number matching approach as crossdevice_windows.go's
// cross-device detection, minus the platform-file split.
func transientRenameError(err error) bool {
	return err != nil &&
		(errors.Is(err, syscall.Errno(5)) || errors.Is(err, syscall.Errno(32)))
}

// atomicRenameWithRetry performs the atomic swap with a bounded retry for the
// transient Windows errors above: a short backoff lets the previous winner's
// handle on the destination be released. Intra-process writers of the same
// target are already serialized by atomicWriteMu, so the retry is a safety
// net (cross-process overlap, future callers bypassing the lock); POSIX
// rename(2) has no such race, so the retry never fires there. Any
// non-transient error returns immediately, and if all attempts fail the last
// error is returned.
func atomicRenameWithRetry(oldPath, newPath string) error {
	err := atomicRename(oldPath, newPath)
	for _, delay := range atomicRenameRetryDelays {
		if !transientRenameError(err) {
			break
		}
		time.Sleep(delay)
		err = atomicRename(oldPath, newPath)
	}
	return err
}

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
//     REPLACE_EXISTING on Windows, rename(2) on POSIX), wrapped in a bounded
//     retry for the transient ERROR_ACCESS_DENIED / ERROR_SHARING_VIOLATION
//     that concurrent swaps on the same target produce on Windows (concurrent
//     config saves race the rename; POSIX rename(2) has no such race),
//  5. best-effort directory fsync where the platform supports it (skipped
//     silently on Windows, matching FreeToken's _fsync_dir pattern).
//
// The temp file is removed on ANY failure — never leave temp litter. The swap
// step is serialized process-wide (atomicWriteMu): on Windows, concurrent
// MoveFileEx(REPLACE_EXISTING) swaps on the SAME destination transiently fail
// with ERROR_ACCESS_DENIED / ERROR_SHARING_VIOLATION while the previous
// winner's handle is not yet released for DELETE, and the app really does
// swap the same config file from concurrent goroutines (SaveModelConfig and
// persistTasksNow both end in saveConfig). The lock covers only the swap —
// the preceding write/fsync stays concurrent, on distinct temp files, so
// concurrent writers' latency does not compound — and the bounded retry in
// atomicRenameWithRetry remains as a safety net for any overlap the lock
// cannot see.
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
	// Serialize only the swap: concurrent MoveFileEx(REPLACE_EXISTING) swaps
	// on the same destination transiently fail on Windows (see the
	// atomicWriteMu doc comment). Holding the lock across the rename alone
	// removes the intra-process race without serializing the fsyncs.
	atomicWriteMu.Lock()
	swapErr := atomicRenameWithRetry(tmpName, path)
	atomicWriteMu.Unlock()
	if swapErr != nil {
		os.Remove(tmpName)
		return fmt.Errorf("replace %s with temp file: %w", path, swapErr)
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
