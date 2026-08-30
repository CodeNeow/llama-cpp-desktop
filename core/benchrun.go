package core

// Deep benchmark: a real-kernel measurement of the currently selected model's
// token-generation speed on this machine, run with the llama-bench binary that
// ships next to llama-server in every llama.cpp install this app manages.
//
// The run loads the full model and takes minutes, so it is strictly a
// user-triggered action (ModelSettings page button, own busy state) and never
// part of the one-click tune path — tuneModelConfig stays synthetic and uses
// the RAM-bandwidth calibration from core/benchbw.go instead. This file adds
// the complementary real-model measurement: same benchbw philosophy (measure
// the real thing, once, with guards) applied to a real llama.cpp kernel.
//
// Design notes:
//   - Args mirror the saved ModelConfig the same way generateModelsPreset
//     writes them for llama-server (threads only when > 0, gpu-layers
//     omitted for "auto", cpu-moe only when enabled), so the measured number
//     describes the config the service would actually run with.
//   - Single-flight via benchRunMu, wait-then-run (same style as the benchbw
//     calibration mutex): a second concurrent click blocks until the first
//     benchmark finishes and then runs its own measurement.
//   - exec.CommandContext with benchRunTimeout kills a hung llama-bench on
//     deadline, so the call can never leave a stray benchmark process behind.
//   - The result is display-only and not persisted (v1).

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"time"
)

// ─── Benchmark constants & single-flight ─────────────────────────

const (
	// benchGenTokens is the generation length per repetition (-n): 128 tokens
	// gives a stable-enough t/s estimate while keeping a minutes-scale run
	// from growing further.
	benchGenTokens = 128
	// benchNGLOffloadAll is the -ngl value passed for GPULayers "all":
	// llama.cpp clamps it to the model's layer count, so it always means
	// "offload everything".
	benchNGLOffloadAll = 99
)

// benchRunTimeout bounds one llama-bench invocation. The benchmark loads the
// full model and generates benchGenTokens tokens — minutes on slow machines —
// so this is deliberately far above runCmd's 8 s system-query timeout (which
// is why runCmd is not used here). It is a package-level var so tests can
// shorten it (same style as cmdTimeout). exec.CommandContext kills the child
// when the deadline fires.
var benchRunTimeout = 15 * time.Minute

// benchRunMu single-flights benchmark runs, wait-then-run (same style as the
// benchbw benchMu calibration mutex, deliberately a separate mutex): the lock
// spans the whole BenchmarkModel call, so a second concurrent call blocks
// here and then runs its own measurement — two llama-bench processes never
// race for the same GPU/RAM.
var benchRunMu sync.Mutex

// Injection points (same style as benchMeasureFn / renameFile): tests swap
// these vars instead of launching a real llama-bench binary or depending on a
// real llama.cpp install layout.
var (
	// benchResolver locates the llama-bench binary; "" means not found.
	benchResolver = resolveLlamaBenchBin
	// benchRunFn executes one llama-bench invocation and returns
	// (stdout, stderr, error). The markdown table is parsed from stdout.
	benchRunFn = runLlamaBench
)

// ─── Binary resolution ────────────────────────────────────────────

// llamaBenchBinName is the platform llama-bench binary name.
func llamaBenchBinName() string {
	if runtime.GOOS == "windows" {
		return "llama-bench.exe"
	}
	return "llama-bench"
}

// resolveLlamaBenchBin locates llama-bench next to the llama-server resolved
// by resolveLlamaServerBin (download dir > custom dir > PATH priority), so a
// custom llama.cpp directory is honored automatically. A PATH-resolved
// llama-server (the bare "llama-server" name) looks llama-bench up on PATH
// too; an absolute llama-server path requires the sibling llama-bench file to
// exist. "" means not found.
func resolveLlamaBenchBin() string {
	p := resolveLlamaServerBin()
	if p == "" {
		return ""
	}
	name := llamaBenchBinName()
	if p == "llama-server" {
		if resolved, err := exec.LookPath(name); err == nil {
			return resolved
		}
		return ""
	}
	candidate := filepath.Join(filepath.Dir(p), name)
	if fi, err := os.Stat(candidate); err == nil && !fi.IsDir() {
		return candidate
	}
	return ""
}

// ─── Command construction (pure) ─────────────────────────────────

// buildBenchCommand translates the saved ModelConfig into one llama-bench
// invocation, mirroring generateModelsPreset's llama-server mapping so the
// measurement describes the config the service would actually run with:
//
//   - -m / -p 0 / -n / -r 1 are fixed: generation-only benchmark (prompt
//     processing is skipped), one repetition — minutes-scale runs cannot
//     afford llama-bench's default of five.
//   - -t <n> only when Threads > 0 (the preset writes threads only when > 0,
//     letting llama.cpp size threads automatically otherwise).
//   - -ngl 99 for GPULayers "all"; -ngl <n> when GPULayers parses as a
//     non-negative number — including "0": the preset writes gpu-layers = 0
//     for a CPU-only config, and llama-bench's own -ngl default is -1
//     (offload everything), so omitting the flag there would benchmark the
//     opposite layer placement of what the saved config means.
//     Empty / "auto" / unparseable values omit the flag, matching the preset
//     which omits gpu-layers for "auto" and lets the tool default apply on
//     both sides.
//   - --cpu-moe when CPUMoe is set. This flag shape is optional in the wild
//     (current builds ship only -ncmoe), so cpu-moe configs fall back through
//     buildBenchNCMoeCommand and finally a no-flags run — see BenchmarkModel.
//
// Pure function: returns the binary path unchanged plus the argument list.
func buildBenchCommand(benchPath string, modelPath string, cfg ModelConfig) (string, []string) {
	args := []string{
		"-m", modelPath,
		"-p", "0",
		"-n", strconv.Itoa(benchGenTokens),
		"-r", "1",
	}
	if cfg.Threads > 0 {
		args = append(args, "-t", strconv.Itoa(cfg.Threads))
	}
	switch {
	case cfg.GPULayers == "all":
		args = append(args, "-ngl", strconv.Itoa(benchNGLOffloadAll))
	default:
		if n, err := strconv.Atoi(strings.TrimSpace(cfg.GPULayers)); err == nil && n >= 0 {
			args = append(args, "-ngl", strconv.Itoa(n))
		}
		// empty / "auto" / anything unparseable: omit the flag and let
		// llama-bench's own default apply.
	}
	if cfg.CPUMoe {
		args = append(args, "--cpu-moe")
	}
	return benchPath, args
}

// buildBenchNCMoeCommand builds the second cpu-moe fallback variant: the saved
// config with CPUMoe cleared, plus llama-bench's -ncmoe N flag — "keep the
// first N MoE layers' expert weights on the CPU". With N = layerCount+1 every
// MoE layer is covered (llama.cpp iterates only over the real MoE layers, so
// the surplus is harmless), which is the closest llama-bench equivalent of
// llama-server's --cpu-moe. Pure function; layerCount is the model's GGUF
// block_count (readGGUFModelMetrics).
func buildBenchNCMoeCommand(benchPath string, modelPath string, cfg ModelConfig, layerCount int) (string, []string) {
	noMoe := cfg
	noMoe.CPUMoe = false
	_, args := buildBenchCommand(benchPath, modelPath, noMoe)
	return benchPath, append(args, "-ncmoe", strconv.Itoa(layerCount+1))
}

// benchEffectiveNgl maps the saved GPULayers value to the effective -ngl the
// benchmark ran with, for display and logs: "all" → "99", a non-negative
// number → itself, an omitted flag ("auto" / empty / unparseable) → "auto".
func benchEffectiveNgl(gpuLayers string) string {
	if gpuLayers == "all" {
		return strconv.Itoa(benchNGLOffloadAll)
	}
	if n, err := strconv.Atoi(strings.TrimSpace(gpuLayers)); err == nil && n >= 0 {
		return strconv.Itoa(n)
	}
	return "auto"
}

// ─── Execution ────────────────────────────────────────────────────

// runLlamaBench executes one llama-bench invocation with benchRunTimeout as
// the hard wall: exec.CommandContext kills the child when the deadline fires,
// so a hung benchmark never outlives the call and never leaves a stray
// llama-bench process. stdout carries the markdown table (llama-bench's
// default -o md); stderr carries diagnostics, surfaced by callers on failure.
// hideWindow prevents a console flash, matching every other child launch.
func runLlamaBench(benchPath string, args []string) (string, string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), benchRunTimeout)
	defer cancel()
	cmd := exec.CommandContext(ctx, benchPath, args...)
	hideWindow(cmd)
	// Android only: llama-bench links against sibling shared libraries
	// (libllama-bench-impl.so, libggml*.so) the Android linker only finds via
	// LD_LIBRARY_PATH (see androidLdEnv); desktop inherits unchanged.
	if ld := androidLdEnv(benchPath); ld != nil {
		cmd.Env = append(os.Environ(), ld...)
	}
	var out, errOut bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &errOut
	if err := cmd.Run(); err != nil {
		if ctx.Err() == context.DeadlineExceeded {
			return out.String(), errOut.String(),
				fmt.Errorf("llama-bench timed out after %v", benchRunTimeout)
		}
		return out.String(), errOut.String(), err
	}
	return out.String(), errOut.String(), nil
}

// ─── Output parsing (pure) ───────────────────────────────────────

// parseBenchOutput extracts the token-generation throughput from llama-bench's
// default markdown output. Table rows are "|"-delimited; the "test" column is
// located once from the header row ("| model | ... | test | t/s |"), and the
// FIRST data row whose test cell starts with "tg" (current builds print
// "tg128" with no space) reports decode speed. The t/s value is the LAST
// non-empty numeric cell of that row: the t/s column is the last column and
// carries "123.45 ± 1.23" (mean ± stddev), so only the leading number parses.
// ok=false covers "no table", "no tg row" and a tg row without any parseable
// t/s cell (malformed) alike.
func parseBenchOutput(out string) (float64, bool) {
	testCol := -1
	for _, line := range strings.Split(out, "\n") {
		if !strings.Contains(line, "|") {
			continue
		}
		cells := strings.Split(line, "|")
		if testCol < 0 {
			for i, c := range cells {
				if strings.TrimSpace(c) == "test" {
					testCol = i
					break
				}
			}
			if testCol >= 0 {
				continue // the header row itself is never a data row
			}
		}
		if !benchRowIsTG(cells, testCol) {
			continue
		}
		// First matching tg row wins: walk cells from the end (the t/s column
		// is last) and take the first cell whose leading token is a number.
		for i := len(cells) - 1; i >= 0; i-- {
			fields := strings.Fields(cells[i])
			if len(fields) == 0 {
				continue
			}
			if v, err := strconv.ParseFloat(fields[0], 64); err == nil {
				return v, true
			}
		}
		return 0, false // tg row without any numeric cell: malformed table
	}
	return 0, false
}

// benchRowIsTG reports whether a table row is a token-generation row: by the
// recorded header index when a header was found, else (headerless output)
// by any cell looking like a tg test label ("tg128" / "tg 128").
func benchRowIsTG(cells []string, testCol int) bool {
	if testCol >= 0 {
		return testCol < len(cells) && strings.HasPrefix(strings.TrimSpace(cells[testCol]), "tg")
	}
	for _, c := range cells {
		tc := strings.TrimSpace(c)
		if len(tc) > 2 && strings.HasPrefix(tc, "tg") && (tc[2] == ' ' || (tc[2] >= '0' && tc[2] <= '9')) {
			return true
		}
	}
	return false
}

// ─── Wails binding ────────────────────────────────────────────────

// ModelBenchResult is the display-only outcome of one deep benchmark run
// (not persisted in v1): the measured token-generation throughput plus the
// effective parameters the benchmark ran with and its wall-clock duration.
// UsedCPUMoe reports whether the MoE experts were measured on the CPU
// (--cpu-moe accepted, or the -ncmoe fallback); false means the run fell back
// to no optional cpu-moe flags, so the measured placement differs from the
// saved config.
type ModelBenchResult struct {
	TGTPS      float64 `json:"tgTps"`
	Ngl        string  `json:"ngl"`
	Threads    int     `json:"threads"`
	UsedCPUMoe bool    `json:"usedCpuMoe"`
	ElapsedS   float64 `json:"elapsedS"`
}

// BenchmarkModel measures the real token-generation speed of modelID on this
// machine by running the llama-bench binary installed next to llama-server
// with the model's CURRENT SAVED ModelConfig (a model without a saved config
// uses defaultModelConfig). Minutes-long and model-loading by nature, it is
// strictly user-triggered (no auto-triggers anywhere), single-flight via
// benchRunMu — wait-then-run: a concurrent second call blocks and then runs
// its own measurement — and bounded by benchRunTimeout per invocation.
//
// cpu-moe configs try the optional-flag shapes in order, stopping at the
// first success (non-cpu-moe configs run the single no-flags attempt; there
// is nothing to retry):
//  1. --cpu-moe — the llama-server flag shape, future-proofing newer
//     llama-bench builds that accept it (UsedCPUMoe=true),
//  2. -ncmoe <blockCount+1> — all MoE expert layers on the CPU, the closest
//     llama-bench equivalent of --cpu-moe; needs readable GGUF metrics with
//     a layer count, otherwise this step is skipped (UsedCPUMoe=true),
//  3. no optional flags — last resort; the measured placement then differs
//     from the saved config and UsedCPUMoe=false records it.
//
// Each attempt is logged at [INFO] (intermediate failures at [WARN]); only
// the final attempt's failure is surfaced as the error.
func (a *App) BenchmarkModel(modelID string) (ModelBenchResult, error) {
	// Single-flight: wait-then-run. The lock spans the whole benchmark, so a
	// concurrent second call waits here and then measures on its own.
	benchRunMu.Lock()
	defer benchRunMu.Unlock()

	// Model lookup by scanModels() display name, same pattern as
	// TuneModelConfig.
	var model *ModelInfo
	models := scanModels()
	for i := range models {
		if models[i].Name == modelID {
			model = &models[i]
			break
		}
	}
	if model == nil {
		return ModelBenchResult{}, fmt.Errorf(tr("未找到模型 %q", "model %q not found"), modelID)
	}

	// The CURRENT SAVED config decides what is measured (cache read under
	// modelConfigsMu; models without a saved config fall back to the
	// defaultModelConfig). Unsaved UI edits are deliberately not measured.
	cfg := a.GetModelConfig(modelID)

	benchPath := benchResolver()
	if benchPath == "" {
		return ModelBenchResult{}, errors.New(tr(
			"在 llama-server 旁未找到 llama-bench，请重新安装运行时环境",
			"llama-bench not found next to llama-server — reinstall the runtime"))
	}

	ngl := benchEffectiveNgl(cfg.GPULayers)
	threads := cfg.Threads
	if threads < 0 {
		threads = 0 // 0 = thread pinning omitted, llama-bench default applied
	}

	// Attempt 1: the saved config mirrored verbatim (--cpu-moe when set).
	// Every attempt is announced at [INFO] and resets start, so ElapsedS
	// reflects the wall clock of the measured (successful) run.
	path, args := buildBenchCommand(benchPath, model.Path, cfg)
	usedMoe := cfg.CPUMoe
	start := time.Now()
	log.Printf("[INFO] bench: running llama-bench for %s (ngl=%s, threads=%d, cpuMoe=%v)",
		modelID, ngl, threads, cfg.CPUMoe)
	stdout, stderr, err := benchRunFn(path, args)

	if err != nil && cfg.CPUMoe {
		// Attempt 2: -ncmoe <blockCount+1> — all MoE expert layers on the
		// CPU, the closest llama-bench equivalent of llama-server's
		// --cpu-moe. Needs readable GGUF metrics with a layer count;
		// anything else (unreadable header, missing block_count) skips
		// straight to attempt 3.
		if metrics, ok := readGGUFModelMetrics(model.Path); ok && metrics.BlockCount > 0 {
			log.Printf("[WARN] bench: run with --cpu-moe failed (%v), retrying with -ncmoe %d",
				err, metrics.BlockCount+1)
			path, args = buildBenchNCMoeCommand(benchPath, model.Path, cfg, metrics.BlockCount)
			log.Printf("[INFO] bench: running llama-bench for %s (ngl=%s, threads=%d, ncmoe=%d) (retry)",
				modelID, ngl, threads, metrics.BlockCount+1)
			start = time.Now()
			stdout, stderr, err = benchRunFn(path, args)
		} else {
			log.Printf("[INFO] bench: no readable layer count for %s, skipping the -ncmoe retry", modelID)
		}
	}
	if err != nil && cfg.CPUMoe {
		// Attempt 3 (last resort): drop every optional cpu-moe flag. The
		// measured placement then differs from the saved config and
		// UsedCPUMoe=false says so.
		log.Printf("[WARN] bench: cpu-moe runs failed (%v), falling back to a run without optional flags", err)
		usedMoe = false
		plainCfg := cfg
		plainCfg.CPUMoe = false
		path, args = buildBenchCommand(benchPath, model.Path, plainCfg)
		log.Printf("[INFO] bench: running llama-bench for %s (ngl=%s, threads=%d, cpuMoe=false) (retry)",
			modelID, ngl, threads)
		start = time.Now()
		stdout, stderr, err = benchRunFn(path, args)
	}
	elapsed := time.Since(start).Seconds()

	if err != nil {
		// Every attempt failed: surface the final attempt's error.
		log.Printf("[ERROR] bench: llama-bench failed for %s: %v", modelID, err)
		detail := strings.TrimSpace(stderr)
		if len(detail) > 400 {
			// Flag-rejection errors echo the whole usage text; keep the tail.
			detail = "..." + detail[len(detail)-400:]
		}
		if detail != "" {
			err = fmt.Errorf("%w (%s)", err, detail)
		}
		return ModelBenchResult{}, fmt.Errorf(tr("运行 llama-bench 失败: %w", "llama-bench failed: %w"), err)
	}

	tps, ok := parseBenchOutput(stdout)
	if !ok {
		// Some builds can route the table elsewhere (--output-err); try
		// stderr before giving up.
		tps, ok = parseBenchOutput(stderr)
	}
	if !ok {
		log.Printf("[ERROR] bench: no tg row found in llama-bench output for %s", modelID)
		return ModelBenchResult{}, errors.New(tr(
			"无法从 llama-bench 输出解析解码速度",
			"could not parse the decode speed from the llama-bench output"))
	}

	log.Printf("[OK] bench: %s tg=%.2f t/s in %.1fs", modelID, tps, elapsed)
	return ModelBenchResult{
		TGTPS:      tps,
		Ngl:        ngl,
		Threads:    threads,
		UsedCPUMoe: usedMoe,
		ElapsedS:   elapsed,
	}, nil
}
