package core

import (
	"os"
	"path/filepath"
	"testing"
)

// makeVariant 在 base/author/variant 下创建模型目录并写入 gguf 文件，
// size 用于控制文件大小（排序断言依据）。
func makeVariant(t *testing.T, base, author, variant string, ggufName string, size int) {
	t.Helper()
	dir := filepath.Join(base, author, variant)
	if err := os.MkdirAll(dir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, ggufName), make([]byte, size), 0644); err != nil {
		t.Fatal(err)
	}
}

// TestScanModelsDir 验证目录扫描：<author>/<variant>/.gguf 结构被识别为
// 一个模型，名称取 variant 目录名，并按大小降序排列。
func TestScanModelsDir(t *testing.T) {
	base := t.TempDir()
	makeVariant(t, base, "author1", "big-model", "big.gguf", 2048)
	makeVariant(t, base, "author1", "small-model", "small.gguf", 512)
	makeVariant(t, base, "author2", "third-model", "third.gguf", 1024)

	models := scanModelsDir(base)
	if len(models) != 3 {
		t.Fatalf("扫描到 %d 个模型, want 3", len(models))
	}
	// 按大小降序：2048 > 1024 > 512
	if models[0].Name != "big-model" || models[1].Name != "third-model" || models[2].Name != "small-model" {
		t.Errorf("排序错误: %v", []string{models[0].Name, models[1].Name, models[2].Name})
	}
	if models[0].Author != "author1" {
		t.Errorf("author = %q, want author1", models[0].Author)
	}
	if models[0].SizeHuman == "" {
		t.Error("SizeHuman 不应为空")
	}
	if models[0].Path == "" || filepath.Dir(models[0].Path) == "" {
		t.Error("Path 应为完整路径")
	}
}

// TestScanModelsDirMMProj 验证 mmproj 文件标记多模态支持且不被当作主模型。
func TestScanModelsDirMMProj(t *testing.T) {
	base := t.TempDir()
	dir := filepath.Join(base, "llava", "llava-v1.6")
	if err := os.MkdirAll(dir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "llava-v1.6.gguf"), []byte("model"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "mmproj-f16.gguf"), []byte("proj"), 0644); err != nil {
		t.Fatal(err)
	}

	models := scanModelsDir(base)
	if len(models) != 1 {
		t.Fatalf("扫描到 %d 个模型, want 1", len(models))
	}
	if !models[0].HasMMProj {
		t.Error("含 mmproj 文件的模型应标记 HasMMProj")
	}
}

// TestScanModelsDirEmpty 验证空目录与无 GGUF 文件的目录返回空列表。
func TestScanModelsDirEmpty(t *testing.T) {
	base := t.TempDir()
	if models := scanModelsDir(base); len(models) != 0 {
		t.Errorf("空目录应返回 0 个模型, 实际 %d", len(models))
	}
	makeVariant(t, base, "a", "v", "readme.txt", 100) // 非 gguf
	if models := scanModelsDir(base); len(models) != 0 {
		t.Errorf("无 gguf 文件的目录应返回 0 个模型, 实际 %d", len(models))
	}
}

// TestScanModelsDirGGUFMeta 验证扫描时读取 GGUF 头部元数据覆盖名称与量化。
func TestScanModelsDirGGUFMeta(t *testing.T) {
	base := t.TempDir()
	dir := filepath.Join(base, "qwen", "qwen2.5-7b")
	if err := os.MkdirAll(dir, 0755); err != nil {
		t.Fatal(err)
	}
	writeTempGGUF(t, dir, "model.gguf", buildGGUF(3,
		strKV("general.name", "Qwen2.5-7B-Instruct"),
		strKV("general.architecture", "Qwen"),
		u32KV("general.file_type", 15),
	))

	models := scanModelsDir(base)
	if len(models) != 1 {
		t.Fatalf("扫描到 %d 个模型, want 1", len(models))
	}
	if models[0].Name != "Qwen2.5-7B-Instruct" {
		t.Errorf("Name = %q, want Qwen2.5-7B-Instruct（应取 GGUF 元数据）", models[0].Name)
	}
	if models[0].Architecture != "Qwen" {
		t.Errorf("Architecture = %q, want Qwen", models[0].Architecture)
	}
	if models[0].Quantization != "Q4_K_M" {
		t.Errorf("Quantization = %q, want Q4_K_M", models[0].Quantization)
	}
}

// TestScanModelsDirQuantFallback 验证无 GGUF 元数据时从目录名/文件名推断量化。
func TestScanModelsDirQuantFallback(t *testing.T) {
	base := t.TempDir()
	makeVariant(t, base, "bge", "bge-small-zh-v1.5", "model-q8_0.gguf", 100)

	models := scanModelsDir(base)
	if len(models) != 1 {
		t.Fatalf("扫描到 %d 个模型, want 1", len(models))
	}
	if models[0].Name != "bge-small-zh-v1.5" {
		t.Errorf("Name = %q, want bge-small-zh-v1.5", models[0].Name)
	}
	// 目录名不含量化，回退到文件名 model-q8_0.gguf
	if models[0].Quantization != "Q8_0" {
		t.Errorf("Quantization = %q, want Q8_0（文件名回退）", models[0].Quantization)
	}
}
