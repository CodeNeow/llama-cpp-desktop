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

// TestGenerateModelsPresetFromRejectsInjection 验证 generateModelsPresetFrom
// 对含换行/首尾空白的 GPULayers / CacheType 值返回错误（#9 第二层防御）。
// 该函数是纯函数，直接写入 INI 文本，若值带换行可注入任意节/键。
func TestGenerateModelsPresetFromRejectsInjection(t *testing.T) {
	models := []ModelInfo{{Name: "m", Path: "/models/m.gguf"}}
	badCfgs := []map[string]ModelConfig{
		{"m": {GPULayers: "99\n[evil]\nmodel=/tmp/x"}},
		{"m": {CacheTypeK: "q8_0\nfoo"}},
		{"m": {CacheTypeV: "f16\nbar"}},
		{"m": {GPULayers: " 99 "}}, // 首尾空白拒绝
	}
	for i, cfgs := range badCfgs {
		if _, err := generateModelsPresetFrom(models, cfgs); err == nil {
			t.Errorf("case %d: 含非法值应返回错误", i)
		}
	}
}

// TestGenerateModelsPresetFromAliasDedup 验证别名去重（#7.1）：sanitizeAlias
// 会把空格/斜杠/大写统一为小写与 '-', 不同模型名可能碰撞出相同段名。
// 按模型顺序对已占用别名追加 -2、-3… 直到唯一，结果确定不依赖随机。
func TestGenerateModelsPresetFromAliasDedup(t *testing.T) {
	models := []ModelInfo{
		{Name: "Model v1", Path: "/models/a.gguf"},
		{Name: "Model/v1", Path: "/models/b.gguf"}, // 碰撞 → model-v1-2
		{Name: "Model-V1", Path: "/models/c.gguf"}, // 再碰撞 → model-v1-3
	}
	path, err := generateModelsPresetFrom(models, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer os.Remove(path)

	data, _ := os.ReadFile(path)
	content := string(data)

	// 三个模型必须各自拥有唯一段名，且都含自己的 model 路径
	if !strings.Contains(content, "[model-v1]\nmodel = /models/a.gguf") {
		t.Errorf("首个模型段名应为 model-v1: %q", content)
	}
	if !strings.Contains(content, "[model-v1-2]\nmodel = /models/b.gguf") {
		t.Errorf("第二个模型段名应为 model-v1-2: %q", content)
	}
	if !strings.Contains(content, "[model-v1-3]\nmodel = /models/c.gguf") {
		t.Errorf("第三个模型段名应为 model-v1-3: %q", content)
	}
}

// TestGenerateModelsPresetFromAcceptsValidValues 验证合法值（空 / auto /
// all / 0 / 正整数 / 缓存白名单）生成成功（#9 对照组）。
func TestGenerateModelsPresetFromAcceptsValidValues(t *testing.T) {
	models := []ModelInfo{{Name: "m", Path: "/models/m.gguf"}}
	cfgs := map[string]ModelConfig{
		"m": {GPULayers: "auto", CacheTypeK: "q8_0", CacheTypeV: "bf16"},
	}
	path, err := generateModelsPresetFrom(models, cfgs)
	if err != nil {
		t.Fatal(err)
	}
	defer os.Remove(path)

	data, _ := os.ReadFile(path)
	content := string(data)
	if !strings.Contains(content, "cache-type-k = q8_0\n") || !strings.Contains(content, "cache-type-v = bf16\n") {
		t.Errorf("合法缓存类型未写入: %q", content)
	}
	if strings.Contains(content, "gpu-layers") {
		t.Errorf("GPULayers=auto 不应输出 gpu-layers 行: %q", content)
	}
}
