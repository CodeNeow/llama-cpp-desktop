package core

import (
	"bytes"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

// ─── readGGUFModelMetrics ────────────────────────────────────────

// TestReadGGUFModelMetricsGQA verifies the standard GQA fixture: architecture
// keys are resolved via the {arch} prefix, u32 and u64 numerics both parse,
// and non-metric / non-numeric KVs (bool, strings) are skipped without
// desynchronizing the stream.
func TestReadGGUFModelMetricsGQA(t *testing.T) {
	dir := t.TempDir()
	path := writeTempGGUF(t, dir, "qwen35.gguf", buildGGUF(3,
		strKV("general.name", "Qwen3.5-4B"),
		strKV("general.architecture", "qwen35"),
		u32KV("general.size", 12345), // non-metric numeric key, ignored by lookups
		boolKV("general.foo", 1),     // bool type exercises the skip branch
		u64KV("qwen35.block_count", 36),
		u64KV("qwen35.context_length", 262144),
		u32KV("qwen35.embedding_length", 2560),
		u32KV("qwen35.attention.head_count", 32),
		u32KV("qwen35.attention.head_count_kv", 8),
		u32KV("qwen35.attention.key_length", 128),
		u32KV("qwen35.attention.value_length", 128),
		u32KV("qwen35.rope.dimension_count", 128),
		u32KV("qwen35.full_attention_interval", 3),
		strKV("tokenizer.ggml.model", "gpt2"), // unrelated string KV, skipped
	))

	m, ok := readGGUFModelMetrics(path)
	if !ok {
		t.Fatal("readGGUFModelMetrics returned false, expected successful parse")
	}
	if m.Arch != "qwen35" || m.Name != "Qwen3.5-4B" {
		t.Errorf("Arch/Name = %q/%q, want qwen35/Qwen3.5-4B", m.Arch, m.Name)
	}
	want := []struct {
		name string
		got  int
		want int
	}{
		{"BlockCount", m.BlockCount, 36},
		{"ContextLength", m.ContextLength, 262144},
		{"EmbeddingLength", m.EmbeddingLength, 2560},
		{"HeadCount", m.HeadCount, 32},
		{"HeadCountKV", m.HeadCountKV, 8},
		{"KeyLength", m.KeyLength, 128},
		{"ValueLength", m.ValueLength, 128},
		{"RopeDimCount", m.RopeDimCount, 128},
		{"FullAttentionInterval", m.FullAttentionInterval, 3},
		{"KVLoRaRank", m.KVLoRaRank, 0},
	}
	for _, w := range want {
		if w.got != w.want {
			t.Errorf("%s = %d, want %d", w.name, w.got, w.want)
		}
	}
}

// TestReadGGUFModelMetricsMLA verifies the MLA fixture (deepseek2): kv_lora_rank
// is picked up, missing head_count_kv stays 0, and rope uses the
// {arch}.rope.dimension_count key.
func TestReadGGUFModelMetricsMLA(t *testing.T) {
	dir := t.TempDir()
	path := writeTempGGUF(t, dir, "deepseek2.gguf", buildGGUF(2,
		strKV("general.architecture", "deepseek2"),
		u32KV("deepseek2.block_count", 61),
		u32KV("deepseek2.context_length", 16384),
		u32KV("deepseek2.attention.kv_lora_rank", 512),
		u32KV("deepseek2.attention.key_length", 576),
		u32KV("deepseek2.attention.value_length", 512),
		u32KV("deepseek2.rope.dimension_count", 64),
	))

	m, ok := readGGUFModelMetrics(path)
	if !ok {
		t.Fatal("readGGUFModelMetrics returned false, expected successful parse")
	}
	if m.Arch != "deepseek2" {
		t.Errorf("Arch = %q, want deepseek2", m.Arch)
	}
	if m.BlockCount != 61 || m.ContextLength != 16384 {
		t.Errorf("BlockCount/ContextLength = %d/%d, want 61/16384", m.BlockCount, m.ContextLength)
	}
	if m.KVLoRaRank != 512 {
		t.Errorf("KVLoRaRank = %d, want 512", m.KVLoRaRank)
	}
	if m.HeadCountKV != 0 {
		t.Errorf("missing head_count_kv should stay 0, got %d", m.HeadCountKV)
	}
	if m.RopeDimCount != 64 {
		t.Errorf("RopeDimCount = %d, want 64", m.RopeDimCount)
	}
}

// TestReadGGUFModelMetricsInvalid verifies invalid magic, a missing file and an
// over-limit kvCount all return false (no panic), consistent with readGGUFMeta.
func TestReadGGUFModelMetricsInvalid(t *testing.T) {
	dir := t.TempDir()
	badPath := filepath.Join(dir, "bad.gguf")
	if err := os.WriteFile(badPath, []byte("NOTGUF DATA"), 0644); err != nil {
		t.Fatal(err)
	}
	if _, ok := readGGUFModelMetrics(badPath); ok {
		t.Error("invalid magic should return false")
	}
	if _, ok := readGGUFModelMetrics(filepath.Join(dir, "nope.gguf")); ok {
		t.Error("missing file should return false")
	}

	// Hand-crafted header with kvCount 5000 (over the 4096 guard) and no KV
	// entries: parsing must abort before the loop, same as readGGUFMeta.
	var buf bytes.Buffer
	buf.WriteString("GGUF")
	putU32(&buf, 3)
	putU64(&buf, 0)    // tensorCount
	putU64(&buf, 5000) // kvCount exceeds the limit
	hugePath := writeTempGGUF(t, dir, "huge.gguf", buf.Bytes())
	if _, ok := readGGUFModelMetrics(hugePath); ok {
		t.Error("kvCount over the 4096 limit should return false")
	}
}

// arrayStrKV builds a GGUF string-array KV (valueType=9: element type u32 +
// count u64 + elements, per the GGUF v3 array layout), used to prove
// full-header parsing survives tokenizer arrays far larger than the shared
// skipGGUFValue's 1000-element cap.
func arrayStrKV(key string, values []string) ggufKV {
	var buf bytes.Buffer
	putU32(&buf, 8) // element type: string
	putU64(&buf, uint64(len(values)))
	for _, v := range values {
		putU64(&buf, uint64(len(v)))
		buf.WriteString(v)
	}
	return ggufKV{key: key, valueType: 9, raw: buf.Bytes()}
}

// TestReadGGUFModelMetricsLargeArraySkipsFully verifies the parser skips a
// complete tokenizer-sized string array (1500 elements, past the shared
// skipGGUFValue 1000-element cap) and still reads metric keys that come after
// it: real GGUF files embed such arrays, and a truncating skip desynchronizes
// the stream (found via the local real-model evidence run).
func TestReadGGUFModelMetricsLargeArraySkipsFully(t *testing.T) {
	dir := t.TempDir()
	tokens := make([]string, 1500)
	for i := range tokens {
		tokens[i] = "tok"
	}
	path := writeTempGGUF(t, dir, "big.gguf", buildGGUF(3,
		strKV("general.architecture", "qwen35"),
		arrayStrKV("tokenizer.ggml.tokens", tokens),
		arrayStrKV("tokenizer.ggml.merges", tokens), // two big arrays back to back
		u32KV("qwen35.block_count", 36),
		u32KV("qwen35.context_length", 262144),
		u32KV("qwen35.attention.head_count", 32),
	))

	m, ok := readGGUFModelMetrics(path)
	if !ok {
		t.Fatal("readGGUFModelMetrics returned false; large arrays must be skipped fully")
	}
	if m.Arch != "qwen35" || m.BlockCount != 36 || m.ContextLength != 262144 || m.HeadCount != 32 {
		t.Errorf("metrics after large arrays = %+v, want block_count=36 context=262144 head_count=32", m)
	}
}

// ─── KV cache estimation ─────────────────────────────────────────

// TestKVBytesPerTokenF16 verifies the three estimator branches: MLA latent,
// standard GQA/MHA geometry (with fallbacks), and the no-head-information
// conservative constant.
func TestKVBytesPerTokenF16(t *testing.T) {
	cases := []struct {
		name string
		m    modelMetrics
		want float64
	}{
		{"MLA with rope dims", modelMetrics{KVLoRaRank: 512, RopeDimCount: 64}, 1152},
		{"MLA without rope dims falls back to 64", modelMetrics{KVLoRaRank: 512}, 1152},
		{"standard GQA", modelMetrics{HeadCount: 8, HeadCountKV: 8, KeyLength: 128, ValueLength: 128}, 4096},
		{"no kv heads falls back to head count and 128 dims", modelMetrics{HeadCount: 8}, 4096},
		{"no head info at all", modelMetrics{}, 1024},
	}
	for _, c := range cases {
		if got := kvBytesPerTokenF16(c.m); math.Abs(got-c.want) > 1e-9 {
			t.Errorf("%s: kvBytesPerTokenF16 = %g, want %g", c.name, got, c.want)
		}
	}
}

// TestKVCacheLayers verifies hybrid-attention folding: only every Nth layer
// holds KV (ceil division), while interval <= 1 caches on all layers.
func TestKVCacheLayers(t *testing.T) {
	cases := []struct {
		m    modelMetrics
		want int
	}{
		{modelMetrics{BlockCount: 36, FullAttentionInterval: 3}, 12},
		{modelMetrics{BlockCount: 37, FullAttentionInterval: 3}, 13}, // ceil(37/3)
		{modelMetrics{BlockCount: 45}, 45},
		{modelMetrics{BlockCount: 45, FullAttentionInterval: 1}, 45},
	}
	for _, c := range cases {
		if got := kvCacheLayers(c.m); got != c.want {
			t.Errorf("kvCacheLayers(%+v) = %d, want %d", c.m, got, c.want)
		}
	}
}

// ─── buildTuneModel (KV folding consistency) ─────────────────────

// TestBuildTuneModelFallback verifies the conservative fallback used when the
// GGUF metadata is unreadable: 32 layers, 1024 bytes/layer/token, ctx 32768.
func TestBuildTuneModelFallback(t *testing.T) {
	tm := buildTuneModel(modelMetrics{}, false, 4096<<20)
	if tm.Layers != 32 || tm.KVBytesPerTokPerLayerF16 != 1024 || tm.TrainCtx != 32768 {
		t.Errorf("fallback tuneModel = %+v, want layers=32 kv=1024 ctx=32768", tm)
	}
	if tm.WeightsBytes != 4096<<20 {
		t.Errorf("WeightsBytes = %d, want %d", tm.WeightsBytes, 4096<<20)
	}
}

// TestBuildTuneModelHybridFold verifies the hybrid-attention KV folding:
// Layers stays the raw block count (per-layer weight math stays correct) while
// the per-layer KV cost is scaled by kvCacheLayers/BlockCount, so per-layer *
// Layers equals the true per-token KV total.
func TestBuildTuneModelHybridFold(t *testing.T) {
	metrics := modelMetrics{
		Arch: "qwen35", BlockCount: 36, ContextLength: 262144,
		HeadCount: 32, HeadCountKV: 8, KeyLength: 128, ValueLength: 128,
		FullAttentionInterval: 3,
	}
	tm := buildTuneModel(metrics, true, 8<<30)
	if tm.Layers != 36 {
		t.Errorf("Layers = %d, want raw block count 36", tm.Layers)
	}
	// Folded per-layer cost = 4096 * 12 / 36.
	wantPerLayer := 4096.0 * 12.0 / 36.0
	if math.Abs(tm.KVBytesPerTokPerLayerF16-wantPerLayer) > 1e-9 {
		t.Errorf("KVBytesPerTokPerLayerF16 = %g, want %g", tm.KVBytesPerTokPerLayerF16, wantPerLayer)
	}
	// Consistency: per-layer * Layers == per-layer(true) * kvCacheLayers.
	foldedTotal := tm.KVBytesPerTokPerLayerF16 * float64(tm.Layers)
	trueTotal := kvBytesPerTokenF16(metrics) * float64(kvCacheLayers(metrics))
	if math.Abs(foldedTotal-trueTotal) > 1e-6 {
		t.Errorf("folded KV total %g != true KV total %g", foldedTotal, trueTotal)
	}

	// Without hybrid attention the per-layer cost is unfolded.
	plain := buildTuneModel(modelMetrics{BlockCount: 36, HeadCount: 8, HeadCountKV: 8, KeyLength: 128, ValueLength: 128, ContextLength: 32768}, true, 0)
	if plain.KVBytesPerTokPerLayerF16 != 4096 {
		t.Errorf("non-hybrid per-layer KV = %g, want 4096", plain.KVBytesPerTokPerLayerF16)
	}
}

// ─── tuneModelConfig ─────────────────────────────────────────────

// TestTuneModelConfig is a table-driven check of the tuning rules with
// hand-verified numbers (all weight fixtures are exact MiB multiples):
//   - 4B  = 2765 MiB, 36 layers, 4096 B/layer/token KV, trained 262144
//   - 23B = 13824 MiB, 45 layers, 1152 B/layer/token KV (MLA), trained 131072
//   - 9B  = 5837 MiB, 40 layers, 4096 B/layer/token KV, trained 262144
//
// Hardware baseline: 8 physical cores / 16 logical, RAM 31 total / 18 free
// (usable RAM = min(18, 31*0.9)*0.85 = 15.3 GiB); NVIDIA 12 GB → 11776 MiB
// usable, 8 GB → 7680 MiB, AMD 16 GB → 15684 MiB.
func TestTuneModelConfig(t *testing.T) {
	hwNvidia12 := tuneHardware{GPUVendor: vendorNvidia, VRAMMB: 12288, RAMTotalGB: 31, RAMFreeGB: 18, PhysicalCores: 8, LogicalCPUs: 16}
	baseNone := tuneHardware{GPUVendor: vendorNone, RAMTotalGB: 31, RAMFreeGB: 18, PhysicalCores: 8, LogicalCPUs: 16}

	tm4B := tuneModel{WeightsBytes: 2765 << 20, Layers: 36, KVBytesPerTokPerLayerF16: 4096, TrainCtx: 262144}
	tm23B := tuneModel{WeightsBytes: 13824 << 20, Layers: 45, KVBytesPerTokPerLayerF16: 1152, TrainCtx: 131072}
	tm9B := tuneModel{WeightsBytes: 5837 << 20, Layers: 40, KVBytesPerTokPerLayerF16: 4096, TrainCtx: 262144}

	cases := []struct {
		name        string
		hw          tuneHardware
		tm          tuneModel
		wantGPU     string
		wantCtx     int
		wantFlash   bool
		wantCacheK  string
		wantCacheV  string
		wantThreads int
	}{
		{
			name: "4B on nvidia 12GB: full offload, f16 cache, max ladder ctx",
			hw:   hwNvidia12, tm: tm4B,
			wantGPU: "all", wantCtx: 32768, wantFlash: true, wantCacheK: "", wantCacheV: "", wantThreads: 8,
		},
		{
			name: "4B on nvidia 8GB: f16 caps at 16384, q8_0 buys 32768 so B wins",
			hw:   tuneHardware{GPUVendor: vendorNvidia, VRAMMB: 8192, RAMTotalGB: 31, RAMFreeGB: 18, PhysicalCores: 8, LogicalCPUs: 16}, tm: tm4B,
			wantGPU: "all", wantCtx: 32768, wantFlash: true, wantCacheK: "q8_0", wantCacheV: "q8_0", wantThreads: 8,
		},
		{
			name: "23B MoE MLA on nvidia 12GB: partial offload n=36 of 45 at ctx 8192",
			hw:   hwNvidia12, tm: tm23B,
			wantGPU: "36", wantCtx: 8192, wantFlash: true, wantCacheK: "", wantCacheV: "", wantThreads: 8,
		},
		{
			name: "23B without GPU: CPU-only, largest ctx fitting usable RAM",
			hw:   baseNone, tm: tm23B,
			wantGPU: "0", wantCtx: 32768, wantFlash: false, wantCacheK: "", wantCacheV: "", wantThreads: 8,
		},
		{
			name: "9B on amd 16GB: full offload, flash-attn off (non-nvidia)",
			hw:   tuneHardware{GPUVendor: vendorAMD, VRAMMB: 16384, RAMTotalGB: 31, RAMFreeGB: 18, PhysicalCores: 8, LogicalCPUs: 16}, tm: tm9B,
			wantGPU: "all", wantCtx: 32768, wantFlash: false, wantCacheK: "", wantCacheV: "", wantThreads: 8,
		},
		{
			name: "512MB iGPU is treated as no GPU even with nvidia vendor",
			hw:   tuneHardware{GPUVendor: vendorNvidia, VRAMMB: 512, RAMTotalGB: 31, RAMFreeGB: 18, PhysicalCores: 8, LogicalCPUs: 16}, tm: tm4B,
			wantGPU: "0", wantCtx: 32768, wantFlash: false, wantCacheK: "", wantCacheV: "", wantThreads: 8,
		},
		{
			name: "23B on nvidia 4GB: tiny partial offload (n=10), fields stay valid",
			hw:   tuneHardware{GPUVendor: vendorNvidia, VRAMMB: 4096, RAMTotalGB: 31, RAMFreeGB: 18, PhysicalCores: 8, LogicalCPUs: 16}, tm: tm23B,
			wantGPU: "10", wantCtx: 8192, wantFlash: true, wantCacheK: "", wantCacheV: "", wantThreads: 8,
		},
		{
			name: "conservative metadata fallback on nvidia 12GB: still a legal plan",
			hw:   hwNvidia12, tm: tuneModel{WeightsBytes: 4096 << 20, Layers: 32, KVBytesPerTokPerLayerF16: 1024, TrainCtx: 32768},
			wantGPU: "all", wantCtx: 32768, wantFlash: true, wantCacheK: "", wantCacheV: "", wantThreads: 8,
		},
		{
			name: "threads fall back to half the logical CPUs without physical cores",
			hw:   tuneHardware{GPUVendor: vendorNone, RAMTotalGB: 31, RAMFreeGB: 18, PhysicalCores: 0, LogicalCPUs: 16}, tm: tm4B,
			wantGPU: "0", wantCtx: 32768, wantFlash: false, wantCacheK: "", wantCacheV: "", wantThreads: 8,
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			cfg := tuneModelConfig(c.hw, c.tm)

			// Invariants that must hold for every plan.
			if !validGPULayersValue(cfg.GPULayers) {
				t.Errorf("GPULayers %q must pass validGPULayersValue", cfg.GPULayers)
			}
			if cfg.CtxSize < tuneCtxMin {
				t.Errorf("CtxSize = %d, must be >= %d", cfg.CtxSize, tuneCtxMin)
			}
			if cfg.Threads < 0 {
				t.Errorf("Threads = %d, must be >= 0", cfg.Threads)
			}
			// Base fields from defaultModelConfig are preserved.
			if cfg.BatchSize != 2048 || cfg.UBatchSize != 512 {
				t.Errorf("BatchSize/UBatchSize = %d/%d, want defaults 2048/512", cfg.BatchSize, cfg.UBatchSize)
			}

			if cfg.GPULayers != c.wantGPU {
				t.Errorf("GPULayers = %q, want %q", cfg.GPULayers, c.wantGPU)
			}
			if cfg.CtxSize != c.wantCtx {
				t.Errorf("CtxSize = %d, want %d", cfg.CtxSize, c.wantCtx)
			}
			if cfg.FlashAttn != c.wantFlash {
				t.Errorf("FlashAttn = %v, want %v", cfg.FlashAttn, c.wantFlash)
			}
			if cfg.CacheTypeK != c.wantCacheK || cfg.CacheTypeV != c.wantCacheV {
				t.Errorf("CacheTypeK/V = %q/%q, want %q/%q", cfg.CacheTypeK, cfg.CacheTypeV, c.wantCacheK, c.wantCacheV)
			}
			if cfg.Threads != c.wantThreads {
				t.Errorf("Threads = %d, want %d", cfg.Threads, c.wantThreads)
			}
		})
	}

	// Partial-offload layer count sanity for the 23B case: n must land in
	// [30, 37] per the task specification (exact value 36 asserted above).
	cfg := tuneModelConfig(hwNvidia12, tm23B)
	n, err := strconv.Atoi(cfg.GPULayers)
	if err != nil {
		t.Fatalf("partial GPULayers %q should be a plain integer: %v", cfg.GPULayers, err)
	}
	if n < 30 || n > 37 {
		t.Errorf("partial offload layers = %d, want within [30, 37]", n)
	}
	if n >= tm23B.Layers {
		t.Errorf("partial offload layers = %d, must stay below block count %d", n, tm23B.Layers)
	}
}

// TestTuneModelConfigHybridEndToEnd runs the folded hybrid-attention tuneModel
// through the planner: an 8 GiB model with 12/36 KV-bearing layers fully
// offloads at 32768 with the f16 cache (the q8_0 plan offers no larger ctx, so
// the headroom guard must not fire).
func TestTuneModelConfigHybridEndToEnd(t *testing.T) {
	hw := tuneHardware{GPUVendor: vendorNvidia, VRAMMB: 12288, RAMTotalGB: 31, RAMFreeGB: 18, PhysicalCores: 8, LogicalCPUs: 16}
	metrics := modelMetrics{
		Arch: "qwen35", BlockCount: 36, ContextLength: 262144,
		HeadCount: 32, HeadCountKV: 8, KeyLength: 128, ValueLength: 128,
		FullAttentionInterval: 3,
	}
	cfg := tuneModelConfig(hw, buildTuneModel(metrics, true, 8<<30))
	if cfg.GPULayers != "all" || cfg.CtxSize != 32768 || cfg.CacheTypeK != "" || !cfg.FlashAttn {
		t.Errorf("hybrid model tune = gpu %q ctx %d cache %q flash %v, want all/32768/f16/true",
			cfg.GPULayers, cfg.CtxSize, cfg.CacheTypeK, cfg.FlashAttn)
	}
}

// ─── Preset integration ──────────────────────────────────────────

// TestTuneModelConfigPresetIntegration feeds a tuned config through the real
// preset generator and asserts the INI gpu-layers / ctx-size lines match the
// config values (the tuned plan survives persistence into the server preset).
func TestTuneModelConfigPresetIntegration(t *testing.T) {
	hw := tuneHardware{GPUVendor: vendorNvidia, VRAMMB: 12288, RAMTotalGB: 31, RAMFreeGB: 18, PhysicalCores: 8, LogicalCPUs: 16}
	tm := tuneModel{WeightsBytes: 2765 << 20, Layers: 36, KVBytesPerTokPerLayerF16: 4096, TrainCtx: 262144}
	cfg := tuneModelConfig(hw, tm)

	models := []ModelInfo{{Name: "m", Path: "/models/m.gguf"}}
	path, err := generateModelsPresetFrom(models, map[string]ModelConfig{"m": cfg})
	if err != nil {
		t.Fatal(err)
	}
	defer os.Remove(path)

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	content := string(data)
	wantLines := []string{
		fmt.Sprintf("gpu-layers = %s", cfg.GPULayers),
		fmt.Sprintf("ctx-size = %d", cfg.CtxSize),
		fmt.Sprintf("threads = %d", cfg.Threads),
	}
	for _, w := range wantLines {
		if !strings.Contains(content, w+"\n") {
			t.Errorf("preset missing %q: %q", w, content)
		}
	}
}
