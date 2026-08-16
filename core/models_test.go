package core

import (
	"os"
	"path/filepath"
	"sync"
	"testing"
)

// saveModelsState snapshots cachedModels/modelsCacheValid globals and restores them
// after the test.
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

// TestConcurrentGetRefreshModels verifies concurrent GetModels/RefreshModels across
// many rounds do not panic and produce consistent results (#4). Previously, after
// modelsOnce reset, cachedModels was read/written without a lock: RefreshModels'
// rescan write and GetModels' return of the same underlying array could race
// concurrently. After refactoring, both write and read happen under modelsMu, and
// GetModels returns a copy.
// Uses an empty LLM-Models directory so scan results are always an empty slice, making
// consistency easy to assert.
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

	// empty directory should always yield an empty list, and GetModels returns a copy
	models := app.GetModels()
	if len(models) != 0 {
		t.Errorf("empty LLM-Models directory should return 0 models, got %d", len(models))
	}
}

// makeVariant creates a model directory at base/author/variant and writes a gguf file;
// size controls the file size (used as the sorting assertion basis).
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

// TestScanModelsDir verifies directory scanning: <author>/<variant>/.gguf layout is
// recognized as one model, the name is taken from the variant directory name, and models
// are sorted by size descending.
func TestScanModelsDir(t *testing.T) {
	base := t.TempDir()
	makeVariant(t, base, "author1", "big-model", "big.gguf", 2048)
	makeVariant(t, base, "author1", "small-model", "small.gguf", 512)
	makeVariant(t, base, "author2", "third-model", "third.gguf", 1024)

	models := scanModelsDir(base)
	if len(models) != 3 {
		t.Fatalf("scanned %d models, want 3", len(models))
	}
	// sorted by size descending: 2048 > 1024 > 512
	if models[0].Name != "big-model" || models[1].Name != "third-model" || models[2].Name != "small-model" {
		t.Errorf("sort error: %v", []string{models[0].Name, models[1].Name, models[2].Name})
	}
	if models[0].Author != "author1" {
		t.Errorf("author = %q, want author1", models[0].Author)
	}
	if models[0].SizeHuman == "" {
		t.Error("SizeHuman must not be empty")
	}
	if models[0].Path == "" || filepath.Dir(models[0].Path) == "" {
		t.Error("Path must be a full path")
	}
}

// TestScanModelsDirMMProj verifies an mmproj file marks multimodal support and is not
// treated as a primary model.
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
		t.Fatalf("scanned %d models, want 1", len(models))
	}
	if !models[0].HasMMProj {
		t.Error("model with mmproj file should be marked HasMMProj")
	}
}

// TestScanModelsDirEmpty verifies empty directories and directories without GGUF files
// return an empty list.
func TestScanModelsDirEmpty(t *testing.T) {
	base := t.TempDir()
	if models := scanModelsDir(base); len(models) != 0 {
		t.Errorf("empty directory should return 0 models, got %d", len(models))
	}
	makeVariant(t, base, "a", "v", "readme.txt", 100) // non-gguf
	if models := scanModelsDir(base); len(models) != 0 {
		t.Errorf("directory without gguf files should return 0 models, got %d", len(models))
	}
}

// TestScanModelsDirGGUFMeta verifies scanning reads GGUF header metadata to override
// the model name and quantization.
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
		t.Fatalf("scanned %d models, want 1", len(models))
	}
	if models[0].Name != "Qwen2.5-7B-Instruct" {
		t.Errorf("Name = %q, want Qwen2.5-7B-Instruct (should use GGUF metadata)", models[0].Name)
	}
	if models[0].Architecture != "Qwen" {
		t.Errorf("Architecture = %q, want Qwen", models[0].Architecture)
	}
	if models[0].Quantization != "Q4_K_M" {
		t.Errorf("Quantization = %q, want Q4_K_M", models[0].Quantization)
	}
}

// TestScanModelsDirQuantFallback verifies quantization is inferred from the directory
// name or filename when GGUF metadata is absent.
func TestScanModelsDirQuantFallback(t *testing.T) {
	base := t.TempDir()
	makeVariant(t, base, "bge", "bge-small-zh-v1.5", "model-q8_0.gguf", 100)

	models := scanModelsDir(base)
	if len(models) != 1 {
		t.Fatalf("scanned %d models, want 1", len(models))
	}
	if models[0].Name != "bge-small-zh-v1.5" {
		t.Errorf("Name = %q, want bge-small-zh-v1.5", models[0].Name)
	}
	// directory name has no quantization info, fallback to filename model-q8_0.gguf
	if models[0].Quantization != "Q8_0" {
		t.Errorf("Quantization = %q, want Q8_0 (filename fallback)", models[0].Quantization)
	}
}

// makeLooseGGUF writes a loose .gguf file directly under base/author (two-level layout,
// matching the old downloader landing path <author>/<file>.gguf).
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

// TestScanModelsDirTwoLevel verifies <author>/<file>.gguf two-level layout is recognized
// as a model: when GGUF metadata is absent, the name falls back to the filename without
// the .gguf extension, and the path points to the loose file itself.
func TestScanModelsDirTwoLevel(t *testing.T) {
	base := t.TempDir()
	makeLooseGGUF(t, base, "unsloth", "Qwen3.5-4B-UD-IQ2_XXS.gguf", 100)

	models := scanModelsDir(base)
	if len(models) != 1 {
		t.Fatalf("scanned %d models, want 1", len(models))
	}
	if models[0].Name != "Qwen3.5-4B-UD-IQ2_XXS" {
		t.Errorf("Name = %q, want Qwen3.5-4B-UD-IQ2_XXS (filename without .gguf)", models[0].Name)
	}
	if models[0].Author != "unsloth" {
		t.Errorf("Author = %q, want unsloth", models[0].Author)
	}
	if models[0].Path != filepath.Join(base, "unsloth", "Qwen3.5-4B-UD-IQ2_XXS.gguf") {
		t.Errorf("Path = %q, want full two-level path", models[0].Path)
	}
	if models[0].SizeHuman == "" {
		t.Error("SizeHuman must not be empty")
	}
}

// TestScanModelsDirTwoLevelGGUFMeta verifies that in a two-level layout, reading GGUF
// header metadata overrides name/architecture/quantization (shared buildModelInfo behavior
// is the same as the three-level TestScanModelsDirGGUFMeta).
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
		t.Fatalf("scanned %d models, want 1", len(models))
	}
	if models[0].Name != "Qwen2.5-7B-Instruct" {
		t.Errorf("Name = %q, want Qwen2.5-7B-Instruct (should use GGUF metadata)", models[0].Name)
	}
	if models[0].Architecture != "Qwen" {
		t.Errorf("Architecture = %q, want Qwen", models[0].Architecture)
	}
	if models[0].Quantization != "Q4_K_M" {
		t.Errorf("Quantization = %q, want Q4_K_M", models[0].Quantization)
	}
}

// TestScanModelsDirTwoLevelMMProj verifies that in a two-level layout, mmproj-*.gguf
// loose files in the author directory are not treated as primary models (consistent with
// three-level variant-directory mmproj not acting as primary model; in two-level layout
// mmproj loose files cannot be associated with a model and are skipped outright).
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
		t.Errorf("only mmproj loose files should scan to 0 models, got %d", len(models))
	}
}

// TestScanModelsDirMixedLayout verifies that two-level loose .gguf files and three-level
// variant subdirectories coexist under the same author directory and are both recognized,
// then uniformly sorted by SizeBytes descending after merging.
func TestScanModelsDirMixedLayout(t *testing.T) {
	base := t.TempDir()
	// two-level loose file (large)
	makeLooseGGUF(t, base, "author1", "loose-big.gguf", 2048)
	// three-level variant directory (small)
	makeVariant(t, base, "author1", "variant-small", "small.gguf", 512)

	models := scanModelsDir(base)
	if len(models) != 2 {
		t.Fatalf("scanned %d models, want 2", len(models))
	}
	// sorted by size descending: 2048 > 512, two-level and three-level outputs sorted uniformly
	if models[0].Name != "loose-big" {
		t.Errorf("models[0].Name = %q, want loose-big (two-level loose file should rank first by size)", models[0].Name)
	}
	if models[1].Name != "variant-small" {
		t.Errorf("models[1].Name = %q, want variant-small", models[1].Name)
	}
}
