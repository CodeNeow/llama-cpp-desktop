package core

import (
	"archive/tar"
	"archive/zip"
	"bytes"
	"compress/gzip"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestExtractZipFileTooLarge 验证 extractZip 拒绝超过单文件上限的条目。
// 测试把 maxExtractFileSize 临时改小到 16 字节，构造含 100 字节文件的 zip
// fixture，断言返回错误且目标文件未被写出（#2 解压炸弹单文件限制）。
func TestExtractZipFileTooLarge(t *testing.T) {
	orig := maxExtractFileSize
	maxExtractFileSize = 16
	defer func() { maxExtractFileSize = orig }()

	zipPath := filepath.Join(t.TempDir(), "big.zip")
	if err := writeTestZip(zipPath, map[string]string{"big.bin": strings.Repeat("A", 100)}); err != nil {
		t.Fatal(err)
	}

	dest := t.TempDir()
	err := extractZip(zipPath, dest)
	if err == nil {
		t.Fatal("超过单文件上限的 zip 应返回错误")
	}
	if !strings.Contains(err.Error(), "超出解压大小上限") {
		t.Errorf("错误信息应说明大小超限: %v", err)
	}
	if _, statErr := os.Stat(filepath.Join(dest, "big.bin")); statErr == nil {
		t.Error("超限文件不应被写出到磁盘")
	}
}

// TestExtractZipTotalTooLarge 验证 extractZip 累计解压总大小超过上限时
// 返回错误（#2 解压炸弹总大小限制）。两个小文件各自未超单文件上限，
// 但累计后触发 totalBytes > maxExtractTotalSize。
func TestExtractZipTotalTooLarge(t *testing.T) {
	origFile, origTotal := maxExtractFileSize, maxExtractTotalSize
	maxExtractFileSize = 16
	maxExtractTotalSize = 25
	defer func() {
		maxExtractFileSize = origFile
		maxExtractTotalSize = origTotal
	}()

	zipPath := filepath.Join(t.TempDir(), "total.zip")
	if err := writeTestZip(zipPath, map[string]string{
		"a.bin": strings.Repeat("A", 15),
		"b.bin": strings.Repeat("B", 15),
	}); err != nil {
		t.Fatal(err)
	}

	if err := extractZip(zipPath, t.TempDir()); err == nil {
		t.Error("累计总大小超限的 zip 应返回错误")
	}
}

// TestExtractZipOK 验证未超限的 zip 正常解压（#2 的对照组：上限不误伤
// 正常小文件）。
func TestExtractZipOK(t *testing.T) {
	orig := maxExtractFileSize
	maxExtractFileSize = 16
	defer func() { maxExtractFileSize = orig }()

	zipPath := filepath.Join(t.TempDir(), "small.zip")
	if err := writeTestZip(zipPath, map[string]string{"ok.bin": strings.Repeat("A", 10)}); err != nil {
		t.Fatal(err)
	}

	dest := t.TempDir()
	if err := extractZip(zipPath, dest); err != nil {
		t.Fatalf("小文件 zip 不应报错: %v", err)
	}
	data, err := os.ReadFile(filepath.Join(dest, "ok.bin"))
	if err != nil {
		t.Fatalf("解压产物缺失: %v", err)
	}
	if len(data) != 10 {
		t.Errorf("文件内容长度 = %d, want 10", len(data))
	}
}

// TestExtractTarGzSymlinkRejected 验证 extractTarGz 对 symlink 条目返回
// 错误（#6）。默认分支仅处理目录与普通文件，符号链接等未知类型必须显式
// 拒绝而非静默跳过，否则解压产物不完整且可能造成误写。
func TestExtractTarGzSymlinkRejected(t *testing.T) {
	tarPath := filepath.Join(t.TempDir(), "link.tar.gz")
	if err := writeTestTarGz(tarPath, []tarFixture{
		{Name: "link", Typeflag: tar.TypeSymlink, Linkname: "/etc/passwd", Size: 0},
	}); err != nil {
		t.Fatal(err)
	}

	if err := extractTarGz(tarPath, t.TempDir()); err == nil {
		t.Error("含 symlink 条目的 tar.gz 应返回错误")
	}
}

// TestExtractTarGzFileTooLarge 验证 extractTarGz 拒绝超过单文件上限的
// 条目（#2）。tar 头声明大小超过上限时提前拦截，不写盘。
func TestExtractTarGzFileTooLarge(t *testing.T) {
	orig := maxExtractFileSize
	maxExtractFileSize = 16
	defer func() { maxExtractFileSize = orig }()

	tarPath := filepath.Join(t.TempDir(), "big.tar.gz")
	if err := writeTestTarGz(tarPath, []tarFixture{
		{Name: "big.bin", Typeflag: tar.TypeReg, Size: 100},
	}); err != nil {
		t.Fatal(err)
	}

	if err := extractTarGz(tarPath, t.TempDir()); err == nil {
		t.Error("超过单文件上限的 tar.gz 应返回错误")
	}
}

// writeTestZip 用内存构造一个含给定文件的 zip 归档并写入 zipPath。
func writeTestZip(zipPath string, files map[string]string) error {
	var buf bytes.Buffer
	w := zip.NewWriter(&buf)
	for name, content := range files {
		fw, err := w.Create(name)
		if err != nil {
			return err
		}
		if _, err := fw.Write([]byte(content)); err != nil {
			return err
		}
	}
	if err := w.Close(); err != nil {
		return err
	}
	return os.WriteFile(zipPath, buf.Bytes(), 0644)
}

// tarFixture 描述一个要写入 tar.gz 的条目。
type tarFixture struct {
	Name     string
	Typeflag byte
	Linkname string
	Size     int64
}

// writeTestTarGz 用内存构造含给定条目的 tar.gz 并写入 tarPath。
func writeTestTarGz(tarPath string, fixtures []tarFixture) error {
	var buf bytes.Buffer
	gz := gzip.NewWriter(&buf)
	tw := tar.NewWriter(gz)
	for _, f := range fixtures {
		hdr := &tar.Header{Name: f.Name, Typeflag: f.Typeflag, Linkname: f.Linkname, Size: f.Size, Mode: 0644}
		if err := tw.WriteHeader(hdr); err != nil {
			return err
		}
		if f.Size > 0 {
			if _, err := tw.Write(make([]byte, f.Size)); err != nil {
				return err
			}
		}
	}
	if err := tw.Close(); err != nil {
		return err
	}
	if err := gz.Close(); err != nil {
		return err
	}
	return os.WriteFile(tarPath, buf.Bytes(), 0644)
}
