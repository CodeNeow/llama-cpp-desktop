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
	// 用相对路径经 filepath.Join 构造（Windows 产出反斜杠、Unix 产出
	// 正斜杠），断言 INI 中 model 行等于 filepath.ToSlash(path)，跨平台
	// 均验证「model 路径使用正斜杠」。
	models := []ModelInfo{
		{Name: "Qwen2.5 7B", Path: filepath.Join("models", "qwen", "model.gguf")},
		{Name: "bge-small-zh", Path: filepath.Join("models", "bge", "model.gguf")},
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
	if !strings.Contains(content, "model = "+filepath.ToSlash(models[0].Path)+"\n") {
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
			LoadMode: "mlock",
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
		"load-mode = mlock",
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

// TestGenerateModelsPresetNewFields 验证 b10342 新字段写入预设 INI：
// 全部非默认值时逐行输出；旧 mlock/noMmap 兼容字段不再直接写入（迁移后
// 由 LoadMode 承担），避免废弃键重新进入预设。
func TestGenerateModelsPresetNewFields(t *testing.T) {
	models := []ModelInfo{{Name: "m", Path: "/models/m.gguf"}}

	t.Run("all new fields", func(t *testing.T) {
		cfgs := map[string]ModelConfig{
			"m": {
				LoadMode: "mlock", CPUMoe: true, NCpuMoe: 2, SplitMode: "row",
				TensorSplit: "3,1", MainGPU: 1, RopeScaling: "yarn", RopeScale: 2.0,
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
			"load-mode = mlock",
			"cpu-moe = on",
			"n-cpu-moe = 2",
			"split-mode = row",
			"tensor-split = 3,1",
			"main-gpu = 1",
			"rope-scaling = yarn",
			"rope-scale = 2",
		}
		for _, w := range wantLines {
			if !strings.Contains(content, w+"\n") {
				t.Errorf("预设缺少 %q: %q", w, content)
			}
		}
	})

	t.Run("defaults not written", func(t *testing.T) {
		// LoadMode=mmap/空、SplitMode=layer/空、MainGPU=0、RopeScale=0、
		// CPUMoe=false 等默认值不应产生键，避免预设噪音。
		for _, lm := range []string{"", "mmap"} {
			for _, sm := range []string{"", "layer"} {
				cfgs := map[string]ModelConfig{
					"m": {LoadMode: lm, SplitMode: sm, MainGPU: 0, RopeScale: 0, RopeScaling: "none"},
				}
				path, err := generateModelsPresetFrom(models, cfgs)
				if err != nil {
					t.Fatal(err)
				}
				data, _ := os.ReadFile(path)
				content := string(data)
				for _, banned := range []string{"load-mode", "split-mode", "cpu-moe", "main-gpu", "rope-scale", "rope-scaling"} {
					if strings.Contains(content, banned+" =") {
						t.Errorf("默认值不应输出 %q（load-mode=%q split-mode=%q）: %q", banned, lm, sm, content)
					}
				}
				os.Remove(path)
			}
		}
	})

	t.Run("legacy mlock noMmap not written", func(t *testing.T) {
		// 模拟迁移后状态：旧布尔清零、LoadMode 已派生；即使兼容字段残留
		// 为 true，预设也只写 load-mode，绝不写废弃键 mlock/no-mmap。
		cfgs := map[string]ModelConfig{
			"m": {LoadMode: "mlock", MLock: true, NoMMap: true},
		}
		path, err := generateModelsPresetFrom(models, cfgs)
		if err != nil {
			t.Fatal(err)
		}
		defer os.Remove(path)

		data, _ := os.ReadFile(path)
		content := string(data)
		if !strings.Contains(content, "load-mode = mlock\n") {
			t.Errorf("迁移后应输出 load-mode = mlock: %q", content)
		}
		if strings.Contains(content, "mlock =") {
			t.Errorf("兼容字段 MLock 不应直接输出 mlock 键: %q", content)
		}
		if strings.Contains(content, "no-mmap") {
			t.Errorf("兼容字段 NoMMap 不应直接输出 no-mmap 键: %q", content)
		}
	})
}

// TestValidCacheTypeValueExtended 验证 cache-type 白名单在 b10342 下扩展：
// 新增 f32/q4_1/iq4_nl/q5_0/q5_1 均应合法，列表外取值非法。
func TestValidCacheTypeValueExtended(t *testing.T) {
	for _, v := range []string{"", "f32", "f16", "bf16", "q8_0", "q4_0", "q4_1", "iq4_nl", "q5_0", "q5_1"} {
		if !validCacheTypeValue(v) {
			t.Errorf("validCacheTypeValue(%q) 应为 true（b10342 支持）", v)
		}
	}
	for _, v := range []string{"q4_2", "q6_k", "f32\nx", " q8_0"} {
		if validCacheTypeValue(v) {
			t.Errorf("validCacheTypeValue(%q) 应为 false", v)
		}
	}
}

// TestValidLoadModeValue 验证 load-mode 白名单（b10342 替代 mlock/no-mmap）。
func TestValidLoadModeValue(t *testing.T) {
	for _, v := range []string{"", "none", "mmap", "mlock", "mmap+mlock", "dio"} {
		if !validLoadModeValue(v) {
			t.Errorf("validLoadModeValue(%q) 应为 true", v)
		}
	}
	for _, v := range []string{"foo", "mmap+", " mlock", "mlock\n"} {
		if validLoadModeValue(v) {
			t.Errorf("validLoadModeValue(%q) 应为 false", v)
		}
	}
}

// TestValidSplitModeValue 验证 split-mode 白名单（多 GPU 切分策略）。
func TestValidSplitModeValue(t *testing.T) {
	for _, v := range []string{"", "none", "layer", "row", "tensor"} {
		if !validSplitModeValue(v) {
			t.Errorf("validSplitModeValue(%q) 应为 true", v)
		}
	}
	for _, v := range []string{"layers", "column", " row"} {
		if validSplitModeValue(v) {
			t.Errorf("validSplitModeValue(%q) 应为 false", v)
		}
	}
}

// TestValidRopeScalingValue 验证 rope-scaling 白名单（长上下文外推）。
func TestValidRopeScalingValue(t *testing.T) {
	for _, v := range []string{"", "none", "linear", "yarn"} {
		if !validRopeScalingValue(v) {
			t.Errorf("validRopeScalingValue(%q) 应为 true", v)
		}
	}
	for _, v := range []string{"dynamic", "linear2", " yarn"} {
		if validRopeScalingValue(v) {
			t.Errorf("validRopeScalingValue(%q) 应为 false", v)
		}
	}
}
