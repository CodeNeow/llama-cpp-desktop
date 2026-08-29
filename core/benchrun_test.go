package core

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// ─── Fixtures & stubs ─────────────────────────────────────────────

// benchFixture mimics llama-bench's default markdown output (header, dashed
// separator, one pp row, one tg row, build footer) in the real format:
// test labels "pp512"/"tg128" without a space, t/s cells "%.2f ± %.2f", a
// ngl column on non-CPU backends. No real benchmark is ever run.
const benchFixture = `| model                           |       size |     params | backend    | ngl | threads |    test |                  t/s |
| ------------------------------- | ---------: | ---------: | ---------- | --: | ------: | ------: | -------------------: |
| bench-author/bench-model Q4_K_M |   2.35 GiB |     4.02 B | CUDA       |  99 |       4 |   pp512 |            999.99 ± 9.99 |
| bench-author/bench-model Q4_K_M |   2.35 GiB |     4.02 B | CUDA       |  99 |       4 |   tg128 |             42.77 ± 0.31 |

build: 5b7d1cd0 (4321)
`

// stubBenchModel isolates the model scan: temp cwd + saved config state, the
// download-dir override pointed at a temp dir holding one scannable model
// (bench-author/bench-model), no imported dir. All touched globals are
// restored by the registered cleanups. Returns the model dir.
func stubBenchModel(t *testing.T) string {
	t.Helper()
	withTempCwd(t)
	saveConfigState(t)
	dir := t.TempDir()
	modelDownloadDirMu.Lock()
	modelDownloadDirOverride = dir
	modelDownloadDirMu.Unlock()
	modelsDirMu.Lock()
	customModelsDir = ""
	modelsDirMu.Unlock()
	makeVariant(t, dir, "bench-author", "bench-model", "model.gguf", 1024)
	return dir
}

// stubBenchResolver swaps the resolver injection point for a fixed fake path.
func stubBenchResolver(t *testing.T, path string) {
	t.Helper()
	old := benchResolver
	benchResolver = func() string { return path }
	t.Cleanup(func() { benchResolver = old })
}

// stubBenchRun swaps the runner injection point for a scripted runner that
// receives the exact args of each invocation. The script's last behavior
// repeats for any further calls. Returns the invocation counter.
func stubBenchRun(t *testing.T, script func(nth int, args []string) (string, error)) *int64 {
	t.Helper()
	oldRun := benchRunFn
	calls := new(int64)
	benchRunFn = func(path string, args []string) (string, string, error) {
		n := int(atomic.AddInt64(calls, 1))
		out, err := script(n, args)
		return out, "", err
	}
	t.Cleanup(func() { benchRunFn = oldRun })
	return calls
}

// saveBenchModelConfig writes a saved ModelConfig straight into the
// modelConfigs cache (saveConfigState's cleanup restores the original map).
func saveBenchModelConfig(t *testing.T, modelID string, cfg ModelConfig) {
	t.Helper()
	modelConfigsMu.Lock()
	cachedModelConfigs[modelID] = cfg
	modelConfigsMu.Unlock()
}

// argsContain reports whether the argument list contains the exact flag.
func argsContain(args []string, flag string) bool {
	for _, a := range args {
		if a == flag {
			return true
		}
	}
	return false
}

// equalArgs compares two argument lists element-wise.
func equalArgs(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

// ─── buildBenchCommand ────────────────────────────────────────────

// TestBuildBenchCommand verifies the exact argument list for the GPULayers ×
// threads × cpuMoe matrix: fixed prefix (-m/-p 0/-n 128/-r 1), -t only when
// Threads > 0, -ngl 99 for "all", -ngl <n> for numbers (including "0": the
// preset writes gpu-layers = 0 for CPU-only, and llama-bench's own default
// -ngl -1 means offload-everything), flag omitted for empty/"auto", and
// --cpu-moe only when enabled.
func TestBuildBenchCommand(t *testing.T) {
	const mp = `C:\models\bench\model.gguf`
	const bin = `/fake/llama-bench`
	fixed := []string{"-m", mp, "-p", "0", "-n", "128", "-r", "1"}

	cases := []struct {
		name string
		cfg  ModelConfig
		want []string
	}{
		{"all no-threads no-moe", ModelConfig{GPULayers: "all"},
			append(append([]string{}, fixed...), "-ngl", "99")},
		{"all threads moe", ModelConfig{GPULayers: "all", Threads: 8, CPUMoe: true},
			append(append([]string{}, fixed...), "-t", "8", "-ngl", "99", "--cpu-moe")},
		{"number threads no-moe", ModelConfig{GPULayers: "12", Threads: 6},
			append(append([]string{}, fixed...), "-t", "6", "-ngl", "12")},
		{"zero stays cpu-only", ModelConfig{GPULayers: "0"},
			append(append([]string{}, fixed...), "-ngl", "0")},
		{"empty omits ngl", ModelConfig{GPULayers: "", Threads: -1},
			append([]string{}, fixed...)},
		{"auto omits ngl", ModelConfig{GPULayers: "auto", Threads: 0, CPUMoe: true},
			append(append([]string{}, fixed...), "--cpu-moe")},
	}

	for _, tc := range cases {
		path, args := buildBenchCommand(bin, mp, tc.cfg)
		if path != bin {
			t.Errorf("%s: path = %q, want %q", tc.name, path, bin)
		}
		if !equalArgs(args, tc.want) {
			t.Errorf("%s: args = %v, want %v", tc.name, args, tc.want)
		}
	}
}

// TestBuildBenchNCMoeCommand verifies the second cpu-moe fallback variant:
// the saved config with CPUMoe cleared (no --cpu-moe) plus -ncmoe with
// blockCount+1, keeping the other config mappings (threads, ngl) intact.
func TestBuildBenchNCMoeCommand(t *testing.T) {
	const mp = `C:\models\bench\model.gguf`
	path, args := buildBenchNCMoeCommand("/fake/llama-bench", mp,
		ModelConfig{Threads: 4, GPULayers: "all", CPUMoe: true}, 32)
	want := []string{"-m", mp, "-p", "0", "-n", "128", "-r", "1", "-t", "4", "-ngl", "99", "-ncmoe", "33"}
	if path != "/fake/llama-bench" {
		t.Errorf("path = %q, want /fake/llama-bench", path)
	}
	if !equalArgs(args, want) {
		t.Errorf("args = %v, want %v", args, want)
	}
}

// TestBenchEffectiveNgl verifies the display mapping: "all" → "99", numbers
// (including 0) → themselves, omitted cases → "auto".
func TestBenchEffectiveNgl(t *testing.T) {
	cases := []struct{ in, want string }{
		{"all", "99"},
		{"12", "12"},
		{"0", "0"},
		{"auto", "auto"},
		{"", "auto"},
		{"garbage", "auto"},
	}
	for _, tc := range cases {
		if got := benchEffectiveNgl(tc.in); got != tc.want {
			t.Errorf("benchEffectiveNgl(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
}

// ─── parseBenchOutput ─────────────────────────────────────────────

// TestParseBenchOutput verifies the table parser against realistic fixtures:
// the tg row's leading number of the t/s cell wins, the first tg row wins
// over later ones, pp-only / malformed / empty output report false, and a
// headerless tg row still parses via the fallback test-cell detection.
func TestParseBenchOutput(t *testing.T) {
	header := `| model |       size |     params | backend    | ngl | threads |    test |          t/s |
| ----- | ---------: | ---------: | ---------- | --: | ------: | ------: | -----------: |
`
	ppRow := "| m | 2 GiB | 1 B | CUDA | 99 | 4 |  pp512 |   999.99 ± 9.99 |\n"
	tgRow := func(v string) string { return "| m | 2 GiB | 1 B | CUDA | 99 | 4 |  tg128 | " + v + " |\n" }

	cases := []struct {
		name string
		out  string
		want float64
		ok   bool
	}{
		{"realistic fixture", benchFixture, 42.77, true},
		{"tg without stddev", header + tgRow("55.10"), 55.10, true},
		{"first tg row wins", header + tgRow("11.11 ± 0.10") + tgRow("22.22 ± 0.20"), 11.11, true},
		{"pp rows ignored", header + ppRow, 0, false},
		{"no table at all", "ggml_cuda_init: found 1 CUDA devices\nbuild: 5b7d1cd0 (4321)\n", 0, false},
		{"empty output", "", 0, false},
		{"malformed tg row", header + "| m | tg128 | not-a-number |\n", 0, false},
		{"headerless tg row", "| m | 2 GiB | CUDA | tg128 | 55.10 ± 0.20 |\n", 55.10, true},
	}

	for _, tc := range cases {
		got, ok := parseBenchOutput(tc.out)
		if ok != tc.ok || got != tc.want {
			t.Errorf("%s: parseBenchOutput = (%v, %v), want (%v, %v)", tc.name, got, ok, tc.want, tc.ok)
		}
	}
}

// ─── BenchmarkModel ───────────────────────────────────────────────

// benchVariantErr maps each attempt variant to its distinct injected error so
// the all-fail assertion can tell exactly which error was surfaced.
var benchVariantErr = map[string]string{
	"flag":  "err-flag: unknown option --cpu-moe",
	"ncmoe": "err-ncmoe: invalid n-cpu-moe",
	"plain": "err-plain: cuda out of memory",
}

// classifyBenchVariant labels one llama-bench attempt by its optional cpu-moe
// flag shape: "flag" (--cpu-moe), "ncmoe" (-ncmoe), "plain" (no flags).
func classifyBenchVariant(args []string) string {
	switch {
	case argsContain(args, "--cpu-moe"):
		return "flag"
	case argsContain(args, "-ncmoe"):
		return "ncmoe"
	default:
		return "plain"
	}
}

// ncmoeArgValue returns the value passed with -ncmoe when present.
func ncmoeArgValue(args []string) (string, bool) {
	for i, a := range args {
		if a == "-ncmoe" && i+1 < len(args) {
			return args[i+1], true
		}
	}
	return "", false
}

// TestBenchmarkModelCPUMoeChain verifies the optional-flag fallback chain per
// config shape, one llama-bench attempt at a time: cpu-moe configs try
// --cpu-moe → -ncmoe <blockCount+1> → plain and stop at the first success
// (a model whose GGUF metrics carry no usable layer count skips the -ncmoe
// step); non-cpu-moe configs run the single plain attempt with nothing to
// retry. UsedCPUMoe records which variant was measured, and an all-fail chain
// surfaces the plain attempt's error.
func TestBenchmarkModelCPUMoeChain(t *testing.T) {
	const failAll = -1
	cases := []struct {
		name         string
		cfg          ModelConfig
		ggufReadable bool // false keeps the zero-byte fixture (unreadable GGUF)
		failFirst    int  // leading attempts that fail before a success; failAll = never succeeds
		wantTried    []string
		wantUsedMoe  bool
		wantErrFrom  string // variant whose error must surface when failAll
	}{
		{
			name:         "cpu-moe all-fail: three variants, plain error surfaces",
			cfg:          ModelConfig{Threads: 4, GPULayers: "all", CPUMoe: true},
			ggufReadable: true,
			failFirst:    failAll,
			wantTried:    []string{"flag", "ncmoe", "plain"},
			wantErrFrom:  "plain",
		},
		{
			name:         "cpu-moe all-fail: no readable layer count skips -ncmoe",
			cfg:          ModelConfig{Threads: 4, GPULayers: "all", CPUMoe: true},
			ggufReadable: false,
			failFirst:    failAll,
			wantTried:    []string{"flag", "plain"},
			wantErrFrom:  "plain",
		},
		{
			name:         "cpu-moe: -ncmoe succeeds on attempt 2",
			cfg:          ModelConfig{Threads: 4, GPULayers: "all", CPUMoe: true},
			ggufReadable: true,
			failFirst:    1,
			wantTried:    []string{"flag", "ncmoe"},
			wantUsedMoe:  true,
		},
		{
			name:         "cpu-moe: plain fallback on attempt 3",
			cfg:          ModelConfig{Threads: 4, GPULayers: "all", CPUMoe: true},
			ggufReadable: true,
			failFirst:    2,
			wantTried:    []string{"flag", "ncmoe", "plain"},
			wantUsedMoe:  false,
		},
		{
			name:         "no cpu-moe: single plain attempt, nothing to retry",
			cfg:          ModelConfig{Threads: 4, GPULayers: "all"},
			ggufReadable: true,
			failFirst:    failAll,
			wantTried:    []string{"plain"},
			wantErrFrom:  "plain",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			dir := stubBenchModel(t)
			modelPath := filepath.Join(dir, "bench-author", "bench-model", "model.gguf")
			if tc.ggufReadable {
				// Overwrite the zero-byte fixture with a minimal valid GGUF
				// carrying block_count 32, so readGGUFModelMetrics succeeds
				// and the -ncmoe fallback can compute its layer count.
				if err := os.WriteFile(modelPath, buildGGUF(3,
					strKV("general.architecture", "bench"),
					u32KV("bench.block_count", 32),
				), 0644); err != nil {
					t.Fatal(err)
				}
			}
			saveBenchModelConfig(t, "bench-model", tc.cfg)

			var tried []string
			var ncmoeValue string
			failed := 0
			stubBenchResolver(t, filepath.Join("fake-dir", "llama-bench.exe"))
			calls := stubBenchRun(t, func(nth int, args []string) (string, error) {
				variant := classifyBenchVariant(args)
				tried = append(tried, variant)
				if v, ok := ncmoeArgValue(args); ok {
					ncmoeValue = v
				}
				if tc.failFirst == failAll || failed < tc.failFirst {
					failed++
					return "", errors.New(benchVariantErr[variant])
				}
				time.Sleep(5 * time.Millisecond) // instant stubs can round ElapsedS to 0 on Windows
				return benchFixture, nil
			})

			res, err := NewApp().BenchmarkModel("bench-model")

			if tc.failFirst == failAll {
				if err == nil {
					t.Fatal("all attempts failed, BenchmarkModel must error")
				}
				wantErr := benchVariantErr[tc.wantErrFrom]
				if !strings.Contains(err.Error(), wantErr) {
					t.Errorf("surfaced error must be the %s attempt's error %q, got: %v",
						tc.wantErrFrom, wantErr, err)
				}
				for variant, msg := range benchVariantErr {
					if variant != tc.wantErrFrom && strings.Contains(err.Error(), msg) {
						t.Errorf("surfaced error must not contain the %s attempt's error, got: %v", variant, err)
					}
				}
				return
			}
			if err != nil {
				t.Fatalf("BenchmarkModel failed: %v", err)
			}
			if !equalArgs(tried, tc.wantTried) {
				t.Errorf("attempt sequence = %v, want %v", tried, tc.wantTried)
			}
			if res.UsedCPUMoe != tc.wantUsedMoe {
				t.Errorf("UsedCPUMoe = %v, want %v", res.UsedCPUMoe, tc.wantUsedMoe)
			}
			if res.TGTPS != 42.77 {
				t.Errorf("TGTPS = %v, want 42.77", res.TGTPS)
			}
			if res.Ngl != "99" || res.Threads != 4 {
				t.Errorf("Ngl/Threads = %q/%d, want 99/4", res.Ngl, res.Threads)
			}
			if res.ElapsedS <= 0 {
				t.Errorf("ElapsedS = %v, want > 0", res.ElapsedS)
			}
			if got := atomic.LoadInt64(calls); int(got) != len(tc.wantTried) {
				t.Errorf("underlying runs = %d, want %d", got, len(tc.wantTried))
			}
			if argsContain(tc.wantTried, "ncmoe") && ncmoeValue != "33" {
				t.Errorf("-ncmoe value = %q, want 33 (blockCount 32 + 1)", ncmoeValue)
			}
		})
	}
}

// TestBenchmarkModelSingleFlight verifies wait-then-run serialization: N
// concurrent BenchmarkModel calls never overlap the underlying llama-bench
// invocation, each caller waits and then runs its own measurement.
func TestBenchmarkModelSingleFlight(t *testing.T) {
	stubBenchModel(t)

	var inFlight, maxInFlight int64
	stubBenchResolver(t, filepath.Join("fake-dir", "llama-bench.exe"))
	calls := stubBenchRun(t, func(nth int, args []string) (string, error) {
		cur := atomic.AddInt64(&inFlight, 1)
		for {
			old := atomic.LoadInt64(&maxInFlight)
			if cur <= old || atomic.CompareAndSwapInt64(&maxInFlight, old, cur) {
				break
			}
		}
		time.Sleep(30 * time.Millisecond) // widen the overlap race window
		atomic.AddInt64(&inFlight, -1)
		return benchFixture, nil
	})

	const n = 3
	results := make([]ModelBenchResult, n)
	errs := make([]error, n)
	var wg sync.WaitGroup
	app := NewApp()
	for i := 0; i < n; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			results[idx], errs[idx] = app.BenchmarkModel("bench-model")
		}(i)
	}
	wg.Wait()

	for i := range errs {
		if errs[i] != nil {
			t.Errorf("caller %d failed: %v", i, errs[i])
			continue
		}
		if results[i].TGTPS != 42.77 {
			t.Errorf("caller %d TGTPS = %v, want 42.77", i, results[i].TGTPS)
		}
	}
	if got := atomic.LoadInt64(&maxInFlight); got != 1 {
		t.Errorf("overlapping llama-bench invocations: max concurrent = %d, want 1", got)
	}
	if got := atomic.LoadInt64(calls); got != n {
		t.Errorf("underlying runs = %d, want %d (wait-then-run: every caller measures)", got, n)
	}
}

// TestBenchmarkModelMissingBinary verifies the clear error when llama-bench
// is not found next to llama-server, and that no run is attempted.
func TestBenchmarkModelMissingBinary(t *testing.T) {
	stubBenchModel(t)
	stubBenchResolver(t, "")
	stubBenchRun(t, func(nth int, args []string) (string, error) {
		t.Error("llama-bench must not run when the binary is missing")
		return benchFixture, nil
	})

	_, err := NewApp().BenchmarkModel("bench-model")
	if err == nil {
		t.Fatal("missing llama-bench must return an error")
	}
	// Both tr() languages mention the binary name; the model-not-found error
	// never does, so this also distinguishes the two failure paths.
	if !strings.Contains(err.Error(), "llama-bench") {
		t.Errorf("error must name the missing binary, got: %v", err)
	}
}

// TestBenchmarkModelModelNotFound verifies the unknown-model error path.
func TestBenchmarkModelModelNotFound(t *testing.T) {
	withTempCwd(t)
	saveConfigState(t)
	dir := t.TempDir()
	modelDownloadDirMu.Lock()
	modelDownloadDirOverride = dir
	modelDownloadDirMu.Unlock()
	modelsDirMu.Lock()
	customModelsDir = ""
	modelsDirMu.Unlock()

	stubBenchResolver(t, filepath.Join("fake-dir", "llama-bench.exe"))
	stubBenchRun(t, func(nth int, args []string) (string, error) {
		t.Error("llama-bench must not run for an unknown model")
		return benchFixture, nil
	})

	_, err := NewApp().BenchmarkModel("no-such-model")
	if err == nil || !strings.Contains(err.Error(), "no-such-model") {
		t.Errorf("unknown model must fail with the model id in the error, got: %v", err)
	}
}

// TestBenchmarkModelDefaultConfig verifies the no-saved-config path: the
// defaultModelConfig (threads -1, gpuLayers "auto", cpuMoe false) produces a
// run without -t / -ngl / --cpu-moe and a result reporting the effective
// ngl="auto", threads=0, UsedCPUMoe=false.
func TestBenchmarkModelDefaultConfig(t *testing.T) {
	stubBenchModel(t) // no cachedModelConfigs entry

	var gotArgs []string
	stubBenchResolver(t, filepath.Join("fake-dir", "llama-bench.exe"))
	stubBenchRun(t, func(nth int, args []string) (string, error) {
		gotArgs = append([]string(nil), args...)
		return benchFixture, nil
	})

	res, err := NewApp().BenchmarkModel("bench-model")
	if err != nil {
		t.Fatalf("BenchmarkModel failed: %v", err)
	}
	if res.TGTPS != 42.77 || res.Ngl != "auto" || res.Threads != 0 || res.UsedCPUMoe {
		t.Errorf("result = %+v, want tgTps=42.77 ngl=auto threads=0 usedCpuMoe=false", res)
	}
	if len(gotArgs) < 2 || gotArgs[0] != "-m" {
		t.Fatalf("args must start with -m <model>, got %v", gotArgs)
	}
	if filepath.Base(gotArgs[1]) != "model.gguf" {
		t.Errorf("-m path = %q, want the scanned model file", gotArgs[1])
	}
	for _, forbidden := range []string{"-t", "-ngl", "--cpu-moe"} {
		if argsContain(gotArgs, forbidden) {
			t.Errorf("default config must omit %s, args = %v", forbidden, gotArgs)
		}
	}
}
