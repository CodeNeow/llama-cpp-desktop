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
