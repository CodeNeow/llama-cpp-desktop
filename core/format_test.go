package core

import "testing"

// ─── formatBytes ─────────────────────────────────────────────────

// TestFormatBytes 验证 formatBytes 的二进制单位换算：
// 小于 1KB 输出 "N B"，KB/MB 级保留 1 位小数并带单位后缀。
func TestFormatBytes(t *testing.T) {
	cases := []struct {
		in   int64
		want string
	}{
		{0, "0 B"},
		{1, "1 B"},
		{1023, "1023 B"},
		{1024, "1.0 KB"},
		{1536, "1.5 KB"},
		{1024 * 1024, "1.0 MB"},
		{3 * 1024 * 1024, "3.0 MB"},
		{1024 * 1024 * 1024, "1.0 GB"},
	}
	for _, c := range cases {
		if got := formatBytes(c.in); got != c.want {
			t.Errorf("formatBytes(%d) = %q, want %q", c.in, got, c.want)
		}
	}
}

// ─── guessQuantFromName ──────────────────────────────────────────

// TestGuessQuantFromName 验证从文件名推断量化格式：
// 大小写不敏感，命中列表中的量化名；未命中返回 "-"。
func TestGuessQuantFromName(t *testing.T) {
	cases := []struct {
		name string
		want string
	}{
		{"qwen2.5-7b-instruct-q4_k_m.gguf", "Q4_K_M"},
		{"bge-small-zh-v1.5-q8_0.gguf", "Q8_0"},
		{"model-f16.gguf", "F16"},
		{"model-iq4_xs.gguf", "IQ4_XS"},
		{"plain-model.gguf", "-"},
	}
	for _, c := range cases {
		if got := guessQuantFromName(c.name); got != c.want {
			t.Errorf("guessQuantFromName(%q) = %q, want %q", c.name, got, c.want)
		}
	}
}

// ─── sanitizeAlias ───────────────────────────────────────────────

// TestSanitizeAlias 验证别名规范化：空格转连字符、非法字符替换、
// 统一小写——生成的 alias 用于 llama-server 预设的 INI 节名。
func TestSanitizeAlias(t *testing.T) {
	cases := []struct {
		in   string
		want string
	}{
		{"My Model 1", "my-model-1"},
		{"Qwen2.5-7B", "qwen2.5-7b"},
		{"x/y*z", "x-y-z"},
	}
	for _, c := range cases {
		if got := sanitizeAlias(c.in); got != c.want {
			t.Errorf("sanitizeAlias(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}

// ─── isEmbeddingModel ────────────────────────────────────────────

// TestIsEmbeddingModel 验证 embedding 模型识别：名称或架构包含
// embedding/bge/gte/e5 等特征即判定为 embedding 模型。
func TestIsEmbeddingModel(t *testing.T) {
	embed := ModelInfo{Name: "bge-small-zh-v1.5", Architecture: "Bert"}
	if !isEmbeddingModel(embed) {
		t.Error("bge 命名模型应被识别为 embedding 模型")
	}
	instruct := ModelInfo{Name: "Qwen2.5-7B-Instruct", Architecture: "Qwen"}
	if isEmbeddingModel(instruct) {
		t.Error("对话模型不应被识别为 embedding 模型")
	}
}
