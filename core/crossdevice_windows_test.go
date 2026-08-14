//go:build windows

package core

import (
	"errors"
	"os"
	"path/filepath"
	"syscall"
	"testing"
)

// TestMoveFileWindowsCrossVolumeRealError 是跨盘保存失败的回归测试：
// Windows 上 os.Rename 跨盘的真实错误是 *os.LinkError 包裹
// ERROR_NOT_SAME_DEVICE（winerror.h 值 17），而 Go 在 Windows 定义的
// syscall.EXDEV 是发明常量（536871040），真实错误与之永不相等——
// 此前 moveFile 用 errors.Is(err, syscall.EXDEV) 判断跨设备，
// 在 Windows 上永不命中，导致更新下载跨盘保存直接报
// "The system cannot move the file to a different disk drive"。
// 断言：注入真实错误形态时 moveFile 回退为复制 + 删除源文件。
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
		t.Fatalf("Windows 真实跨盘错误应触发复制回退而非返回错误: %v", err)
	}
	got, err := os.ReadFile(dst)
	if err != nil {
		t.Fatalf("读取目标文件失败: %v", err)
	}
	if string(got) != string(payload) {
		t.Errorf("目标内容 = %q, want %q", got, payload)
	}
	if _, err := os.Stat(src); !errors.Is(err, os.ErrNotExist) {
		t.Errorf("源文件应已被删除, stat err = %v", err)
	}
}
