package core

import (
	_ "embed"

	"archive/tar"
	"archive/zip"
	"bytes"
	"compress/gzip"
	"context"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

// ─── System info structs ─────────────────────────────────────────

type SystemInfo struct {
	OS       string       `json:"os"`
	Arch     string       `json:"arch"`
	CPU      CPUInfo      `json:"cpu"`
	Memory   MemoryInfo   `json:"memory"`
	GPU      []GPUInfo    `json:"gpu"`
	CUDA     CUDAInfo     `json:"cuda"`
	LlamaCpp LlamaCppInfo `json:"llamaCpp"`
}

// ─── GitHub API structs ──────────────────────────────────────────

type GitHubRelease struct {
	TagName     string        `json:"tag_name"`
	Name        string        `json:"name"`
	Body        string        `json:"body"`
	PublishedAt string        `json:"published_at"`
	Assets      []GitHubAsset `json:"assets"`
}

type GitHubAsset struct {
	Name               string `json:"name"`
	Size               int64  `json:"size"`
	BrowserDownloadURL string `json:"browser_download_url"`
}

type CPUInfo struct {
	Model       string `json:"model"`
	Cores       int    `json:"cores"`
	LogicalCPUs int    `json:"logicalCpus"`
}

type MemoryInfo struct {
	TotalGB float64 `json:"totalGb"`
	FreeGB  float64 `json:"freeGb"`
}

type GPUInfo struct {
	Name          string `json:"name"`
	MemoryMB      int    `json:"memoryMb"`
	DriverVersion string `json:"driverVersion"`
	CUDACores     int    `json:"cudaCores"`
}

type CUDAInfo struct {
	Available      bool   `json:"available"`
	DriverVersion  string `json:"driverVersion"`
	ToolkitVersion string `json:"toolkitVersion"`
}

type LlamaCppInfo struct {
	Installed bool   `json:"installed"`
	Path      string `json:"path"`
	Version   string `json:"version"`
}

// ─── HF Mirror types ─────────────────────────────────────────────

type HFSearchResult struct {
	ID          string   `json:"id"`
	ModelID     string   `json:"modelId"`
	Author      string   `json:"author"`
	Downloads   int      `json:"downloads"`
	Likes       int      `json:"likes"`
	PipelineTag string   `json:"pipelineTag"`
	Tags        []string `json:"tags"`
	Siblings    []HFFile `json:"siblings"`
}

type HFFile struct {
	Filename string `json:"rfilename"`
	Size     int64  `json:"size"`
}

// HFFileOut is the frontend-facing file info with `filename` JSON key.
type HFFileOut struct {
	Filename string `json:"filename"`
	Size     int64  `json:"size"`
}

// ─── Download task types ─────────────────────────────────────────

type DlTask struct {
	ID         string `json:"id"`
	ModelID    string `json:"modelId"`
	FileName   string `json:"fileName"`
	DestDir    string `json:"destDir"`
	URL        string `json:"-"`
	Status     string `json:"status"`
	Progress   int    `json:"progress"`
	Total      int64  `json:"total"`
	Downloaded int64  `json:"downloaded"`
	SizeHuman  string `json:"sizeHuman"`
	Error      string `json:"error"`
	ctx        context.Context
	cancel     context.CancelFunc
	resumeCh   chan struct{}
}

type ModelInfo struct {
	Author       string `json:"author"`
	Name         string `json:"name"`
	Path         string `json:"path"`
	SizeBytes    int64  `json:"sizeBytes"`
	SizeHuman    string `json:"sizeHuman"`
	Architecture string `json:"architecture"`
	Quantization string `json:"quantization"`
	HasMMProj    bool   `json:"hasMmproj"`
}

// ─── Cached system info (collected once at startup) ─────────────

var cachedSystemInfo SystemInfo
var systemInfoOnce sync.Once

// Per-section caches for async loading
var cachedCPU CPUInfo
var cachedMemory MemoryInfo
var cachedGPU []GPUInfo
var cachedCUDA CUDAInfo
var cachedLlamaCpp LlamaCppInfo
var cpuOnce, memOnce, gpuOnce, cudaOnce sync.Once
var llamaCacheValid atomic.Bool

var cachedModels []ModelInfo

// modelsCacheValid 标记模型缓存是否有效（原子读，供 GetModels 快速路径）。
// 写入在 modelsMu 锁内完成；不能用 sync.Once 的变量重赋值来实现失效，
// 因为并发 Do 与赋值会破坏 Once 内部互斥状态（#4）。
var modelsCacheValid atomic.Bool

// modelsMu 保护 cachedModels 的读写与缓存失效标记，避免并发扫描/刷新
// 模型缓存时发生数据竞争（#4）。读取侧统一在锁内拷贝副本再返回。
var modelsMu sync.Mutex

// invalidateModelCache 使模型缓存失效：下次 GetModels 会重新扫描目录。
// 供下载完成与手动刷新等路径调用，保证缓存失效与访问并发安全。
func invalidateModelCache() {
	modelsMu.Lock()
	modelsCacheValid.Store(false)
	modelsMu.Unlock()
}

// ─── Download state ──────────────────────────────────────────────

var downloadState = &DownloadState{Status: "idle"}
var downloadMu sync.Mutex
var downloadCancel context.CancelFunc
var downloadResumeCh = make(chan struct{}, 1)
var customLlamaCppDir string
var customLlamaCppMu sync.Mutex

// ─── Download task queue ─────────────────────────────────────────

var dlTasks []*DlTask
var dlTasksMu sync.Mutex
var dlTaskCounter int

type DownloadState struct {
	Status     string `json:"status"` // idle, fetching, downloading, paused, extracting, done, error
	Paused     bool   `json:"paused"`
	Progress   int    `json:"progress"`
	Total      int64  `json:"total"`
	Downloaded int64  `json:"downloaded"`
	FileName   string `json:"fileName"`
	Version    string `json:"version"`
	Error      string `json:"error"`
}

func defaultModelConfig() ModelConfig {
	return ModelConfig{
		Threads: -1, GPULayers: "auto",
		CtxSize: 4096, BatchSize: 2048, UBatchSize: 512,
	}
}

// ─── System info collection ──────────────────────────────────────

func collectSystemInfo() SystemInfo {
	info := SystemInfo{
		OS:   runtime.GOOS,
		Arch: runtime.GOARCH,
		CPU:  getCPUInfo(),
		Memory: MemoryInfo{
			TotalGB: getTotalMemoryGB(),
		},
		GPU:      getGPUInfo(),
		CUDA:     getCUDAInfo(),
		LlamaCpp: getLlamaCppInfo(),
	}

	// Free memory
	info.Memory.FreeGB = getFreeMemoryGB()

	return info
}

// ─── CPU ─────────────────────────────────────────────────────────

func getCPUInfo() CPUInfo {
	info := CPUInfo{
		LogicalCPUs: runtime.NumCPU(),
	}

	switch runtime.GOOS {
	case "windows":
		info.Model = strings.TrimSpace(runCmd("powershell", "-NoProfile", "-Command",
			"Get-CimInstance -ClassName Win32_Processor | Select-Object -ExpandProperty Name"))
		coresStr := strings.TrimSpace(runCmd("powershell", "-NoProfile", "-Command",
			"Get-CimInstance -ClassName Win32_Processor | Select-Object -ExpandProperty NumberOfCores"))
		if n, err := strconv.Atoi(coresStr); err == nil {
			info.Cores = n
		}
	case "linux":
		info.Model = parseLinuxCPUModel(runCmd("cat", "/proc/cpuinfo"))
		info.Cores = countString(runCmd("cat", "/proc/cpuinfo"), "processor")
	case "darwin":
		info.Model = runCmd("sysctl", "-n", "machdep.cpu.brand_string")
		info.Cores = parseCoresDarwin(runCmd("sysctl", "-n", "machdep.cpu.core_count"))
	}

	if info.Model == "" {
		info.Model = fmt.Sprintf("%s %s (unknown model)", runtime.GOOS, runtime.GOARCH)
	}
	if info.Cores == 0 {
		info.Cores = runtime.NumCPU()
	}

	return info
}

// ─── Memory ──────────────────────────────────────────────────────

func getTotalMemoryGB() float64 {
	switch runtime.GOOS {
	case "windows":
		out := strings.TrimSpace(runCmd("powershell", "-NoProfile", "-Command",
			"Get-CimInstance -ClassName Win32_ComputerSystem | Select-Object -ExpandProperty TotalPhysicalMemory"))
		if b, err := strconv.ParseUint(out, 10, 64); err == nil {
			return float64(b) / (1024 * 1024 * 1024)
		}
	case "linux":
		out := runCmd("cat", "/proc/meminfo")
		kb := parseMemInfo(out, "MemTotal")
		if kb > 0 {
			return float64(kb) / (1024 * 1024)
		}
	case "darwin":
		out := runCmd("sysctl", "-n", "hw.memsize")
		if b, err := strconv.ParseUint(strings.TrimSpace(out), 10, 64); err == nil {
			return float64(b) / (1024 * 1024 * 1024)
		}
	}
	return 0
}

func getFreeMemoryGB() float64 {
	switch runtime.GOOS {
	case "windows":
		out := strings.TrimSpace(runCmd("powershell", "-NoProfile", "-Command",
			"Get-CimInstance -ClassName Win32_OperatingSystem | Select-Object -ExpandProperty FreePhysicalMemory"))
		if kb, err := strconv.ParseUint(out, 10, 64); err == nil {
			return float64(kb) / (1024 * 1024)
		}
	case "linux":
		out := runCmd("cat", "/proc/meminfo")
		kb := parseMemInfo(out, "MemAvailable")
		if kb == 0 {
			kb = parseMemInfo(out, "MemFree")
		}
		if kb > 0 {
			return float64(kb) / (1024 * 1024)
		}
	case "darwin":
		// macOS memory pressure — approximate free
		out := runCmd("vm_stat")
		freePages := parseVMStat(out, "free")
		if freePages > 0 {
			pageSizeStr := strings.TrimSpace(runCmd("sysctl", "-n", "hw.pagesize"))
			pageSize, err := strconv.ParseUint(pageSizeStr, 10, 64)
			if err != nil || pageSize == 0 {
				pageSize = 16384 // fallback for arm64
			}
			return float64(freePages*pageSize) / (1024 * 1024 * 1024)
		}
	}
	return 0
}

// ─── GPU ─────────────────────────────────────────────────────────

func getGPUInfo() []GPUInfo {
	out := runCmd("nvidia-smi",
		"--query-gpu=name,memory.total,driver_version",
		"--format=csv,noheader,nounits",
	)
	if out == "" {
		return nil
	}

	var gpus []GPUInfo
	for _, line := range strings.Split(strings.TrimSpace(out), "\n") {
		parts := strings.Split(line, ",")
		if len(parts) < 3 {
			continue
		}
		name := strings.TrimSpace(parts[0])
		memStr := strings.TrimSpace(parts[1])
		driver := strings.TrimSpace(parts[2])

		memMB, _ := strconv.Atoi(memStr)

		gpus = append(gpus, GPUInfo{
			Name:          name,
			MemoryMB:      memMB,
			DriverVersion: driver,
		})
	}
	return gpus
}

// ─── CUDA ────────────────────────────────────────────────────────

func getCUDAInfo() CUDAInfo {
	info := CUDAInfo{}

	// Driver version from nvidia-smi
	out := runCmd("nvidia-smi", "--query-gpu=driver_version", "--format=csv,noheader")
	if out != "" {
		info.Available = true
		info.DriverVersion = strings.TrimSpace(out)
	}

	// Toolkit version from nvcc
	nvccOut := runCmd("nvcc", "--version")
	if nvccOut != "" {
		// Typical output: "Cuda compilation tools, release X.Y, VX.Y.Z"
		for _, line := range strings.Split(nvccOut, "\n") {
			if strings.Contains(line, "release") {
				// Extract version after "release "
				idx := strings.Index(line, "release")
				if idx >= 0 {
					rest := line[idx+len("release"):]
					rest = strings.TrimSpace(rest)
					// Take until comma or space
					if commaIdx := strings.IndexAny(rest, ", "); commaIdx > 0 {
						rest = rest[:commaIdx]
					}
					info.ToolkitVersion = rest
				}
				break
			}
		}
	}

	return info
}

// ─── llama.cpp ───────────────────────────────────────────────────

// findLlamaBinInDir 在 dir 下查找 llama.cpp 二进制 bin：先查 dir 根目录，
// 再查一层子目录（下载 zip 解压可能带顶层文件夹，如 llama-b9999-bin/）。
// Windows 上同时兼容不带 .exe 后缀的文件。命中返回绝对路径，未命中返回空串。
func findLlamaBinInDir(dir, bin string) string {
	name := bin
	if runtime.GOOS == "windows" {
		name += ".exe"
	}
	check := func(p string) string {
		if fi, err := os.Stat(p); err == nil && !fi.IsDir() {
			if abs, err := filepath.Abs(p); err == nil {
				return abs
			}
			return p
		}
		return ""
	}
	if p := check(filepath.Join(dir, name)); p != "" {
		return p
	}
	// Windows 后备：不带 .exe 后缀的文件
	if runtime.GOOS == "windows" {
		if p := check(filepath.Join(dir, bin)); p != "" {
			return p
		}
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		return ""
	}
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		if p := check(filepath.Join(dir, e.Name(), name)); p != "" {
			return p
		}
		if runtime.GOOS == "windows" {
			if p := check(filepath.Join(dir, e.Name(), bin)); p != "" {
				return p
			}
		}
	}
	return ""
}

// findLlamaBin 在 dir 下查找 llama.cpp 二进制 bin：dir 为空串表示走 PATH
// （exec.LookPath），返回 LookPath 解析结果；非空目录委托 findLlamaBinInDir。
func findLlamaBin(dir, bin string) string {
	if dir == "" {
		path, err := exec.LookPath(bin)
		if err != nil {
			return ""
		}
		return path
	}
	return findLlamaBinInDir(dir, bin)
}

// resolveLlamaServerBin 按 customLlamaCppDir > llama-cpp/ 下载目录 > PATH
// 的优先级解析 llama-server 可执行文件路径，供 getLlamaCppInfo 与
// buildServerCommand 共用，避免两处查找逻辑漂移。目录命中返回绝对路径；
// PATH 命中返回二进制名 "llama-server"（交给 exec.Command 解析）；
// 全部未命中返回空串。
func resolveLlamaServerBin() string {
	customLlamaCppMu.Lock()
	customDir := customLlamaCppDir
	customLlamaCppMu.Unlock()
	if customDir != "" {
		if p := findLlamaBinInDir(customDir, "llama-server"); p != "" {
			return p
		}
	}
	if p := findLlamaBinInDir(downloadDir, "llama-server"); p != "" {
		return p
	}
	if _, err := exec.LookPath("llama-server"); err == nil {
		return "llama-server"
	}
	return ""
}

// fillLlamaCppVersion 尝试运行二进制读取版本号填充 info.Version。运行失败
// （如 Windows 上 stub 非可执行文件）只影响 Version 为空，不影响 Installed。
func fillLlamaCppVersion(info *LlamaCppInfo, path string) {
	versionOut := runCmd(path, "--version")
	if versionOut != "" {
		info.Version = strings.TrimSpace(versionOut)
		for _, line := range strings.Split(versionOut, "\n") {
			trimmed := strings.TrimSpace(line)
			if strings.HasPrefix(trimmed, "version") || strings.Contains(trimmed, "build") {
				info.Version = trimmed
				break
			}
		}
		return
	}
	versionOut = runCmd(path, "-v")
	if versionOut != "" {
		info.Version = strings.TrimSpace(versionOut)
	}
}

// getLlamaCppInfo 检测 llama.cpp 运行时：按 customLlamaCppDir > llama-cpp/
// 下载目录 > PATH 的优先级查找二进制。llama-server 走公共 helper
// resolveLlamaServerBin（下载目录支持根目录与一层子目录两种布局）；
// 其余候选二进制（llama-cli / llama.cpp / llama）沿用同一目录优先级。
func getLlamaCppInfo() LlamaCppInfo {
	info := LlamaCppInfo{}

	// PATH 命中的 llama-server 由 helper 返回裸二进制名，这里还原
	// exec.LookPath 的解析结果，便于前端展示完整路径
	if p := resolveLlamaServerBin(); p != "" {
		if p == "llama-server" {
			if resolved, err := exec.LookPath(p); err == nil {
				p = resolved
			}
		}
		info.Installed = true
		info.Path = p
		fillLlamaCppVersion(&info, p)
		return info
	}

	customLlamaCppMu.Lock()
	customDir := customLlamaCppDir
	customLlamaCppMu.Unlock()

	dirsToCheck := make([]string, 0, 3)
	if customDir != "" {
		dirsToCheck = append(dirsToCheck, customDir)
	}
	dirsToCheck = append(dirsToCheck, downloadDir)
	dirsToCheck = append(dirsToCheck, "") // 空串表示 PATH

	for _, dir := range dirsToCheck {
		for _, bin := range []string{"llama-cli", "llama.cpp", "llama"} {
			if p := findLlamaBin(dir, bin); p != "" {
				info.Installed = true
				info.Path = p
				fillLlamaCppVersion(&info, p)
				return info
			}
		}
	}

	return info
}

// ─── Command helpers ─────────────────────────────────────────────

func runCmd(name string, args ...string) string {
	cmd := exec.Command(name, args...)
	hideWindow(cmd)
	var out, errOut bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &errOut
	if err := cmd.Run(); err != nil && errOut.Len() > 0 {
		log.Printf("[CMD] %s %v stderr: %s", name, args, strings.TrimSpace(errOut.String()))
	}
	return strings.TrimSpace(out.String())
}

// ─── Parsing helpers ─────────────────────────────────────────────

func parseLinuxCPUModel(out string) string {
	for _, line := range strings.Split(out, "\n") {
		if strings.HasPrefix(line, "model name") {
			parts := strings.SplitN(line, ":", 2)
			if len(parts) == 2 {
				return strings.TrimSpace(parts[1])
			}
		}
	}
	return ""
}

func countString(out, substr string) int {
	count := 0
	for _, line := range strings.Split(out, "\n") {
		if strings.Contains(line, substr) {
			count++
		}
	}
	return count
}

func parseCoresDarwin(out string) int {
	n, _ := strconv.Atoi(strings.TrimSpace(out))
	return n
}

func parseMemInfo(out, key string) uint64 {
	for _, line := range strings.Split(out, "\n") {
		if strings.Contains(line, key+":") {
			parts := strings.Fields(line)
			if len(parts) >= 2 {
				val, err := strconv.ParseUint(parts[len(parts)-2], 10, 64)
				if err != nil {
					val, err = strconv.ParseUint(parts[1], 10, 64)
					if err != nil {
						return 0
					}
				}
				return val
			}
		}
	}
	return 0
}

func parseVMStat(out, key string) uint64 {
	for _, line := range strings.Split(out, "\n") {
		fields := strings.Fields(line)
		if len(fields) < 2 {
			continue
		}
		// 匹配 "<key>:" 形式的字段（macOS 输出为 "Pages free:    123456."，
		// 值在 key 字段之后且以句号结尾），避免解析失败导致可用内存恒为 0。
		for i, f := range fields {
			if strings.TrimRight(f, ":") != key {
				continue
			}
			if i+1 >= len(fields) {
				break
			}
			val, err := strconv.ParseUint(strings.TrimSuffix(fields[i+1], "."), 10, 64)
			if err == nil {
				return val
			}
		}
	}
	return 0
}

// ─── Model scanning ──────────────────────────────────────────────

const modelsDir = "LLM-Models"

// customModelsDir 为自定义模型目录（空表示未配置，使用默认 modelsDir）。
// modelsDirMu 保护其读写，与 customLlamaCppMu 保护 customLlamaCppDir 的
// 风格保持一致。
var customModelsDir string
var modelsDirMu sync.Mutex

// effectiveModelsDir 返回当前生效的模型目录：配置了自定义目录时返回自定义
// 目录，否则回退到默认 modelsDir。读取在 modelsDirMu 锁内完成，保证与
// SetModelsDir / loadConfig / saveConfig 的写入并发安全。
func effectiveModelsDir() string {
	modelsDirMu.Lock()
	dir := customModelsDir
	modelsDirMu.Unlock()
	if dir != "" {
		return dir
	}
	return modelsDir
}

// scanModels scans the effective model directory (custom when set), creating
// the default LLM-Models directory only when no custom dir is configured.
func scanModels() []ModelInfo {
	dir := effectiveModelsDir()
	// 自定义目录由用户选择，应已存在；仅默认目录需要惰性创建。
	if dir == modelsDir {
		if err := os.MkdirAll(dir, 0755); err != nil {
			log.Printf("[WARN] Failed to create %s dir: %v", dir, err)
			return make([]ModelInfo, 0)
		}
	}
	return scanModelsDir(dir)
}

// scanModelsDir scans an <author>/<variant>/ directory tree for GGUF models.
// A variant directory counts as a model when it contains at least one
// non-mmproj .gguf file; mmproj files only flag multimodal support.
func scanModelsDir(dir string) []ModelInfo {
	models := make([]ModelInfo, 0)

	// Top-level: author directories
	authors, err := os.ReadDir(dir)
	if err != nil {
		log.Printf("[WARN] Failed to read %s dir: %v", dir, err)
		return models
	}

	for _, authorEntry := range authors {
		if !authorEntry.IsDir() {
			continue
		}
		author := authorEntry.Name()
		authorDir := filepath.Join(dir, author)

		// Second-level: model variant directories
		variants, err := os.ReadDir(authorDir)
		if err != nil {
			continue
		}

		for _, variantEntry := range variants {
			if !variantEntry.IsDir() {
				continue
			}
			variantName := variantEntry.Name()
			variantDir := filepath.Join(authorDir, variantName)

			// Find .gguf files in this variant directory
			files, err := os.ReadDir(variantDir)
			if err != nil {
				continue
			}

			var mainGGUF string
			var mainSize int64
			var hasMMProj bool

			for _, f := range files {
				if f.IsDir() {
					continue
				}
				name := f.Name()
				if !strings.HasSuffix(strings.ToLower(name), ".gguf") {
					continue
				}
				lower := strings.ToLower(name)
				if strings.HasPrefix(lower, "mmproj") {
					hasMMProj = true
					continue
				}
				// Found main model file (non-mmproj .gguf)
				if mainGGUF == "" {
					mainGGUF = filepath.Join(variantDir, name)
					if fi, err := f.Info(); err == nil {
						mainSize = fi.Size()
					}
				}
			}

			if mainGGUF == "" {
				continue
			}

			model := ModelInfo{
				Author:    author,
				Path:      mainGGUF,
				SizeBytes: mainSize,
				SizeHuman: formatBytes(mainSize),
				HasMMProj: hasMMProj,
			}

			// Derive model name from directory name
			model.Name = variantName

			// Try to read GGUF metadata for better name/arch/quant
			if metadata := readGGUFMeta(mainGGUF); metadata != nil {
				// Only use GGUF name if it looks readable (not a hash)
				if n := metadata["name"]; n != "" && isReadableName(n) {
					model.Name = n
				}
				if a := metadata["arch"]; a != "" {
					model.Architecture = a
				}
				if q := metadata["quant"]; q != "" {
					model.Quantization = q
				}
			}

			// Fallback quantization from filename or directory name
			if model.Quantization == "" {
				model.Quantization = guessQuantFromName(variantName)
				if model.Quantization == "-" {
					model.Quantization = guessQuantFromName(filepath.Base(mainGGUF))
				}
			}

			// Fallback architecture from directory/author name
			if model.Architecture == "" {
				model.Architecture = guessArchFromName(variantName + " " + author)
			}

			models = append(models, model)
		}
	}

	sort.Slice(models, func(i, j int) bool {
		return models[i].SizeBytes > models[j].SizeBytes
	})

	return models
}

// isReadableName returns true if the name doesn't look like a hash/UUID.
func isReadableName(name string) bool {
	if len(name) < 3 {
		return false
	}
	// If it's all hex and over 32 chars, it's likely a hash
	hexCount := 0
	for _, c := range name {
		if (c >= '0' && c <= '9') || (c >= 'a' && c <= 'f') || (c >= 'A' && c <= 'F') {
			hexCount++
		}
	}
	if hexCount > len(name)*3/4 && len(name) > 10 {
		return false
	}
	// If it starts with "Unsloth_Gguf" it's an auto-generated name
	if strings.HasPrefix(name, "Unsloth_Gguf") {
		return false
	}
	// If it contains spaces or dashes, it's likely human-readable
	if strings.ContainsAny(name, " -.") {
		return true
	}
	return true
}

func guessArchFromName(name string) string {
	lower := strings.ToLower(name)
	arches := []struct {
		key  string
		arch string
	}{
		{"qwen", "Qwen"},
		{"qwopus", "Qwen"},
		{"llama", "LLaMA"},
		{"mistral", "Mistral"},
		{"phi", "Phi"},
		{"gemma", "Gemma"},
		{"deepseek", "DeepSeek"},
		{"yi", "Yi"},
		{"chatglm", "ChatGLM"},
		{"baichuan", "Baichuan"},
		{"falcon", "Falcon"},
		{"mpt", "MPT"},
		{"starcoder", "StarCoder"},
		{"codellama", "CodeLLaMA"},
		{"claude", "Claude-Distilled"},
	}
	for _, a := range arches {
		if strings.Contains(lower, a.key) {
			return a.arch
		}
	}
	return "-"
}

// ─── GGUF header reader ──────────────────────────────────────────

// readGGUFMeta reads the GGUF header and extracts key metadata fields.
// Returns nil if the file is not a valid GGUF file.
func readGGUFMeta(path string) map[string]string {
	f, err := os.Open(path)
	if err != nil {
		return nil
	}
	defer f.Close()

	// Read magic
	var magic uint32
	if err := binary.Read(f, binary.LittleEndian, &magic); err != nil {
		return nil
	}
	if magic != 0x46554747 { // "GGUF" in little-endian
		return nil
	}

	// Read version
	var version uint32
	if err := binary.Read(f, binary.LittleEndian, &version); err != nil {
		return nil
	}
	if version < 2 || version > 3 {
		return nil
	}

	// Read tensor count and metadata kv count
	var tensorCount, kvCount uint64
	if err := binary.Read(f, binary.LittleEndian, &tensorCount); err != nil {
		return nil
	}
	if err := binary.Read(f, binary.LittleEndian, &kvCount); err != nil {
		return nil
	}

	// 防御恶意/损坏文件：kvCount 无上限时循环可能解析超长 KV 列表并放大
	// 解析开销（#7.2）。正常 GGUF 元数据键极少，超过 4096 直接放弃解析。
	if kvCount > 4096 {
		return nil
	}

	result := make(map[string]string)
	targets := map[string]string{
		"general.name":         "name",
		"general.architecture": "arch",
		"general.file_type":    "quant",
	}
	found := 0

	for i := uint64(0); i < kvCount && found < len(targets); i++ {
		key, err := readGGUFString(f)
		if err != nil {
			break
		}

		var valueType uint32
		if err := binary.Read(f, binary.LittleEndian, &valueType); err != nil {
			break
		}

		field, wanted := targets[key]
		if !wanted {
			skipGGUFValue(f, valueType)
			continue
		}

		switch valueType {
		case 8: // string
			val, err := readGGUFString(f)
			if err != nil {
				break
			}
			result[field] = val
			found++
		case 4: // uint32 (file_type is uint32)
			var val uint32
			if err := binary.Read(f, binary.LittleEndian, &val); err != nil {
				break
			}
			result[field] = ggufQuantName(val)
			found++
		case 10: // uint64
			var val uint64
			if err := binary.Read(f, binary.LittleEndian, &val); err != nil {
				break
			}
			result[field] = ggufQuantName(uint32(val))
			found++
		default:
			skipGGUFValue(f, valueType)
		}
	}

	return result
}

func readGGUFString(r io.Reader) (string, error) {
	var length uint64
	if err := binary.Read(r, binary.LittleEndian, &length); err != nil {
		return "", err
	}
	if length > 1024*1024 { // sanity check
		return "", fmt.Errorf("string too long: %d", length)
	}
	buf := make([]byte, length)
	if _, err := io.ReadFull(r, buf); err != nil {
		return "", err
	}
	return string(buf), nil
}

func skipGGUFValue(r io.Reader, valueType uint32) {
	switch valueType {
	case 0, 1: // uint8, int8
		binary.Read(r, binary.LittleEndian, make([]byte, 1))
	case 2, 3: // uint16, int16
		binary.Read(r, binary.LittleEndian, make([]byte, 2))
	case 4, 5: // uint32, int32
		binary.Read(r, binary.LittleEndian, make([]byte, 4))
	case 6: // float32
		binary.Read(r, binary.LittleEndian, make([]byte, 4))
	case 7: // bool
		binary.Read(r, binary.LittleEndian, make([]byte, 1))
	case 8: // string
		readGGUFString(r)
	case 10, 11: // uint64, int64
		binary.Read(r, binary.LittleEndian, make([]byte, 8))
	case 12: // float64
		binary.Read(r, binary.LittleEndian, make([]byte, 8))
	case 9: // array
		var arrType uint32
		var arrLen uint32
		binary.Read(r, binary.LittleEndian, &arrType)
		binary.Read(r, binary.LittleEndian, &arrLen)
		for j := uint32(0); j < arrLen && j < 1000; j++ {
			skipGGUFValue(r, arrType)
		}
	}
}

func ggufQuantName(fileType uint32) string {
	// Common GGUF file_type values
	names := map[uint32]string{
		0:  "F32",
		1:  "F16",
		2:  "Q4_0",
		3:  "Q4_1",
		6:  "Q5_0",
		7:  "Q5_1",
		8:  "Q8_0",
		9:  "Q8_1",
		10: "Q2_K",
		11: "Q3_K_S",
		12: "Q3_K_M",
		13: "Q3_K_L",
		14: "Q4_K_S",
		15: "Q4_K_M",
		16: "Q5_K_S",
		17: "Q5_K_M",
		18: "Q6_K",
		19: "Q8_K",
		20: "IQ2_XXS",
		21: "IQ2_XS",
		22: "IQ3_XXS",
		23: "IQ3_S",
		24: "IQ3_M",
		25: "IQ4_XS",
		26: "IQ4_NL",
	}
	if name, ok := names[fileType]; ok {
		return name
	}
	return fmt.Sprintf("Q%d", fileType)
}

func guessQuantFromName(name string) string {
	name = strings.ToLower(name)
	quants := []string{
		"Q8_K", "Q8_0", "Q6_K", "Q5_K_M", "Q5_K_S", "Q5_1", "Q5_0",
		"Q4_K_M", "Q4_K_S", "Q4_1", "Q4_0", "Q3_K_L", "Q3_K_M", "Q3_K_S",
		"Q2_K", "Q2_K_S",
		"IQ4_NL", "IQ4_XS", "IQ3_M", "IQ3_S", "IQ3_XXS", "IQ2_XS", "IQ2_XXS",
		"F16", "F32", "BF16",
	}
	for _, q := range quants {
		if strings.Contains(name, strings.ToLower(q)) {
			return q
		}
	}
	return "-"
}

func formatBytes(bytes int64) string {
	const unit = 1024
	if bytes < unit {
		return fmt.Sprintf("%d B", bytes)
	}
	div, exp := int64(unit), 0
	for n := bytes / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %cB", float64(bytes)/float64(div), "KMGTPE"[exp])
}

// ─── llama.cpp download ──────────────────────────────────────────

// githubReleasesAPI 指向 llama.cpp 的 latest release API，声明为 var 以便
// 测试通过替换包级变量注入本地 httptest 服务器（与 updateRepoAPI 同风格）。
var githubReleasesAPI = "https://api.github.com/repos/ggml-org/llama.cpp/releases/latest"

const downloadDir = "llama-cpp"

// configFile 是配置持久化路径，声明为 var 以便测试通过 chdir 覆盖。
var configFile = "llama-gui-config.json"

// renameFile 为测试注入点（与 configFile 同风格），用于模拟下载完成后
// 重命名临时文件失败的分支（#10）。
var renameFile = os.Rename

func downloadLlamaCpp() {
	ctx, cancel := context.WithCancel(context.Background())
	downloadMu.Lock()
	downloadCancel = cancel
	downloadResumeCh = make(chan struct{}, 1)
	downloadMu.Unlock()

	defer func() {
		downloadMu.Lock()
		downloadCancel = nil
		downloadMu.Unlock()
		cancel()
	}()

	// Step 1: Fetch latest release info
	downloadMu.Lock()
	downloadState.Status = "fetching"
	downloadMu.Unlock()

	release, err := fetchLatestRelease()
	if err != nil {
		setDownloadError("获取发布信息失败: " + err.Error())
		return
	}

	// Step 2: Find best asset
	asset := pickBestAsset(release.Assets)
	if asset == nil {
		setDownloadError("未找到适用于当前平台的 llama.cpp 构建")
		return
	}

	downloadMu.Lock()
	downloadState.Status = "downloading"
	downloadState.FileName = asset.Name
	downloadState.Total = asset.Size
	downloadState.Version = release.TagName
	downloadState.Downloaded = 0
	downloadMu.Unlock()

	// Step 3: Download with pause/stop support
	tmpPath, err := downloadWithResume(ctx, asset.BrowserDownloadURL, asset.Size)
	if err != nil {
		if ctx.Err() != nil {
			// Cancelled by user (stop)
			downloadMu.Lock()
			if downloadState.Status != "paused" {
				downloadState.Status = "idle"
				downloadState.Error = ""
			}
			downloadMu.Unlock()
			log.Println("⏹️ llama.cpp download stopped by user")
		}
		return
	}

	defer os.Remove(tmpPath)

	// Check if stopped during download
	select {
	case <-ctx.Done():
		return
	default:
	}

	// Step 4: Extract
	downloadMu.Lock()
	downloadState.Status = "extracting"
	downloadState.Progress = 100
	downloadMu.Unlock()

	if err := os.MkdirAll(downloadDir, 0755); err != nil {
		setDownloadError("创建目录失败: " + err.Error())
		return
	}

	if strings.HasSuffix(asset.Name, ".zip") {
		err = extractZip(tmpPath, downloadDir)
	} else if strings.HasSuffix(asset.Name, ".tar.gz") {
		err = extractTarGz(tmpPath, downloadDir)
	} else {
		setDownloadError("不支持的文件格式: " + asset.Name)
		return
	}

	if err != nil {
		setDownloadError("解压失败: " + err.Error())
		return
	}

	// Step 5: Done
	downloadMu.Lock()
	downloadState.Status = "done"
	downloadState.Progress = 100
	downloadMu.Unlock()

	// Reset model cache so new models are picked up
	invalidateModelCache()
	// 失效 llama.cpp 检测缓存：GetLlamaCpp 在挂载时缓存的结果（Installed=false）
	// 已过期，解压成功后需重新检测，否则主页一直显示"未找到"
	llamaCacheValid.Store(false)

	log.Printf("[OK] llama.cpp %s downloaded and extracted to %s/", release.TagName, downloadDir)
}

// downloadWithResume downloads a file with pause/resume support.
// Returns the path to the downloaded temp file.
func downloadWithResume(ctx context.Context, url string, totalSize int64) (string, error) {
	tmpFile, err := os.CreateTemp("", "llamacpp-download-*"+filepath.Ext(url[strings.LastIndex(url, "."):]))
	if err != nil {
		return "", fmt.Errorf("创建临时文件失败: %w", err)
	}
	tmpPath := tmpFile.Name()

	// We'll loop to handle pause → resume cycles
	for {
		// Check if cancelled
		select {
		case <-ctx.Done():
			tmpFile.Close()
			return tmpPath, ctx.Err()
		default:
		}

		// Get current file size for Range header
		fi, err := os.Stat(tmpPath)
		var offset int64
		if err == nil {
			offset = fi.Size()
		}

		// Reset downloaded count to current file size on resume
		downloadMu.Lock()
		downloadState.Downloaded = offset
		downloadMu.Unlock()

		// Build request
		req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
		if err != nil {
			tmpFile.Close()
			return tmpPath, err
		}
		req.Header.Set("User-Agent", "llama-gui")
		if offset > 0 {
			req.Header.Set("Range", fmt.Sprintf("bytes=%d-", offset))
		}

		client := &http.Client{Timeout: 30 * time.Minute}
		resp, err := client.Do(req)
		if err != nil {
			// If paused, don't return error — wait for resume
			downloadMu.Lock()
			isPaused := downloadState.Paused
			resumeCh := downloadResumeCh
			downloadMu.Unlock()
			if isPaused {
				waitForResume(ctx, resumeCh)
				continue
			}
			tmpFile.Close()
			return tmpPath, err
		}

		// Handle response
		expectedStatus := http.StatusOK
		if offset > 0 {
			expectedStatus = http.StatusPartialContent
		}
		if resp.StatusCode != expectedStatus && resp.StatusCode != http.StatusOK {
			resp.Body.Close()
			tmpFile.Close()
			return tmpPath, fmt.Errorf("HTTP %d", resp.StatusCode)
		}

		// Update total from Content-Length or Content-Range
		if resp.ContentLength > 0 {
			effectiveSize := resp.ContentLength
			if offset > 0 {
				effectiveSize += offset
			}
			downloadMu.Lock()
			downloadState.Total = effectiveSize
			downloadMu.Unlock()
		}

		// Read body with pause/stop checking
		buf := make([]byte, 32*1024)
		downloaded := offset

		for {
			// Check pause
			downloadMu.Lock()
			paused := downloadState.Paused
			resumeCh := downloadResumeCh
			downloadMu.Unlock()
			if paused {
				resp.Body.Close()
				waitForResume(ctx, resumeCh)
				tmpFile.Close()
				break // breaks inner for, outer for will re-establish
			}

			// Interruptible read: do Read in goroutine, select on ctx.Done
			type readRes struct {
				n   int
				err error
			}
			ch := make(chan readRes, 1)
			go func() {
				n, err := resp.Body.Read(buf)
				ch <- readRes{n, err}
			}()

			var rr readRes
			select {
			case <-ctx.Done():
				resp.Body.Close()
				tmpFile.Close()
				return tmpPath, ctx.Err()
			case rr = <-ch:
			}

			if rr.n > 0 {
				if _, writeErr := tmpFile.Write(buf[:rr.n]); writeErr != nil {
					resp.Body.Close()
					tmpFile.Close()
					return tmpPath, writeErr
				}
				downloaded += int64(rr.n)

				downloadMu.Lock()
				downloadState.Downloaded = downloaded
				if downloadState.Total > 0 {
					downloadState.Progress = int(float64(downloaded) * 100 / float64(downloadState.Total))
				}
				downloadMu.Unlock()
			}
			if rr.err == io.EOF {
				resp.Body.Close()
				tmpFile.Close()
				return tmpPath, nil
			}
			if rr.err != nil {
				resp.Body.Close()
				tmpFile.Close()
				return tmpPath, rr.err
			}
		}
	}
}

// waitForResume blocks until the download is resumed (via resumeCh) or stopped (via ctx).
func waitForResume(ctx context.Context, resumeCh chan struct{}) {
	select {
	case <-resumeCh:
		// Resumed
	case <-ctx.Done():
		// Stopped
	}
}

// fetchLatestRelease fetches the latest llama.cpp release from the default API URL.
func fetchLatestRelease() (*GitHubRelease, error) {
	return fetchLatestReleaseAt(githubReleasesAPI)
}

// fetchLatestReleaseAt fetches and decodes a GitHub-style latest release JSON
// document from the given URL. The URL is injectable so tests can use a local
// httptest server instead of hitting the network.
func fetchLatestReleaseAt(apiURL string) (*GitHubRelease, error) {
	req, err := http.NewRequest("GET", apiURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("User-Agent", "llama-gui")

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("GitHub API returned status %d", resp.StatusCode)
	}

	var release GitHubRelease
	if err := json.NewDecoder(resp.Body).Decode(&release); err != nil {
		return nil, err
	}
	return &release, nil
}

// pickBestAsset picks the most suitable release asset for the current platform.
func pickBestAsset(assets []GitHubAsset) *GitHubAsset {
	cudaVer := ""
	if cudaInfo := getCUDAInfo(); cudaInfo.ToolkitVersion != "" {
		if parts := strings.Split(cudaInfo.ToolkitVersion, "."); len(parts) >= 2 {
			cudaVer = parts[0] + "." + parts[1]
		}
	}
	return pickBestAssetFor(assets, runtime.GOOS, runtime.GOARCH, len(getGPUInfo()) > 0, cudaVer)
}

// pickBestAssetFor scores release assets for a given platform/arch and returns
// the best match. hasCUDA and cudaVer allow preferring matching CUDA builds on
// Windows. Returns nil when no asset matches the platform.
func pickBestAssetFor(assets []GitHubAsset, platform, arch string, hasCUDA bool, cudaVer string) *GitHubAsset {
	if len(assets) == 0 {
		return nil
	}

	// Map GOOS/GOARCH to release naming conventions
	platformKey := ""
	archKey := ""
	switch platform {
	case "windows":
		platformKey = "win"
	case "darwin":
		platformKey = "macos"
	case "linux":
		platformKey = "linux"
	}
	switch arch {
	case "amd64":
		archKey = "x64"
	case "arm64":
		archKey = "arm64"
	}

	// Score each asset — higher is better
	type scored struct {
		asset *GitHubAsset
		score int
	}
	var candidates []scored

	for i := range assets {
		a := &assets[i]
		name := strings.ToLower(a.Name)

		// Must match platform
		if !strings.Contains(name, platformKey) {
			continue
		}

		// Skip if wrong arch (but "x64" is sometimes implicit for win)
		if platformKey == "win" {
			// CUDA builds on Windows: "cudart-llama-bin-win-cuda-XX.X-x64.zip"
			// Regular builds: "llama-b*-bin-win-avx2-x64.zip" etc.
		} else {
			if !strings.Contains(name, archKey) {
				continue
			}
		}

		score := 0

		// Prefer CUDA builds on Windows when GPU is available
		if platformKey == "win" && hasCUDA && strings.Contains(name, "cuda") {
			score += 100

			// Match CUDA version — prefer closest to installed toolkit
			if cudaVer != "" && strings.Contains(name, "cuda-"+cudaVer) {
				score += 50 // Exact version match
			}
		}

		// Prefer AVX2 builds (most compatible for modern CPUs)
		if strings.Contains(name, "avx2") {
			score += 30
		}

		// Prefer AVX512 (best performance on supported CPUs)
		if strings.Contains(name, "avx512") {
			score += 20
		}

		// Prefer builds without extra suffixes (generic/basic builds)
		if platformKey == "win" && !strings.Contains(name, "cuda") {
			// Basic win-x64 build
			if !strings.Contains(name, "avx") && !strings.Contains(name, "vulkan") && !strings.Contains(name, "opencl") {
				score += 10
			}
		}

		// Simple generic matches for macOS/Linux
		if platformKey != "win" && !strings.Contains(name, "kleidiai") {
			score += 10
		}

		candidates = append(candidates, scored{asset: a, score: score})
	}

	if len(candidates) == 0 {
		return nil
	}

	// Pick highest score
	best := candidates[0]
	for _, c := range candidates[1:] {
		if c.score > best.score {
			best = c
		}
	}

	return best.asset
}

// 解压大小上限（声明为 var 便于测试改小验证）。防止 zip/tar 解压炸弹
// 造成磁盘写满或内存耗尽（#2）。
var maxExtractFileSize int64 = 4 << 30   // 单文件解压上限 4GB
var maxExtractTotalSize int64 = 16 << 30 // 单次解压总上限 16GB

func extractZip(src, dest string) error {
	r, err := zip.OpenReader(src)
	if err != nil {
		return err
	}
	defer r.Close()

	var totalBytes int64
	for _, f := range r.File {
		path := filepath.Join(dest, f.Name)

		// Prevent zip slip
		cleanPath := filepath.Clean(path)
		if !strings.HasPrefix(cleanPath, filepath.Clean(dest)+string(os.PathSeparator)) {
			continue
		}

		if f.FileInfo().IsDir() {
			os.MkdirAll(path, 0755)
			continue
		}

		// 提前按声明大小拦截超大文件，避免先写出再发现超限
		if f.UncompressedSize64 > uint64(maxExtractFileSize) {
			return fmt.Errorf("文件超出解压大小上限: %s", path)
		}

		if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
			return err
		}

		rc, err := f.Open()
		if err != nil {
			return err
		}

		outFile, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, f.Mode())
		if err != nil {
			rc.Close()
			return err
		}

		// io.CopyN 只拷贝 maxExtractFileSize+1 字节：src 恰好等于上限时返回
		// (max, io.EOF)，src 超出上限时返回 (max+1, nil)，因此用
		// n > maxExtractFileSize 判断超限（#2）。
		n, copyErr := io.CopyN(outFile, rc, maxExtractFileSize+1)
		rc.Close()
		outFile.Close()
		if copyErr != nil && copyErr != io.EOF {
			return copyErr
		}
		if n > maxExtractFileSize {
			return fmt.Errorf("文件超出解压大小上限: %s", path)
		}
		totalBytes += n
		if totalBytes > maxExtractTotalSize {
			return fmt.Errorf("解压总大小超出上限: %d 字节", totalBytes)
		}
	}
	return nil
}

func extractTarGz(src, dest string) error {
	f, err := os.Open(src)
	if err != nil {
		return err
	}
	defer f.Close()

	gzReader, err := gzip.NewReader(f)
	if err != nil {
		return err
	}
	defer gzReader.Close()

	tarReader := tar.NewReader(gzReader)
	var totalBytes int64
	for {
		header, err := tarReader.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return err
		}

		path := filepath.Join(dest, header.Name)

		// Prevent path traversal
		cleanPath := filepath.Clean(path)
		if !strings.HasPrefix(cleanPath, filepath.Clean(dest)+string(os.PathSeparator)) {
			continue
		}

		switch header.Typeflag {
		case tar.TypeDir:
			os.MkdirAll(path, 0755)
		case tar.TypeReg:
			// 提前按声明的条目大小拦截超大文件（#2）
			if header.Size > maxExtractFileSize {
				return fmt.Errorf("文件超出解压大小上限: %s", path)
			}
			if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
				return err
			}
			outFile, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, os.FileMode(header.Mode))
			if err != nil {
				return err
			}
			n, copyErr := io.CopyN(outFile, tarReader, maxExtractFileSize+1)
			outFile.Close()
			if copyErr != nil && copyErr != io.EOF {
				return copyErr
			}
			if n > maxExtractFileSize {
				return fmt.Errorf("文件超出解压大小上限: %s", path)
			}
			totalBytes += n
			if totalBytes > maxExtractTotalSize {
				return fmt.Errorf("解压总大小超出上限: %d 字节", totalBytes)
			}
		default:
			// 符号链接/硬链接/设备文件等未知类型显式拒绝，避免静默跳过
			// 产生不完整解压或潜在安全问题（#6）
			return fmt.Errorf("不支持的 tar 条目类型 %d: %s", header.Typeflag, header.Name)
		}
	}
	return nil
}

func setDownloadError(msg string) {
	downloadMu.Lock()
	downloadState.Status = "error"
	downloadState.Error = msg
	downloadMu.Unlock()
	log.Printf("[ERROR] llama.cpp download error: %s", msg)
}

// ─── App update check & download ─────────────────────────────────

//go:embed VERSION
var versionFile []byte

// currentVersion 是应用当前版本，与 GitHub 发布 tag 对齐（如 v0.1.0）。
// 版本号来自 core/VERSION 文件（编译时嵌入，类似前端 .env），
// 发布新版本时修改该文件再打同名的 tag。
var currentVersion = strings.TrimSpace(string(versionFile))

// updateRepoAPI 指向本仓库的 latest release API。URL 由 CheckForUpdateAt
// 接收以支持测试注入本地 httptest 服务器。声明为 var 以便测试通过替换
// 包级变量模拟网络（与 configFile / renameFile 同风格）。
var updateRepoAPI = "https://api.github.com/repos/CodeNeow/llama-cpp-gui/releases/latest"

// compareVersions 比较两个形如 v1.2.3 的版本号（忽略前导 v / V）。
// 返回 -1 表示 a < b，0 相等，1 表示 a > b；无法解析的分段按 0 处理。
func compareVersions(a, b string) int {
	parse := func(s string) []int {
		s = strings.TrimPrefix(strings.TrimPrefix(s, "v"), "V")
		var parts []int
		for _, seg := range strings.Split(s, ".") {
			n, err := strconv.Atoi(seg)
			if err != nil {
				n = 0
			}
			parts = append(parts, n)
		}
		return parts
	}
	pa, pb := parse(a), parse(b)
	for i := 0; i < len(pa) || i < len(pb); i++ {
		var x, y int
		if i < len(pa) {
			x = pa[i]
		}
		if i < len(pb) {
			y = pb[i]
		}
		if x < y {
			return -1
		}
		if x > y {
			return 1
		}
	}
	return 0
}

// UpdateCheckResult 是更新检查结果，返回给前端判断是否有新版本。
type UpdateCheckResult struct {
	HasUpdate bool   `json:"hasUpdate"`
	Version   string `json:"version"`   // 最新版本号（tag 名，如 v0.1.1）
	Notes     string `json:"notes"`     // 发布说明
	Published string `json:"published"` // 发布时间
}

// CheckForUpdateAt 请求指定仓库的 latest release 并比较版本号。
// apiURL 可注入以便测试用 httptest 替代真实网络。
func CheckForUpdateAt(apiURL string) (*UpdateCheckResult, error) {
	release, err := fetchLatestReleaseAt(apiURL)
	if err != nil {
		return nil, err
	}
	return &UpdateCheckResult{
		HasUpdate: compareVersions(release.TagName, currentVersion) > 0,
		Version:   release.TagName,
		Notes:     release.Body,
		Published: release.PublishedAt,
	}, nil
}

// updateDownloadState 跟踪应用更新下载（更新 exe）的进度。
// 下载状态机取值：idle / downloading / done / error。
type UpdateDownloadState struct {
	Status     string `json:"status"`
	Progress   int    `json:"progress"`
	Total      int64  `json:"total"`
	Downloaded int64  `json:"downloaded"`
	Version    string `json:"version"`
	FilePath   string `json:"filePath"`
	Error      string `json:"error"`
}

var updateDownloadState = &UpdateDownloadState{Status: "idle"}
var updateDownloadMu sync.Mutex
var updateDownloadCancel context.CancelFunc

// updateExePath 为测试注入点（与 renameFile / configFile 同风格），
// 返回当前可执行文件路径，用于确定更新 exe 的目标目录。
var updateExePath = os.Executable

// pickUpdateAsset 挑选更新下载使用的资产：优先主程序 exe，跳过 installer。
// 若版本低于当前版本或没有可用的 exe 资产，返回 nil。
func pickUpdateAsset(assets []GitHubAsset) *GitHubAsset {
	for i := range assets {
		a := &assets[i]
		name := strings.ToLower(a.Name)
		if strings.HasSuffix(name, ".exe") && !strings.Contains(name, "installer") {
			return a
		}
	}
	return nil
}

// downloadUpdateRelease 下载新版本 exe 到可执行文件同目录（无法直接替换
// 正在运行的自身），完成后提示用户关闭应用后手动替换。
func downloadUpdateRelease(version string) {
	ctx, cancel := context.WithCancel(context.Background())
	updateDownloadMu.Lock()
	updateDownloadCancel = cancel
	updateDownloadMu.Unlock()

	defer func() {
		updateDownloadMu.Lock()
		updateDownloadCancel = nil
		updateDownloadMu.Unlock()
		cancel()
	}()

	// Step 1: 拉取最新发布信息，挑主程序 exe 资产
	updateDownloadMu.Lock()
	updateDownloadState.Status = "downloading"
	updateDownloadState.Progress = 0
	updateDownloadState.Downloaded = 0
	updateDownloadState.Total = 0
	updateDownloadState.Version = version
	updateDownloadState.Error = ""
	updateDownloadMu.Unlock()

	release, err := fetchLatestReleaseAt(updateRepoAPI)
	if err != nil {
		setUpdateDownloadError("获取发布信息失败: " + err.Error())
		return
	}
	asset := pickUpdateAsset(release.Assets)
	if asset == nil {
		setUpdateDownloadError("未找到适用于当前平台的主程序")
		return
	}

	updateDownloadMu.Lock()
	updateDownloadState.Total = asset.Size
	updateDownloadMu.Unlock()

	// Step 2: 下载到可执行文件同目录，命名 llama-gui-v<tag>.exe
	exePath, err := updateExePath()
	if err != nil {
		setUpdateDownloadError("无法定位可执行文件路径: " + err.Error())
		return
	}
	dir := filepath.Dir(exePath)
	fileName := "llama-gui-" + release.TagName + ".exe"
	destPath := filepath.Join(dir, fileName)

	tmpPath, err := downloadUpdateWithResume(ctx, asset.BrowserDownloadURL, asset.Size)
	if err != nil {
		if ctx.Err() != nil {
			updateDownloadMu.Lock()
			updateDownloadState.Status = "idle"
			updateDownloadMu.Unlock()
			log.Println("[INFO] update download stopped by user")
		} else {
			setUpdateDownloadError("下载失败: " + err.Error())
		}
		return
	}
	defer os.Remove(tmpPath)

	// Step 3: 移动到目标路径（同目录 + 目标存在则先删除旧文件）
	if err := os.Rename(tmpPath, destPath); err != nil {
		if removeErr := os.Remove(destPath); removeErr == nil {
			err = os.Rename(tmpPath, destPath)
		}
		if err != nil {
			setUpdateDownloadError("保存文件失败: " + err.Error())
			return
		}
	}

	updateDownloadMu.Lock()
	updateDownloadState.Status = "done"
	updateDownloadState.Progress = 100
	updateDownloadState.FilePath = destPath
	updateDownloadMu.Unlock()

	log.Printf("[OK] update %s downloaded to %s", release.TagName, destPath)
}

func setUpdateDownloadError(msg string) {
	updateDownloadMu.Lock()
	updateDownloadState.Status = "error"
	updateDownloadState.Error = msg
	updateDownloadMu.Unlock()
	log.Printf("[ERROR] update download error: %s", msg)
}

// downloadUpdateWithResume 下载更新文件到临时文件并上报进度，支持取消。
// 与 downloadWithResume 不同：更新 exe 体量小、不支持暂停/断点续传，
// 仅响应 context 取消（应用退出/停止下载）。
func downloadUpdateWithResume(ctx context.Context, url string, totalSize int64) (string, error) {
	tmpFile, err := os.CreateTemp("", "llama-gui-update-*.exe")
	if err != nil {
		return "", fmt.Errorf("创建临时文件失败: %w", err)
	}
	tmpPath := tmpFile.Name()

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		tmpFile.Close()
		return tmpPath, err
	}
	req.Header.Set("User-Agent", "llama-gui")

	client := &http.Client{Timeout: 30 * time.Minute}
	resp, err := client.Do(req)
	if err != nil {
		tmpFile.Close()
		return tmpPath, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		tmpFile.Close()
		return tmpPath, fmt.Errorf("HTTP %d", resp.StatusCode)
	}

	buf := make([]byte, 32*1024)
	downloaded := int64(0)
	for {
		type readRes struct {
			n   int
			err error
		}
		ch := make(chan readRes, 1)
		go func() {
			n, err := resp.Body.Read(buf)
			ch <- readRes{n, err}
		}()

		var rr readRes
		select {
		case <-ctx.Done():
			tmpFile.Close()
			return tmpPath, ctx.Err()
		case rr = <-ch:
		}

		if rr.n > 0 {
			if _, writeErr := tmpFile.Write(buf[:rr.n]); writeErr != nil {
				tmpFile.Close()
				return tmpPath, writeErr
			}
			downloaded += int64(rr.n)
			updateDownloadMu.Lock()
			updateDownloadState.Downloaded = downloaded
			if updateDownloadState.Total > 0 {
				updateDownloadState.Progress = int(float64(downloaded) * 100 / float64(updateDownloadState.Total))
			}
			updateDownloadMu.Unlock()
		}
		if rr.err != nil {
			if rr.err == io.EOF {
				tmpFile.Close()
				return tmpPath, nil
			}
			tmpFile.Close()
			return tmpPath, rr.err
		}
	}
}

// ─── llama-server manager ────────────────────────────────────────

var serverCmd *exec.Cmd
var serverLogs []string
var serverLogsMu sync.Mutex
var serverRunning bool

// serverMu 保护 serverCmd 与 serverRunning 的完整生命周期（创建/启停/清理），
// 与只保护 serverLogs 的 serverLogsMu 职责分离（#3）。任何同时持有两把锁的
// 路径必须按「先 serverMu 后 serverLogsMu」的顺序获取，避免死锁。
var serverMu sync.Mutex

type serverLogWriter struct{}

func (w *serverLogWriter) Write(p []byte) (int, error) {
	addServerLog(strings.TrimSpace(string(p)))
	return len(p), nil
}

func addServerLog(msg string) {
	serverLogsMu.Lock()
	serverLogs = append(serverLogs, msg)
	if len(serverLogs) > 200 {
		serverLogs = serverLogs[len(serverLogs)-100:]
	}
	serverLogsMu.Unlock()
	log.Println("[llama-server]", msg)
}

// validIniValue 校验将写入 INI 预设的值：不得含换行/空字符（防止配置注入）
// 且不得有首尾空白（避免值被静默裁剪引起歧义）。
func validIniValue(s string) bool {
	return !strings.ContainsAny(s, "\n\r\x00") && s == strings.TrimSpace(s)
}

// validGPULayersValue 校验 gpu-layers 取值：允许空、auto、all、0 或纯正整数。
func validGPULayersValue(s string) bool {
	if s == "" || s == "auto" || s == "all" || s == "0" {
		return true
	}
	n, err := strconv.Atoi(s)
	return err == nil && n > 0
}

// validCacheTypeValue 校验 cache-type-k/v 取值白名单。
func validCacheTypeValue(s string) bool {
	switch s {
	case "", "q8_0", "q4_0", "f16", "bf16":
		return true
	}
	return false
}

// generateModelsPreset scans the default model directory and writes a llama-server
// INI preset to a temp file, returning its path.
func generateModelsPreset() (string, error) {
	models := scanModels()
	if len(models) == 0 {
		return "", fmt.Errorf("LLM-Models 目录中没有模型")
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
		return "", fmt.Errorf("LLM-Models 目录中没有模型")
	}

	var buf bytes.Buffer
	// 稳定去重别名：sanitizeAlias 会把空格/斜杠等不同字符统一为 '-'，
	// 不同模型名可能碰撞出相同段名（#7.1）。按模型顺序对已占用的别名
	// 追加 -2、-3… 直到唯一，结果确定、不依赖随机/时间。
	used := make(map[string]int)
	for _, m := range models {
		alias := sanitizeAlias(m.Name)
		if used[alias] > 0 {
			for n := used[alias] + 1; ; n++ {
				candidate := fmt.Sprintf("%s-%d", alias, n)
				if used[candidate] == 0 {
					alias = candidate
					break
				}
			}
		}
		used[alias]++
		buf.WriteString(fmt.Sprintf("[%s]\n", alias))
		buf.WriteString(fmt.Sprintf("model = %s\n", filepath.ToSlash(m.Path)))

		// Auto-detect embedding model from name or architecture
		if isEmbeddingModel(m) {
			buf.WriteString("embeddings = true\n")
		}

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
					return "", fmt.Errorf("非法 GPULayers 值 %q：不能包含换行或首尾空白", cfg.GPULayers)
				}
				buf.WriteString(fmt.Sprintf("gpu-layers = %s\n", cfg.GPULayers))
			}
			if cfg.FlashAttn {
				buf.WriteString("flash-attn = on\n")
			}
			if cfg.CacheTypeK != "" {
				if !validIniValue(cfg.CacheTypeK) {
					return "", fmt.Errorf("非法 CacheTypeK 值 %q：不能包含换行或首尾空白", cfg.CacheTypeK)
				}
				buf.WriteString(fmt.Sprintf("cache-type-k = %s\n", cfg.CacheTypeK))
			}
			if cfg.CacheTypeV != "" {
				if !validIniValue(cfg.CacheTypeV) {
					return "", fmt.Errorf("非法 CacheTypeV 值 %q：不能包含换行或首尾空白", cfg.CacheTypeV)
				}
				buf.WriteString(fmt.Sprintf("cache-type-v = %s\n", cfg.CacheTypeV))
			}
			if cfg.MLock {
				buf.WriteString("mlock = true\n")
			}
			if cfg.NoMMap {
				buf.WriteString("no-mmap = true\n")
			}
		}
		if m.HasMMProj {
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

func sanitizeAlias(name string) string {
	name = strings.ReplaceAll(name, " ", "-")
	name = strings.Map(func(r rune) rune {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '-' || r == '_' || r == '.' {
			return r
		}
		return '-'
	}, name)
	return strings.ToLower(name)
}

// ─── Config persistence ─────────────────────────────────────────

type appConfig struct {
	LlamaCppDir  string                 `json:"llamaCppDir"`
	ModelDir     string                 `json:"modelDir"`
	Theme        string                 `json:"theme"`
	ModelConfigs map[string]ModelConfig `json:"modelConfigs"`
	ServerConfig ServerConfig           `json:"serverConfig"`
}

type ServerConfig struct {
	Host      string `json:"host"`
	Port      int    `json:"port"`
	MaxModels int    `json:"maxModels"`
	CacheRAM  int    `json:"cacheRam"`
}

type ModelConfig struct {
	Threads    int    `json:"threads"`
	GPULayers  string `json:"gpuLayers"`
	CtxSize    int    `json:"ctxSize"`
	BatchSize  int    `json:"batchSize"`
	UBatchSize int    `json:"ubatchSize"`
	FlashAttn  bool   `json:"flashAttn"`
	CacheTypeK string `json:"cacheTypeK"`
	CacheTypeV string `json:"cacheTypeV"`
	MLock      bool   `json:"mlock"`
	NoMMap     bool   `json:"noMmap"`
}

func loadConfig() {
	data, err := os.ReadFile(configFile)
	if err != nil {
		return // file doesn't exist yet, that's ok
	}
	var cfg appConfig
	if err := json.Unmarshal(data, &cfg); err != nil {
		log.Printf("[WARN] Failed to parse config file: %v", err)
		return
	}
	if cfg.LlamaCppDir != "" {
		customLlamaCppMu.Lock()
		customLlamaCppDir = cfg.LlamaCppDir
		customLlamaCppMu.Unlock()
		log.Printf("[DIR] Loaded custom llama.cpp dir from config: %s", cfg.LlamaCppDir)
	}
	// 自定义模型目录：值为空或路径不存在/非目录时忽略并回退默认目录，
	// 防止配置损坏或目录被删除后扫描/下载落在无效路径上。
	if cfg.ModelDir != "" {
		if fi, err := os.Stat(cfg.ModelDir); err != nil || !fi.IsDir() {
			log.Printf("[WARN] Ignoring invalid model dir from config: %s", cfg.ModelDir)
		} else {
			modelsDirMu.Lock()
			customModelsDir = cfg.ModelDir
			modelsDirMu.Unlock()
			log.Printf("[DIR] Loaded custom models dir from config: %s", cfg.ModelDir)
		}
	}
	if cfg.Theme == "" {
		cfg.Theme = "light"
	}
	currentTheme = cfg.Theme
	if cfg.ModelConfigs == nil {
		cfg.ModelConfigs = make(map[string]ModelConfig)
	}
	cachedModelConfigs = cfg.ModelConfigs
	// Merge server config with defaults
	scfg := defaultServerConfig()
	if cfg.ServerConfig.Host != "" {
		scfg.Host = cfg.ServerConfig.Host
	}
	if cfg.ServerConfig.Port != 0 {
		scfg.Port = cfg.ServerConfig.Port
	}
	if cfg.ServerConfig.MaxModels != 0 {
		scfg.MaxModels = cfg.ServerConfig.MaxModels
	}
	if cfg.ServerConfig.CacheRAM != 0 {
		scfg.CacheRAM = cfg.ServerConfig.CacheRAM
	}
	cachedServerConfig = scfg
}

var cachedModelConfigs = make(map[string]ModelConfig)
var modelConfigsMu sync.Mutex
var cachedServerConfig = defaultServerConfig()
var serverConfigMu sync.Mutex

func defaultServerConfig() ServerConfig {
	return ServerConfig{
		Host: "127.0.0.1", Port: 8080, MaxModels: 1, CacheRAM: 8192,
	}
}

var currentTheme = "light"
var configMu sync.Mutex

func saveConfig() {
	customLlamaCppMu.Lock()
	dir := customLlamaCppDir
	customLlamaCppMu.Unlock()

	modelsDirMu.Lock()
	modelDir := customModelsDir
	modelsDirMu.Unlock()

	configMu.Lock()
	theme := currentTheme
	configMu.Unlock()

	modelConfigsMu.Lock()
	mcfgs := make(map[string]ModelConfig, len(cachedModelConfigs))
	for k, v := range cachedModelConfigs {
		mcfgs[k] = v
	}
	modelConfigsMu.Unlock()

	serverConfigMu.Lock()
	scfg := cachedServerConfig
	serverConfigMu.Unlock()

	cfg := appConfig{LlamaCppDir: dir, ModelDir: modelDir, Theme: theme, ModelConfigs: mcfgs, ServerConfig: scfg}
	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		log.Printf("[WARN] Failed to marshal config: %v", err)
		return
	}
	if err := os.WriteFile(configFile, data, 0644); err != nil {
		log.Printf("[WARN] Failed to write config file: %v", err)
	}
}

// ─── HF Mirror API ───────────────────────────────────────────────

const hfMirrorBase = "https://hf-mirror.com"

// searchHFMirror queries the default HF Mirror endpoint.
func searchHFMirror(q string, filter string) ([]HFSearchResult, error) {
	return searchHFMirrorAt(hfMirrorBase, q, filter)
}

// searchHFMirrorAt queries an HF-compatible API base for models matching q,
// filtering to models containing GGUF files. filter 参数已弃用，仅保留签名兼容，
// 不再按 pipeline_tag 类型过滤（embedding / llm 分类已移除）。
// API 不支持 library 过滤与分页，只能以较大 limit 拉取候选后过滤 GGUF。
// 为覆盖尽可能多的候选，并行请求 downloads / likes / lastModified 三种排序
// （各 limit=200&full=true），各自过滤 GGUF 后按 downloads → likes →
// lastModified 顺序按 modelId 合并去重（已见过的跳过）。任一排序请求失败仅跳过
// 该路（打 [WARN]），三路全部失败才返回错误。
func searchHFMirrorAt(baseURL, q, filter string) ([]HFSearchResult, error) {
	sorts := []string{"downloads", "likes", "lastModified"}

	type routeResult struct {
		results []HFSearchResult
		err     error
	}
	routeResults := make([]routeResult, len(sorts))

	// 三路排序并行拉取，每路独立结果切片，无共享写入
	var wg sync.WaitGroup
	for i, sort := range sorts {
		wg.Add(1)
		go func(i int, sort string) {
			defer wg.Done()
			routeResults[i].results, routeResults[i].err = searchHFMirrorSortAt(baseURL, q, sort)
		}(i, sort)
	}
	wg.Wait()

	var results []HFSearchResult
	seen := make(map[string]bool)
	failed := 0
	for i, sort := range sorts {
		if routeResults[i].err != nil {
			failed++
			log.Printf("[WARN] HF 搜索排序 %s 请求失败，跳过该路: %v", sort, routeResults[i].err)
			continue
		}
		for _, r := range routeResults[i].results {
			key := r.ModelID
			if key == "" {
				key = r.ID
			}
			if seen[key] {
				continue
			}
			seen[key] = true
			results = append(results, r)
		}
	}

	if failed == len(sorts) {
		return nil, fmt.Errorf("HF 搜索三路排序（downloads/likes/lastModified）请求全部失败")
	}
	return results, nil
}

// searchHFMirrorSortAt 以指定 sort 向 HF 兼容 API 拉取一页候选
// （limit=200&full=true），过滤出含 GGUF 文件的结果。请求失败或非 200 时返回
// 错误，由 searchHFMirrorAt 决定整路跳过（不影响其他排序）。
func searchHFMirrorSortAt(baseURL, q, sort string) ([]HFSearchResult, error) {
	apiURL := fmt.Sprintf("%s/api/models?search=%s&sort=%s&limit=200&full=true", baseURL, q, sort)

	req, err := http.NewRequest("GET", apiURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "llama-gui")

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("HF API returned status %d", resp.StatusCode)
	}

	var rawResults []struct {
		ID          string   `json:"id"`
		ModelID     string   `json:"modelId"`
		Author      string   `json:"author"`
		Downloads   int      `json:"downloads"`
		Likes       int      `json:"likes"`
		PipelineTag string   `json:"pipeline_tag"`
		Tags        []string `json:"tags"`
		Siblings    []HFFile `json:"siblings"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&rawResults); err != nil {
		return nil, err
	}

	var results []HFSearchResult
	for _, r := range rawResults {
		result := HFSearchResult{
			ID:          r.ID,
			ModelID:     r.ModelID,
			Author:      r.Author,
			Downloads:   r.Downloads,
			Likes:       r.Likes,
			PipelineTag: r.PipelineTag,
			Tags:        r.Tags,
			Siblings:    r.Siblings,
		}

		// Only include models that have .gguf files
		if !hasGGUF(result) {
			continue
		}

		results = append(results, result)
	}

	return results, nil
}

// hasGGUF 判断 HF 搜索结果是否包含 .gguf 后缀文件（HF 搜索候选的 GGUF 过滤）。
func hasGGUF(r HFSearchResult) bool {
	for _, s := range r.Siblings {
		if strings.HasSuffix(strings.ToLower(s.Filename), ".gguf") {
			return true
		}
	}
	return false
}

// getModelDescription fetches a model's README description via the default mirror.
func getModelDescription(modelID string) (string, error) {
	return getModelDescriptionAt(hfMirrorBase, modelID)
}

// getModelDescriptionAt fetches the README of a model on an HF-compatible base
// and extracts its natural-language description:
//   - GET {base}/{modelID}/raw/main/README.md（User-Agent llama-gui，30s 超时）
//   - 非 200 返回错误；YAML front-matter（首行为 --- 的块）会被跳过
//   - 按空行分段，取第一个「非空且不以 # 开头」的段落，trim 后截断 200 rune
//   - README 存在但没有描述段落时返回空串与 nil 错误（静默）
func getModelDescriptionAt(baseURL, modelID string) (string, error) {
	readmeURL := fmt.Sprintf("%s/%s/raw/main/README.md", baseURL, modelID)

	req, err := http.NewRequest("GET", readmeURL, nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("User-Agent", "llama-gui")

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("README 获取失败: HTTP %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}

	lines := strings.Split(string(body), "\n")
	start := 0
	// 跳过 YAML front-matter：首行 trim 后为 ---，则跳过到下一个 --- 之后
	if len(lines) > 0 && strings.TrimSpace(lines[0]) == "---" {
		for i := 1; i < len(lines); i++ {
			if strings.TrimSpace(lines[i]) == "---" {
				start = i + 1
				break
			}
		}
	}

	// 按空行分段，取第一个「非空且不以 # 开头」的段落
	var paragraphs []string
	var cur strings.Builder
	flush := func() {
		if cur.Len() > 0 {
			paragraphs = append(paragraphs, cur.String())
			cur.Reset()
		}
	}
	for _, line := range lines[start:] {
		if strings.TrimSpace(line) == "" {
			flush()
			continue
		}
		if cur.Len() > 0 {
			cur.WriteString("\n")
		}
		cur.WriteString(line)
	}
	flush()

	for _, p := range paragraphs {
		trimmed := strings.TrimSpace(p)
		if trimmed == "" || strings.HasPrefix(trimmed, "#") {
			continue
		}
		// 截断 200 个 rune，超出加省略号
		runes := []rune(trimmed)
		if len(runes) > 200 {
			return string(runes[:200]) + "...", nil
		}
		return trimmed, nil
	}

	return "", nil
}

// getHFModelFiles lists downloadable GGUF files for a model via the default mirror.
func getHFModelFiles(modelID string) ([]HFFileOut, error) {
	return getHFModelFilesAt(hfMirrorBase, modelID)
}

// getHFModelFilesAt lists the GGUF siblings of a model on an HF-compatible API base.
func getHFModelFilesAt(baseURL, modelID string) ([]HFFileOut, error) {
	apiURL := fmt.Sprintf("%s/api/models/%s", baseURL, modelID)

	req, err := http.NewRequest("GET", apiURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "llama-gui")

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("HF API returned status %d", resp.StatusCode)
	}

	var raw struct {
		Siblings []HFFile `json:"siblings"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&raw); err != nil {
		return nil, err
	}

	// Filter to only .gguf files
	var files []HFFileOut
	for _, s := range raw.Siblings {
		name := strings.TrimPrefix(s.Filename, "/")
		if !strings.HasSuffix(name, "/") && !strings.HasPrefix(name, ".") && strings.HasSuffix(strings.ToLower(name), ".gguf") {
			files = append(files, HFFileOut{Filename: name, Size: s.Size})
		}
	}

	return files, nil
}

// ─── Download task runner ────────────────────────────────────────

func downloadTask(task *DlTask) {
	dlTasksMu.Lock()
	task.Status = "downloading"
	dlTasksMu.Unlock()

	// Create dest directory
	if err := os.MkdirAll(task.DestDir, 0755); err != nil {
		dlTasksMu.Lock()
		task.Status = "error"
		task.Error = "创建目录失败: " + err.Error()
		dlTasksMu.Unlock()
		return
	}

	destPath := filepath.Join(task.DestDir, task.FileName)
	tmpPath := destPath + ".part"

	// Check if partial download exists for resume
	var offset int64
	if fi, err := os.Stat(tmpPath); err == nil {
		offset = fi.Size()
	}

	client := &http.Client{Timeout: 30 * time.Minute}

	for {
		// Check cancellation
		select {
		case <-task.ctx.Done():
			dlTasksMu.Lock()
			if task.Status != "paused" {
				task.Status = "cancelled"
			}
			dlTasksMu.Unlock()
			return
		default:
		}

		req, err := buildDownloadRequest(task.URL, offset)
		if err != nil {
			dlTasksMu.Lock()
			task.Status = "error"
			task.Error = err.Error()
			dlTasksMu.Unlock()
			return
		}

		resp, err := client.Do(req)
		if err != nil {
			dlTasksMu.Lock()
			if task.Status == "paused" {
				resumeCh := task.resumeCh
				dlTasksMu.Unlock()
				waitForTaskResume(task, resumeCh)
				continue
			}
			task.Status = "error"
			task.Error = err.Error()
			dlTasksMu.Unlock()
			return
		}

		if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusPartialContent {
			resp.Body.Close()
			dlTasksMu.Lock()
			task.Status = "error"
			task.Error = fmt.Sprintf("HTTP %d", resp.StatusCode)
			dlTasksMu.Unlock()
			return
		}

		if resp.ContentLength > 0 {
			dlTasksMu.Lock()
			task.Total = offset + resp.ContentLength
			task.SizeHuman = formatBytes(task.Total)
			dlTasksMu.Unlock()
		}

		// Open temp file for append
		out, err := os.OpenFile(tmpPath, os.O_WRONLY|os.O_CREATE|os.O_APPEND, 0644)
		if err != nil {
			resp.Body.Close()
			dlTasksMu.Lock()
			task.Status = "error"
			task.Error = err.Error()
			dlTasksMu.Unlock()
			return
		}

		buf := make([]byte, 32*1024)
		downloaded := offset

		for {
			// Check pause
			dlTasksMu.Lock()
			paused := task.Status == "paused"
			resumeCh := task.resumeCh
			dlTasksMu.Unlock()
			if paused {
				resp.Body.Close()
				out.Close()
				waitForTaskResume(task, resumeCh)
				// Update offset for resume
				if fi, err := os.Stat(tmpPath); err == nil {
					offset = fi.Size()
				}
				break // outer loop will re-establish connection
			}

			// Interruptible read
			type readRes struct {
				n   int
				err error
			}
			ch := make(chan readRes, 1)
			go func() {
				n, err := resp.Body.Read(buf)
				ch <- readRes{n, err}
			}()

			var rr readRes
			select {
			case <-task.ctx.Done():
				resp.Body.Close()
				out.Close()
				dlTasksMu.Lock()
				task.Status = "cancelled"
				dlTasksMu.Unlock()
				return
			case rr = <-ch:
			}

			if rr.n > 0 {
				if _, err := out.Write(buf[:rr.n]); err != nil {
					resp.Body.Close()
					out.Close()
					dlTasksMu.Lock()
					task.Status = "error"
					task.Error = err.Error()
					dlTasksMu.Unlock()
					return
				}
				downloaded += int64(rr.n)
				dlTasksMu.Lock()
				task.Downloaded = downloaded
				if task.Total > 0 {
					task.Progress = int(float64(downloaded) * 100 / float64(task.Total))
				}
				dlTasksMu.Unlock()
			}
			if rr.err == io.EOF {
				resp.Body.Close()
				out.Close()
				// 重命名失败时标记任务错误并返回，不再推进到 done（#10）。
				// 注意 renameFile 是可注入的包级变量，测试可模拟失败。
				if err := renameFile(tmpPath, destPath); err != nil {
					dlTasksMu.Lock()
					task.Status = "error"
					task.Error = "重命名失败: " + err.Error()
					dlTasksMu.Unlock()
					return
				}
				dlTasksMu.Lock()
				task.Status = "done"
				task.Progress = 100
				dlTasksMu.Unlock()
				invalidateModelCache()
				return
			}
			if rr.err != nil {
				resp.Body.Close()
				out.Close()
				dlTasksMu.Lock()
				task.Status = "error"
				task.Error = rr.err.Error()
				dlTasksMu.Unlock()
				return
			}
		}
	}
}

// buildDownloadRequest creates a GET request for a download URL with the
// llama-gui User-Agent, adding a Range header when resuming from an offset.
func buildDownloadRequest(downloadURL string, offset int64) (*http.Request, error) {
	req, err := http.NewRequest("GET", downloadURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "llama-gui")
	if offset > 0 {
		req.Header.Set("Range", fmt.Sprintf("bytes=%d-", offset))
	}
	return req, nil
}

func waitForTaskResume(task *DlTask, resumeCh chan struct{}) {
	select {
	case <-resumeCh:
	case <-task.ctx.Done():
	}
}
