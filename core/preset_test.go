package core

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestGenerateModelsPresetFrom 验证预设 INI 生成：节名取别名，
// model 行使用正斜杠路径，embedding 模型自动加 embeddings=true。
func TestGenerateModelsPresetFrom(t *testing.T) {
	models := []ModelInfo{
		{Name: "Qwen2.5 7B", Path: `C:\models\qwen\model.gguf`},
		{Name: "bge-small-zh", Path: `C:\models\bge\model.gguf`},
	}
	path, err := generateModelsPresetFrom(models, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer os.Remove(path)

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	content := string(data)

	if !strings.Contains(content, "[qwen2.5-7b]\n") {
		t.Errorf("缺少 qwen2.5-7b 节: %q", content)
	}
	if !strings.Contains(content, "model = C:/models/qwen/model.gguf\n") {
		t.Errorf("model 路径应转正斜杠: %q", content)
	}
	if !strings.Contains(content, "[bge-small-zh]\n") || !strings.Contains(content, "embeddings = true\n") {
		t.Errorf("embedding 模型应输出 embeddings=true: %q", content)
	}
}

// TestGenerateModelsPresetFromConfigs 验证逐模型参数完整写入预设。
func TestGenerateModelsPresetFromConfigs(t *testing.T) {
	models := []ModelInfo{{Name: "deepseek-r1", Path: "/models/deepseek.gguf"}}
	cfgs := map[string]ModelConfig{
		"deepseek-r1": {
			Threads: 8, GPULayers: "99", CtxSize: 8192, BatchSize: 512,
			UBatchSize: 256, FlashAttn: true, CacheTypeK: "q8_0", CacheTypeV: "q8_0",
			MLock: true, NoMMap: false,
		},
	}
	path, err := generateModelsPresetFrom(models, cfgs)
	if err != nil {
		t.Fatal(err)
	}
	defer os.Remove(path)

	data, _ := os.ReadFile(path)
	content := string(data)
	wantLines := []string{
		"ctx-size = 8192",
		"batch-size = 512",
		"ubatch-size = 256",
		"threads = 8",
		"gpu-layers = 99",
		"flash-attn = on",
		"cache-type-k = q8_0",
		"cache-type-v = q8_0",
		"mlock = true",
	}
	for _, w := range wantLines {
		if !strings.Contains(content, w+"\n") {
			t.Errorf("预设缺少 %q: %q", w, content)
		}
	}
	if strings.Contains(content, "no-mmap") {
		t.Errorf("NoMMap=false 不应输出 no-mmap: %q", content)
	}
}

// TestGenerateModelsPresetFromNoConfig 验证未配置参数的模型仅输出 model 行。
func TestGenerateModelsPresetFromNoConfig(t *testing.T) {
	models := []ModelInfo{{Name: "plain", Path: "/models/plain.gguf"}}
	path, err := generateModelsPresetFrom(models, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer os.Remove(path)

	data, _ := os.ReadFile(path)
	content := string(data)
	if content != "[plain]\nmodel = /models/plain.gguf\n\n" {
		t.Errorf("预设内容不符合预期: %q", content)
	}
}

// TestGenerateModelsPresetFromEmpty 验证空模型列表返回错误。
func TestGenerateModelsPresetFromEmpty(t *testing.T) {
	if _, err := generateModelsPresetFrom(nil, nil); err == nil {
		t.Error("空模型列表应返回错误")
	}
}

// TestGenerateModelsPresetFromMMProj 验证多模态模型输出 mmproj 行。
func TestGenerateModelsPresetFromMMProj(t *testing.T) {
	dir := t.TempDir()
	mmprojPath := filepath.Join(dir, "mmproj-f16.gguf")
	if err := os.WriteFile(mmprojPath, []byte("proj"), 0644); err != nil {
		t.Fatal(err)
	}
	modelPath := filepath.Join(dir, "llava.gguf")
	if err := os.WriteFile(modelPath, []byte("model"), 0644); err != nil {
		t.Fatal(err)
	}

	models := []ModelInfo{{Name: "llava", Path: modelPath, HasMMProj: true}}
	path, err := generateModelsPresetFrom(models, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer os.Remove(path)

	data, _ := os.ReadFile(path)
	content := string(data)
	if !strings.Contains(content, "mmproj = "+filepath.ToSlash(mmprojPath)) {
		t.Errorf("预设缺少 mmproj 行: %q", content)
	}
}
