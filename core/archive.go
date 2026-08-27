package core

import (
	"archive/tar"
	"archive/zip"
	"compress/gzip"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

// ─── Archive extraction ─────────────────────────────────────────
// Safe zip / tar.gz extraction with per-file and total size caps against
// extraction bombs.

// Extraction size caps (declared as vars so tests can shrink them for
// verification). Prevent zip/tar extraction bombs from filling the disk or
// exhausting memory (#2).
var maxExtractFileSize int64 = 4 << 30   // per-file extraction cap: 4GB
var maxExtractTotalSize int64 = 16 << 30 // per-run total extraction cap: 16GB

func extractZip(src, dest string) error {
	r, err := zip.OpenReader(src)
	if err != nil {
		return err
	}
	defer r.Close()

	var totalBytes int64
	for _, f := range r.File {
		path := filepath.Join(dest, f.Name)

		// Prevent zip slip
		cleanPath := filepath.Clean(path)
		if !strings.HasPrefix(cleanPath, filepath.Clean(dest)+string(os.PathSeparator)) {
			continue
		}

		if f.FileInfo().IsDir() {
			os.MkdirAll(path, 0755)
			continue
		}

		// Reject oversized files up front by declared size, instead of
		// writing them out first and discovering the limit afterwards
		if f.UncompressedSize64 > uint64(maxExtractFileSize) {
			return fmt.Errorf(tr("文件超出解压大小上限: %s", "file exceeds the extraction size limit: %s"), path)
		}

		if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
			return err
		}

		rc, err := f.Open()
		if err != nil {
			return err
		}

		outFile, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, f.Mode())
		if err != nil {
			rc.Close()
			return err
		}

		// io.CopyN copies at most maxExtractFileSize+1 bytes: a src exactly at
		// the cap returns (max, io.EOF), a src beyond the cap returns
		// (max+1, nil); hence the n > maxExtractFileSize over-limit check (#2).
		n, copyErr := io.CopyN(outFile, rc, maxExtractFileSize+1)
		rc.Close()
		outFile.Close()
		if copyErr != nil && copyErr != io.EOF {
			return copyErr
		}
		if n > maxExtractFileSize {
			return fmt.Errorf(tr("文件超出解压大小上限: %s", "file exceeds the extraction size limit: %s"), path)
		}
		totalBytes += n
		if totalBytes > maxExtractTotalSize {
			return fmt.Errorf(tr("解压总大小超出上限: %d 字节", "total extraction size exceeds the limit: %d bytes"), totalBytes)
		}
	}
	return nil
}

func extractTarGz(src, dest string) error {
	f, err := os.Open(src)
	if err != nil {
		return err
	}
	defer f.Close()

	gzReader, err := gzip.NewReader(f)
	if err != nil {
		return err
	}
	defer gzReader.Close()

	tarReader := tar.NewReader(gzReader)
	var totalBytes int64
	for {
		header, err := tarReader.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return err
		}

		path := filepath.Join(dest, header.Name)

		// Prevent path traversal
		cleanPath := filepath.Clean(path)
		if !strings.HasPrefix(cleanPath, filepath.Clean(dest)+string(os.PathSeparator)) {
			continue
		}

		switch header.Typeflag {
		case tar.TypeDir:
			os.MkdirAll(path, 0755)
		case tar.TypeReg:
			// Reject oversized files up front by declared entry size (#2)
			if header.Size > maxExtractFileSize {
				return fmt.Errorf(tr("文件超出解压大小上限: %s", "file exceeds the extraction size limit: %s"), path)
			}
			if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
				return err
			}
			outFile, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, os.FileMode(header.Mode))
			if err != nil {
				return err
			}
			n, copyErr := io.CopyN(outFile, tarReader, maxExtractFileSize+1)
			outFile.Close()
			if copyErr != nil && copyErr != io.EOF {
				return copyErr
			}
			if n > maxExtractFileSize {
				return fmt.Errorf(tr("文件超出解压大小上限: %s", "file exceeds the extraction size limit: %s"), path)
			}
			totalBytes += n
			if totalBytes > maxExtractTotalSize {
				return fmt.Errorf(tr("解压总大小超出上限: %d 字节", "total extraction size exceeds the limit: %d bytes"), totalBytes)
			}
		default:
			// Explicitly reject symlinks/hardlinks/device files and other
			// unknown types, avoiding silently skipped entries that would
			// leave an incomplete extraction or a potential security issue (#6)
			return fmt.Errorf(tr("不支持的 tar 条目类型 %d: %s", "unsupported tar entry type %d: %s"), header.Typeflag, header.Name)
		}
	}
	return nil
}
