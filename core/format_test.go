package core

import "testing"

// ─── formatBytes ─────────────────────────────────────────────────

// TestFormatBytes verifies formatBytes binary-unit conversion:
// values below 1KB output "N B"; KB/MB levels keep 1 decimal place with unit suffix.
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

// TestGuessQuantFromName verifies quantization-format inference from filename:
// case-insensitive, matches quantization names in the list; no match returns "-".
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

// TestSanitizeAlias verifies alias normalization: spaces become hyphens, illegal
// characters are replaced, and casing is preserved — the alias equals the shown
// display name so copy-paste ids match llama-server's case-sensitive lookup.
func TestSanitizeAlias(t *testing.T) {
	cases := []struct {
		in   string
		want string
	}{
		{"My Model 1", "My-Model-1"},
		{"Qwen2.5-7B", "Qwen2.5-7B"},
		{"x/y*z", "x-y-z"},
	}
	for _, c := range cases {
		if got := sanitizeAlias(c.in); got != c.want {
			t.Errorf("sanitizeAlias(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}

// ─── isEmbeddingModel ────────────────────────────────────────────

// TestIsEmbeddingModel verifies embedding-model identification: a model is classified
// as embedding if its name or architecture contains features like embedding/bge/gte/e5.
func TestIsEmbeddingModel(t *testing.T) {
	embed := ModelInfo{Name: "bge-small-zh-v1.5", Architecture: "Bert"}
	if !isEmbeddingModel(embed) {
		t.Error("bge-named model should be identified as an embedding model")
	}
	instruct := ModelInfo{Name: "Qwen2.5-7B-Instruct", Architecture: "Qwen"}
	if isEmbeddingModel(instruct) {
		t.Error("chat model should not be identified as an embedding model")
	}
}
