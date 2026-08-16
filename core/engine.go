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
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
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
	ID         string  `json:"id"`
	ModelID    string  `json:"modelId"`
	FileName   string  `json:"fileName"`
	DestDir    string  `json:"destDir"`
	Source     string  `json:"source"` // 下载源: hf / modelscope
	URL        string  `json:"-"`
	Status     string  `json:"status"`
	Progress   int     `json:"progress"`
	Total      int64   `json:"total"`
	Downloaded int64   `json:"downloaded"`
	SizeHuman  string  `json:"sizeHuman"`
	Speed      float64 `json:"speed"` // 当前下载速度（字节/秒）
	Error      string  `json:"error"`
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

// ─── Download source (HF Mirror / ModelScope) ────────────────────

const (
	sourceHF              = "hf"
	sourceModelScope      = "modelscope"
	defaultDownloadSource = sourceHF
)

// downloadSource 为当前模型下载源（hf / modelscope），downloadSourceMu 保护其
// 读写，与 customLlamaCppDir 等配置项的风格保持一致。搜索、文件列表、描述与
// 下载 URL 构建均以 activeDownloadSource() 的当前值路由。
var downloadSource = defaultDownloadSource
var downloadSourceMu sync.Mutex

// activeDownloadSource 返回当前生效的下载源，在锁内读取。
func activeDownloadSource() string {
	downloadSourceMu.Lock()
	s := downloadSource
	downloadSourceMu.Unlock()
	return s
}

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

// llamaVersionProbeTimeout 为 llama.cpp 版本探测的超时上限。正常二进制
// --version 毫秒级返回；超时说明二进制异常（如把 -v 误当版本标志而启动
// 完整 HTTP 服务器并无限运行），此时 kill 子进程并返回空，保证
// getLlamaCppInfo 快速返回、检测链不被任何异常二进制冻结。设计为包级 var
// 而非 const，便于测试临时缩短超时验证 kill 行为（与 probeLlamaVersion 同
// 风格的注入点，测试用后立即恢复）。
var llamaVersionProbeTimeout = 5 * time.Second

// probeLlamaVersion 为 llama.cpp 版本探测命令执行注入点（与
// githubReleasesAPI / renameFile / updateRepoAPI 同风格的包级 var）：
// 默认实现带超时运行 `path --version` 并合并 stdout+stderr。测试可替换该
// 变量注入假探测命令，避免真实启动二进制；同时由于探针参数（--version）
// 封装在默认实现内部，替换后可直接断言只调用了 --version、从不回退 -v。
var probeLlamaVersion = func(path string) string {
	ctx, cancel := context.WithTimeout(context.Background(), llamaVersionProbeTimeout)
	defer cancel()

	cmd := exec.CommandContext(ctx, path, "--version")
	hideWindow(cmd)
	var out, errOut bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &errOut
	// runCmd 只捕获 stdout，而 llama-server 的 --version 输出全部走 stderr
	// （实证 stdout 为空），这里合并两者才能拿到版本号
	if err := cmd.Run(); err != nil && errOut.Len() > 0 {
		log.Printf("[CMD] %s --version stderr: %s", path, strings.TrimSpace(errOut.String()))
	}
	return strings.TrimSpace(out.String() + errOut.String())
}

// parseLlamaVersion 从版本探测输出中提取版本号：优先取以 "version" 开头或
// 含 "build" 的行（llama.cpp --version 的典型输出，如 "version: 1234"），
// 否则返回整体 trim 后的输出。纯字符串逻辑，供单元测试直接断言。
func parseLlamaVersion(versionOut string) string {
	versionOut = strings.TrimSpace(versionOut)
	if versionOut == "" {
		return ""
	}
	for _, line := range strings.Split(versionOut, "\n") {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "version") || strings.Contains(trimmed, "build") {
			return trimmed
		}
	}
	return versionOut
}

// fillLlamaCppVersion 尝试运行二进制读取版本号填充 info.Version。探测只调用
// `--version`（合并 stdout+stderr），不再回退 `-v`：llama-server 10342 的 -v
// 不是版本标志而是启动完整 HTTP 服务器，曾导致版本探测无限阻塞、主页永久
// 显示"未找到"。带超时保护，任何异常二进制都不会冻结检测链。运行失败（如
// Windows 上 stub 非可执行文件）只影响 Version 为空，不影响 Installed。
func fillLlamaCppVersion(info *LlamaCppInfo, path string) {
	versionOut := probeLlamaVersion(path)
	if versionOut != "" {
		info.Version = parseLlamaVersion(versionOut)
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

// scanModelsDir scans the model directory tree for GGUF models. Both the
// <author>/<variant>/<files>.gguf layout (three-level, current HF download
// destination) and the <author>/<file>.gguf layout (two-level, produced by
// earlier HF download versions) are recognized. A variant directory counts
// as a model when it contains at least one non-mmproj .gguf file; mmproj
// files only flag multimodal support.
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

		// Second-level: variant directories and/or loose .gguf files
		entries, err := os.ReadDir(authorDir)
		if err != nil {
			continue
		}

		for _, entry := range entries {
			if entry.IsDir() {
				// Three-level layout: <author>/<variant>/<files>.gguf
				variantName := entry.Name()
				variantDir := filepath.Join(authorDir, variantName)

				// Find .gguf files in this variant directory
				files, err := os.ReadDir(variantDir)
				if err != nil {
					continue
				}

				var mainGGUF string
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
					}
				}

				if mainGGUF == "" {
					continue
				}

				model := buildModelInfo(mainGGUF, author, variantName)
				model.HasMMProj = hasMMProj
				models = append(models, model)
				continue
			}

			// Two-level layout: loose .gguf files directly under the author
			// directory. Non-.gguf files are skipped; loose mmproj-*.gguf
			// files cannot be tied to a model here and are skipped too,
			// matching the three-level rule that mmproj never counts as a
			// main model.
			name := entry.Name()
			lower := strings.ToLower(name)
			if !strings.HasSuffix(lower, ".gguf") {
				continue
			}
			if strings.HasPrefix(lower, "mmproj") {
				continue
			}

			model := buildModelInfo(filepath.Join(authorDir, name), author,
				strings.TrimSuffix(name, filepath.Ext(name)))
			models = append(models, model)
		}
	}

	sort.Slice(models, func(i, j int) bool {
		return models[i].SizeBytes > models[j].SizeBytes
	})

	return models
}

// buildModelInfo 从一个 GGUF 主文件路径构建 ModelInfo：读取 GGUF 元数据覆盖
// 名称/架构/量化，缺失时用 fallbackName/author 兜底。两级与三级扫描共用，
// 避免 variant 目录与 author 散文件两处重复同样的元数据读取与回退逻辑。
func buildModelInfo(path, author, fallbackName string) ModelInfo {
	model := ModelInfo{Author: author, Path: path, Name: fallbackName}
	if fi, err := os.Stat(path); err == nil {
		model.SizeBytes = fi.Size()
		model.SizeHuman = formatBytes(model.SizeBytes)
	}
	// Try to read GGUF metadata for better name/arch/quant
	if metadata := readGGUFMeta(path); metadata != nil {
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
	// Fallback quantization from fallbackName, then from file name
	if model.Quantization == "" {
		model.Quantization = guessQuantFromName(fallbackName)
		if model.Quantization == "-" {
			model.Quantization = guessQuantFromName(filepath.Base(path))
		}
	}
	// Fallback architecture from fallbackName/author
	if model.Architecture == "" {
		model.Architecture = guessArchFromName(fallbackName + " " + author)
	}
	return model
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

// llamaCppDownloadDir 返回 llama.cpp 下载解压的目标目录：用户设置过自定义
// llama.cpp 目录（customLlamaCppDir）时优先安装到该目录，否则退回默认的
// llama-cpp/。与 getLlamaCppInfo / resolveLlamaServerBin 的检测优先级
// （customLlamaCppDir > downloadDir > PATH）保持一致，保证下载产物落点
// 与检测位置一致。
func llamaCppDownloadDir() string {
	customLlamaCppMu.Lock()
	customDir := customLlamaCppDir
	customLlamaCppMu.Unlock()
	if customDir != "" {
		return customDir
	}
	return downloadDir
}

// configFile 是配置持久化路径，声明为 var 以便测试通过 chdir 覆盖。
var configFile = "llama-desktop-config.json"

// legacyConfigFile 是 llama-gui → llama-desktop 更名前的配置文件名。仅作
// 一次性迁移来源（见 migrateLegacyConfig）：新文件不存在而旧文件存在时整体
// 改名复用，老用户的主题 / 目录 / 模型参数 / 下载队列无损保留。
var legacyConfigFile = "llama-gui-config.json"

// renameFile 为测试注入点（与 configFile 同风格），用于模拟下载完成后
// 重命名临时文件失败的分支（#10）。
var renameFile = os.Rename

// copyFile 把 src 复制到 dst：以 src 的 FileMode 创建 dst 并显式 chmod，
// 保留执行权限（Linux 更新 exe 依赖 +x），避免受 umask 影响。
func copyFile(src, dst string) error {
	srcF, err := os.Open(src)
	if err != nil {
		return fmt.Errorf(tr("打开源文件失败: %w", "failed to open source file: %w"), err)
	}
	defer srcF.Close()

	fi, err := srcF.Stat()
	if err != nil {
		return fmt.Errorf(tr("读取源文件信息失败: %w", "failed to read source file info: %w"), err)
	}

	dstF, err := os.OpenFile(dst, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, fi.Mode())
	if err != nil {
		return fmt.Errorf(tr("创建目标文件失败: %w", "failed to create destination file: %w"), err)
	}
	_, copyErr := io.Copy(dstF, srcF)
	closeErr := dstF.Close()
	if copyErr != nil {
		return fmt.Errorf(tr("复制文件内容失败: %w", "failed to copy file contents: %w"), copyErr)
	}
	if closeErr != nil {
		return fmt.Errorf(tr("关闭目标文件失败: %w", "failed to close destination file: %w"), closeErr)
	}
	// 显式 chmod 保证目标权限与源完全一致（不受 umask 影响）
	if err := os.Chmod(dst, fi.Mode()); err != nil {
		return fmt.Errorf(tr("设置目标文件权限失败: %w", "failed to set destination file permissions: %w"), err)
	}
	return nil
}

// moveFile 把 src 移动到 dst：优先 renameFile（包级注入点，测试可模拟失败）；
// 跨设备（Windows 跨盘 ERROR_NOT_SAME_DEVICE / Unix 跨挂载点 EXDEV）时
// os.Rename 必然失败，回退为 copyFile + os.Remove(src)，并保留源文件权限。
// 跨设备判定用平台常量 crossDeviceRenameErr：Windows 上 syscall.EXDEV 是
// Go 发明的常量，与真实错误码永不相等，不能用它判断。
// 其他失败（如目标已存在）保持原语义：删除 dst 后重试一次 renameFile。
// 关键顺序：必须先判定跨设备再走删旧重试，避免跨设备时误删已存在的旧文件。
func moveFile(src, dst string) error {
	err := renameFile(src, dst)
	if err == nil {
		return nil
	}
	if errors.Is(err, crossDeviceRenameErr) {
		// 跨设备无法 rename：复制到目标（覆盖已存在的同名文件）后删除源文件
		if copyErr := copyFile(src, dst); copyErr != nil {
			return copyErr
		}
		if removeErr := os.Remove(src); removeErr != nil {
			return fmt.Errorf(tr("删除源文件失败: %w", "failed to remove source file: %w"), removeErr)
		}
		return nil
	}
	// 目标已存在等其他失败：先删除旧目标再重试一次，与原有更新逻辑一致
	if removeErr := os.Remove(dst); removeErr == nil {
		return renameFile(src, dst)
	}
	return err
}

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
		setDownloadError(tr("获取发布信息失败: ", "Failed to fetch release info: ") + err.Error())
		return
	}

	// Step 2: Find best asset（主程序资产；cudart 运行库为附加资产）
	mainAsset := pickBestAsset(release.Assets)
	if mainAsset == nil {
		setDownloadError(tr("未找到适用于当前平台的 llama.cpp 构建", "No llama.cpp build found for the current platform"))
		return
	}

	// Windows 的 CUDA 构建自 b10342 起将运行库拆为独立 cudart zip，主程序
	// zip 不再内置运行库，需附加下载并解压到同一目录。以主程序资产名是否
	// 含 "cuda" 判定（pickBestAsset 仅在 Windows 检测到 GPU 时才会选中 cuda
	// 构建，资产名即选择结果，避免二次 GPU 检测）；非 Windows 平台的 cuda
	// 构建（如有）不附加 win 专属的 cudart 资产。
	assets := []*GitHubAsset{mainAsset}
	if runtime.GOOS == "windows" && strings.Contains(strings.ToLower(mainAsset.Name), "cuda") {
		if cudart := pickCudartAssetFor(release.Assets, cudaVersionFromToolkit()); cudart != nil {
			assets = append(assets, cudart)
		}
	}

	downloadMu.Lock()
	downloadState.Status = "downloading"
	downloadState.FileName = mainAsset.Name
	// 多资产顺序下载：Total 为全部资产大小之和，Downloaded 跨资产累加
	var totalBytes int64
	for _, a := range assets {
		totalBytes += a.Size
	}
	downloadState.Total = totalBytes
	downloadState.Version = release.TagName
	downloadState.Downloaded = 0
	downloadMu.Unlock()

	// 目标目录：自定义 llama.cpp 目录优先，否则默认 llama-cpp/；解压前建一次
	targetDir := llamaCppDownloadDir()
	if err := os.MkdirAll(targetDir, 0755); err != nil {
		setDownloadError(tr("创建目录失败: ", "Failed to create directory: ") + err.Error())
		return
	}

	// Step 3: 顺序下载并解压各资产（先主程序后 cudart），pause/stop 语义不变；
	// 进度以 baseDownloaded 叠加前序资产已完成字节
	var baseDownloaded int64
	for _, asset := range assets {
		// FileName 随循环更新为当前正在下载的资产名
		downloadMu.Lock()
		downloadState.FileName = asset.Name
		downloadMu.Unlock()

		tmpPath, err := downloadWithResume(ctx, asset.BrowserDownloadURL, asset.Size, baseDownloaded)
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
			} else {
				setDownloadError(tr("下载失败: ", "Download failed: ") + err.Error())
			}
			return
		}

		// 临时文件在解压后清理（含取消/出错路径）
		defer os.Remove(tmpPath)

		// Check if stopped during download
		select {
		case <-ctx.Done():
			return
		default:
		}

		// Step 4: Extract（与主程序解压到同一目录）
		downloadMu.Lock()
		downloadState.Status = "extracting"
		downloadState.Progress = 100
		downloadMu.Unlock()

		var extractErr error
		switch {
		case strings.HasSuffix(asset.Name, ".zip"):
			extractErr = extractZip(tmpPath, targetDir)
		case strings.HasSuffix(asset.Name, ".tar.gz"):
			extractErr = extractTarGz(tmpPath, targetDir)
		default:
			// 与原有单资产逻辑一致：不支持格式直接报错，不带"解压失败"前缀
			setDownloadError(tr("不支持的文件格式: ", "Unsupported file format: ") + asset.Name)
			return
		}
		if extractErr != nil {
			setDownloadError(tr("解压失败: ", "Extraction failed: ") + extractErr.Error())
			return
		}

		baseDownloaded += asset.Size
	}

	// Step 5: Done（全部资产下载并解压成功后才置完成）
	downloadMu.Lock()
	downloadState.Status = "done"
	downloadState.Progress = 100
	downloadMu.Unlock()

	// Reset model cache so new models are picked up
	invalidateModelCache()
	// 失效 llama.cpp 检测缓存：GetLlamaCpp 在挂载时缓存的结果（Installed=false）
	// 已过期，解压成功后需重新检测，否则主页一直显示"未找到"
	llamaCacheValid.Store(false)

	log.Printf("[OK] llama.cpp %s downloaded and extracted to %s/", release.TagName, targetDir)
}

// downloadWithResume downloads a file with pause/resume support.
// baseDownloaded 是该文件之前已完成资产的总字节数：多资产顺序下载（如
// llama.cpp 主程序 + cudart 运行库）时进度需叠加前序累计值，单一资产
// 调用传入 0。
// Returns the path to the downloaded temp file.
func downloadWithResume(ctx context.Context, url string, totalSize int64, baseDownloaded int64) (string, error) {
	tmpFile, err := os.CreateTemp("", "llamacpp-download-*"+filepath.Ext(url[strings.LastIndex(url, "."):]))
	if err != nil {
		return "", fmt.Errorf(tr("创建临时文件失败: %w", "failed to create temporary file: %w"), err)
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
		downloadState.Downloaded = baseDownloaded + offset
		downloadMu.Unlock()

		// Build request
		req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
		if err != nil {
			tmpFile.Close()
			return tmpPath, err
		}
		req.Header.Set("User-Agent", "llama-desktop")
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
			downloadState.Total = baseDownloaded + effectiveSize
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
				downloadState.Downloaded = baseDownloaded + downloaded
				if downloadState.Total > 0 {
					// 进度按含基偏移的累计字节计算，跨资产单调递增不回落
					downloadState.Progress = int(float64(baseDownloaded+downloaded) * 100 / float64(downloadState.Total))
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
	req.Header.Set("User-Agent", "llama-desktop")

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
	return pickBestAssetFor(assets, runtime.GOOS, runtime.GOARCH, len(getGPUInfo()) > 0, cudaVersionFromToolkit())
}

// cudaVersionFromToolkit 从本机 CUDA Toolkit 版本（nvcc 输出，如 "12.4.131"）
// 解析出资产命名使用的"主.次"版本（如 "12.4"）；无 Toolkit 或解析失败返回
// 空串。pickBestAssetFor 的精确版本加分与 pickCudartAssetFor 的运行库匹配
// 共用该推导，保证两处版本口径一致。
func cudaVersionFromToolkit() string {
	cudaInfo := getCUDAInfo()
	if cudaInfo.ToolkitVersion == "" {
		return ""
	}
	parts := strings.Split(cudaInfo.ToolkitVersion, ".")
	if len(parts) < 2 {
		return ""
	}
	return parts[0] + "." + parts[1]
}

// pickBestAssetFor scores release assets for a given platform/arch and returns
// the best match. hasCUDA and cudaVer allow preferring matching CUDA builds on
// Windows. Windows 下跳过 cudart 运行库资产（主程序与运行库拆分下载，运行库
// 由 pickCudartAssetFor 单独匹配）。Returns nil when no asset matches the platform.
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
			// 跳过 cudart 运行库资产：llama.cpp b10342 起 Windows CUDA 构建
			// 拆分为主程序 zip（llama-b*-bin-win-cuda-*-x64.zip）与 cudart
			// 运行库 zip 两个资产，两者评分相同且 cudart 在 release 列表中排
			// 在更前，若不排除会只选中运行库、漏掉主程序（解压产物只有
			// cudart64_12.dll 等运行库、没有 llama-server.exe）。运行库由
			// pickCudartAssetFor 单独匹配后随主程序一并下载。
			if strings.HasPrefix(name, "cudart") {
				continue
			}
			// CUDA builds on Windows: "llama-b*-bin-win-cuda-XX.X-x64.zip"
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

// pickCudartAssetFor 返回与给定 CUDA 版本精确匹配的 cudart 运行库资产
// （cudart-llama-bin-win-cuda-<cudaVer>-x64.zip，大小写不敏感），未找到返回
// nil。cudaVer 为空时不匹配精确版本，回退为任一 win cudart 资产（best-effort：
// 覆盖无 nvcc 但 GPU 检测成功、toolkit 版本解析失败的主机，此时主程序选中
// 了 CUDA 构建，仍需要运行库才能启动）。
// 本函数不判断平台：是否附加运行库由调用方（downloadLlamaCpp 的
// Windows+CUDA 判定）决定，便于测试跨平台直接构造 cudart 资产断言匹配逻辑。
func pickCudartAssetFor(assets []GitHubAsset, cudaVer string) *GitHubAsset {
	for i := range assets {
		a := &assets[i]
		lower := strings.ToLower(a.Name)
		if !strings.HasPrefix(lower, "cudart-llama-bin-win-cuda-") {
			continue
		}
		if cudaVer == "" {
			return a
		}
		if strings.EqualFold(a.Name, "cudart-llama-bin-win-cuda-"+cudaVer+"-x64.zip") {
			return a
		}
	}
	return nil
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
			return fmt.Errorf(tr("文件超出解压大小上限: %s", "file exceeds the extraction size limit: %s"), path)
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
			return fmt.Errorf(tr("文件超出解压大小上限: %s", "file exceeds the extraction size limit: %s"), path)
		}
		totalBytes += n
		if totalBytes > maxExtractTotalSize {
			return fmt.Errorf(tr("解压总大小超出上限: %d 字节", "total extraction size exceeds the limit: %d bytes"), totalBytes)
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
				return fmt.Errorf(tr("文件超出解压大小上限: %s", "file exceeds the extraction size limit: %s"), path)
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
				return fmt.Errorf(tr("文件超出解压大小上限: %s", "file exceeds the extraction size limit: %s"), path)
			}
			totalBytes += n
			if totalBytes > maxExtractTotalSize {
				return fmt.Errorf(tr("解压总大小超出上限: %d 字节", "total extraction size exceeds the limit: %d bytes"), totalBytes)
			}
		default:
			// 符号链接/硬链接/设备文件等未知类型显式拒绝，避免静默跳过
			// 产生不完整解压或潜在安全问题（#6）
			return fmt.Errorf(tr("不支持的 tar 条目类型 %d: %s", "unsupported tar entry type %d: %s"), header.Typeflag, header.Name)
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
var updateRepoAPI = "https://api.github.com/repos/CodeNeow/llama-cpp-desktop/releases/latest"

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
	Kind       string `json:"kind"` // 本次下载产物类型：setup（安装器） / portable（便携版）
}

var updateDownloadState = &UpdateDownloadState{Status: "idle"}
var updateDownloadMu sync.Mutex
var updateDownloadCancel context.CancelFunc

// updateExePath 为测试注入点（与 renameFile / configFile 同风格），
// 返回当前可执行文件路径，用于确定更新 exe 的目标目录。
var updateExePath = os.Executable

// 安装类型常量：setup 为 NSIS 安装版（下载 setup 安装器），
// portable 为便携版（下载便携版 exe）。用于更新产物挑选与前端提示区分。
const (
	installKindSetup    = "setup"
	installKindPortable = "portable"
)

// detectInstallKind 判断当前安装类型：setup 安装版由 NSIS 安装，安装目录必有
// uninstall.exe；portable 便携版为绿色版，目录下无 uninstall.exe。纯文件系统
// 判断，跨平台。复用 updateExePath（os.Executable 的测试注入点）保证可测。
func detectInstallKind() string {
	exePath, err := updateExePath()
	if err != nil {
		return installKindPortable
	}
	if _, err := os.Stat(filepath.Join(filepath.Dir(exePath), "uninstall.exe")); err == nil {
		return installKindSetup
	}
	return installKindPortable
}

// pickUpdateAsset 按安装类型挑选更新下载使用的资产，按关键词匹配（与产物
// 前缀无关），兼容三代命名：
//   - 现命名：setup 安装器 llama-desktop-setup-vX.Y.Z-amd64.exe、便携版
//     llama-desktop-portable-vX.Y.Z-amd64.exe；
//   - 旧命名（v0.1.7 起）：llama-gui-setup- / llama-gui-portable- 前缀；
//   - 最旧命名（v0.1.6）：安装器 llama-gui-amd64-installer.exe、便携版 llama-gui.exe。
//
// setup 返回第一个安装器资产（名字含 installer 或 setup）；
// portable 返回第一个含 portable 或非安装器的 exe 资产（最旧命名 llama-gui.exe
// 不含 portable/installer/setup，命中「非安装器」分支）。
func pickUpdateAsset(assets []GitHubAsset, kind string) *GitHubAsset {
	for i := range assets {
		a := &assets[i]
		name := strings.ToLower(a.Name)
		if !strings.HasSuffix(name, ".exe") {
			continue
		}
		isInstaller := strings.Contains(name, "installer") || strings.Contains(name, "setup")
		switch kind {
		case installKindSetup:
			if isInstaller {
				return a
			}
		default: // installKindPortable（含未知取值兜底为便携版语义）
			if strings.Contains(name, "portable") || !isInstaller {
				return a
			}
		}
	}
	return nil
}

// downloadUpdateRelease 按当前安装类型下载新版本对应产物到可执行文件同目录：
// setup 安装版下载安装器、portable 便携版下载便携版 exe（无法直接替换正在运行
// 的自身），完成后提示用户关闭应用后按安装类型完成更新。
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

	// 下载开始前先判定安装类型：决定挑选哪种资产与下载命名（setup 安装器 /
	// portable 便携版 exe）。
	kind := detectInstallKind()

	// Step 1: 拉取最新发布信息，按安装类型挑对应 exe 资产
	updateDownloadMu.Lock()
	updateDownloadState.Status = "downloading"
	updateDownloadState.Progress = 0
	updateDownloadState.Downloaded = 0
	updateDownloadState.Total = 0
	updateDownloadState.Version = version
	updateDownloadState.Error = ""
	updateDownloadState.Kind = kind
	updateDownloadMu.Unlock()

	release, err := fetchLatestReleaseAt(updateRepoAPI)
	if err != nil {
		setUpdateDownloadError(tr("获取发布信息失败: ", "Failed to fetch release info: ") + err.Error())
		return
	}
	asset := pickUpdateAsset(release.Assets, kind)
	if asset == nil {
		setUpdateDownloadError(tr("未找到适用于当前平台的主程序", "No main executable found for the current platform"))
		return
	}

	updateDownloadMu.Lock()
	updateDownloadState.Total = asset.Size
	updateDownloadMu.Unlock()

	// Step 2: 下载到可执行文件同目录，命名按安装类型区分：
	// setup → llama-desktop-setup-v<tag>.exe；portable → llama-desktop-portable-v<tag>.exe
	exePath, err := updateExePath()
	if err != nil {
		setUpdateDownloadError(tr("无法定位可执行文件路径: ", "Unable to locate the executable path: ") + err.Error())
		return
	}
	dir := filepath.Dir(exePath)
	var fileName string
	if kind == installKindSetup {
		fileName = "llama-desktop-setup-" + release.TagName + ".exe"
	} else {
		fileName = "llama-desktop-portable-" + release.TagName + ".exe"
	}
	destPath := filepath.Join(dir, fileName)

	tmpPath, err := downloadUpdateWithResume(ctx, asset.BrowserDownloadURL, asset.Size)
	if err != nil {
		if ctx.Err() != nil {
			updateDownloadMu.Lock()
			updateDownloadState.Status = "idle"
			updateDownloadMu.Unlock()
			log.Println("[INFO] update download stopped by user")
		} else {
			setUpdateDownloadError(tr("下载失败: ", "Download failed: ") + err.Error())
		}
		return
	}
	defer os.Remove(tmpPath)

	// Step 3: 移动到目标路径（跨设备时 moveFile 回退为复制，保留源权限；
	// 非跨设备失败且目标已存在时先删旧文件再重试）
	if err := moveFile(tmpPath, destPath); err != nil {
		setUpdateDownloadError(tr("保存文件失败: ", "Failed to save file: ") + err.Error())
		return
	}

	updateDownloadMu.Lock()
	updateDownloadState.Status = "done"
	updateDownloadState.Progress = 100
	updateDownloadState.FilePath = destPath
	updateDownloadState.Kind = kind
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
	tmpFile, err := os.CreateTemp("", "llama-desktop-update-*.exe")
	if err != nil {
		return "", fmt.Errorf(tr("创建临时文件失败: %w", "failed to create temporary file: %w"), err)
	}
	tmpPath := tmpFile.Name()

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		tmpFile.Close()
		return tmpPath, err
	}
	req.Header.Set("User-Agent", "llama-desktop")

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

// serverStartTime 记录 llama-server 成功启动的时刻（serverMu 保护），供
// GetMonitorStatus 计算运行时长；进程退出（cmd.Wait goroutine）置零。
var serverStartTime time.Time

// serverPort 记录 llama-server 成功启动时使用的端口（serverMu 保护），
// 0 表示未运行。路由器 API 查询用此值而非当前配置，避免运行中修改配置
// 导致查询到错误的地址。
var serverPort int

// serverLogWriter 把子进程 stdout/stderr 的写入按行重组后再进入环形日志，避免
// 日志条目被任意分片拦腰截断。此前 Write 把每次 stderr 写入的任意分片直接当作
// 一条日志（addServerLog(strings.TrimSpace(string(p)))）：llama-server 输出按小块
// 写入，print_timing 行可能被拆成多次 Write——用户实贴日志出现「0.00.136.078」
// 单独成条，以及「( 0.63 ms per token, 2362.80 tokens per second)」这种只剩后半句
// 的分片；后者不再含 "prompt eval time" 标记，parseTPS 无法把它归类为预填充行，
// 2362.80 这类长 prompt 预填充速度就漏进了 TPS。按行缓冲让 addServerLog 永远收到
// 完整行，从根上消除截断（日志显示拆行与 TPS 误读同源）。
//
// 每个实例持有独立缓冲与互斥锁（项目「显式互斥」惯例），bridge.go 中
// cmd.Stdout 与 cmd.Stderr 各挂一个实例，各自缓冲自己的流，互不干扰。
type serverLogWriter struct {
	mu  sync.Mutex
	buf bytes.Buffer
}

func (w *serverLogWriter) Write(p []byte) (int, error) {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.buf.Write(p)
	for {
		line, err := w.buf.ReadString('\n')
		if err != nil {
			// 无换行结尾：ReadString 已消费整个缓冲，line 为未完成残片。
			// 清空后把残片写回，与下一次 Write 拼成完整行；无残片时直接
			// 清空，防止缓冲随已消费字节无限增长。
			w.buf.Reset()
			if line != "" {
				w.buf.WriteString(line)
			}
			break
		}
		if line = strings.TrimSpace(line); line != "" {
			addServerLog(line)
		}
	}
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

// validCacheTypeValue 校验 cache-type-k/v 取值白名单（b10342 实际支持列表）。
func validCacheTypeValue(s string) bool {
	switch s {
	case "", "f32", "f16", "bf16", "q8_0", "q4_0", "q4_1", "iq4_nl", "q5_0", "q5_1":
		return true
	}
	return false
}

// validLoadModeValue 校验 load-mode 取值白名单（b10342 起替代 mlock/no-mmap，
// 空值表示使用 llama-server 默认 mmap）。
func validLoadModeValue(s string) bool {
	switch s {
	case "", "none", "mmap", "mlock", "mmap+mlock", "dio":
		return true
	}
	return false
}

// validSplitModeValue 校验 split-mode 取值白名单（多 GPU 张量切分策略）。
func validSplitModeValue(s string) bool {
	switch s {
	case "", "none", "layer", "row", "tensor":
		return true
	}
	return false
}

// validRopeScalingValue 校验 rope-scaling 取值白名单（长上下文外推策略）。
func validRopeScalingValue(s string) bool {
	switch s {
	case "", "none", "linear", "yarn":
		return true
	}
	return false
}

// validSpecTypeValue 校验 spec-type 取值白名单（MTP 多 token 预测策略，
// 空值表示使用 llama-server 默认单 token 预测）。
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

		// 显式 mmproj 路径覆盖时跳过下方同目录自动检测（避免输出两条 mmproj）
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
			// 以下新参数 b10342 起生效。LoadMode/SplitMode/RopeScaling 空值或
			// 等于 llama-server 默认时不写入，避免噪音；MLock/NoMMap 已废弃，
			// 仅由 loadConfig 迁移为 LoadMode，不再直接写入预设。
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
			// 显式 mmproj 路径覆盖：非空且通过 INI 注入校验时优先于同目录自动检测，
			// 不要求文件存在（模型可能移动，llama-server 启动时自行报错）。
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
	LlamaCppDir      string                 `json:"llamaCppDir"`
	ModelDir         string                 `json:"modelDir"`
	Theme            string                 `json:"theme"`
	ModelConfigs     map[string]ModelConfig `json:"modelConfigs"`
	ServerConfig     ServerConfig           `json:"serverConfig"`
	DownloadSource   string                 `json:"downloadSource"`
	Language         string                 `json:"language"`         // 语言偏好: zh / en / auto（空或非法值兜底 auto）
	TrayEnabled      bool                   `json:"trayEnabled"`      // Windows 系统托盘开关，默认 true
	SidebarCollapsed bool                   `json:"sidebarCollapsed"` // 侧边栏收起状态，默认 true（收起）
	DownloadTasks    []PersistedDlTask      `json:"downloadTasks,omitempty"`
}

// PersistedDlTask 是下载任务队列的持久化形态（写入 llama-desktop-config.json）。
// 与 DlTask 的区别：URL / ctx / cancel / resumeCh 这类运行时状态不持久化，
// URL 在 loadConfig 恢复时按 Source + buildModelDownloadURL 重建。
type PersistedDlTask struct {
	ID         string `json:"id"`
	ModelID    string `json:"modelId"`
	FileName   string `json:"fileName"`
	DestDir    string `json:"destDir"`
	Source     string `json:"source"`
	Status     string `json:"status"`
	Progress   int    `json:"progress"`
	Total      int64  `json:"total"`
	Downloaded int64  `json:"downloaded"`
	SizeHuman  string `json:"sizeHuman"`
	Error      string `json:"error"`
}

// 服务访问范围取值：local 表示仅本机可访问（监听 127.0.0.1），lan 表示允许
// 同网络设备访问（监听 0.0.0.0）。ServerConfig.AccessMode 只接受这两个值，
// 其余（含空串）一律兜底 local。
const accessLocal = "local"
const accessLAN = "lan"

type ServerConfig struct {
	// AccessMode 是服务访问范围（"local" | "lan"，默认 "local"）；Host 为
	// 按 AccessMode 派生后的实际监听地址，不直接接受用户输入。
	AccessMode string `json:"accessMode"`
	Host       string `json:"host"`
	Port       int    `json:"port"`
	MaxModels  int    `json:"maxModels"`
	CacheRAM   int    `json:"cacheRam"`
}

// effectiveHost 按访问范围派生实际监听地址：lan → "0.0.0.0"，其余任何取值
// （含空串与非法值）→ "127.0.0.1"。纯函数，供 SaveServerConfig 归一化、
// loadConfig 兼容与 buildServerCommand 共用，保证各处 Host 口径一致。
func effectiveHost(mode string) string {
	if mode == accessLAN {
		return "0.0.0.0"
	}
	return "127.0.0.1"
}

type ModelConfig struct {
	Threads       int     `json:"threads"`
	GPULayers     string  `json:"gpuLayers"`
	CtxSize       int     `json:"ctxSize"`
	BatchSize     int     `json:"batchSize"`
	UBatchSize    int     `json:"ubatchSize"`
	FlashAttn     bool    `json:"flashAttn"`
	CacheTypeK    string  `json:"cacheTypeK"`
	CacheTypeV    string  `json:"cacheTypeV"`
	LoadMode      string  `json:"loadMode"`         // "", none, mmap, mlock, mmap+mlock, dio
	CPUMoe        bool    `json:"cpuMoe"`           // 所有 MoE 专家留 CPU
	NCpuMoe       int     `json:"nCpuMoe"`          // 前 N 层 MoE 留 CPU, 0=不启用
	SplitMode     string  `json:"splitMode"`        // "", none, layer, row, tensor
	TensorSplit   string  `json:"tensorSplit"`      // 如 "3,1"
	MainGPU       int     `json:"mainGpu"`          // 默认 0
	RopeScaling   string  `json:"ropeScaling"`      // "", none, linear, yarn
	RopeScale     float64 `json:"ropeScale"`        // 0=不启用
	MMProj        string  `json:"mmproj"`           // 显式 mmproj 路径覆盖, 空=自动检测
	Reasoning     bool    `json:"reasoning"`        // 关闭思考（写 reasoning = off）
	SpecType      string  `json:"specType"`         // "", draft-mtp
	SpecDraftNMax int     `json:"specDraftNMax"`    // >0 时写 spec-draft-n-max
	MLock         bool    `json:"mlock,omitempty"`  // 已废弃,仅为读取旧配置迁移
	NoMMap        bool    `json:"noMmap,omitempty"` // 已废弃,仅为读取旧配置迁移
}

// migrateLegacyConfig 把 llama-gui 时代的旧配置文件内容复制到新文件名：
// 仅当新文件不存在且旧文件存在时，读出旧文件内容后写入新文件（0644 与
// saveConfig 写入惯例一致），不做任何删除或改名。旧文件保留原处、内容不变。
// 之所以用复制而非改名：wails dev 的文件监视器监视项目根目录，启动期删除/
// 改名根目录文件会触发 Wails CLI 的 GetFileAttributesEx 竞态导致崩溃退出；
// 复制不删源文件，且新文件存在即短路、旧文件残留无副作用——仅当用户删掉
// 新文件后迁移才会再次触发。失败只记警告并走 loadConfig 的默认配置兜底，
// 不阻断启动。
func migrateLegacyConfig() {
	if _, err := os.Stat(configFile); err == nil {
		return
	}
	if _, err := os.Stat(legacyConfigFile); err != nil {
		return
	}
	data, err := os.ReadFile(legacyConfigFile)
	if err != nil {
		log.Printf("[WARN] Failed to migrate legacy config %s: %v", legacyConfigFile, err)
		return
	}
	if err := os.WriteFile(configFile, data, 0644); err != nil {
		log.Printf("[WARN] Failed to migrate legacy config %s: %v", legacyConfigFile, err)
		return
	}
	log.Printf("[OK] Migrated legacy config %s -> %s", legacyConfigFile, configFile)
}

func loadConfig() {
	migrateLegacyConfig()
	data, err := os.ReadFile(configFile)
	if err != nil {
		return // file doesn't exist yet, that's ok
	}
	var cfg appConfig
	// 预置默认值后再 Unmarshal：Go 零值 false 无法区分「旧配置缺字段」与
	// 「显式设为 false」。trayEnabled 缺省时必须兜底 true（历史配置升级后
	// 托盘默认开启，与 4aacac2 无条件启托盘的行为一致）；sidebarCollapsed
	// 缺省时兜底 true（侧边栏默认收起，无保存偏好即收起）。
	cfg.TrayEnabled = true
	cfg.SidebarCollapsed = true
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
	// 迁移旧 mlock/noMmap 到 load-mode（b10342 起两者 DEPRECATED）：
	// 旧配置若未显式设置 loadMode，则按旧布尔组合推导并清零兼容字段，
	// saveConfig 时 omitempty 保证不再写回旧键（渐进清理）。
	for k, c := range cachedModelConfigs {
		if c.LoadMode == "" && (c.MLock || c.NoMMap) {
			switch {
			case c.MLock && c.NoMMap:
				c.LoadMode = "mlock" // mlock 语义优先
			case c.MLock:
				c.LoadMode = "mlock"
			case c.NoMMap:
				c.LoadMode = "none"
			}
		}
		c.MLock = false
		c.NoMMap = false
		cachedModelConfigs[k] = c
	}
	// Merge server config with defaults
	scfg := defaultServerConfig()
	// 访问范围：空值或不在 {local,lan} 白名单时兜底 local（旧配置无
	// accessMode 字段或数据损坏时不报错）。Host 一律由 effectiveHost 按
	// accessMode 派生，不信任旧配置里可能存在的非法 host 值（#5 防御延续）。
	if cfg.ServerConfig.AccessMode != accessLocal && cfg.ServerConfig.AccessMode != accessLAN {
		cfg.ServerConfig.AccessMode = accessLocal
	}
	scfg.AccessMode = cfg.ServerConfig.AccessMode
	scfg.Host = effectiveHost(scfg.AccessMode)
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

	// 下载源：空值或非法值兜底默认 hf（旧配置无此字段或数据损坏时不报错）。
	if cfg.DownloadSource != sourceHF && cfg.DownloadSource != sourceModelScope {
		cfg.DownloadSource = defaultDownloadSource
	}
	downloadSourceMu.Lock()
	downloadSource = cfg.DownloadSource
	downloadSourceMu.Unlock()

	// 语言偏好：空值或不在 zh/en/auto 白名单时兜底 auto（旧配置无此字段或
	// 数据损坏时不报错）。与 downloadSource 同策略：非法值一律规整回默认。
	if cfg.Language != "zh" && cfg.Language != "en" && cfg.Language != "auto" {
		cfg.Language = "auto"
	}
	languageMu.Lock()
	currentLanguage = cfg.Language
	languageMu.Unlock()

	// 系统托盘开关：字段缺失时保持预置默认 true；显式 false 才禁用
	//（旧配置升级后托盘默认开启，与 4aacac2 无条件启托盘的行为一致）。
	configMu.Lock()
	trayEnabled = cfg.TrayEnabled
	configMu.Unlock()

	// 侧边栏收起状态：缺字段时保持预置默认 true（收起，见上方 appConfig
	// 预置）；仅显式 false（用户展开偏好）才为 false，与 trayEnabled 同模式。
	configMu.Lock()
	currentSidebarCollapsed = cfg.SidebarCollapsed
	configMu.Unlock()

	// 恢复下载任务队列（进程重启后无活跃 goroutine，任何任务都不自动启动下载）：
	// Source 兜底 hf；Status 白名单外与 downloading 一律规整为 paused（downloading
	// 状态的 goroutine 已随进程退出消亡，前端可对任务继续/重试）；URL 按
	// buildModelDownloadURL 重建；resumeCh 新建缓冲 channel，ctx/cancel 留 nil
	//（RetryDownloadTask 重建 ctx 后再启动）。恢复后调整 dlTaskCounter 避免与
	// 既有任务 id 冲突。
	restored := make([]*DlTask, 0, len(cfg.DownloadTasks))
	for _, pt := range cfg.DownloadTasks {
		src := pt.Source
		if src == "" {
			src = sourceHF
		}
		status := pt.Status
		switch status {
		case "done", "error", "cancelled", "queued", "paused":
			// 终态与可控状态保持原样
		default:
			// 空值、非法值或 downloading → paused
			status = "paused"
		}
		task := &DlTask{
			ID:         pt.ID,
			ModelID:    pt.ModelID,
			FileName:   pt.FileName,
			DestDir:    pt.DestDir,
			Source:     src,
			Status:     status,
			Progress:   pt.Progress,
			Total:      pt.Total,
			Downloaded: pt.Downloaded,
			SizeHuman:  pt.SizeHuman,
			Error:      pt.Error,
			resumeCh:   make(chan struct{}, 1),
		}
		if url, err := buildModelDownloadURL(src, pt.ModelID, pt.FileName); err == nil {
			task.URL = url
		}
		restored = append(restored, task)
	}
	dlTasksMu.Lock()
	dlTasks = restored
	// id 取已恢复任务的最大序号 +1（解析 "dl-N"），避免新增任务 id 冲突；
	// 解析失败或没有已恢复任务时保持原值。
	maxSeq := 0
	for _, t := range restored {
		if n, err := strconv.Atoi(strings.TrimPrefix(t.ID, "dl-")); err == nil && n > maxSeq {
			maxSeq = n
		}
	}
	if maxSeq > dlTaskCounter {
		dlTaskCounter = maxSeq
	}
	dlTasksMu.Unlock()
}

var cachedModelConfigs = make(map[string]ModelConfig)
var modelConfigsMu sync.Mutex
var cachedServerConfig = defaultServerConfig()
var serverConfigMu sync.Mutex

func defaultServerConfig() ServerConfig {
	return ServerConfig{
		AccessMode: accessLocal, Host: "127.0.0.1", Port: 8080, MaxModels: 1, CacheRAM: 8192,
	}
}

var currentTheme = "light"
var configMu sync.Mutex

// trayEnabled 表示是否启用 Windows 系统托盘（关闭窗口缩到托盘），默认 true；
// 受 configMu 保护，持久化到配置文件的 trayEnabled 字段。旧配置缺该字段时
// loadConfig 兜底 true（见 loadConfig 的 appConfig{TrayEnabled: true} 预置）。
var trayEnabled = true

// currentSidebarCollapsed 表示侧边栏是否处于收起状态（纯图标栏），默认 true
// （收起）；受 configMu 保护，持久化到配置文件的 sidebarCollapsed 字段。旧配置
// 缺该字段时 loadConfig 预置默认 true（见 loadConfig 的 appConfig 预置），与
// trayEnabled 的兜底模式一致。
var currentSidebarCollapsed = true

// TrayEnabled 返回当前托盘启用偏好（并发安全，configMu 保护）。供 main.go 的
// OnStartup 按持久化配置决定是否启动托盘。
func TrayEnabled() bool {
	configMu.Lock()
	defer configMu.Unlock()
	return trayEnabled
}

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

	downloadSourceMu.Lock()
	dlsrc := downloadSource
	downloadSourceMu.Unlock()

	languageMu.Lock()
	lang := currentLanguage
	languageMu.Unlock()

	configMu.Lock()
	tray := trayEnabled
	configMu.Unlock()

	configMu.Lock()
	sidebarCollapsed := currentSidebarCollapsed
	configMu.Unlock()

	// 锁序铁律：saveConfig 内 dlTasksMu 必须是最后获取的锁。任何调用点都不得
	// 在持有 dlTasksMu 时调用 saveConfig——调用方须先锁内取副本、解锁、再
	// save（如 CancelDownloadTask 在 defer Unlock 前不调 saveConfig）。否则会
	// 违反 dlTasksMu 与其他锁（configMu 等）的全局顺序，造成死锁。
	dlTasksMu.Lock()
	persistedTasks := make([]PersistedDlTask, 0, len(dlTasks))
	for _, t := range dlTasks {
		persistedTasks = append(persistedTasks, PersistedDlTask{
			ID:         t.ID,
			ModelID:    t.ModelID,
			FileName:   t.FileName,
			DestDir:    t.DestDir,
			Source:     t.Source,
			Status:     t.Status,
			Progress:   t.Progress,
			Total:      t.Total,
			Downloaded: t.Downloaded,
			SizeHuman:  t.SizeHuman,
			Error:      t.Error,
		})
	}
	dlTasksMu.Unlock()

	cfg := appConfig{
		LlamaCppDir:      dir,
		ModelDir:         modelDir,
		Theme:            theme,
		ModelConfigs:     mcfgs,
		ServerConfig:     scfg,
		DownloadSource:   dlsrc,
		Language:         lang,
		TrayEnabled:      tray,
		SidebarCollapsed: sidebarCollapsed,
		DownloadTasks:    persistedTasks,
	}
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

// buildModelDownloadURL 按下载源构建模型文件下载 URL：
//   - hf：{hfMirrorBase}/{modelID}/resolve/main/{fileName}（PathEscape 转义文件名）
//   - modelscope：委托 buildModelScopeDownloadURL（legacy API 的 repo 端点）
//   - 未知 source 返回错误（防御纵深，调用方不应传入非法值）
func buildModelDownloadURL(source, modelID, fileName string) (string, error) {
	switch source {
	case sourceHF:
		return fmt.Sprintf("%s/%s/resolve/main/%s", hfMirrorBase, modelID, url.PathEscape(fileName)), nil
	case sourceModelScope:
		return buildModelScopeDownloadURL(modelscopeLegacyBase, modelID, fileName), nil
	default:
		return "", fmt.Errorf(tr("未知下载源 %q", "unknown download source %q"), source)
	}
}

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
		return nil, errors.New(tr("HF 搜索三路排序（downloads/likes/lastModified）请求全部失败", "all three HF search sort routes (downloads/likes/lastModified) failed"))
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
	req.Header.Set("User-Agent", "llama-desktop")

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
//   - GET {base}/{modelID}/raw/main/README.md（User-Agent llama-desktop，30s 超时）
//   - 非 200 返回错误；YAML front-matter（首行为 --- 的块）会被跳过
//   - 按空行分段，取第一个「非空且不以 # 开头」的段落，trim 后截断 200 rune
//   - README 存在但没有描述段落时返回空串与 nil 错误（静默）
func getModelDescriptionAt(baseURL, modelID string) (string, error) {
	readmeURL := fmt.Sprintf("%s/%s/raw/main/README.md", baseURL, modelID)

	req, err := http.NewRequest("GET", readmeURL, nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("User-Agent", "llama-desktop")

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf(tr("README 获取失败: HTTP %d", "failed to fetch README: HTTP %d"), resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}

	return extractDescription(string(body)), nil
}

// extractDescription 从 README 正文提取自然语言描述（HF 与 ModelScope 共用）：
//   - 跳过 YAML front-matter（首行 trim 后为 --- 的块）
//   - 按空行分段，取第一个「非空且不以 # 开头」的段落，trim 后截断 200 rune
//   - 正文没有描述段落时返回空串（静默，不视为失败）
func extractDescription(body string) string {
	lines := strings.Split(body, "\n")
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
			return string(runes[:200]) + "..."
		}
		return trimmed
	}

	return ""
}

// getHFModelFiles lists downloadable GGUF files for a model via the default mirror.
func getHFModelFiles(modelID string) ([]HFFileOut, error) {
	return getHFModelFilesAt(hfMirrorBase, modelID)
}

// getHFModelFilesAt lists the GGUF siblings of a model on an HF-compatible API base.
// blobs=true 让接口返回文件真实大小（HF 搜索/详情接口默认 siblings 不带 size）。
func getHFModelFilesAt(baseURL, modelID string) ([]HFFileOut, error) {
	apiURL := fmt.Sprintf("%s/api/models/%s?blobs=true", baseURL, modelID)

	req, err := http.NewRequest("GET", apiURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "llama-desktop")

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

// getHFModelMaxGGUFSize 返回模型最大的 GGUF 文件大小（走默认镜像）。
func getHFModelMaxGGUFSize(modelID string) (int64, error) {
	return getHFModelMaxGGUFSizeAt(hfMirrorBase, modelID)
}

// getHFModelMaxGGUFSizeAt 查询模型详情接口（blobs=true 才有真实 size），返回
// 该模型最大的 .gguf 文件大小，无 GGUF 时返回 0 与 nil。HF 搜索接口的 siblings
// 不带 size（实测全为 null），搜索卡片上的模型大小只能按 modelId 走详情接口
// 逐个获取；取最大文件而非全部 GGUF 总和，避免多量化模型（数十个量化文件）
// 的总和虚高误导用户对模型规模的判断。
func getHFModelMaxGGUFSizeAt(baseURL, modelID string) (int64, error) {
	apiURL := fmt.Sprintf("%s/api/models/%s?blobs=true", baseURL, modelID)

	req, err := http.NewRequest("GET", apiURL, nil)
	if err != nil {
		return 0, err
	}
	req.Header.Set("User-Agent", "llama-desktop")

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return 0, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return 0, fmt.Errorf("HF API returned status %d", resp.StatusCode)
	}

	var raw struct {
		Siblings []HFFile `json:"siblings"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&raw); err != nil {
		return 0, err
	}

	var max int64
	for _, s := range raw.Siblings {
		name := strings.TrimPrefix(s.Filename, "/")
		if strings.HasSuffix(name, "/") || strings.HasPrefix(name, ".") || !strings.HasSuffix(strings.ToLower(name), ".gguf") {
			continue
		}
		if s.Size > max {
			max = s.Size
		}
	}
	return max, nil
}

// ─── Download task runner ────────────────────────────────────────

// computeSpeed 由采样间隔（秒）与间隔内下载字节数计算下载速度（字节/秒）。
// 纯函数：elapsed 非正或 delta 非正时返回 0（无法计算或没有有效进度）。
func computeSpeed(elapsedSec float64, deltaBytes int64) float64 {
	if elapsedSec <= 0 || deltaBytes <= 0 {
		return 0
	}
	return float64(deltaBytes) / elapsedSec
}

// lastTaskPersist 为下载任务队列最近一次持久化时间戳，lastTaskPersistMu 保护其
// 读写。进度更新路径用 persistTasksThrottled 节流：距上次保存不足 5 秒跳过，
// 避免下载高频进度把配置文件写入打满（#12 队列持久化）。
var lastTaskPersist time.Time
var lastTaskPersistMu sync.Mutex

// persistTasksNow 立即持久化下载任务队列（入队、状态变更与终态路径）。
// 调用方必须不持有 dlTasksMu：saveConfig 末尾会再获取 dlTasksMu 做快照。
func persistTasksNow() {
	lastTaskPersistMu.Lock()
	lastTaskPersist = time.Now()
	lastTaskPersistMu.Unlock()
	saveConfig()
}

// persistTasksThrottled 节流持久化下载任务队列（进度更新路径）：距上次保存
// （无论 persistTasksNow 还是本函数触发的保存）不足 5 秒时直接跳过。
func persistTasksThrottled() {
	lastTaskPersistMu.Lock()
	if time.Since(lastTaskPersist) < 5*time.Second {
		lastTaskPersistMu.Unlock()
		return
	}
	lastTaskPersist = time.Now()
	lastTaskPersistMu.Unlock()
	saveConfig()
}

// retryDownloadTask 重建任务的下载上下文并重新启动下载 goroutine。任务进入
// error/cancelled/done 终态时 ctx 已结束（goroutine 已退出），不能复用旧 ctx，
// 需要新建 context.WithCancel；downloadTask 启动时会读取 .part 文件大小作为
// 续传 offset，天然复用断点续传。调用方必须持有 dlTasksMu。
func retryDownloadTask(task *DlTask) {
	task.ctx, task.cancel = context.WithCancel(context.Background())
	// 清空错误与旧进度显示，避免前端继续展示上一次失败的红色错误框；
	// downloadTask 会根据 .part 续传 offset 重新填充 Downloaded/Total/Progress。
	task.Error = ""
	task.Downloaded = 0
	task.Total = 0
	task.Progress = 0
	task.SizeHuman = ""
	task.Speed = 0
	task.Status = "queued"
	go downloadTask(task)
}

func downloadTask(task *DlTask) {
	dlTasksMu.Lock()
	task.Status = "downloading"
	dlTasksMu.Unlock()
	persistTasksNow()

	// Create dest directory
	if err := os.MkdirAll(task.DestDir, 0755); err != nil {
		dlTasksMu.Lock()
		task.Status = "error"
		task.Error = tr("创建目录失败: ", "Failed to create directory: ") + err.Error()
		task.Speed = 0
		dlTasksMu.Unlock()
		persistTasksNow()
		return
	}

	destPath := filepath.Join(task.DestDir, task.FileName)
	tmpPath := destPath + ".part"

	// Check if partial download exists for resume
	var offset int64
	if fi, err := os.Stat(tmpPath); err == nil {
		offset = fi.Size()
	}

	// 速度采样状态（downloadTask goroutine 独占，无需加锁）：记录上次采样时间与
	// 字节数，读循环内间隔 ≥1s 时更新 task.Speed。
	var lastSampleTime time.Time
	var lastSampleBytes int64

	client := &http.Client{Timeout: 30 * time.Minute}

	for {
		// Check cancellation
		select {
		case <-task.ctx.Done():
			dlTasksMu.Lock()
			if task.Status != "paused" {
				task.Status = "cancelled"
			}
			task.Speed = 0
			dlTasksMu.Unlock()
			persistTasksNow()
			return
		default:
		}

		req, err := buildDownloadRequest(task.URL, offset)
		if err != nil {
			dlTasksMu.Lock()
			task.Status = "error"
			task.Error = err.Error()
			task.Speed = 0
			dlTasksMu.Unlock()
			persistTasksNow()
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
			// 取消与网络错误的竞态防御：ctx 已取消（如用户刚点取消）时
			// 任务应标记 cancelled 而非 error，避免取消竞态把任务打回
			// error 终态（如 hf-mirror 主动取消流的网络错误）。
			if task.ctx.Err() != nil {
				task.Status = "cancelled"
				task.Speed = 0
				dlTasksMu.Unlock()
				persistTasksNow()
				return
			}
			task.Status = "error"
			task.Error = err.Error()
			task.Speed = 0
			dlTasksMu.Unlock()
			persistTasksNow()
			return
		}

		if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusPartialContent {
			resp.Body.Close()
			dlTasksMu.Lock()
			task.Status = "error"
			task.Error = fmt.Sprintf("HTTP %d", resp.StatusCode)
			task.Speed = 0
			dlTasksMu.Unlock()
			persistTasksNow()
			return
		}

		// 服务器忽略 Range 的健壮性（#B3）：offset>0 时本次请求带了 Range 头，
		// 但部分服务器（如 ModelScope repo 端点）不保证支持 Range，会忽略该头
		// 返回 200 全量内容。若仍按 offset 追加写入 .part，会把全量内容重复追加
		// 到已有部分导致文件损坏。处理：关闭响应、截断 .part 到 0、offset 归零、
		// 清零进度显示，随后 continue 走外层循环重连。offset=0 后再请求不带 Range
		// 头，服务器若继续忽略 Range 返回 200 也从零开始写——内容正确且只会重连
		// 这一次，不会无限循环。
		if offset > 0 && resp.StatusCode == http.StatusOK {
			resp.Body.Close()
			dlTasksMu.Lock()
			task.Downloaded = 0
			task.Total = 0
			task.Progress = 0
			task.SizeHuman = ""
			task.Speed = 0
			dlTasksMu.Unlock()
			if err := os.Truncate(tmpPath, 0); err != nil {
				dlTasksMu.Lock()
				task.Status = "error"
				task.Error = tr("重置 .part 文件失败: ", "Failed to reset the .part file: ") + err.Error()
				task.Speed = 0
				dlTasksMu.Unlock()
				persistTasksNow()
				return
			}
			offset = 0
			// 重置速度采样基线：全量重写后 downloaded 从 0 重新累计。
			lastSampleTime = time.Time{}
			lastSampleBytes = 0
			continue
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
			task.Speed = 0
			dlTasksMu.Unlock()
			persistTasksNow()
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
				dlTasksMu.Lock()
				task.Speed = 0
				dlTasksMu.Unlock()
				persistTasksNow()
				waitForTaskResume(task, resumeCh)
				// Update offset for resume
				if fi, err := os.Stat(tmpPath); err == nil {
					offset = fi.Size()
				}
				// 重置速度采样基线：暂停期间 elapsed 会虚高，恢复后从新 offset
				// 重新建立采样，避免第一段速度被暂停时长拉低。
				lastSampleTime = time.Time{}
				lastSampleBytes = 0
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
				task.Speed = 0
				dlTasksMu.Unlock()
				persistTasksNow()
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
					task.Speed = 0
					dlTasksMu.Unlock()
					persistTasksNow()
					return
				}
				downloaded += int64(rr.n)
				dlTasksMu.Lock()
				task.Downloaded = downloaded
				if task.Total > 0 {
					task.Progress = int(float64(downloaded) * 100 / float64(task.Total))
				}
				// 速度采样：间隔 ≥1s 才更新，避免高频计算与波动。暂停恢复后
				// downloaded 从新 offset 重新累计，delta 为负时按 0 处理。
				now := time.Now()
				if lastSampleTime.IsZero() {
					lastSampleTime = now
					lastSampleBytes = downloaded
				} else if elapsed := now.Sub(lastSampleTime).Seconds(); elapsed >= 1.0 {
					delta := downloaded - lastSampleBytes
					if delta < 0 {
						delta = 0
					}
					task.Speed = computeSpeed(elapsed, delta)
					lastSampleTime = now
					lastSampleBytes = downloaded
				}
				dlTasksMu.Unlock()
				persistTasksThrottled()
			}
			if rr.err == io.EOF {
				resp.Body.Close()
				out.Close()
				// 移动失败时标记任务错误并返回，不再推进到 done（#10）。
				// moveFile 内部使用可注入的包级变量 renameFile，测试可模拟失败；
				// 跨设备（Windows 跨盘）时回退为复制 + 删除源文件。
				if err := moveFile(tmpPath, destPath); err != nil {
					dlTasksMu.Lock()
					task.Status = "error"
					task.Error = tr("重命名失败: ", "Rename failed: ") + err.Error()
					task.Speed = 0
					dlTasksMu.Unlock()
					persistTasksNow()
					return
				}
				dlTasksMu.Lock()
				task.Status = "done"
				task.Progress = 100
				task.Speed = 0
				dlTasksMu.Unlock()
				persistTasksNow()
				invalidateModelCache()
				return
			}
			if rr.err != nil {
				resp.Body.Close()
				out.Close()
				dlTasksMu.Lock()
				// 取消与读取错误的竞态防御：ctx 已取消时标记 cancelled 而非
				// error（与 client.Do 错误分支同策略）。
				if task.ctx.Err() != nil {
					task.Status = "cancelled"
				} else {
					task.Status = "error"
					task.Error = rr.err.Error()
				}
				task.Speed = 0
				dlTasksMu.Unlock()
				persistTasksNow()
				return
			}
		}
	}
}

// buildDownloadRequest creates a GET request for a download URL with the
// llama-desktop User-Agent, adding a Range header when resuming from an offset.
func buildDownloadRequest(downloadURL string, offset int64) (*http.Request, error) {
	req, err := http.NewRequest("GET", downloadURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "llama-desktop")
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
