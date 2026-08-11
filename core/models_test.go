package core

import (
	"os"
	"path/filepath"
	"sync"
	"testing"
)

// saveModelsState 记录 cachedModels/modelsCacheValid 全局状态并在测试
// 结束后恢复。
func saveModelsState(t *testing.T) {
	t.Helper()
	modelsMu.Lock()
	origModels := cachedModels
	origValid := modelsCacheValid.Load()
	modelsMu.Unlock()
	t.Cleanup(func() {
		modelsMu.Lock()
		cachedModels = origModels
		modelsCacheValid.Store(origValid)
		modelsMu.Unlock()
	})
}

// TestConcurrentGetRefreshModels 验证并发 GetModels/RefreshModels 多轮
// 不 panic 且结果一致（#4）。此前 modelsOnce 重置后 cachedModels 无锁
// 读写，RefreshModels 重扫写入与 GetModels 返回同一底层数组并发会数据
// 竞争；重构后写与读都在 modelsMu 内完成，GetModels 返回拷贝副本。
// 使用空 LLM-Models 目录，扫描结果恒为空数组，便于断言一致性。
func TestConcurrentGetRefreshModels(t *testing.T) {
	withTempCwd(t)
	saveModelsState(t)

	app := &App{}
	var wg sync.WaitGroup
	for i := 0; i < 200; i++ {
		wg.Add(2)
		go func() {
			defer wg.Done()
			app.GetModels()
		}()
		go func() {
			defer wg.Done()
			app.RefreshModels()
		}()
	}
	wg.Wait()

	// 空目录下每次扫描都应得到空列表，且 GetModels 返回副本
	models := app.GetModels()
	if len(models) != 0 {
		t.Errorf("空 LLM-Models 目录应返回 0 个模型, 实际 %d", len(models))
	}
}

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

// makeLooseGGUF 在 base/author 下直接写入一个散 .gguf 文件（两级结构，
// 对应旧版下载器落地路径 <author>/<file>.gguf）。
func makeLooseGGUF(t *testing.T, base, author, ggufName string, size int) {
	t.Helper()
	dir := filepath.Join(base, author)
	if err := os.MkdirAll(dir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, ggufName), make([]byte, size), 0644); err != nil {
		t.Fatal(err)
	}
}

// TestScanModelsDirTwoLevel 验证 <author>/<file>.gguf 两级结构被识别为模型：
// 无 GGUF 元数据时名称回退为文件名去掉 .gguf 扩展名，路径指向散文件本身。
func TestScanModelsDirTwoLevel(t *testing.T) {
	base := t.TempDir()
	makeLooseGGUF(t, base, "unsloth", "Qwen3.5-4B-UD-IQ2_XXS.gguf", 100)

	models := scanModelsDir(base)
	if len(models) != 1 {
		t.Fatalf("扫描到 %d 个模型, want 1", len(models))
	}
	if models[0].Name != "Qwen3.5-4B-UD-IQ2_XXS" {
		t.Errorf("Name = %q, want Qwen3.5-4B-UD-IQ2_XXS（文件名去 .gguf）", models[0].Name)
	}
	if models[0].Author != "unsloth" {
		t.Errorf("Author = %q, want unsloth", models[0].Author)
	}
	if models[0].Path != filepath.Join(base, "unsloth", "Qwen3.5-4B-UD-IQ2_XXS.gguf") {
		t.Errorf("Path = %q, want 两级完整路径", models[0].Path)
	}
	if models[0].SizeHuman == "" {
		t.Error("SizeHuman 不应为空")
	}
}

// TestScanModelsDirTwoLevelGGUFMeta 验证两级结构下读取 GGUF 头部元数据
// 覆盖名称/架构/量化（与三级 TestScanModelsDirGGUFMeta 共用 buildModelInfo，
// 行为一致）。
func TestScanModelsDirTwoLevelGGUFMeta(t *testing.T) {
	base := t.TempDir()
	authorDir := filepath.Join(base, "qwen")
	if err := os.MkdirAll(authorDir, 0755); err != nil {
		t.Fatal(err)
	}
	writeTempGGUF(t, authorDir, "model.gguf", buildGGUF(3,
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

// TestScanModelsDirTwoLevelMMProj 验证两级结构下 author 目录中的
// mmproj-*.gguf 散文件不算主模型（与三级 variant 目录内 mmproj 不充当
// 主模型一致；两级下 mmproj 散文件无法关联到模型，直接跳过）。
func TestScanModelsDirTwoLevelMMProj(t *testing.T) {
	base := t.TempDir()
	authorDir := filepath.Join(base, "llava")
	if err := os.MkdirAll(authorDir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(authorDir, "mmproj-f16.gguf"), []byte("proj"), 0644); err != nil {
		t.Fatal(err)
	}

	models := scanModelsDir(base)
	if len(models) != 0 {
		t.Errorf("仅 mmproj 散文件应扫到 0 个模型, 实际 %d", len(models))
	}
}

// TestScanModelsDirMixedLayout 验证两级散 .gguf 与三级 variant 子目录在同一
// author 目录下共存时都被识别，合并后统一按 SizeBytes 降序排序。
func TestScanModelsDirMixedLayout(t *testing.T) {
	base := t.TempDir()
	// 两级散文件（大）
	makeLooseGGUF(t, base, "author1", "loose-big.gguf", 2048)
	// 三级 variant 目录（小）
	makeVariant(t, base, "author1", "variant-small", "small.gguf", 512)

	models := scanModelsDir(base)
	if len(models) != 2 {
		t.Fatalf("扫描到 %d 个模型, want 2", len(models))
	}
	// 按大小降序：2048 > 512，两级与三级产物统一排序
	if models[0].Name != "loose-big" {
		t.Errorf("models[0].Name = %q, want loose-big（两级散文件按大小应排首位）", models[0].Name)
	}
	if models[1].Name != "variant-small" {
		t.Errorf("models[1].Name = %q, want variant-small", models[1].Name)
	}
}
