package core

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestGenerateModelsPresetFrom verifies preset INI generation: section names use the
// alias, model lines use forward-slash paths, and embedding models automatically get
// embeddings=true.
func TestGenerateModelsPresetFrom(t *testing.T) {
	// construct paths via filepath.Join (Windows produces backslashes, Unix produces
	// forward slashes); assert INI model lines equal filepath.ToSlash(path), cross-platform
	// verification that "model paths use forward slashes".
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
		t.Errorf("missing qwen2.5-7b section: %q", content)
	}
	if !strings.Contains(content, "model = "+filepath.ToSlash(models[0].Path)+"\n") {
		t.Errorf("model path should use forward slashes: %q", content)
	}
	if !strings.Contains(content, "[bge-small-zh]\n") || !strings.Contains(content, "embeddings = true\n") {
		t.Errorf("embedding model should output embeddings=true: %q", content)
	}
}

// TestGenerateModelsPresetFromConfigs verifies per-model parameters are fully written
// into the preset.
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
			t.Errorf("preset missing %q: %q", w, content)
		}
	}
	if strings.Contains(content, "no-mmap") {
		t.Errorf("NoMMap=false should not output no-mmap: %q", content)
	}
}

// TestGenerateModelsPresetFromNoConfig verifies a model with no configured parameters
// only outputs a model line.
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
		t.Errorf("preset content does not match expected: %q", content)
	}
}

// TestGenerateModelsPresetFromEmpty verifies an empty model list returns an error.
func TestGenerateModelsPresetFromEmpty(t *testing.T) {
	if _, err := generateModelsPresetFrom(nil, nil); err == nil {
		t.Error("empty model list should return error")
	}
}

// TestGenerateModelsPresetFromMMProj verifies a multimodal model outputs an mmproj line.
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
		t.Errorf("preset missing mmproj line: %q", content)
	}
}

// TestGenerateModelsPresetFromRejectsInjection verifies generateModelsPresetFrom
// returns errors for GPULayers / CacheType values containing newlines or leading/trailing
// whitespace (#9 second defense layer).
// This function is a pure function that directly writes INI text; if values carry newlines,
// arbitrary sections/keys can be injected.
func TestGenerateModelsPresetFromRejectsInjection(t *testing.T) {
	models := []ModelInfo{{Name: "m", Path: "/models/m.gguf"}}
	badCfgs := []map[string]ModelConfig{
		{"m": {GPULayers: "99\n[evil]\nmodel=/tmp/x"}},
		{"m": {CacheTypeK: "q8_0\nfoo"}},
		{"m": {CacheTypeV: "f16\nbar"}},
		{"m": {GPULayers: " 99 "}}, // leading/trailing whitespace rejected
	}
	for i, cfgs := range badCfgs {
		if _, err := generateModelsPresetFrom(models, cfgs); err == nil {
			t.Errorf("case %d: illegal values should return error", i)
		}
	}
}

// TestGenerateModelsPresetFromAliasDedup verifies alias deduplication (#7.1): sanitizeAlias
// unifies spaces/slashes/uppercase into lowercase and '-', so different model names may
// collide into the same section name.
// Already-occupied aliases get -2, -3… appended in model order until unique; the result
// is deterministic and does not depend on randomness.
func TestGenerateModelsPresetFromAliasDedup(t *testing.T) {
	models := []ModelInfo{
		{Name: "Model v1", Path: "/models/a.gguf"},
		{Name: "Model/v1", Path: "/models/b.gguf"}, // collision → model-v1-2
		{Name: "Model-V1", Path: "/models/c.gguf"}, // another collision → model-v1-3
	}
	path, err := generateModelsPresetFrom(models, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer os.Remove(path)

	data, _ := os.ReadFile(path)
	content := string(data)

	// three models must each have a unique section name, and each must contain its own model path
	if !strings.Contains(content, "[model-v1]\nmodel = /models/a.gguf") {
		t.Errorf("first model section should be model-v1: %q", content)
	}
	if !strings.Contains(content, "[model-v1-2]\nmodel = /models/b.gguf") {
		t.Errorf("second model section should be model-v1-2: %q", content)
	}
	if !strings.Contains(content, "[model-v1-3]\nmodel = /models/c.gguf") {
		t.Errorf("third model section should be model-v1-3: %q", content)
	}
}

// TestGenerateModelsPresetFromAcceptsValidValues verifies valid values (empty / auto /
// all / 0 / positive integers / cache whitelist) generate successfully (#9 control group).
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
		t.Errorf("valid cache types not written: %q", content)
	}
	if strings.Contains(content, "gpu-layers") {
		t.Errorf("GPULayers=auto should not output gpu-layers line: %q", content)
	}
}

// TestGenerateModelsPresetNewFields verifies b10342 new fields are written to preset INI:
// all non-default values are output line-by-line; legacy mlock/noMmap compatibility
// fields are no longer written directly (post-migration LoadMode takes over), preventing
// deprecated keys from re-entering presets.
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
				t.Errorf("preset missing %q: %q", w, content)
			}
		}
	})

	t.Run("defaults not written", func(t *testing.T) {
		// LoadMode=mmap/empty, SplitMode=layer/empty, MainGPU=0, RopeScale=0,
		// CPUMoe=false etc. defaults must not produce keys, avoiding preset noise.
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
						t.Errorf("defaults must not output %q (load-mode=%q split-mode=%q): %q", banned, lm, sm, content)
					}
				}
				os.Remove(path)
			}
		}
	})

	t.Run("legacy mlock noMmap not written", func(t *testing.T) {
		// simulate post-migration state: legacy booleans zeroed, LoadMode already derived;
		// even if compatibility fields residue to true, preset only writes load-mode,
		// never writes deprecated keys mlock/no-mmap.
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
			t.Errorf("post-migration should output load-mode = mlock: %q", content)
		}
		if strings.Contains(content, "mlock =") {
			t.Errorf("compatibility field MLock must not output mlock key directly: %q", content)
		}
		if strings.Contains(content, "no-mmap") {
			t.Errorf("compatibility field NoMMap must not output no-mmap key directly: %q", content)
		}
	})
}

// TestValidCacheTypeValueExtended verifies cache-type whitelist expansion under b10342:
// newly added f32/q4_1/iq4_nl/q5_0/q5_1 must all be valid, values outside the list are illegal.
func TestValidCacheTypeValueExtended(t *testing.T) {
	for _, v := range []string{"", "f32", "f16", "bf16", "q8_0", "q4_0", "q4_1", "iq4_nl", "q5_0", "q5_1"} {
		if !validCacheTypeValue(v) {
			t.Errorf("validCacheTypeValue(%q) should be true (b10342 supported)", v)
		}
	}
	for _, v := range []string{"q4_2", "q6_k", "f32\nx", " q8_0"} {
		if validCacheTypeValue(v) {
			t.Errorf("validCacheTypeValue(%q) should be false", v)
		}
	}
}

// TestValidLoadModeValue verifies load-mode whitelist (b10342 replacement for mlock/no-mmap).
func TestValidLoadModeValue(t *testing.T) {
	for _, v := range []string{"", "none", "mmap", "mlock", "mmap+mlock", "dio"} {
		if !validLoadModeValue(v) {
			t.Errorf("validLoadModeValue(%q) should be true", v)
		}
	}
	for _, v := range []string{"foo", "mmap+", " mlock", "mlock\n"} {
		if validLoadModeValue(v) {
			t.Errorf("validLoadModeValue(%q) should be false", v)
		}
	}
}

// TestValidSplitModeValue verifies split-mode whitelist (multi-GPU split strategy).
func TestValidSplitModeValue(t *testing.T) {
	for _, v := range []string{"", "none", "layer", "row", "tensor"} {
		if !validSplitModeValue(v) {
			t.Errorf("validSplitModeValue(%q) should be true", v)
		}
	}
	for _, v := range []string{"layers", "column", " row"} {
		if validSplitModeValue(v) {
			t.Errorf("validSplitModeValue(%q) should be false", v)
		}
	}
}

// TestValidRopeScalingValue verifies rope-scaling whitelist (long-context extrapolation).
func TestValidRopeScalingValue(t *testing.T) {
	for _, v := range []string{"", "none", "linear", "yarn"} {
		if !validRopeScalingValue(v) {
			t.Errorf("validRopeScalingValue(%q) should be true", v)
		}
	}
	for _, v := range []string{"dynamic", "linear2", " yarn"} {
		if validRopeScalingValue(v) {
			t.Errorf("validRopeScalingValue(%q) should be false", v)
		}
	}
}

// TestValidSpecTypeValue verifies spec-type whitelist (MTP multi-token prediction).
func TestValidSpecTypeValue(t *testing.T) {
	for _, v := range []string{"", "draft-mtp"} {
		if !validSpecTypeValue(v) {
			t.Errorf("validSpecTypeValue(%q) should be true", v)
		}
	}
	for _, v := range []string{"draft", "mtp", "draft-mtp2", " draft-mtp"} {
		if validSpecTypeValue(v) {
			t.Errorf("validSpecTypeValue(%q) should be false", v)
		}
	}
}

// TestGenerateModelsPresetFromMMProjReasoningSpec verifies new model parameters are
// written to preset INI: explicit mmproj path (takes priority over auto-detection,
// outputs exactly one mmproj line), reasoning=off, spec-type / spec-draft-n-max.
// When MMProj is empty and HasMMProj=true, auto-detection is maintained.
func TestGenerateModelsPresetFromMMProjReasoningSpec(t *testing.T) {
	// explicit mmproj scenario: place a real mmproj file in the same directory;
	// explicit path should override auto-detection
	dir := t.TempDir()
	autoMMProj := filepath.Join(dir, "mmproj-auto.gguf")
	if err := os.WriteFile(autoMMProj, []byte("proj"), 0644); err != nil {
		t.Fatal(err)
	}
	modelPath := filepath.Join(dir, "llava.gguf")

	models := []ModelInfo{{Name: "llava", Path: modelPath, HasMMProj: true}}
	cfgs := map[string]ModelConfig{
		"llava": {
			MMProj:        filepath.Join(dir, "mmproj-explicit.gguf"),
			Reasoning:     true,
			SpecType:      "draft-mtp",
			SpecDraftNMax: 4,
		},
	}
	path, err := generateModelsPresetFrom(models, cfgs)
	if err != nil {
		t.Fatal(err)
	}
	defer os.Remove(path)

	data, _ := os.ReadFile(path)
	content := string(data)

	// explicit mmproj line exists and uses forward slashes
	wantMM := "mmproj = " + filepath.ToSlash(cfgs["llava"].MMProj)
	if !strings.Contains(content, wantMM+"\n") {
		t.Errorf("preset missing explicit mmproj line %q: %q", wantMM, content)
	}
	// when explicit path exists, auto-detected mmproj line must not be output (only one mmproj line)
	if strings.Contains(content, "mmproj-auto.gguf") {
		t.Errorf("explicit mmproj exists, auto-detected mmproj must not be output: %q", content)
	}
	if strings.Count(content, "mmproj =") != 1 {
		t.Errorf("only one mmproj line should be output: %q", content)
	}
	if !strings.Contains(content, "reasoning = off\n") {
		t.Errorf("preset missing reasoning = off: %q", content)
	}
	if !strings.Contains(content, "spec-type = draft-mtp\n") {
		t.Errorf("preset missing spec-type = draft-mtp: %q", content)
	}
	if !strings.Contains(content, "spec-draft-n-max = 4\n") {
		t.Errorf("preset missing spec-draft-n-max = 4: %q", content)
	}
}

// TestGenerateModelsPresetFromMMProjAutoDetection verifies that when MMProj is empty,
// the existing same-directory auto-detection logic is maintained (HasMMProj=true and
// directory contains mmproj-*.gguf → mmproj line is output).
func TestGenerateModelsPresetFromMMProjAutoDetection(t *testing.T) {
	dir := t.TempDir()
	mmprojPath := filepath.Join(dir, "mmproj-f16.gguf")
	if err := os.WriteFile(mmprojPath, []byte("proj"), 0644); err != nil {
		t.Fatal(err)
	}
	modelPath := filepath.Join(dir, "llava.gguf")

	models := []ModelInfo{{Name: "llava", Path: modelPath, HasMMProj: true}}
	// MMProj is empty: behavior matches unconfigured field, uses auto-detection
	path, err := generateModelsPresetFrom(models, map[string]ModelConfig{"llava": {}})
	if err != nil {
		t.Fatal(err)
	}
	defer os.Remove(path)

	data, _ := os.ReadFile(path)
	content := string(data)
	if !strings.Contains(content, "mmproj = "+filepath.ToSlash(mmprojPath)) {
		t.Errorf("empty MMProj should auto-detect mmproj: %q", content)
	}
}

// TestGenerateModelsPresetFromRejectsSpecAndMMProj verifies preset generation returns
// errors for illegal SpecType and illegal mmproj (INI injection payload) (second defense layer).
func TestGenerateModelsPresetFromRejectsSpecAndMMProj(t *testing.T) {
	models := []ModelInfo{{Name: "m", Path: "/models/m.gguf"}}
	badCfgs := []map[string]ModelConfig{
		{"m": {SpecType: "draft-unknown"}},
		{"m": {MMProj: "x.gguf\n[evil]\nmodel=/tmp/x"}},
		{"m": {MMProj: " /etc/x.gguf"}}, // leading/trailing whitespace rejected
	}
	for i, cfgs := range badCfgs {
		if _, err := generateModelsPresetFrom(models, cfgs); err == nil {
			t.Errorf("case %d: illegal SpecType/MMProj should return error", i)
		}
	}

	// control group: SpecDraftNMax default 0 and valid SpecType generate successfully
	okCfgs := map[string]ModelConfig{"m": {SpecType: "draft-mtp", SpecDraftNMax: 0}}
	if _, err := generateModelsPresetFrom(models, okCfgs); err != nil {
		t.Errorf("valid SpecType should generate successfully: %v", err)
	}
}
