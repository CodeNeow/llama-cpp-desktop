package core

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

// ─── Model preset generation ────────────────────────────────────
// Generates the llama-server INI preset from the scanned models and their
// per-model configs, with the INI value validation helpers.

// validIniValue validates values written into INI presets: no newlines/null
// bytes (prevents config injection) and no leading/trailing whitespace (avoids
// ambiguity from values being silently trimmed).
func validIniValue(s string) bool {
	return !strings.ContainsAny(s, "\n\r\x00") && s == strings.TrimSpace(s)
}

// validGPULayersValue validates gpu-layers values: empty, auto, all, 0, or a
// pure positive integer are allowed.
func validGPULayersValue(s string) bool {
	if s == "" || s == "auto" || s == "all" || s == "0" {
		return true
	}
	n, err := strconv.Atoi(s)
	return err == nil && n > 0
}

// validCacheTypeValue validates the cache-type-k/v whitelist (the list actually
// supported by b10342).
func validCacheTypeValue(s string) bool {
	switch s {
	case "", "f32", "f16", "bf16", "q8_0", "q4_0", "q4_1", "iq4_nl", "q5_0", "q5_1":
		return true
	}
	return false
}

// validLoadModeValue validates the load-mode whitelist (replaces
// mlock/no-mmap since b10342; empty means use llama-server's default mmap).
func validLoadModeValue(s string) bool {
	switch s {
	case "", "none", "mmap", "mlock", "mmap+mlock", "dio":
		return true
	}
	return false
}

// validSplitModeValue validates the split-mode whitelist (multi-GPU tensor
// split strategy).
func validSplitModeValue(s string) bool {
	switch s {
	case "", "none", "layer", "row", "tensor":
		return true
	}
	return false
}

// validRopeScalingValue validates the rope-scaling whitelist (long-context
// extrapolation strategy).
func validRopeScalingValue(s string) bool {
	switch s {
	case "", "none", "linear", "yarn":
		return true
	}
	return false
}

// validSpecTypeValue validates the spec-type whitelist (MTP multi-token
// prediction strategy; empty means llama-server's default single-token
// prediction).
func validSpecTypeValue(s string) bool {
	switch s {
	case "", "draft-mtp":
		return true
	}
	return false
}

// generateModelsPreset scans the default model directory and writes a llama-server
// INI preset to a temp file, returning its path.
func generateModelsPreset() (string, error) {
	models := scanModels()
	if len(models) == 0 {
		return "", errors.New(tr("LLM-Models 目录中没有模型", "no models found in the LLM-Models directory"))
	}
	modelConfigsMu.Lock()
	cfgs := cachedModelConfigs
	modelConfigsMu.Unlock()
	return generateModelsPresetFrom(models, cfgs)
}

// generateModelsPresetFrom writes a llama-server INI preset for the given models
// and per-model configs to a temp file and returns its path. Models without a
// matching config entry only emit the model path (plus auto-detected options).
func generateModelsPresetFrom(models []ModelInfo, cfgs map[string]ModelConfig) (string, error) {
	if len(models) == 0 {
		return "", errors.New(tr("LLM-Models 目录中没有模型", "no models found in the LLM-Models directory"))
	}

	var buf bytes.Buffer
	// Deterministic alias dedup: sanitizeAlias maps different characters like
	// spaces and slashes all to '-', so distinct model names can collide into
	// the same section name (#7.1). Aliases preserve the display name's casing
	// (llama-server matches model ids case-sensitively) — what the UI shows is
	// exactly the id the API accepts; aliasDedup appends -2, -3... to
	// case-insensitive collisions in model order until unique. The result is
	// deterministic, independent of randomness/time, and identical to the
	// aliases assigned by scanModels for the same model order.
	used := make(map[string]int)
	for _, m := range models {
		alias := aliasDedup(m.Name, used)
		buf.WriteString(fmt.Sprintf("[%s]\n", alias))
		buf.WriteString(fmt.Sprintf("model = %s\n", filepath.ToSlash(m.Path)))

		// Auto-detect embedding model from name or architecture
		if isEmbeddingModel(m) {
			buf.WriteString("embeddings = true\n")
		}

		// With an explicit mmproj path override, skip the same-directory
		// auto-detection below (avoids emitting two mmproj lines)
		explicitMMProj := false

		// Apply per-model config if set
		if cfg, ok := cfgs[m.Name]; ok {
			if cfg.CtxSize > 0 {
				buf.WriteString(fmt.Sprintf("ctx-size = %d\n", cfg.CtxSize))
			}
			if cfg.BatchSize > 0 {
				buf.WriteString(fmt.Sprintf("batch-size = %d\n", cfg.BatchSize))
			}
			if cfg.UBatchSize > 0 {
				buf.WriteString(fmt.Sprintf("ubatch-size = %d\n", cfg.UBatchSize))
			}
			if cfg.Threads > 0 {
				buf.WriteString(fmt.Sprintf("threads = %d\n", cfg.Threads))
			}
			if cfg.GPULayers != "" && cfg.GPULayers != "auto" {
				if !validIniValue(cfg.GPULayers) {
					return "", fmt.Errorf(tr("非法 GPULayers 值 %q：不能包含换行或首尾空白", "invalid GPULayers value %q: must not contain newlines or leading/trailing whitespace"), cfg.GPULayers)
				}
				buf.WriteString(fmt.Sprintf("gpu-layers = %s\n", cfg.GPULayers))
			}
			if cfg.FlashAttn {
				buf.WriteString("flash-attn = on\n")
			}
			if cfg.CacheTypeK != "" {
				if !validIniValue(cfg.CacheTypeK) {
					return "", fmt.Errorf(tr("非法 CacheTypeK 值 %q：不能包含换行或首尾空白", "invalid CacheTypeK value %q: must not contain newlines or leading/trailing whitespace"), cfg.CacheTypeK)
				}
				buf.WriteString(fmt.Sprintf("cache-type-k = %s\n", cfg.CacheTypeK))
			}
			if cfg.CacheTypeV != "" {
				if !validIniValue(cfg.CacheTypeV) {
					return "", fmt.Errorf(tr("非法 CacheTypeV 值 %q：不能包含换行或首尾空白", "invalid CacheTypeV value %q: must not contain newlines or leading/trailing whitespace"), cfg.CacheTypeV)
				}
				buf.WriteString(fmt.Sprintf("cache-type-v = %s\n", cfg.CacheTypeV))
			}
			// New params below take effect since b10342. LoadMode/SplitMode/
			// RopeScaling are omitted when empty or equal to llama-server's
			// default to avoid noise; MLock/NoMMap are deprecated — loadConfig
			// migrates them into LoadMode and they are never written directly
			// into the preset anymore.
			if cfg.LoadMode != "" && cfg.LoadMode != "mmap" {
				if !validIniValue(cfg.LoadMode) {
					return "", fmt.Errorf(tr("非法 LoadMode 值 %q：不能包含换行或首尾空白", "invalid LoadMode value %q: must not contain newlines or leading/trailing whitespace"), cfg.LoadMode)
				}
				buf.WriteString(fmt.Sprintf("load-mode = %s\n", cfg.LoadMode))
			}
			if cfg.CPUMoe {
				buf.WriteString("cpu-moe = on\n")
			}
			if cfg.NCpuMoe > 0 {
				buf.WriteString(fmt.Sprintf("n-cpu-moe = %d\n", cfg.NCpuMoe))
			}
			if cfg.SplitMode != "" && cfg.SplitMode != "layer" {
				if !validIniValue(cfg.SplitMode) {
					return "", fmt.Errorf(tr("非法 SplitMode 值 %q：不能包含换行或首尾空白", "invalid SplitMode value %q: must not contain newlines or leading/trailing whitespace"), cfg.SplitMode)
				}
				buf.WriteString(fmt.Sprintf("split-mode = %s\n", cfg.SplitMode))
			}
			if cfg.TensorSplit != "" {
				if !validIniValue(cfg.TensorSplit) {
					return "", fmt.Errorf(tr("非法 TensorSplit 值 %q：不能包含换行或首尾空白", "invalid TensorSplit value %q: must not contain newlines or leading/trailing whitespace"), cfg.TensorSplit)
				}
				buf.WriteString(fmt.Sprintf("tensor-split = %s\n", cfg.TensorSplit))
			}
			if cfg.MainGPU > 0 {
				buf.WriteString(fmt.Sprintf("main-gpu = %d\n", cfg.MainGPU))
			}
			if cfg.RopeScaling != "" && cfg.RopeScaling != "none" {
				if !validIniValue(cfg.RopeScaling) {
					return "", fmt.Errorf(tr("非法 RopeScaling 值 %q：不能包含换行或首尾空白", "invalid RopeScaling value %q: must not contain newlines or leading/trailing whitespace"), cfg.RopeScaling)
				}
				buf.WriteString(fmt.Sprintf("rope-scaling = %s\n", cfg.RopeScaling))
			}
			if cfg.RopeScale > 0 {
				buf.WriteString(fmt.Sprintf("rope-scale = %g\n", cfg.RopeScale))
			}
			// Explicit mmproj path override: when non-empty and passing INI
			// injection validation it takes priority over same-directory
			// auto-detection; file existence is not required (the model may
			// have been moved; llama-server reports the error at startup).
			if cfg.MMProj != "" {
				if !validIniValue(cfg.MMProj) {
					return "", fmt.Errorf(tr("非法 MMProj 值 %q：不能包含换行或首尾空白", "invalid MMProj value %q: must not contain newlines or leading/trailing whitespace"), cfg.MMProj)
				}
				buf.WriteString(fmt.Sprintf("mmproj = %s\n", filepath.ToSlash(cfg.MMProj)))
				explicitMMProj = true
			}
			if cfg.Reasoning {
				buf.WriteString("reasoning = off\n")
			}
			if cfg.SpecType != "" {
				if !validSpecTypeValue(cfg.SpecType) {
					return "", fmt.Errorf(tr("非法 SpecType 值 %q：仅允许 draft-mtp", "invalid SpecType value %q: only draft-mtp"), cfg.SpecType)
				}
				buf.WriteString(fmt.Sprintf("spec-type = %s\n", cfg.SpecType))
			}
			if cfg.SpecDraftNMax > 0 {
				buf.WriteString(fmt.Sprintf("spec-draft-n-max = %d\n", cfg.SpecDraftNMax))
			}
		}
		if m.HasMMProj && !explicitMMProj {
			// Look for mmproj file in same directory
			dir := filepath.Dir(m.Path)
			entries, _ := os.ReadDir(dir)
			for _, e := range entries {
				if strings.HasPrefix(strings.ToLower(e.Name()), "mmproj") && strings.HasSuffix(strings.ToLower(e.Name()), ".gguf") {
					buf.WriteString(fmt.Sprintf("mmproj = %s\n", filepath.ToSlash(filepath.Join(dir, e.Name()))))
					break
				}
			}
		}
		buf.WriteString("\n")
	}

	tmpFile, err := os.CreateTemp("", "llama-models-*.ini")
	if err != nil {
		return "", err
	}
	path := tmpFile.Name()
	if _, err := tmpFile.Write(buf.Bytes()); err != nil {
		tmpFile.Close()
		return "", err
	}
	tmpFile.Close()
	return path, nil
}

func isEmbeddingModel(m ModelInfo) bool {
	lower := strings.ToLower(m.Name + " " + m.Architecture)
	return strings.Contains(lower, "embedding") || strings.Contains(lower, "embd") ||
		strings.Contains(lower, "all-minilm") || strings.Contains(lower, "bge-") ||
		strings.Contains(lower, "gte-") || strings.Contains(lower, "e5-")
}

// sanitizeAlias maps a display name to a llama-server INI section name: spaces
// become hyphens, characters outside [A-Za-z0-9-_.] are replaced with hyphens.
// Casing is preserved so the model id equals what the UI displays — users
// copy-paste the shown name and llama-server (case-sensitive lookup) matches.
func sanitizeAlias(name string) string {
	name = strings.ReplaceAll(name, " ", "-")
	return strings.Map(func(r rune) rune {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '-' || r == '_' || r == '.' {
			return r
		}
		return '-'
	}, name)
}

// aliasDedup returns a section-name-unique alias for name: the first
// occurrence keeps the sanitized name as-is; later names colliding with it
// case-insensitively get -2, -3... appended in order. The used map is keyed by
// the lowercased alias so two sections can never differ only by casing (both
// would be valid INI sections but ambiguous as copy-paste ids). Deterministic
// for a fixed input order.
func aliasDedup(name string, used map[string]int) string {
	alias := sanitizeAlias(name)
	if n := used[strings.ToLower(alias)]; n > 0 {
		for i := n + 1; ; i++ {
			candidate := fmt.Sprintf("%s-%d", alias, i)
			if used[strings.ToLower(candidate)] == 0 {
				alias = candidate
				break
			}
		}
	}
	used[strings.ToLower(alias)]++
	return alias
}
