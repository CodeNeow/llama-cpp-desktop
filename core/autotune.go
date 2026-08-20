package core

// One-click auto-tuning: reads the model's real GGUF metrics (layers, attention
// heads, KV geometry, training context) and the local hardware snapshot
// (GPU vendor / VRAM, RAM, CPU cores), then computes optimal llama-server
// parameters for that model. The sizing core is the pure function
// tuneModelConfig — deterministic, no globals, no time — so every rule below is
// unit-testable in isolation.

import (
	"encoding/binary"
	"fmt"
	"io"
	"math"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

// GPU vendor classification used by the tuner.
const (
	vendorNvidia = "nvidia"
	vendorAMD    = "amd"
	vendorNone   = "none"
)

// ─── GGUF metrics reader ─────────────────────────────────────────

// modelMetrics holds the architecture-relevant metadata extracted from a GGUF
// header. Missing keys stay 0; estimators apply documented fallbacks.
type modelMetrics struct {
	Arch, Name string
	// BlockCount is {arch}.block_count (transformer layers).
	BlockCount int
	// ContextLength is {arch}.context_length (trained context window).
	ContextLength   int
	EmbeddingLength int
	// HeadCount / HeadCountKV are {arch}.attention.head_count[_kv]; HeadCountKV
	// carries the GQA group size and falls back to HeadCount when absent.
	HeadCount   int
	HeadCountKV int
	// KeyLength / ValueLength are {arch}.attention.key_length /
	// {arch}.attention.value_length, falling back to 128 when absent.
	KeyLength, ValueLength int
	// KVLoRaRank is {arch}.attention.kv_lora_rank; >0 marks MLA architectures
	// (e.g. deepseek2) whose KV cache is one compressed latent per token.
	KVLoRaRank int
	// RopeDimCount is {arch}.rope.dimension_count (per-head rope dimensions).
	RopeDimCount int
	// FullAttentionInterval is {arch}.full_attention_interval; >1 marks hybrid
	// attention (e.g. qwen3.5) where only every Nth layer holds a KV cache.
	FullAttentionInterval int
}

// readGGUFModelMetrics parses the GGUF header of path and extracts the model
// metrics listed above. Validation mirrors readGGUFMeta (magic, version 2-3,
// kvCount <= 4096); numeric KVs (type 4 = u32, 10 = u64) are collected under
// their full key names and the {arch}.* lookups happen after the loop, once
// general.architecture is known. Returns false when the file is unreadable or
// not a valid GGUF.
func readGGUFModelMetrics(path string) (modelMetrics, bool) {
	f, err := os.Open(path)
	if err != nil {
		return modelMetrics{}, false
	}
	defer f.Close()

	var magic uint32
	if err := binary.Read(f, binary.LittleEndian, &magic); err != nil || magic != 0x46554747 {
		return modelMetrics{}, false
	}
	var version uint32
	if err := binary.Read(f, binary.LittleEndian, &version); err != nil || version < 2 || version > 3 {
		return modelMetrics{}, false
	}
	var tensorCount, kvCount uint64
	if err := binary.Read(f, binary.LittleEndian, &tensorCount); err != nil {
		return modelMetrics{}, false
	}
	if err := binary.Read(f, binary.LittleEndian, &kvCount); err != nil {
		return modelMetrics{}, false
	}
	// Same corrupt-file guard as readGGUFMeta (#7.2): real GGUF metadata has
	// few KVs; refuse to parse beyond 4096 entries.
	if kvCount > 4096 {
		return modelMetrics{}, false
	}

	nums := make(map[string]int, 64)
	strs := make(map[string]string, 8)
	for i := uint64(0); i < kvCount; i++ {
		key, err := readGGUFString(f)
		if err != nil {
			return modelMetrics{}, false
		}
		var valueType uint32
		if err := binary.Read(f, binary.LittleEndian, &valueType); err != nil {
			return modelMetrics{}, false
		}
		switch valueType {
		case 4: // uint32
			var v uint32
			if err := binary.Read(f, binary.LittleEndian, &v); err != nil {
				return modelMetrics{}, false
			}
			nums[key] = int(v)
		case 10: // uint64
			var v uint64
			if err := binary.Read(f, binary.LittleEndian, &v); err != nil {
				return modelMetrics{}, false
			}
			nums[key] = int(v)
		case 8: // string
			s, err := readGGUFString(f)
			if err != nil {
				return modelMetrics{}, false
			}
			strs[key] = s
		default:
			// Other types (bool/float/array/...) carry no metric we need;
			// skip them so the stream stays in sync.
			if err := skipMetricsValue(f, valueType); err != nil {
				return modelMetrics{}, false
			}
		}
	}

	m := modelMetrics{
		Arch: strs["general.architecture"],
		Name: strs["general.name"],
	}
	// Architecture-prefixed lookups happen after the loop: real key names are
	// e.g. qwen35.attention.head_count, deepseek2.attention.kv_lora_rank and
	// {arch}.rope.dimension_count (verified against local qwen3.5 / deepseek2
	// GGUF files).
	get := func(name string) int { return nums[m.Arch+"."+name] }
	m.BlockCount = get("block_count")
	m.ContextLength = get("context_length")
	m.EmbeddingLength = get("embedding_length")
	m.HeadCount = get("attention.head_count")
	m.HeadCountKV = get("attention.head_count_kv")
	m.KeyLength = get("attention.key_length")
	m.ValueLength = get("attention.value_length")
	m.KVLoRaRank = get("attention.kv_lora_rank")
	m.RopeDimCount = get("rope.dimension_count")
	m.FullAttentionInterval = get("full_attention_interval")
	return m, true
}

// skipMetricsValue skips one metadata value of the given GGUF type on f,
// returning an error when the value cannot be consumed. Unlike the shared
// skipGGUFValue (whose 1000-element cap is fine for readGGUFMeta's early-exit
// parsing), this variant skips complete arrays: parsing the whole header of a
// real model must step over tokenizer arrays holding hundreds of thousands of
// elements. Fixed-size elements are skipped with a single seek; crafted files
// terminate at EOF (every skipped element still consumes file bytes), so the
// walk is bounded by the file size.
func skipMetricsValue(f *os.File, valueType uint32) error {
	if size, ok := ggufFixedValueSize(valueType); ok {
		_, err := f.Seek(int64(size), io.SeekCurrent)
		return err
	}
	switch valueType {
	case 8: // string
		var length uint64
		if err := binary.Read(f, binary.LittleEndian, &length); err != nil {
			return err
		}
		// Seek instead of reading: skipped strings never enter memory; the
		// 1 GiB sanity cap rejects absurd declared lengths without overflow.
		if length > 1<<30 {
			return fmt.Errorf("string too long: %d", length)
		}
		_, err := f.Seek(int64(length), io.SeekCurrent)
		return err
	case 9: // array: element type u32 + count u64, then the elements
		var arrType uint32
		if err := binary.Read(f, binary.LittleEndian, &arrType); err != nil {
			return err
		}
		var arrLen uint64
		if err := binary.Read(f, binary.LittleEndian, &arrLen); err != nil {
			return err
		}
		// Sanity cap against crafted headers: real vocab arrays stay far
		// below 2^40 elements, and the cap keeps arrLen*elemSize inside int64.
		if arrLen > 1<<40 {
			return fmt.Errorf("array too long: %d", arrLen)
		}
		if size, ok := ggufFixedValueSize(arrType); ok {
			_, err := f.Seek(int64(arrLen)*int64(size), io.SeekCurrent)
			return err
		}
		// String or nested-array elements must be walked one by one.
		for i := uint64(0); i < arrLen; i++ {
			if err := skipMetricsValue(f, arrType); err != nil {
				return err
			}
		}
		return nil
	}
	return fmt.Errorf("unknown GGUF value type %d", valueType)
}

// ggufFixedValueSize returns the on-disk byte size of a fixed-width GGUF
// scalar type; ok=false for strings, arrays and unknown types.
func ggufFixedValueSize(valueType uint32) (int, bool) {
	switch valueType {
	case 0, 1, 7: // uint8, int8, bool
		return 1, true
	case 2, 3: // uint16, int16
		return 2, true
	case 4, 5, 6: // uint32, int32, float32
		return 4, true
	case 10, 11, 12: // uint64, int64, float64
		return 8, true
	}
	return 0, false
}

// ─── KV cache estimation ─────────────────────────────────────────

// kvBytesPerTokenF16 estimates the f16 KV cache bytes one token needs on one
// standard (full-attention) layer:
//   - MLA (kv_lora_rank > 0): one compressed latent vector of
//     (kv_lora_rank + rope_dims) elements, 2 bytes each; rope dims fall back
//     to 64 when absent (typical MLA rope dimensionality);
//   - standard MHA/GQA: kv_heads * (key_len + value_len) * 2 bytes, where
//     kv_heads falls back to HeadCount (no GQA) and lens fall back to 128;
//   - no head information at all: conservative 1024 bytes (8 KV heads at the
//     default 128-dim geometry).
func kvBytesPerTokenF16(m modelMetrics) float64 {
	if m.KVLoRaRank > 0 {
		rope := m.RopeDimCount
		if rope <= 0 {
			rope = 64
		}
		return float64(m.KVLoRaRank+rope) * 2.0
	}
	kvHeads := m.HeadCountKV
	if kvHeads <= 0 {
		kvHeads = m.HeadCount
	}
	if kvHeads <= 0 {
		return 1024.0
	}
	keyLen := m.KeyLength
	if keyLen <= 0 {
		keyLen = 128
	}
	valLen := m.ValueLength
	if valLen <= 0 {
		valLen = 128
	}
	return float64(kvHeads*(keyLen+valLen)) * 2.0
}

// kvCacheLayers returns how many of the model's layers actually hold a KV
// cache: hybrid-attention models (full_attention_interval > 1, e.g. qwen3.5)
// keep KV on only every Nth layer, i.e. ceil(block_count / interval); every
// other architecture caches on all layers.
func kvCacheLayers(m modelMetrics) int {
	if m.FullAttentionInterval > 1 && m.BlockCount > 0 {
		return (m.BlockCount + m.FullAttentionInterval - 1) / m.FullAttentionInterval
	}
	return m.BlockCount
}

// ─── Tuning inputs ────────────────────────────────────────────────

// tuneHardware is the hardware snapshot the tuner plans against.
type tuneHardware struct {
	GPUVendor     string // "nvidia" | "amd" | "none"
	VRAMMB        int    // MemoryMB of the largest-VRAM GPU; 0 when no GPU
	RAMTotalGB    float64
	RAMFreeGB     float64
	PhysicalCores int // CPUInfo.Cores
	LogicalCPUs   int
}

// tuneModel is the model-side input derived from the GGUF metrics and the
// on-disk weight size.
type tuneModel struct {
	// WeightsBytes is the main GGUF size (plus the mmproj projector for
	// multimodal models).
	WeightsBytes int64
	// Layers is the raw block count: per-layer weight math (partial offload)
	// must divide the weights by the real block count, never by the
	// KV-bearing subset.
	Layers int
	// KVBytesPerTokPerLayerF16 is the f16 KV bytes one token needs per layer,
	// already folded for hybrid attention (see buildTuneModel): multiplying it
	// by Layers yields the true per-token KV total.
	KVBytesPerTokPerLayerF16 float64
	// TrainCtx is the trained context window; <=0 is treated as 32768.
	TrainCtx int
}

// buildTuneModel converts parsed GGUF metrics into the tuneModel inputs.
//
// KV folding consistency: tuneModelConfig computes the per-token KV total as
// KVBytesPerTokPerLayerF16 * Layers and the partial-offload per-layer KV as
// KVBytesPerTokPerLayerF16 * ctx. To keep both correct on hybrid-attention
// models (where only kvCacheLayers = ceil(block_count/interval) layers hold
// KV), Layers stays the raw BlockCount and the per-layer KV cost is scaled by
// kvCacheLayers / BlockCount — so per-layer * Layers == per-layer(true) *
// kvCacheLayers == per-token total, and the per-layer weight estimate W/L
// still uses the true block count.
//
// When ok is false (unreadable/invalid GGUF), conservative fallbacks are used:
// 32 layers, 1024 bytes/layer/token, 32768 training context.
func buildTuneModel(m modelMetrics, ok bool, weightsBytes int64) tuneModel {
	if !ok {
		return tuneModel{
			WeightsBytes:             weightsBytes,
			Layers:                   32,
			KVBytesPerTokPerLayerF16: 1024,
			TrainCtx:                 32768,
		}
	}
	layers := m.BlockCount
	if layers <= 0 {
		layers = 32
	}
	trainCtx := m.ContextLength
	if trainCtx <= 0 {
		trainCtx = 32768
	}
	kvPerLayer := kvBytesPerTokenF16(m)
	if kvLayers := kvCacheLayers(m); kvLayers > 0 && kvLayers != layers {
		kvPerLayer = kvPerLayer * float64(kvLayers) / float64(layers)
	}
	return tuneModel{
		WeightsBytes:             weightsBytes,
		Layers:                   layers,
		KVBytesPerTokPerLayerF16: kvPerLayer,
		TrainCtx:                 trainCtx,
	}
}

// ─── Tuning pure function ─────────────────────────────────────────

// Tuning constants. All budgets/reserves are bytes unless suffixed MB/GB.
const (
	// RAM: plan against min(free, 90% of total) further reduced by a 15%
	// safety margin for the OS and other processes; floored at 1 GB so
	// degenerate snapshots still yield a plan.
	tuneRAMTotalCapRatio = 0.90
	tuneRAMSafetyFactor  = 0.85
	tuneMinUsableRAM     = 1 << 30
	// A discrete GPU needs at least 1536 MB; 512MB-class iGPUs (shared system
	// memory) are treated as no GPU.
	tuneGPUActiveMinVRAMMB = 1536
	// VRAM reserved for the display driver and compute context: NVIDIA's CUDA
	// context is smaller than AMD's ROCm stack.
	tuneNvidiaVRAMReserveMB = 512
	tuneOtherVRAMReserveMB  = 700
	// q8_0 KV cache occupies 8.5 bits per weight vs f16's 16 bits:
	// 8.5/16 = 0.53125.
	tuneQ8KVSizeRatio = 0.53125
	// Compute buffer (logits + activation workspace): a base 384 MB, plus
	// 256 MB for contexts >= 16384 where the logits/graph grow.
	tuneComputeBufBaseMB    = 384
	tuneComputeBufLargeMB   = 256
	tuneComputeBufLargeFrom = 16384
	// Flash-attention headroom guard: if the chosen f16 full-offload plan
	// leaves less than 15% of usable VRAM free and a q8_0 plan buys a larger
	// context, prefer the q8_0 plan.
	tuneHeadroomGuardRatio = 0.15
	// Context ladder (descending) and the partial-offload context candidates.
	tuneCtxMax = 32768
	tuneCtxMin = 2048
)

// ctxLadderFor filters ladder (descending) to entries not exceeding trainCtx
// (trainCtx <= 0 is treated as 32768), always keeping at least the smallest
// entry so every plan has a context.
func ctxLadderFor(trainCtx int, ladder []int) []int {
	if trainCtx <= 0 {
		trainCtx = tuneCtxMax
	}
	out := make([]int, 0, len(ladder))
	for _, ctx := range ladder {
		if ctx <= trainCtx {
			out = append(out, ctx)
		}
	}
	if len(out) == 0 {
		out = append(out, ladder[len(ladder)-1])
	}
	return out
}

// tuneModelConfig computes hardware-aware llama-server parameters for one
// model. Pure and deterministic: same inputs, same ModelConfig.
//
// Strategy:
//  1. No usable GPU → CPU-only plan: GPULayers=0, FlashAttn off, f16 cache
//     (no cache-type written), largest ladder context whose weights + KV fit
//     in usable RAM (smallest entry when none fits).
//  2. GPU present → try full offload first:
//     A) f16 KV everywhere, or B) q8_0 KV (NVIDIA only, 0.53125 the size).
//     Prefer A unless B enables a strictly larger context, or A leaves less
//     than 15% VRAM headroom while B buys more context.
//  3. Full offload impossible → partial offload: for each candidate context
//     (8192/4096/2048) offload n = floor((vramUsable - computeBuf) /
//     (perLayerWeights + perLayerKV)) layers, keeping the CPU-side remainder
//     within usable RAM; cache stays f16.
//  4. Even partial offload impossible → CPU-only plan.
func tuneModelConfig(hw tuneHardware, m tuneModel) ModelConfig {
	const MB = 1 << 20
	// GB in bytes: RAMTotalGB/RAMFreeGB are GiB values (getTotalMemoryGB and
	// getFreeMemoryGB divide by 1024^3), so GiB -> bytes is x1024xMB.
	const GB = 1024 * MB

	usableRAM := math.Min(hw.RAMFreeGB, hw.RAMTotalGB*tuneRAMTotalCapRatio) * tuneRAMSafetyFactor * GB
	if usableRAM < tuneMinUsableRAM {
		usableRAM = tuneMinUsableRAM
	}

	gpuActive := hw.GPUVendor != vendorNone && hw.VRAMMB >= tuneGPUActiveMinVRAMMB
	vramUsable := 0.0
	if gpuActive {
		reserveMB := tuneOtherVRAMReserveMB
		if hw.GPUVendor == vendorNvidia {
			reserveMB = tuneNvidiaVRAMReserveMB
		}
		vramUsable = math.Max(0, float64(hw.VRAMMB)*MB-float64(reserveMB)*MB)
	}

	// Threads: physical cores are llama.cpp's guidance; when unknown, half the
	// logical CPUs (hyperthreads add little); 0 keeps llama-server's default.
	threads := 0
	if hw.PhysicalCores > 0 {
		threads = hw.PhysicalCores
	} else if hw.LogicalCPUs > 1 {
		threads = hw.LogicalCPUs / 2
	}

	layers := m.Layers
	if layers <= 0 {
		layers = 32
	}
	// Per-token KV total (f16) = per-layer cost x layer count; see
	// buildTuneModel for the hybrid-attention folding that keeps this equal to
	// kvBytesPerTokenF16 * kvCacheLayers.
	kvPerTok := m.KVBytesPerTokPerLayerF16 * float64(layers)
	w := float64(m.WeightsBytes)

	ladder := ctxLadderFor(m.TrainCtx, []int{32768, 16384, 8192, 4096, tuneCtxMin})
	computeBuf := func(ctx int) float64 {
		buf := tuneComputeBufBaseMB
		if ctx >= tuneComputeBufLargeFrom {
			buf += tuneComputeBufLargeMB
		}
		return float64(buf) * MB
	}

	cfg := defaultModelConfig()
	cfg.Threads = threads

	// cpuOnlyPlan: no GPU offload; largest ladder context whose weights + KV
	// fit in usable RAM, falling back to the smallest ladder entry.
	cpuOnlyPlan := func() {
		cfg.GPULayers = "0"
		cfg.FlashAttn = false
		cfg.CacheTypeK, cfg.CacheTypeV = "", ""
		cfg.CtxSize = ladder[len(ladder)-1]
		for _, ctx := range ladder {
			if w+kvPerTok*float64(ctx) <= usableRAM {
				cfg.CtxSize = ctx
				break
			}
		}
	}

	if !gpuActive {
		cpuOnlyPlan()
		return cfg
	}

	// Plan A: full offload with f16 KV.
	aCtx := 0
	for _, ctx := range ladder {
		if w+kvPerTok*float64(ctx)+computeBuf(ctx) <= vramUsable {
			aCtx = ctx
			break
		}
	}
	// Plan B: full offload with q8_0 KV (NVIDIA only; llama.cpp's q8_0 cache
	// quantization is a CUDA-backend feature).
	bCtx := 0
	if hw.GPUVendor == vendorNvidia {
		for _, ctx := range ladder {
			if w+kvPerTok*float64(ctx)*tuneQ8KVSizeRatio+computeBuf(ctx) <= vramUsable {
				bCtx = ctx
				break
			}
		}
	}

	useB, fullCtx := false, 0
	if aCtx > 0 && (bCtx == 0 || aCtx >= bCtx) {
		fullCtx = aCtx
		if bCtx > 0 {
			// Headroom guard: a tight f16 plan (< 15% VRAM left) with a q8_0
			// plan offering a larger context is swapped for the q8_0 plan.
			used := w + kvPerTok*float64(aCtx) + computeBuf(aCtx)
			if vramUsable-used < tuneHeadroomGuardRatio*vramUsable && bCtx > aCtx {
				useB, fullCtx = true, bCtx
			}
		}
	} else if bCtx > 0 {
		// B enables a strictly larger context than A (or A does not exist).
		useB, fullCtx = true, bCtx
	}
	if fullCtx > 0 {
		cfg.GPULayers = "all"
		cfg.CtxSize = fullCtx
		cfg.FlashAttn = hw.GPUVendor == vendorNvidia
		if useB {
			cfg.CacheTypeK, cfg.CacheTypeV = "q8_0", "q8_0"
		}
		return cfg
	}

	// Partial offload: offload as many whole layers as fit in VRAM for each
	// candidate context, keeping the CPU-side remainder within usable RAM.
	perLayerW := w / float64(layers)
	for _, ctx := range ctxLadderFor(m.TrainCtx, []int{8192, 4096, tuneCtxMin}) {
		perLayer := perLayerW + m.KVBytesPerTokPerLayerF16*float64(ctx)
		if perLayer <= 0 {
			continue
		}
		n := int(math.Floor((vramUsable - computeBuf(ctx)) / perLayer))
		if n < 0 {
			n = 0
		}
		if n > layers {
			n = layers
		}
		if n >= 1 && float64(layers-n)*perLayer <= usableRAM {
			cfg.GPULayers = strconv.Itoa(n)
			cfg.CtxSize = ctx
			cfg.FlashAttn = hw.GPUVendor == vendorNvidia
			// Partial offload keeps the f16 cache: quantized cache gains are
			// marginal when only part of the pipeline sits on the GPU.
			cfg.CacheTypeK, cfg.CacheTypeV = "", ""
			return cfg
		}
	}

	cpuOnlyPlan()
	return cfg
}

// ─── Hardware snapshot & weight budget ────────────────────────────

// tuneHardware snapshots the hardware through the same caches the Home page
// uses (GetCPU/GetMemory/GetGPU/GetCUDA — one detection chain per process, no
// separate probing) and derives the vendor classification for the tuner:
// any GPU named nvidia (case-insensitive) or an available CUDA driver →
// nvidia; else any GPU named amd/radeon → amd; else none. VRAMMB is the
// largest MemoryMB across the GPU list.
func (a *App) tuneHardware() tuneHardware {
	cpu := a.GetCPU()
	mem := a.GetMemory()
	gpus := a.GetGPU()
	cuda := a.GetCUDA()

	hw := tuneHardware{
		RAMTotalGB:    mem.TotalGB,
		RAMFreeGB:     mem.FreeGB,
		PhysicalCores: cpu.Cores,
		LogicalCPUs:   cpu.LogicalCPUs,
	}
	hasNVIDIA, hasAMD := false, false
	for _, g := range gpus {
		if g.MemoryMB > hw.VRAMMB {
			hw.VRAMMB = g.MemoryMB
		}
		name := strings.ToLower(g.Name)
		if strings.Contains(name, vendorNvidia) {
			hasNVIDIA = true
		}
		if strings.Contains(name, "amd") || strings.Contains(name, "radeon") {
			hasAMD = true
		}
	}
	switch {
	case hasNVIDIA || cuda.Available:
		hw.GPUVendor = vendorNvidia
	case hasAMD:
		hw.GPUVendor = vendorAMD
	default:
		hw.GPUVendor = vendorNone
	}
	return hw
}

// tuneWeightsBytes returns the tuner's weight budget: the main GGUF size plus
// the same-directory mmproj-*.gguf projector when the model is multimodal.
// Projector stat failures are ignored (best-effort extra weight).
func tuneWeightsBytes(m ModelInfo) int64 {
	weights := m.SizeBytes
	if !m.HasMMProj {
		return weights
	}
	matches, err := filepath.Glob(filepath.Join(filepath.Dir(m.Path), "mmproj-*.gguf"))
	if err != nil {
		return weights
	}
	for _, p := range matches {
		if fi, err := os.Stat(p); err == nil {
			weights += fi.Size()
		}
	}
	return weights
}

// tuneFallbackLogLine is the log message emitted when a model's GGUF metrics
// cannot be read and the conservative fallback is used. Separate helper so the
// exact wording is stable and greppable.
func tuneFallbackLogLine(modelID string) string {
	return "[INFO] tune: " + modelID + " has unreadable GGUF metadata, using conservative fallback (layers=32, kv=1024B/layer/token, ctx=32768)"
}
