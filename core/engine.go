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
	"regexp"
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
	Disk     *DiskUsage   `json:"disk,omitempty"`
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
	Name              string  `json:"name"`
	MemoryMB          int     `json:"memoryMb"`
	MemoryUsedMB      int     `json:"memoryUsedMb"`
	DriverVersion     string  `json:"driverVersion"`
	CUDACores         int     `json:"cudaCores"`
	ComputeCapability float64 `json:"computeCapability"`
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
	// CudartInstalled reports whether the CUDA runtime DLLs (cudart/cublas,
	// co-downloaded with CUDA builds since llama.cpp b10342) sit next to the
	// resolved binary; always false on non-Windows and for CPU/Vulkan builds.
	CudartInstalled bool `json:"cudartInstalled"`
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
	Source     string  `json:"source"` // download source: hf / modelscope
	URL        string  `json:"-"`
	Status     string  `json:"status"`
	Progress   int     `json:"progress"`
	Total      int64   `json:"total"`
	Downloaded int64   `json:"downloaded"`
	SizeHuman  string  `json:"sizeHuman"`
	Speed      float64 `json:"speed"` // current download speed (bytes/sec)
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
	// SourceDir is the root directory the model was scanned from (the model
	// download directory or the user-imported model directory). Lets the
	// frontend show which of the two sources a model belongs to when both are
	// configured.
	SourceDir string `json:"sourceDir"`
	// Alias is the llama-server model id for this model: the display Name
	// sanitized for INI section use with its original casing preserved, plus a
	// deterministic -2/-3 suffix on case-insensitive collisions (see
	// aliasDedup). The API page shows it so users copy-paste an id that
	// llama-server matches exactly (model lookup is case-sensitive).
	Alias string `json:"alias"`
}

// ─── Cached system info (collected once per process) ─────────────

// sysInfoCacheMu guards sysInfoCache so the six Home-page probes (fired in
// parallel by the frontend) trigger exactly one collectSystemInfo instead of
// six concurrent shell-outs.
var sysInfoCacheMu sync.Mutex
var sysInfoCache *SystemInfo

// windowsSysOnce guards the batched Windows PowerShell probe (CPU model,
// cores, total/free memory) so all Windows getters share a single process.
var windowsSysOnce sync.Once
var windowsSys windowsSystemSnapshot

var cachedLlamaCpp LlamaCppInfo
var llamaCacheValid atomic.Bool

var cachedModels []ModelInfo

// modelsCacheValid marks whether the model cache is valid (atomic read, used by
// the GetModels fast path). Writes happen inside the modelsMu lock; invalidation
// cannot be implemented by reassigning a sync.Once variable, because concurrent
// Do plus assignment corrupts Once's internal mutex state (#4).
var modelsCacheValid atomic.Bool

// modelsMu guards reads/writes of cachedModels and the cache-validity flag,
// preventing data races during concurrent model scans/refreshes (#4). Readers
// copy the slice under the lock before returning.
var modelsMu sync.Mutex

// invalidateModelCache invalidates the model cache: the next GetModels rescans
// the directory. Called from paths such as download completion and manual
// refresh, keeping invalidation safe under concurrent access.
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

// llamaCppDownloadDirOverride is the user-chosen download path for new
// llama.cpp installs (empty means unset, use the default downloadDir).
// Distinct from customLlamaCppDir (the imported existing install): new
// downloads land in the download path, while detection falls back to the
// imported directory second.
var llamaCppDownloadDirOverride string
var llamaCppDownloadDirMu sync.Mutex

// modelDownloadDirOverride is the user-chosen download path for new model
// downloads (empty means unset, use the default modelsDir). Distinct from
// customModelsDir (the imported existing model directory): downloads land in
// the download path, and the model list merges both sources.
var modelDownloadDirOverride string
var modelDownloadDirMu sync.Mutex

// ─── Download task queue ─────────────────────────────────────────

var dlTasks []*DlTask
var dlTasksMu sync.Mutex
var dlTaskCounter int

// ─── Download source (HF Mirror / ModelScope) ────────────────────

const (
	sourceHF              = "hf"
	sourceHuggingFace     = "huggingface"
	sourceModelScope      = "modelscope"
	defaultDownloadSource = sourceHF
)

// downloadSource is the current model download source (hf / modelscope);
// downloadSourceMu guards its reads/writes, consistent with the style of
// customLlamaCppDir and other config entries. Search, file listing, description,
// and download URL construction all route on the current activeDownloadSource().
var downloadSource = defaultDownloadSource
var downloadSourceMu sync.Mutex

// activeDownloadSource returns the currently active download source, read under the lock.
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

// systemInfo returns the static system snapshot, computing it once. Static
// hardware facts (CPU model, GPU identity, CUDA driver) do not change during a
// session, so every binding and manual refresh reuses the same collection.
func systemInfo() SystemInfo {
	sysInfoCacheMu.Lock()
	defer sysInfoCacheMu.Unlock()
	if sysInfoCache != nil {
		return *sysInfoCache
	}
	s := collectSystemInfo()
	sysInfoCache = &s
	return s
}

func collectSystemInfo() SystemInfo {
	info := SystemInfo{
		OS:   runtime.GOOS,
		Arch: runtime.GOARCH,
		CPU:  getCPUInfo(),
		Memory: MemoryInfo{
			TotalGB: getTotalMemoryGB(),
		},
		GPU:      getGPUInfo(),
		LlamaCpp: getLlamaCppInfo(),
	}

	// Free memory
	info.Memory.FreeGB = getFreeMemoryGB()

	// CUDA: reuse the driver version already fetched by the GPU probe so the
	// collection avoids a redundant nvidia-smi call.
	var driverHint string
	if len(info.GPU) > 0 {
		driverHint = info.GPU[0].DriverVersion
	}
	info.CUDA = getCUDAInfoWithDriver(driverHint)

	// Disk usage (sampleDiskUsage returns nil on failure so it never blocks
	// other metrics)
	info.Disk = sampleDiskUsage()

	return info
}

// ─── CPU ─────────────────────────────────────────────────────────

func getCPUInfo() CPUInfo {
	info := CPUInfo{
		LogicalCPUs: runtime.NumCPU(),
	}

	switch runtime.GOOS {
	case "windows":
		w := getWindowsSystem()
		info.Model = w.cpuModel
		if w.cpuCores > 0 {
			info.Cores = w.cpuCores
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
		if w := getWindowsSystem(); w.totalMemBytes > 0 {
			return float64(w.totalMemBytes) / (1024 * 1024 * 1024)
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
		if w := getWindowsSystem(); w.freeMemKB > 0 {
			return float64(w.freeMemKB) / (1024 * 1024)
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

// windowsSystemSnapshot holds the Windows-only facts that used to require four
// separate PowerShell processes (CPU model, core count, total/free memory).
// They are collected together in a single invocation and memoized per process.
type windowsSystemSnapshot struct {
	cpuModel      string
	cpuCores      int
	totalMemBytes uint64
	freeMemKB     uint64
}

// getWindowsSystem runs one PowerShell query returning CPU model, core count,
// total physical memory (bytes) and free physical memory (KB) as JSON, then
// caches the result. Every Windows getter (CPU / total memory / free memory)
// shares this, collapsing four powershell.exe launches into one.
func getWindowsSystem() windowsSystemSnapshot {
	windowsSysOnce.Do(func() {
		out := runCmd("powershell", "-NoProfile", "-Command", `
$cs = Get-CimInstance Win32_ComputerSystem
$cpu = Get-CimInstance Win32_Processor | Select-Object -First 1
$os = Get-CimInstance Win32_OperatingSystem
[PSCustomObject]@{
  cpuModel = $cpu.Name
  cpuCores = $cpu.NumberOfCores
  totalMem = $cs.TotalPhysicalMemory
  freeMem  = $os.FreePhysicalMemory
} | ConvertTo-Json -Compress`)
		windowsSys = parseWindowsSystemJSON(out)
	})
	return windowsSys
}

// parseWindowsSystemJSON parses the batched PowerShell JSON into a snapshot.
// Invalid/empty output yields a zero snapshot so callers fall back to their
// own defaults.
func parseWindowsSystemJSON(out string) windowsSystemSnapshot {
	var v struct {
		CpuModel string `json:"cpuModel"`
		CpuCores int    `json:"cpuCores"`
		TotalMem uint64 `json:"totalMem"`
		FreeMem  uint64 `json:"freeMem"`
	}
	if err := json.Unmarshal([]byte(out), &v); err != nil {
		return windowsSystemSnapshot{}
	}
	return windowsSystemSnapshot{
		cpuModel:      strings.TrimSpace(v.CpuModel),
		cpuCores:      v.CpuCores,
		totalMemBytes: v.TotalMem,
		freeMemKB:     v.FreeMem,
	}
}

// ─── GPU ─────────────────────────────────────────────────────────

func getGPUInfo() []GPUInfo {
	out := runCmd("nvidia-smi",
		"--query-gpu=name,memory.used,memory.total,driver_version,compute_cap",
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
		memUsedStr := strings.TrimSpace(parts[1])
		memStr := strings.TrimSpace(parts[2])
		driver := strings.TrimSpace(parts[3])

		memUsedMB := 0
		if memUsedStr != "" {
			if v, err := strconv.Atoi(memUsedStr); err == nil {
				memUsedMB = v
			}
		}
		memMB := 0
		if memStr != "" {
			if v, err := strconv.Atoi(memStr); err == nil {
				memMB = v
			}
		}

		gpu := GPUInfo{
			Name:          name,
			MemoryMB:      memMB,
			MemoryUsedMB:  memUsedMB,
			DriverVersion: driver,
		}

		// Compute capability (5th column, index 4): nvidia-smi returns the
		// decimal form directly (e.g. "9.0", "8.9", "12.0"), NOT an integer ×10.
		if len(parts) >= 5 {
			gpu.ComputeCapability = parseGPUComputeCapability(parts[4])
		}

		gpus = append(gpus, gpu)
	}
	return gpus
}

// probeGPUComputeCap is the injection point for the GPU compute-capability
// probe (a package-level var in the same style as probeLlamaVersion): the
// default implementation runs `nvidia-smi --query-gpu=compute_cap` and returns
// raw stdout. Tests replace this variable instead of shelling out.
var probeGPUComputeCap = func() string {
	return runCmd("nvidia-smi", "--query-gpu=compute_cap", "--format=csv,noheader")
}

// gpuComputeCap parses the first value of the compute-capability probe output
// (e.g. "12.0" for RTX 50 series); ok=false on empty output or parse failure.
// Only the first value is used: on multi-GPU hosts the first enumerated GPU
// decides the CUDA floor.
func gpuComputeCap() (float64, bool) {
	out := strings.TrimSpace(probeGPUComputeCap())
	if out == "" {
		return 0, false
	}
	first := out
	if idx := strings.IndexAny(first, "\r\n"); idx >= 0 {
		first = first[:idx]
	}
	cc, err := strconv.ParseFloat(strings.TrimSpace(first), 64)
	if err != nil {
		return 0, false
	}
	return cc, true
}

// parseGPUComputeCapability parses the compute-capability field returned by
// `nvidia-smi --query-gpu=compute_cap` (the decimal form, e.g. "9.0", "8.9",
// "12.0"). Returns 0 on empty/garbage input so callers can treat 0 as unknown.
func parseGPUComputeCapability(s string) float64 {
	v, err := strconv.ParseFloat(strings.TrimSpace(s), 64)
	if err != nil {
		return 0
	}
	return v
}

// cudaFloorForComputeCap returns the minimum CUDA runtime version the GPU can
// actually run: Blackwell GPUs (compute capability >= 12.0, e.g. RTX 50 series)
// need CUDA >= 12.8 or binaries fail with "no kernel image"; earlier (or
// unknown) GPUs have no floor.
func cudaFloorForComputeCap(cc float64) float64 {
	if cc >= 12.0 {
		return 12.8
	}
	return 0
}

// ─── CUDA ────────────────────────────────────────────────────────

// getCUDAInfo returns CUDA availability: the driver version is taken from the
// GPU probe when driverHint is non-empty, otherwise queried via nvidia-smi.
// The collection path passes the GPU driver so it avoids a redundant nvidia-smi
// process.
func getCUDAInfo() CUDAInfo {
	return getCUDAInfoWithDriver("")
}

func getCUDAInfoWithDriver(driverHint string) CUDAInfo {
	info := CUDAInfo{}

	// Driver version: prefer the one already collected from the GPU probe.
	if driverHint != "" {
		info.Available = true
		info.DriverVersion = driverHint
	} else {
		out := runCmd("nvidia-smi", "--query-gpu=driver_version", "--format=csv,noheader")
		if out != "" {
			info.Available = true
			info.DriverVersion = strings.TrimSpace(out)
		}
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

// findLlamaBinInDir searches for llama.cpp binary bin under dir: first the dir
// root, then one-level subdirectories (the downloaded zip may extract with a
// top-level folder, e.g. llama-b9999-bin/). On Windows, also accepts files
// without the .exe suffix. Returns the absolute path on hit, empty string otherwise.
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
	// Windows fallback: files without the .exe suffix
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

// findLlamaBin searches for llama.cpp binary bin under dir: an empty dir means
// PATH lookup (exec.LookPath), returning the LookPath-resolved result; a
// non-empty directory delegates to findLlamaBinInDir.
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

// resolveLlamaServerBin resolves the llama-server executable path by priority
// llamaCppDownloadDir() > customLlamaCppDir (imported install) > PATH, shared by
// getLlamaCppInfo and buildServerCommand to keep the two lookups from drifting.
// A directory hit returns an absolute path; a PATH hit returns the bare binary
// name "llama-server" (left for exec.Command to resolve); no hit returns "".
func resolveLlamaServerBin() string {
	if p := findLlamaBinInDir(llamaCppDownloadDir(), "llama-server"); p != "" {
		return p
	}
	customLlamaCppMu.Lock()
	customDir := customLlamaCppDir
	customLlamaCppMu.Unlock()
	if customDir != "" {
		if p := findLlamaBinInDir(customDir, "llama-server"); p != "" {
			return p
		}
	}
	if _, err := exec.LookPath("llama-server"); err == nil {
		return "llama-server"
	}
	return ""
}

// llamaVersionProbeTimeout is the upper bound for llama.cpp version probing.
// A healthy binary returns from --version in milliseconds; a timeout means the
// binary is misbehaving (e.g. treating -v as a version flag and starting a full
// HTTP server that runs forever), in which case the child process is killed and
// an empty string returned so getLlamaCppInfo returns quickly and the detection
// chain is never frozen by a broken binary. It is a package-level var instead
// of a const so tests can temporarily shorten it to verify the kill behavior
// (an injection point in the same style as probeLlamaVersion; tests restore it
// immediately after use).
var llamaVersionProbeTimeout = 5 * time.Second

// probeLlamaVersion is the injection point for the llama.cpp version-probe
// command execution (a package-level var in the same style as
// githubReleasesAPI / renameFile / updateRepoAPI): the default implementation
// runs `path --version` with a timeout and merges stdout+stderr. Tests can
// replace this variable to inject a fake probe command and avoid launching a
// real binary; and since the probe argument (--version) is encapsulated inside
// the default implementation, tests can directly assert that only --version is
// ever invoked, with no fallback to -v.
var probeLlamaVersion = func(path string) string {
	ctx, cancel := context.WithTimeout(context.Background(), llamaVersionProbeTimeout)
	defer cancel()

	cmd := exec.CommandContext(ctx, path, "--version")
	hideWindow(cmd)
	var out, errOut bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &errOut
	// runCmd only captures stdout, while llama-server's --version output goes
	// entirely to stderr (stdout empirically empty); merge both to get the version
	if err := cmd.Run(); err != nil && errOut.Len() > 0 {
		log.Printf("[CMD] %s --version stderr: %s", path, strings.TrimSpace(errOut.String()))
	}
	return strings.TrimSpace(out.String() + errOut.String())
}

// parseLlamaVersion extracts the version string from probe output: prefer lines
// starting with "version" or containing "build" (typical llama.cpp --version
// output, e.g. "version: 1234"), otherwise return the whole trimmed output.
// Pure string logic, directly assertable by unit tests.
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

// fillLlamaCppVersion tries to run the binary to read its version into
// info.Version. Probing only invokes `--version` (merging stdout+stderr), never
// falling back to `-v`: llama-server 10342's -v is not a version flag but starts
// a full HTTP server, which previously caused version probing to block forever
// and the home page to permanently show "not found". Timeout-protected, so no
// misbehaving binary can freeze the detection chain. A failed run (e.g. a stub
// that is not executable on Windows) only leaves Version empty; Installed is
// unaffected.
func fillLlamaCppVersion(info *LlamaCppInfo, path string) {
	versionOut := probeLlamaVersion(path)
	if versionOut != "" {
		info.Version = parseLlamaVersion(versionOut)
	}
}

// getLlamaCppInfo detects the llama.cpp runtime: searches for the binary by
// priority llamaCppDownloadDir() > customLlamaCppDir (imported install) > PATH.
// llama-server goes through the shared helper resolveLlamaServerBin (the
// download directory supports both root and one-level-subdir layouts); the
// other candidate binaries (llama-cli / llama.cpp / llama) follow the same
// directory priority.
func getLlamaCppInfo() LlamaCppInfo {
	info := LlamaCppInfo{}

	// A PATH hit for llama-server is returned by the helper as the bare binary
	// name; restore the exec.LookPath-resolved result so the frontend can show
	// the full path
	if p := resolveLlamaServerBin(); p != "" {
		if p == "llama-server" {
			if resolved, err := exec.LookPath(p); err == nil {
				p = resolved
			}
		}
		info.Installed = true
		info.Path = p
		fillLlamaCppVersion(&info, p)
		info.CudartInstalled = detectCudartRuntime(filepath.Dir(p))
		return info
	}

	customLlamaCppMu.Lock()
	customDir := customLlamaCppDir
	customLlamaCppMu.Unlock()

	// Download path first, then the imported install, then PATH — the same
	// order as resolveLlamaServerBin / llamaCppDownloadDir.
	dirsToCheck := []string{llamaCppDownloadDir()}
	if customDir != "" {
		dirsToCheck = append(dirsToCheck, customDir)
	}
	dirsToCheck = append(dirsToCheck, "") // empty string means PATH

	for _, dir := range dirsToCheck {
		for _, bin := range []string{"llama-cli", "llama.cpp", "llama"} {
			if p := findLlamaBin(dir, bin); p != "" {
				info.Installed = true
				info.Path = p
				fillLlamaCppVersion(&info, p)
				info.CudartInstalled = detectCudartRuntime(filepath.Dir(p))
				return info
			}
		}
	}

	return info
}

// detectCudartRuntime reports whether the CUDA runtime DLLs shipped by the
// cudart asset (cudart64_*.dll plus the cublas family) are present in dir.
// The DLLs land next to llama-server.exe because both assets extract into the
// same directory, so their presence marks a CUDA build with its runtime
// installed. Non-Windows never carries the Windows-exclusive cudart asset.
func detectCudartRuntime(dir string) bool {
	if runtime.GOOS != "windows" || dir == "" {
		return false
	}
	patterns := []string{"cudart*.dll", "cublas*.dll"}
	for _, pat := range patterns {
		if matches, err := filepath.Glob(filepath.Join(dir, pat)); err == nil && len(matches) > 0 {
			return true
		}
	}
	return false
}

// ─── Command helpers ─────────────────────────────────────────────

// cmdTimeout is the upper bound for runCmd child-process execution. System
// collection queries (WMI/CIM via powershell, nvidia-smi, sysctl, ...) are
// expected to return in well under a second; a hung query (e.g. the WMI
// service stalling) must not freeze the whole info fetch, so the child is
// killed and runCmd returns "" (callers fall back to their defaults). It is a
// package-level var so tests can shorten it (same style as
// llamaVersionProbeTimeout).
var cmdTimeout = 8 * time.Second

func runCmd(name string, args ...string) string {
	ctx, cancel := context.WithTimeout(context.Background(), cmdTimeout)
	defer cancel()
	cmd := exec.CommandContext(ctx, name, args...)
	hideWindow(cmd)
	var out, errOut bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &errOut
	if err := cmd.Run(); err != nil {
		if ctx.Err() == context.DeadlineExceeded {
			log.Printf("[WARN] runCmd timeout after %v: %s %v", cmdTimeout, name, args)
			return "" // timed-out output may be truncated; treat as unavailable
		}
		if errOut.Len() > 0 {
			log.Printf("[CMD] %s %v stderr: %s", name, args, strings.TrimSpace(errOut.String()))
		}
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
		// Match fields of the form "<key>:" (macOS output is "Pages free:
		// 123456." — the value follows the key field and ends with a period),
		// avoiding parse failures that would leave free memory stuck at 0.
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

// customModelsDir is the imported model directory (empty means unset). It is
// the directory of models the user already has and wants to reuse; distinct
// from modelDownloadDirOverride, where new downloads land. modelsDirMu guards
// its reads/writes, consistent with the style of customLlamaCppMu guarding
// customLlamaCppDir.
var customModelsDir string
var modelsDirMu sync.Mutex

// effectiveModelDownloadDir returns the directory new model downloads land in:
// the user-chosen download path when configured, otherwise the default
// modelsDir.
func effectiveModelDownloadDir() string {
	modelDownloadDirMu.Lock()
	dir := modelDownloadDirOverride
	modelDownloadDirMu.Unlock()
	if dir != "" {
		return dir
	}
	return modelsDir
}

// modelScanDirs returns the roots the model list is scanned from, in priority
// order: the model download directory first, then the imported model directory
// when set. Directories are resolved to absolute paths and duplicates removed,
// so pointing both settings at the same place does not double-list models, and
// so the SourceDir annotated on scanned models matches the absolute download
// path GetConfig reports to the frontend (the default is cwd-relative, e.g.
// LLM-Models).
func modelScanDirs() []string {
	dirs := []string{effectiveModelDownloadDir()}
	modelsDirMu.Lock()
	imported := customModelsDir
	modelsDirMu.Unlock()
	if imported != "" {
		dirs = append(dirs, imported)
	}
	seen := make(map[string]bool, len(dirs))
	out := make([]string, 0, len(dirs))
	for _, d := range dirs {
		clean := filepath.Clean(d)
		if abs, err := filepath.Abs(clean); err == nil {
			clean = abs
		}
		if seen[clean] {
			continue
		}
		seen[clean] = true
		out = append(out, clean)
	}
	return out
}

// scanModels scans all model sources (the model download directory and the
// imported model directory, when set), creating the default LLM-Models
// directory only when no custom path is configured. Results are merged by
// model identity (author + name): the download path is scanned first, so a
// model present in both sources is listed once with the download path's copy
// (the imported duplicate is dropped); each result is annotated with the root
// it was scanned from.
func scanModels() []ModelInfo {
	merged := make([]ModelInfo, 0)
	seen := make(map[string]bool)
	for _, dir := range modelScanDirs() {
		// Custom paths are user-picked and expected to exist already; only
		// the default directory needs lazy creation (compare both the
		// relative default and its absolute form, as scan roots are
		// resolved to absolute paths).
		isDefault := dir == modelsDir
		if abs, err := filepath.Abs(modelsDir); err == nil {
			isDefault = isDefault || dir == abs
		}
		if isDefault {
			if err := os.MkdirAll(dir, 0755); err != nil {
				log.Printf("[WARN] Failed to create %s dir: %v", dir, err)
				continue
			}
		}
		for _, m := range scanModelsDir(dir) {
			// Dedupe by author+name (the identity shown in the model list):
			// a copy in the imported directory is dropped when the download
			// path already has the same model.
			key := m.Author + "\x00" + m.Name
			if seen[key] {
				continue
			}
			seen[key] = true
			m.SourceDir = dir
			merged = append(merged, m)
		}
	}
	sort.Slice(merged, func(i, j int) bool {
		return merged[i].SizeBytes > merged[j].SizeBytes
	})
	// Assign llama-server model ids after the final ordering so the alias the
	// UI shows (copy-paste target) is the same section name the preset writes;
	// generateModelsPresetFrom recomputes with the same deterministic helper.
	usedAliases := make(map[string]int)
	for i := range merged {
		merged[i].Alias = aliasDedup(merged[i].Name, usedAliases)
	}
	return merged
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

// buildModelInfo builds a ModelInfo from one GGUF main-file path: reads GGUF
// metadata to override name/architecture/quantization, falling back to
// fallbackName/author when missing. Shared by the two-level and three-level
// scans, avoiding duplicated metadata reading and fallback logic between
// variant directories and author loose files.
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
			// Some converters embed the full source repo id ("org/model")
			// into general.name; display only the model segment.
			if i := strings.LastIndex(n, "/"); i >= 0 && i+1 < len(n) {
				n = n[i+1:]
			}
			model.Name = n
		}
		if a := metadata["arch"]; a != "" {
			model.Architecture = a
		}
		if q := metadata["quant"]; q != "" {
			model.Quantization = q
		}
	}
	// The main file name identifies the actual variant on disk when it is
	// strictly more specific than the resolved name: either the resolved name
	// is only its prefix (unsloth writes the bare base-model name into
	// general.name for every quant in a repo, e.g. "Qwen3.5-9B" for
	// Qwen3.5-9B-UD-Q4_K_XL.gguf), or the resolved name is a "<model>-GGUF"
	// variant-directory fallback that hides the quant the file name carries.
	if fileBase := strings.TrimSuffix(filepath.Base(path), filepath.Ext(path)); fileBase != model.Name &&
		(preferFileNameVariant(model.Name, fileBase) || preferFileNameOverGenericSuffix(model.Name, fileBase)) {
		model.Name = fileBase
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

// preferFileNameVariant reports whether fileBase carries strictly more
// information than name: name is a proper prefix of fileBase and the next
// character is a separator ("-" or "_"). This catches converters that write
// the bare base-model name ("Qwen3.5-9B") while the file name carries the
// quant variant ("Qwen3.5-9B-UD-Q4_K_XL"); generic file names ("model.gguf")
// never qualify because name does not prefix them.
func preferFileNameVariant(name, fileBase string) bool {
	if len(name) == 0 || len(fileBase) <= len(name) {
		return false
	}
	if !strings.EqualFold(fileBase[:len(name)], name) {
		return false
	}
	c := fileBase[len(name)]
	return c == '-' || c == '_'
}

// genericVariantSuffixes lists variant-name suffixes that only mark the file
// container format rather than the model itself; HF download destinations
// typically name the variant directory after the source repo ("<model>-GGUF").
var genericVariantSuffixes = []string{"-GGUF", "_GGUF"}

// preferFileNameOverGenericSuffix reports whether fileBase carries strictly
// more information than name: name ends with a generic "-GGUF"/"_GGUF" suffix
// and fileBase begins with the suffix-trimmed name followed by a separator
// ("-" or "_"). The variant-directory fallback hides the actual quant variant
// ("Qwen3.5-9B-GGUF" holding "Qwen3.5-9B-Q4_K_M.gguf"); the main file name
// identifies the real variant on disk. Mirrors preferFileNameVariant's
// prefix/separator style.
func preferFileNameOverGenericSuffix(name, fileBase string) bool {
	for _, suffix := range genericVariantSuffixes {
		if len(name) <= len(suffix) || !strings.EqualFold(name[len(name)-len(suffix):], suffix) {
			continue
		}
		trimmed := name[:len(name)-len(suffix)]
		if len(fileBase) <= len(trimmed) || !strings.EqualFold(fileBase[:len(trimmed)], trimmed) {
			continue
		}
		if c := fileBase[len(trimmed)]; c == '-' || c == '_' {
			return true
		}
	}
	return false
}

// converterPlaceholderNames lists placeholder values converters write into
// general.name when the real model name is unknown ("Unsloth_Gguf" from
// unsloth, "Hf_Model" from some HF-space converters). A name equal to or
// starting with any entry is treated as unreadable (case-insensitive), so the
// scanner falls back to the variant directory / file name.
var converterPlaceholderNames = []string{"Unsloth_Gguf", "Hf_Model"}

// isReadableName returns true if the name doesn't look like a hash/UUID or a
// converter placeholder.
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
	// Converter placeholders are auto-generated names, not real model names
	for _, ph := range converterPlaceholderNames {
		if len(name) >= len(ph) && strings.EqualFold(name[:len(ph)], ph) {
			return false
		}
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

	// Guard against malicious/corrupt files: without a cap on kvCount, the
	// loop could parse an extremely long KV list and amplify parsing cost
	// (#7.2). Real GGUF metadata keys are few; give up parsing beyond 4096.
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

// githubReleasesAPI points to the llama.cpp latest release API, declared as a
// var so tests can replace the package-level variable to inject a local
// httptest server (same style as updateRepoAPI).
var githubReleasesAPI = "https://api.github.com/repos/ggml-org/llama.cpp/releases/latest"

// githubReleasesListAPI lists recent llama.cpp releases newest-first (including
// prereleases), declared as a var for test injection like githubReleasesAPI.
// Since 2026-08 upstream ships binaries only in nightly prereleases while
// releases/latest (non-prerelease only) points to an asset-less marker, so the
// download flow falls back to this list to find the newest release with binaries.
var githubReleasesListAPI = "https://api.github.com/repos/ggml-org/llama.cpp/releases?per_page=10"

const downloadDir = "llama-cpp"

// llamaCppDownloadDir returns the target directory for llama.cpp download
// extraction: the user-chosen download path when configured, otherwise the
// default llama-cpp/. Matches the detection priority of getLlamaCppInfo /
// resolveLlamaServerBin (download path > imported customLlamaCppDir > PATH) so
// the download landing spot and the detection location stay consistent.
func llamaCppDownloadDir() string {
	llamaCppDownloadDirMu.Lock()
	dir := llamaCppDownloadDirOverride
	llamaCppDownloadDirMu.Unlock()
	if dir != "" {
		return dir
	}
	return downloadDir
}

// configFile is the config persistence path, declared as a var so tests can
// override it via chdir.
var configFile = "llama-desktop-config.json"

// legacyConfigFile is the config filename from before the llama-gui →
// llama-desktop rename. It serves only as a one-shot migration source (see
// migrateLegacyConfig): when the new file does not exist but the old one does,
// it is renamed wholesale and reused, preserving theme / directories / model
// params / download queue for existing users losslessly.
var legacyConfigFile = "llama-gui-config.json"

// renameFile is a test injection point (same style as configFile), used to
// simulate the branch where renaming the temp file after download fails (#10).
var renameFile = os.Rename

// copyFile copies src to dst: creates dst with src's FileMode and explicitly
// chmods, preserving the executable permission (updating the exe on Linux
// needs +x), independent of umask.
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
	// Explicit chmod guarantees destination permissions exactly match the
	// source (independent of umask)
	if err := os.Chmod(dst, fi.Mode()); err != nil {
		return fmt.Errorf(tr("设置目标文件权限失败: %w", "failed to set destination file permissions: %w"), err)
	}
	return nil
}

// moveFile moves src to dst: prefers renameFile (a package-level injection
// point so tests can simulate failure); across devices (Windows cross-drive
// ERROR_NOT_SAME_DEVICE / Unix cross-mount EXDEV) os.Rename always fails, so
// it falls back to copyFile + os.Remove(src), preserving source permissions.
// Cross-device detection uses the platform constant crossDeviceRenameErr: on
// Windows, syscall.EXDEV is an invented Go constant that never equals the real
// error code, so it must not be used for the check.
// Other failures (e.g. destination already exists) keep the original
// semantics: delete dst and retry renameFile once.
// Critical ordering: the cross-device check must run before the delete-old-
// and-retry path, to avoid deleting an existing old file in cross-device cases.
func moveFile(src, dst string) error {
	err := renameFile(src, dst)
	if err == nil {
		return nil
	}
	if errors.Is(err, crossDeviceRenameErr) {
		// Rename across devices is impossible: copy to the destination
		// (overwriting an existing same-name file), then remove the source
		if copyErr := copyFile(src, dst); copyErr != nil {
			return copyErr
		}
		if removeErr := os.Remove(src); removeErr != nil {
			return fmt.Errorf(tr("删除源文件失败: %w", "failed to remove source file: %w"), removeErr)
		}
		return nil
	}
	// Other failures such as destination-exists: delete the old destination
	// and retry once, consistent with the original update logic
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

	// Step 1: Fetch latest release info (ctx-bound: cancel during the fetch
	// aborts the in-flight request immediately instead of waiting out the
	// HTTP timeout, and resets the state to idle rather than an error)
	downloadMu.Lock()
	downloadState.Status = "fetching"
	downloadMu.Unlock()

	release, err := fetchLatestRelease(ctx)
	if err != nil {
		if ctx.Err() != nil {
			// Cancelled by user while fetching release metadata
			downloadMu.Lock()
			if downloadState.Status != "idle" {
				downloadState.Status = "idle"
				downloadState.Error = ""
				downloadState.Paused = false
			}
			downloadMu.Unlock()
			log.Println("⏹️ llama.cpp download stopped by user")
			return
		}
		setDownloadError(tr("获取发布信息失败: ", "Failed to fetch release info: ") + err.Error())
		return
	}

	// Step 2: Find best asset (the main-program asset; the cudart runtime is an additional asset)
	mainAsset := pickBestAsset(release.Assets)
	if mainAsset == nil {
		// Since 2026-08 llama.cpp ships binaries only in nightly prereleases;
		// releases/latest (non-prerelease only) points to an asset-less marker
		// — fall back to the newest listed release that actually carries a
		// matching build. Fallback fetch errors stay silent: the final error
		// below is the more actionable one.
		if list, listErr := fetchReleaseListAt(ctx, githubReleasesListAPI); listErr == nil {
			if rel := newestReleaseWithAssets(list); rel != nil {
				release = rel
				mainAsset = pickBestAsset(release.Assets)
			}
		}
	}
	if mainAsset == nil {
		if ctx.Err() != nil {
			// Cancelled by user during the fallback release-list fetch
			downloadMu.Lock()
			if downloadState.Status != "idle" {
				downloadState.Status = "idle"
				downloadState.Error = ""
				downloadState.Paused = false
			}
			downloadMu.Unlock()
			log.Println("⏹️ llama.cpp download stopped by user")
			return
		}
		setDownloadError(tr("未找到适用于当前平台的 llama.cpp 构建", "No llama.cpp build found for the current platform"))
		return
	}

	// Since b10342, Windows CUDA builds split the runtime into a separate
	// cudart zip; the main-program zip no longer bundles it, so the cudart
	// asset must be co-downloaded and extracted into the same directory.
	// Detected by whether the main asset name contains "cuda"
	// (pickBestAsset only picks a cuda build on Windows when a GPU is
	// detected, so the asset name is the selection result — no second GPU
	// probe needed); non-Windows cuda builds (if any) do not get the
	// Windows-exclusive cudart asset attached.
	assets := []*GitHubAsset{mainAsset}
	if runtime.GOOS == "windows" && strings.Contains(strings.ToLower(mainAsset.Name), "cuda") {
		// Pair the cudart runtime with the CUDA version and arch of the chosen
		// main asset (both extracted from its name, not from the local nvcc
		// toolkit): the runtime DLLs must match the build actually downloaded,
		// which floor/tie-break selection may pick independently of toolkit.
		cudartVer, _ := cudaVerTagOf(mainAsset.Name)
		if cudart := pickCudartAssetFor(release.Assets, cudartVer, archKeyOf(runtime.GOARCH)); cudart != nil {
			assets = append(assets, cudart)
		}
	}

	downloadMu.Lock()
	downloadState.Status = "downloading"
	downloadState.FileName = mainAsset.Name
	// Sequential multi-asset download: Total is the sum of all asset sizes,
	// Downloaded accumulates across assets
	var totalBytes int64
	for _, a := range assets {
		totalBytes += a.Size
	}
	downloadState.Total = totalBytes
	downloadState.Version = release.TagName
	downloadState.Downloaded = 0
	downloadMu.Unlock()

	// Target directory: custom llama.cpp directory first, otherwise the
	// default llama-cpp/; created once before extraction
	targetDir := llamaCppDownloadDir()
	if err := os.MkdirAll(targetDir, 0755); err != nil {
		setDownloadError(tr("创建目录失败: ", "Failed to create directory: ") + err.Error())
		return
	}

	// Step 3: Download and extract each asset sequentially (main program
	// first, then cudart); pause/stop semantics unchanged; progress overlays
	// baseDownloaded with bytes already completed by previous assets
	var baseDownloaded int64
	for _, asset := range assets {
		// Multi-asset downloads must re-enter "downloading" per asset: the
		// status is set once before this loop, but asset 1's extraction
		// flips it to "extracting"; without resetting here, asset 2+ (the
		// cudart runtime zip) downloads under the stale "extracting" label —
		// the UI claims "extracting" while bytes are still being fetched,
		// and a network failure there surfaces as a confusing "download
		// failed" right after "extracting".
		downloadMu.Lock()
		downloadState.Status = "downloading"
		downloadState.FileName = asset.Name
		downloadMu.Unlock()

		tmpPath, err := downloadWithResume(ctx, asset.BrowserDownloadURL, asset.Size, baseDownloaded)
		if err != nil {
			if ctx.Err() != nil {
				// Cancelled by user (stop) — also from the paused state: the
				// previous guard skipped the reset when status was "paused",
				// leaving the state machine stranded in paused with the
				// download goroutine already gone (Cancel and Resume both
				// appeared dead afterwards)
				downloadMu.Lock()
				if downloadState.Status != "idle" {
					downloadState.Status = "idle"
					downloadState.Error = ""
					downloadState.Paused = false
				}
				downloadMu.Unlock()
				log.Println("⏹️ llama.cpp download stopped by user")
			} else {
				setDownloadError(tr("下载失败: ", "Download failed: ") + err.Error())
			}
			return
		}

		// Temp file cleaned up after extraction (including cancel/error paths)
		defer os.Remove(tmpPath)

		// Check if stopped during download
		select {
		case <-ctx.Done():
			return
		default:
		}

		// Step 4: Extract (into the same directory as the main program)
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
			// Same as the original single-asset logic: unsupported formats
			// error out directly, without the "extraction failed" prefix
			setDownloadError(tr("不支持的文件格式: ", "Unsupported file format: ") + asset.Name)
			return
		}
		if extractErr != nil {
			setDownloadError(tr("解压失败: ", "Extraction failed: ") + extractErr.Error())
			return
		}

		baseDownloaded += asset.Size
	}

	// Step 5: Done (only set after all assets downloaded and extracted)
	downloadMu.Lock()
	downloadState.Status = "done"
	downloadState.Progress = 100
	downloadMu.Unlock()

	// Reset model cache so new models are picked up
	invalidateModelCache()
	// Invalidate the llama.cpp detection cache: the result cached at mount
	// time (Installed=false) is stale; re-detect after successful extraction,
	// otherwise the home page keeps showing "not found"
	llamaCacheValid.Store(false)

	log.Printf("[OK] llama.cpp %s downloaded and extracted to %s/", release.TagName, targetDir)
}

// ─── Download retry policy ─────────────────────────────────────────
//
// All download paths (llama.cpp, model files, app update) share the same
// automatic retry policy: a transient failure (network error, HTTP 429/5xx)
// is retried internally up to downloadRetryCount times — keeping the
// downloading state, never losing the bytes already on disk — before the
// error is surfaced for a manual user retry. Permanent failures (other 4xx,
// disk errors) and user-initiated cancellation are never retried.

var (
	// downloadRetryCount bounds the automatic retries per failure episode.
	// Package-level var so tests can keep runs fast without sleeping.
	downloadRetryCount = 3
	// downloadRetryDelay is the backoff between automatic retries.
	downloadRetryDelay = 3 * time.Second
)

// retryableDownloadStatus reports whether an HTTP status is transient and
// worth retrying: 429 and 5xx qualify; other 4xx (404/403/...) are permanent.
func retryableDownloadStatus(status int) bool {
	return status == http.StatusTooManyRequests || status >= 500
}

// sleepDownloadRetry waits the retry delay, interruptible by ctx; returns
// false when the context was cancelled first (the caller then gives up).
func sleepDownloadRetry(ctx context.Context) bool {
	select {
	case <-ctx.Done():
		return false
	case <-time.After(downloadRetryDelay):
		return true
	}
}

// downloadWithResume downloads a file with pause/resume support.
// baseDownloaded is the total bytes of assets already completed before this
// file: in sequential multi-asset downloads (e.g. llama.cpp main program +
// cudart runtime) progress must overlay the previous cumulative value; pass 0
// for a single-asset call.
// Robustness: after a pause the temp file is reopened before writing (the
// pre-pause handle is closed); a 200 answer to a Range resume truncates the
// temp file and restarts clean; a clean EOF before totalSize (a truncated
// body) auto-resumes via Range, giving up after 3 retries with a clear error
// instead of returning a corrupt zip for extraction.
// Returns the path to the downloaded temp file.
func downloadWithResume(ctx context.Context, url string, totalSize int64, baseDownloaded int64) (string, error) {
	tmpFile, err := os.CreateTemp("", "llamacpp-download-*"+filepath.Ext(url[strings.LastIndex(url, "."):]))
	if err != nil {
		return "", fmt.Errorf(tr("创建临时文件失败: %w", "failed to create temporary file: %w"), err)
	}
	tmpPath := tmpFile.Name()

	// closeTmp closes the current handle and clears it so the next
	// outer-loop pass knows to reopen the file: the resume path must
	// reopen the temp file, because writing to the pre-pause handle fails
	// with "file already closed".
	closeTmp := func() {
		if tmpFile != nil {
			tmpFile.Close()
			tmpFile = nil
		}
	}
	// reopenTmp reopens the temp file for append when a previous pass
	// (pause or truncated-EOF retry) closed the handle; O_APPEND makes
	// every Write land at the current end of file, no seek needed.
	reopenTmp := func() error {
		if tmpFile != nil {
			return nil
		}
		f, err := os.OpenFile(tmpPath, os.O_WRONLY|os.O_CREATE|os.O_APPEND, 0644)
		if err != nil {
			return fmt.Errorf(tr("打开临时文件失败: %w", "failed to reopen temporary file: %w"), err)
		}
		tmpFile = f
		return nil
	}

	// A clean EOF before the declared size means a truncated body: the
	// loop below auto-resumes with a Range request from the bytes already
	// on disk, giving up after maxTruncResumes retries with a clear error
	// instead of extracting a corrupt zip.
	const maxTruncResumes = 3
	truncResumes := 0

	// Automatic transient-failure retries (see the download retry policy
	// block): network errors and HTTP 429/5xx reconnect up to
	// downloadRetryCount times — the outer loop re-stats the temp file, so
	// every retry resumes from the bytes already on disk.
	retries := 0

	// We'll loop to handle pause → resume cycles
	for {
		// Check if cancelled
		select {
		case <-ctx.Done():
			closeTmp()
			return tmpPath, ctx.Err()
		default:
		}

		// Reopen the temp file if a previous pass (pause / truncated-EOF
		// retry) closed it; downloads resume by appending to the bytes
		// already on disk.
		if err := reopenTmp(); err != nil {
			return tmpPath, err
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
			closeTmp()
			return tmpPath, err
		}
		req.Header.Set("User-Agent", appUserAgent())
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
			closeTmp()
			if ctx.Err() == nil && retries < downloadRetryCount {
				retries++
				log.Printf("[WARN] llama.cpp download attempt failed (%v), retrying %d/%d", err, retries, downloadRetryCount)
				if sleepDownloadRetry(ctx) {
					continue
				}
				return tmpPath, ctx.Err()
			}
			return tmpPath, err
		}

		// Handle response
		expectedStatus := http.StatusOK
		if offset > 0 {
			expectedStatus = http.StatusPartialContent
		}
		if resp.StatusCode != expectedStatus && resp.StatusCode != http.StatusOK {
			status := resp.StatusCode
			resp.Body.Close()
			closeTmp()
			if retryableDownloadStatus(status) && ctx.Err() == nil && retries < downloadRetryCount {
				retries++
				log.Printf("[WARN] llama.cpp download got HTTP %d, retrying %d/%d", status, retries, downloadRetryCount)
				if sleepDownloadRetry(ctx) {
					continue
				}
				return tmpPath, ctx.Err()
			}
			return tmpPath, fmt.Errorf("HTTP %d", status)
		}

		// Robustness against servers ignoring Range: this request carried
		// a Range header (offset>0) but the server answered 200 with the
		// full body from byte 0. Appending that body at the current offset
		// would yield "partial prefix + full body" corruption, so truncate
		// the temp file to zero — with the O_APPEND handle the next writes
		// start from 0, no seek needed — reset the offset and the progress
		// display, then read this same response as a clean full
		// re-download. Truncate goes through the path, not the handle:
		// O_APPEND handles are opened with FILE_APPEND_DATA on Windows and
		// reject SetEndOfFile with "Access is denied" (same approach as
		// the downloadTask Range-ignored branch).
		if offset > 0 && resp.StatusCode == http.StatusOK {
			if err := os.Truncate(tmpPath, 0); err != nil {
				resp.Body.Close()
				closeTmp()
				return tmpPath, fmt.Errorf(tr("重置临时文件失败: %w", "failed to reset temporary file: %w"), err)
			}
			offset = 0
			downloadMu.Lock()
			downloadState.Downloaded = baseDownloaded
			downloadMu.Unlock()
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
				closeTmp()
				break // breaks inner for, outer for reopens the file and re-establishes
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
				closeTmp()
				return tmpPath, ctx.Err()
			case rr = <-ch:
			}

			if rr.n > 0 {
				if _, writeErr := tmpFile.Write(buf[:rr.n]); writeErr != nil {
					resp.Body.Close()
					closeTmp()
					return tmpPath, writeErr
				}
				downloaded += int64(rr.n)

				downloadMu.Lock()
				downloadState.Downloaded = baseDownloaded + downloaded
				if downloadState.Total > 0 {
					// progress computed from cumulative bytes including the
					// base offset, monotonically non-decreasing across assets
					downloadState.Progress = int(float64(baseDownloaded+downloaded) * 100 / float64(downloadState.Total))
				}
				downloadMu.Unlock()
			}
			if rr.err == io.EOF {
				resp.Body.Close()
				// A clean EOF before the declared size means a truncated
				// body (e.g. a proxy cutting the stream exactly at a chunk
				// boundary): never return the partial file as success — a
				// corrupt zip would only blow up later at extraction with
				// a confusing error. Auto-resume with a Range request from
				// the bytes already on disk; give up after maxTruncResumes
				// retries with a clear error. A Content-Length smaller than
				// the remaining asset size ends up here too (same
				// truncated-body handling).
				if totalSize > 0 && downloaded < totalSize {
					truncResumes++
					if truncResumes > maxTruncResumes {
						closeTmp()
						return tmpPath, fmt.Errorf(tr("下载不完整: 已下载 %d / %d 字节", "incomplete download: got %d of %d bytes"), downloaded, totalSize)
					}
					closeTmp()
					break // back to the outer loop: reopen the file, Range resume from the current file size
				}
				closeTmp()
				return tmpPath, nil
			}
			if rr.err != nil {
				resp.Body.Close()
				closeTmp()
				// Mid-body read failures (connection reset, stream errors) are
				// transient: reconnect and resume from the bytes on disk.
				if rr.err != io.EOF && ctx.Err() == nil && retries < downloadRetryCount {
					retries++
					log.Printf("[WARN] llama.cpp download stream failed (%v), retrying %d/%d", rr.err, retries, downloadRetryCount)
					if sleepDownloadRetry(ctx) {
						break // back to the outer loop: reopen + Range resume
					}
					return tmpPath, ctx.Err()
				}
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

// fetchLatestRelease fetches the latest llama.cpp release from the default API
// URL. The request is bound to ctx so a user cancel aborts an in-flight fetch
// immediately (the click otherwise appears dead until the HTTP timeout).
func fetchLatestRelease(ctx context.Context) (*GitHubRelease, error) {
	return fetchLatestReleaseAt(ctx, githubReleasesAPI)
}

// fetchLatestReleaseAt fetches and decodes a GitHub-style latest release JSON
// document from the given URL. The URL is injectable so tests can use a local
// httptest server instead of hitting the network.
func fetchLatestReleaseAt(ctx context.Context, apiURL string) (*GitHubRelease, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", apiURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("User-Agent", appUserAgent())

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

// fetchReleaseListAt fetches and decodes a GitHub-style release list document
// (newest-first, including prereleases) from the given URL; the URL is
// injectable for tests, mirroring fetchLatestReleaseAt.
func fetchReleaseListAt(ctx context.Context, apiURL string) ([]GitHubRelease, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", apiURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("User-Agent", appUserAgent())

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("GitHub API returned status %d", resp.StatusCode)
	}

	var list []GitHubRelease
	if err := json.NewDecoder(resp.Body).Decode(&list); err != nil {
		return nil, err
	}
	return list, nil
}

// newestReleaseWithAssets returns the newest release of a newest-first list
// that carries a main-asset candidate for the current platform, or nil when
// no listed release has binaries for this host.
func newestReleaseWithAssets(list []GitHubRelease) *GitHubRelease {
	for i := range list {
		if pickBestAsset(list[i].Assets) != nil {
			return &list[i]
		}
	}
	return nil
}

// pickBestAsset picks the most suitable release asset for the current platform.
// The CUDA floor is derived from the GPU compute capability (Blackwell needs
// CUDA >= 12.8); a probe failure or pre-Blackwell GPU yields no floor.
func pickBestAsset(assets []GitHubAsset) *GitHubAsset {
	floor := 0.0
	if cc, ok := gpuComputeCap(); ok {
		floor = cudaFloorForComputeCap(cc)
	}
	return pickBestAssetFor(assets, runtime.GOOS, runtime.GOARCH, len(getGPUInfo()) > 0, cudaVersionFromToolkit(), floor)
}

// cudaVersionFromToolkit derives the "major.minor" version used in asset
// naming (e.g. "12.4") from the local CUDA Toolkit version (nvcc output, e.g.
// "12.4.131"); returns an empty string with no Toolkit or on parse failure.
// Used only by pickBestAssetFor's exact-version bonus; the cudart runtime
// pairing derives its version from the chosen main asset name instead.
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

// archKeyOf maps a GOARCH value to the arch tag used in llama.cpp release
// asset names ("x64" / "arm64"); empty when unmapped. Shared by
// pickBestAssetFor and the cudart pairing in downloadLlamaCpp.
func archKeyOf(goarch string) string {
	switch goarch {
	case "amd64":
		return "x64"
	case "arm64":
		return "arm64"
	}
	return ""
}

// cudaVerRe matches the CUDA version tag embedded in asset names, e.g. the
// "12.4" in "llama-b*-bin-win-cuda-12.4-x64.zip".
var cudaVerRe = regexp.MustCompile(`cuda-(\d+\.\d+)`)

// cudaVerTagOf extracts the CUDA version tag from an asset name as a string
// (e.g. "12.4"); ok=false when the name carries no cuda-<major.minor> tag.
func cudaVerTagOf(name string) (string, bool) {
	m := cudaVerRe.FindStringSubmatch(strings.ToLower(name))
	if m == nil {
		return "", false
	}
	return m[1], true
}

// cudaVerOf parses the asset's CUDA version tag as a float for ordering
// comparisons (e.g. 12.4); ok=false when the tag is absent or unparseable.
func cudaVerOf(name string) (float64, bool) {
	tag, ok := cudaVerTagOf(name)
	if !ok {
		return 0, false
	}
	v, err := strconv.ParseFloat(tag, 64)
	if err != nil {
		return 0, false
	}
	return v, true
}

// pickBestAssetFor scores release assets for a given platform/arch and returns
// the best match. hasCUDA and cudaVer allow preferring matching CUDA builds on
// Windows; cudaFloor is the minimum CUDA version the GPU can run (Blackwell
// compute capability >= 12.0 needs >= 12.8; 0 means no constraint) — cuda
// assets below the floor are skipped entirely because the hardware cannot run
// them. Returns nil when no asset matches the platform.
//
// Matching rules:
//   - Platform: "win" for Windows, "macos" for macOS; Linux accepts both the
//     "ubuntu" keyword (current upstream naming, e.g. llama-b*-bin-ubuntu-x64)
//     and the legacy "linux" keyword, so a future rename back keeps working.
//   - Arch: enforced for every platform via the arch tag in the asset name
//     ("x64" / "arm64"); assets with no tag are accepted only on x64 hosts
//     (historical implicit-x64 naming). This drops wrong-arch builds such as
//     win-cpu-arm64 on x64 hosts, and ubuntu-s390x (no tag) / android-arm64
//     (wrong tag) everywhere they do not belong.
//   - Windows without an NVIDIA GPU: the "-cpu-" build gets a decisive bonus
//     over rocm/sycl/openvino/cuda; legacy "avx2"-style names keep their bonus
//     for releases predating the "-cpu-" naming.
//   - Windows with an NVIDIA GPU: among cuda builds the toolkit exact match
//     wins (+50); the version tie-break prefers the lowest available version
//     (widest GPU compatibility — CUDA 13 dropped pre-Turing support), or the
//     highest version >= floor when a floor is active (newest GPUs need the
//     newest runtime).
//   - Linux with an NVIDIA GPU: ubuntu-vulkan is the only GPU-accelerated
//     Linux build (no ubuntu cuda variant exists) and wins decisively.
//
// On Windows, cudart runtime assets are skipped (the main program and runtime
// are downloaded separately; the runtime is matched by pickCudartAssetFor).
func pickBestAssetFor(assets []GitHubAsset, platform, arch string, hasCUDA bool, cudaVer string, cudaFloor float64) *GitHubAsset {
	if len(assets) == 0 {
		return nil
	}

	// Map GOOS to release naming conventions
	platformKey := ""
	switch platform {
	case "windows":
		platformKey = "win"
	case "darwin":
		platformKey = "macos"
	case "linux":
		platformKey = "ubuntu"
	}
	archKey := archKeyOf(arch)

	matchesPlatform := func(name string) bool {
		if platformKey == "ubuntu" {
			return strings.Contains(name, "ubuntu") || strings.Contains(name, "linux")
		}
		return strings.Contains(name, platformKey)
	}

	// Arch-tag rule: the asset must carry the host's arch tag, or no tag at
	// all on x64 hosts (historical implicit-x64 naming).
	matchesArch := func(name string) bool {
		hasX64 := strings.Contains(name, "x64")
		hasArm64 := strings.Contains(name, "arm64")
		if hasArm64 && archKey != "arm64" {
			return false
		}
		if hasX64 && archKey != "x64" {
			return false
		}
		return hasX64 || hasArm64 || archKey == "x64"
	}

	isMainCandidate := func(name string) bool {
		// Skip cudart runtime assets: since llama.cpp b10342, Windows CUDA
		// builds are split into a main-program zip and a separate cudart
		// runtime zip; both contain "win-cuda" and would score equally, and
		// cudart is listed earlier in the release — without exclusion only the
		// runtime would be picked and the main program lost (extraction
		// artifacts would contain only runtime DLLs like cudart64_12.dll, no
		// llama-server.exe). The runtime is matched separately by
		// pickCudartAssetFor and downloaded alongside the main program.
		return !strings.HasPrefix(name, "cudart") && matchesPlatform(name) && matchesArch(name)
	}

	// Precompute the preferred CUDA version among surviving windows cuda
	// candidates: lowest when no floor is active (widest GPU compatibility),
	// highest when a floor is active (newest GPUs need the newest runtime).
	var targetCuda float64
	var hasTargetCuda bool
	if platformKey == "win" && hasCUDA {
		for i := range assets {
			name := strings.ToLower(assets[i].Name)
			if !isMainCandidate(name) || !strings.Contains(name, "cuda") {
				continue
			}
			v, ok := cudaVerOf(name)
			if !ok || (cudaFloor > 0 && v < cudaFloor) {
				continue
			}
			if !hasTargetCuda ||
				(cudaFloor > 0 && v > targetCuda) ||
				(cudaFloor <= 0 && v < targetCuda) {
				targetCuda = v
				hasTargetCuda = true
			}
		}
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
		if !isMainCandidate(name) {
			continue
		}

		score := 0

		// Prefer CUDA builds on Windows when GPU is available
		if platformKey == "win" && hasCUDA && strings.Contains(name, "cuda") {
			v, hasVer := cudaVerOf(name)
			if hasVer && cudaFloor > 0 && v < cudaFloor {
				// Below the GPU's usable CUDA floor: hardware cannot run it
				continue
			}
			score += 100
			if hasVer && hasTargetCuda && v == targetCuda {
				score += 30 // Preferred version (see targetCuda computation)
			}
			// Match CUDA version — prefer exact match to installed toolkit
			if cudaVer != "" && strings.Contains(name, "cuda-"+cudaVer) {
				score += 50
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

		if platformKey == "win" && !strings.Contains(name, "cuda") {
			if strings.Contains(name, "-cpu-") {
				// Decisive bonus: the plain CPU build beats rocm/sycl/openvino
				score += 40
			} else if !strings.Contains(name, "avx") && !strings.Contains(name, "vulkan") && !strings.Contains(name, "opencl") {
				// Basic win-x64 build without extra suffixes (legacy naming)
				score += 10
			}
		}

		// Generic non-win scoring: plain distro build outranks specialized
		// variants (vulkan/sycl/openvino); kleidiai builds stay excluded; an
		// explicit arch tag outranks untagged oddities (ubuntu-s390x carries
		// no x64/arm64 tag).
		if platformKey != "win" && !strings.Contains(name, "kleidiai") {
			score += 10
			if !strings.Contains(name, "vulkan") && !strings.Contains(name, "sycl") && !strings.Contains(name, "openvino") {
				score += 10
			}
			if strings.Contains(name, "x64") || strings.Contains(name, "arm64") {
				score += 5
			}
			// NVIDIA GPU present: ubuntu-vulkan is the only GPU-accelerated
			// Linux build (no ubuntu cuda variant exists)
			if hasCUDA && strings.Contains(name, "vulkan") {
				score += 80
			}
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

// pickCudartAssetFor returns the cudart runtime asset pairing with the chosen
// main CUDA asset. cudaVer and arch must be derived from the main asset name
// (cudaVerTagOf / archKeyOf), not from the local nvcc toolkit: the runtime DLLs
// must match the build actually downloaded, which floor/tie-break selection
// may pick independently of the installed toolkit. It matches
// cudart-llama-bin-win-cuda-<cudaVer>-<arch>.zip case-insensitively. With an
// empty cudaVer it skips exact-version matching and falls back to the first
// cudart asset with a matching arch (best-effort when the main asset carries
// no parseable cuda version but still needs the runtime to launch).
// This function does not check the platform: whether to attach the runtime is
// decided by the caller (the Windows+CUDA check in downloadLlamaCpp), letting
// tests construct cudart assets directly and assert matching across platforms.
func pickCudartAssetFor(assets []GitHubAsset, cudaVer, arch string) *GitHubAsset {
	for i := range assets {
		a := &assets[i]
		lower := strings.ToLower(a.Name)
		if !strings.HasPrefix(lower, "cudart-llama-bin-win-cuda-") {
			continue
		}
		// The runtime arch tag must pair with the main asset (x64 / arm64)
		if arch != "" && !strings.Contains(lower, arch) {
			continue
		}
		if cudaVer == "" {
			return a
		}
		if arch != "" && lower == "cudart-llama-bin-win-cuda-"+cudaVer+"-"+arch+".zip" {
			return a
		}
	}
	return nil
}

// Extraction size caps (declared as vars so tests can shrink them for
// verification). Prevent zip/tar extraction bombs from filling the disk or
// exhausting memory (#2).
var maxExtractFileSize int64 = 4 << 30   // per-file extraction cap: 4GB
var maxExtractTotalSize int64 = 16 << 30 // per-run total extraction cap: 16GB

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

		// Reject oversized files up front by declared size, instead of
		// writing them out first and discovering the limit afterwards
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

		// io.CopyN copies at most maxExtractFileSize+1 bytes: a src exactly at
		// the cap returns (max, io.EOF), a src beyond the cap returns
		// (max+1, nil); hence the n > maxExtractFileSize over-limit check (#2).
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
			// Reject oversized files up front by declared entry size (#2)
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
			// Explicitly reject symlinks/hardlinks/device files and other
			// unknown types, avoiding silently skipped entries that would
			// leave an incomplete extraction or a potential security issue (#6)
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

// currentVersion is the current app version, aligned with GitHub release tags
// (e.g. v0.1.0). The version comes from the core/VERSION file (embedded at
// compile time, similar to the frontend .env); bump that file and tag the same
// name when releasing.
var currentVersion = strings.TrimSpace(string(versionFile))

// appUserAgent returns the User-Agent sent with every outbound HTTP request
// (GitHub API, HF mirror, ModelScope, model downloads). It carries the app
// name, the current version and the repository URL so recipients can
// attribute the traffic to this project.
func appUserAgent() string {
	return "llama-cpp-desktop/" + currentVersion + " (+https://github.com/CodeNeow/llama-cpp-desktop)"
}

// updateRepoAPI points to this repository's latest release API. The URL is
// received by CheckForUpdateAt to support test injection of a local httptest
// server. Declared as a var so tests can replace the package-level variable to
// simulate the network (same style as configFile / renameFile).
var updateRepoAPI = "https://api.github.com/repos/CodeNeow/llama-cpp-desktop/releases/latest"

// compareVersions compares two version strings like v1.2.3 (leading v / V
// ignored). Returns -1 when a < b, 0 when equal, 1 when a > b; unparseable
// segments are treated as 0.
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

// UpdateCheckResult is the update-check result returned to the frontend to
// decide whether a new version exists.
type UpdateCheckResult struct {
	HasUpdate bool   `json:"hasUpdate"`
	Version   string `json:"version"`   // latest version (tag name, e.g. v0.1.1)
	Notes     string `json:"notes"`     // release notes
	Published string `json:"published"` // publish time
}

// CheckForUpdateAt requests the latest release of the given repository and
// compares versions. apiURL is injectable so tests can use httptest instead
// of the real network.
func CheckForUpdateAt(apiURL string) (*UpdateCheckResult, error) {
	release, err := fetchLatestReleaseAt(context.Background(), apiURL)
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

// updateDownloadState tracks the progress of the app update download
// (updating the exe). State machine values: idle / downloading / installing /
// done / error; "installing" means the downloaded setup installer has been
// launched and the app is about to exit (installUpdateNow).
type UpdateDownloadState struct {
	Status     string `json:"status"`
	Progress   int    `json:"progress"`
	Total      int64  `json:"total"`
	Downloaded int64  `json:"downloaded"`
	Version    string `json:"version"`
	FilePath   string `json:"filePath"`
	Error      string `json:"error"`
	Kind       string `json:"kind"` // install kind of the running app: setup (NSIS install) / portable
	// Installer reports whether the downloaded artifact is the setup
	// installer (a portable install may fall back to the installer asset);
	// only installer artifacts support the install-now flow.
	Installer bool `json:"installer"`
}

var updateDownloadState = &UpdateDownloadState{Status: "idle"}
var updateDownloadMu sync.Mutex
var updateDownloadCancel context.CancelFunc

// updateExePath is a test injection point (same style as renameFile /
// configFile) returning the current executable path, used to determine the
// target directory for the update exe.
var updateExePath = os.Executable

// updateLauncher is a test injection point (same style as renameFile /
// updateExePath) launching the downloaded setup installer as a detached child
// process. On Windows the installer needs elevation (UAC), so launchInstaller
// goes through ShellExecute runas instead of a plain exec.
var updateLauncher = func(path string) error {
	return launchInstaller(path)
}

// updateQuitDelay is the pause between launching the installer and quitting
// the app: it gives the frontend time to resolve the InstallUpdate call and
// render the exiting state before the window closes. Var so tests can zero it.
var updateQuitDelay = 500 * time.Millisecond

// Install-kind constants: setup is the NSIS installer build (downloads the
// setup installer), portable is the portable build (downloads the portable
// exe). Used for update artifact selection and distinguishing frontend hints.
const (
	installKindSetup    = "setup"
	installKindPortable = "portable"
)

// detectInstallKind detects the current install type: a setup install is done
// by NSIS and the install directory always contains uninstall.exe; a portable
// install is a green build with no uninstall.exe. Pure filesystem detection,
// cross-platform. Reuses updateExePath (the os.Executable test injection point)
// to stay testable.
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

// pickUpdateAsset picks the update-download asset by install kind, matching
// by keyword (independent of artifact prefix), compatible with three naming
// generations:
//   - Current naming: setup installer llama-desktop-setup-vX.Y.Z-windows-amd64.exe
//     (portable builds are no longer published);
//   - Old naming (since v0.1.7): llama-gui-setup- / llama-gui-portable- prefixes;
//   - Oldest naming (v0.1.6): installer llama-gui-amd64-installer.exe,
//     portable llama-gui.exe.
//
// setup returns the first installer asset (name contains installer or setup);
// portable returns the first asset containing portable or any non-installer
// exe (the oldest llama-gui.exe contains none of portable/installer/setup and
// hits the "non-installer" branch). Portable builds are no longer published:
// existing portable installs update to the setup installer going forward, so
// when no portable/non-installer exe matches, the portable branch falls back
// to the first installer asset instead of failing.
func pickUpdateAsset(assets []GitHubAsset, kind string) *GitHubAsset {
	var firstInstaller *GitHubAsset
	for i := range assets {
		a := &assets[i]
		name := strings.ToLower(a.Name)
		if !strings.HasSuffix(name, ".exe") {
			continue
		}
		isInstaller := strings.Contains(name, "installer") || strings.Contains(name, "setup")
		if isInstaller && firstInstaller == nil {
			firstInstaller = a
		}
		switch kind {
		case installKindSetup:
			if isInstaller {
				return a
			}
		default: // installKindPortable (unknown values fall back to portable semantics)
			if strings.Contains(name, "portable") || !isInstaller {
				return a
			}
		}
	}
	if kind != installKindSetup {
		// Portable builds are no longer published: releases ship only the
		// setup installer, so existing portable installs fall back to it.
		return firstInstaller
	}
	return nil
}

// downloadUpdateRelease downloads the artifact matching the current install
// kind to the executable's directory: a setup install downloads the installer,
// a portable install downloads the portable exe (the running exe cannot be
// replaced directly); when the release ships no portable exe (portable builds
// are retired), a portable install falls back to the setup installer. The
// saved file is named by the selected asset type (setup installer / portable
// exe), not by the local install kind, so the fallback download keeps the
// setup filename. When finished, the user is prompted to close the app and
// complete the update per install kind.
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

	// Detect the install kind before the download starts: it decides which
	// asset to pick and the install kind reported to the frontend (the saved
	// filename follows the selected asset type, see Step 2).
	kind := detectInstallKind()

	// Step 1: fetch the latest release info and pick the matching exe asset by install kind
	updateDownloadMu.Lock()
	updateDownloadState.Status = "downloading"
	updateDownloadState.Progress = 0
	updateDownloadState.Downloaded = 0
	updateDownloadState.Total = 0
	updateDownloadState.Version = version
	updateDownloadState.Error = ""
	updateDownloadState.Kind = kind
	updateDownloadState.Installer = false
	updateDownloadMu.Unlock()

	release, err := fetchLatestReleaseAt(context.Background(), updateRepoAPI)
	if err != nil {
		setUpdateDownloadError(tr("获取发布信息失败: ", "Failed to fetch release info: ") + err.Error())
		return
	}
	asset := pickUpdateAsset(release.Assets, kind)
	if asset == nil {
		setUpdateDownloadError(tr("未找到适用于当前平台的主程序", "No main executable found for the current platform"))
		return
	}

	// Whether the picked asset is the setup installer decides the saved
	// filename (Step 2) and the Installer flag reported to the frontend (the
	// install-now flow only applies to installer artifacts).
	assetName := strings.ToLower(asset.Name)
	isInstallerAsset := strings.Contains(assetName, "setup") || strings.Contains(assetName, "installer")

	updateDownloadMu.Lock()
	updateDownloadState.Total = asset.Size
	updateDownloadState.Installer = isInstallerAsset
	updateDownloadMu.Unlock()

	// Step 2: download into the executable's directory, named by the selected
	// asset type (not the local install kind): installer assets (name contains
	// setup / installer) → llama-desktop-setup-v<tag>.exe, anything else →
	// llama-desktop-portable-v<tag>.exe. Non-fallback paths pick by kind, so
	// their filenames are unchanged; only the portable→installer fallback
	// switches to the setup name, keeping the filename honest about content.
	exePath, err := updateExePath()
	if err != nil {
		setUpdateDownloadError(tr("无法定位可执行文件路径: ", "Unable to locate the executable path: ") + err.Error())
		return
	}
	dir := filepath.Dir(exePath)
	var fileName string
	if isInstallerAsset {
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

	// Step 3: move to the destination path (across devices moveFile falls back
	// to copy, preserving source permissions; on non-cross-device failure with
	// an existing destination, delete the old file first and retry)
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

// installUpdateNow launches the downloaded setup installer and then quits the
// app (via quit) so the installer can replace the program files without the
// user closing anything manually. Guards: only a completed download (status
// done) whose artifact is the setup installer (Installer) and whose file still
// exists can be installed. The status moves to "installing" before launching,
// which both tells the frontend the app is exiting and rejects a double click
// (this function requires status done). A launch failure restores status
// "done" so the user can retry from the update modal.
func installUpdateNow(quit func()) error {
	updateDownloadMu.Lock()
	status := updateDownloadState.Status
	installer := updateDownloadState.Installer
	filePath := updateDownloadState.FilePath
	updateDownloadMu.Unlock()

	if status != "done" {
		return errors.New(tr("更新尚未完成，无法安装", "update download is not finished; cannot install"))
	}
	if !installer {
		return errors.New(tr("下载的文件不是安装器，请手动完成更新", "the downloaded file is not the installer; finish the update manually"))
	}
	if _, err := os.Stat(filePath); err != nil {
		return fmt.Errorf(tr("找不到安装器文件: %w", "cannot find the installer file: %w"), err)
	}

	updateDownloadMu.Lock()
	updateDownloadState.Status = "installing"
	updateDownloadMu.Unlock()

	if err := updateLauncher(filePath); err != nil {
		updateDownloadMu.Lock()
		updateDownloadState.Status = "done"
		updateDownloadMu.Unlock()
		return fmt.Errorf(tr("启动安装器失败: %w", "failed to launch the installer: %w"), err)
	}
	log.Printf("[OK] update installer launched (%s), quitting app", filePath)

	// Quit from a goroutine after a short delay so the InstallUpdate binding
	// call resolves and the frontend can render the exiting state first.
	go func() {
		time.Sleep(updateQuitDelay)
		quit()
	}()
	return nil
}

// downloadUpdateWithResume downloads the update file to a temp file and
// reports progress, supporting cancellation. Unlike downloadWithResume: the
// update exe is small and does not support pause/resume; it only responds to
// context cancellation (app exit / stop download). Transient failures
// (network errors, HTTP 429/5xx, mid-stream errors) are retried internally
// up to downloadRetryCount times (each attempt restarts clean); the error is
// surfaced for a manual retry only after the retries are exhausted.
func downloadUpdateWithResume(ctx context.Context, url string, totalSize int64) (string, error) {
	tmpFile, err := os.CreateTemp("", "llama-desktop-update-*.exe")
	if err != nil {
		return "", fmt.Errorf(tr("创建临时文件失败: %w", "failed to create temporary file: %w"), err)
	}
	tmpPath := tmpFile.Name()

	retries := 0
	for {
		// Each attempt restarts clean: truncate and rewind the handle (no
		// Range resume for updates) and reset the progress display.
		if err := tmpFile.Truncate(0); err != nil {
			tmpFile.Close()
			return tmpPath, err
		}
		if _, err := tmpFile.Seek(0, 0); err != nil {
			tmpFile.Close()
			return tmpPath, err
		}
		updateDownloadMu.Lock()
		updateDownloadState.Downloaded = 0
		updateDownloadMu.Unlock()

		req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
		if err != nil {
			tmpFile.Close()
			return tmpPath, err
		}
		req.Header.Set("User-Agent", appUserAgent())

		client := &http.Client{Timeout: 30 * time.Minute}
		resp, err := client.Do(req)
		if err != nil {
			if ctx.Err() == nil && retries < downloadRetryCount {
				retries++
				log.Printf("[WARN] update download attempt failed (%v), retrying %d/%d", err, retries, downloadRetryCount)
				if sleepDownloadRetry(ctx) {
					continue
				}
			}
			tmpFile.Close()
			return tmpPath, err
		}

		if resp.StatusCode != http.StatusOK {
			status := resp.StatusCode
			resp.Body.Close()
			if retryableDownloadStatus(status) && ctx.Err() == nil && retries < downloadRetryCount {
				retries++
				log.Printf("[WARN] update download got HTTP %d, retrying %d/%d", status, retries, downloadRetryCount)
				if sleepDownloadRetry(ctx) {
					continue
				}
			}
			tmpFile.Close()
			return tmpPath, fmt.Errorf("HTTP %d", status)
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
				updateDownloadMu.Lock()
				updateDownloadState.Downloaded = downloaded
				if updateDownloadState.Total > 0 {
					updateDownloadState.Progress = int(float64(downloaded) * 100 / float64(updateDownloadState.Total))
				}
				updateDownloadMu.Unlock()
			}
			if rr.err != nil {
				resp.Body.Close()
				if rr.err == io.EOF {
					tmpFile.Close()
					return tmpPath, nil
				}
				// Mid-body stream failures are transient: restart the
				// download (clean truncate) up to downloadRetryCount times.
				if ctx.Err() == nil && retries < downloadRetryCount {
					retries++
					log.Printf("[WARN] update download stream failed (%v), retrying %d/%d", rr.err, retries, downloadRetryCount)
					if sleepDownloadRetry(ctx) {
						break // back to the attempt loop
					}
				}
				tmpFile.Close()
				return tmpPath, rr.err
			}
		}
	}
}

// ─── llama-server manager ────────────────────────────────────────

var serverCmd *exec.Cmd
var serverLogs []string
var serverLogsMu sync.Mutex
var serverRunning bool

// serverMu guards the full lifecycle of serverCmd and serverRunning
// (create/start/stop/cleanup), separate in responsibility from serverLogsMu
// which only guards serverLogs (#3). Any path holding both locks must acquire
// them in the order "serverMu first, then serverLogsMu" to avoid deadlock.
var serverMu sync.Mutex

// serverStartTime records when llama-server started successfully (guarded by
// serverMu), used by GetMonitorStatus to compute uptime; zeroed when the
// process exits (in the cmd.Wait goroutine).
var serverStartTime time.Time

// serverPort records the port used by the successfully started llama-server
// (guarded by serverMu), 0 means not running. Router API queries use this
// value instead of the current config, so editing the config mid-run cannot
// redirect queries to the wrong address.
var serverPort int

// serverLogWriter reassembles child-process stdout/stderr writes into whole
// lines before they enter the ring log, preventing log entries from being cut
// in half by arbitrary chunks. Previously Write treated each arbitrary stderr
// chunk as one log entry (addServerLog(strings.TrimSpace(string(p)))):
// llama-server writes in small chunks, so a print_timing line could be split
// across multiple Writes — user-pasted logs showed "0.00.136.078" appearing as
// a standalone entry, and fragments like "( 0.63 ms per token, 2362.80 tokens
// per second)" reduced to the second half; the latter no longer contains the
// "prompt eval time" marker, so parseTPS cannot classify it as a prefill line
// and long-prompt prefill speeds like 2362.80 leaked into TPS. Line buffering
// makes addServerLog always receive complete lines, eliminating truncation at
// the root (the log line-splitting and the TPS misreading share the same
// cause).
//
// Each instance holds its own buffer and mutex (the project's "explicit mutex"
// convention); in bridge.go, cmd.Stdout and cmd.Stderr each get their own
// instance buffering its own stream without interference.
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
			// No trailing newline: ReadString has already consumed the whole
			// buffer and line is an unfinished fragment. Reset and write the
			// fragment back so the next Write completes the line; with no
			// fragment just reset, preventing the buffer from growing without
			// bound on consumed bytes.
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

// ─── Config persistence ─────────────────────────────────────────

type appConfig struct {
	LlamaCppDir         string                 `json:"llamaCppDir"`
	ModelDir            string                 `json:"modelDir"`
	LlamaCppDownloadDir string                 `json:"llamaCppDownloadDir,omitempty"`
	ModelDownloadDir    string                 `json:"modelDownloadDir,omitempty"`
	Theme               string                 `json:"theme"`
	ModelConfigs        map[string]ModelConfig `json:"modelConfigs"`
	ServerConfig        ServerConfig           `json:"serverConfig"`
	DownloadSource      string                 `json:"downloadSource"`
	Language            string                 `json:"language"`         // language preference: zh / en / auto (empty or invalid falls back to auto)
	TrayEnabled         bool                   `json:"trayEnabled"`      // Windows system tray toggle, default true
	SidebarCollapsed    bool                   `json:"sidebarCollapsed"` // sidebar collapsed state, default true (collapsed)
	// OnboardingDismissed records that the user closed (or auto-completed) the
	// Home page quick-start checklist. False is the Go zero value, so old
	// configs missing the field fall back to false (checklist shown) naturally.
	OnboardingDismissed bool `json:"onboardingDismissed"`
	// ApiRouteMode is the API-route (headless) mode toggle, default false:
	// when true, the next app start skips the GUI and runs as tray +
	// llama-server only (Windows; see core/headless.go). False is the Go zero
	// value, so old configs missing the field fall back to false naturally.
	ApiRouteMode  bool              `json:"apiRouteMode"`
	DownloadTasks []PersistedDlTask `json:"downloadTasks,omitempty"`
}

// PersistedDlTask is the persisted form of download queue tasks (written to
// llama-desktop-config.json). Differs from DlTask: runtime state such as URL /
// ctx / cancel / resumeCh is not persisted; the URL is rebuilt on loadConfig
// restore from Source + buildModelDownloadURL.
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

// Service access scope values: local means reachable only from this machine
// (listen on 127.0.0.1), lan means reachable from devices on the same network
// (listen on 0.0.0.0). ServerConfig.AccessMode accepts only these two values;
// anything else (including empty) falls back to local.
const accessLocal = "local"
const accessLAN = "lan"

type ServerConfig struct {
	// AccessMode is the service access scope ("local" | "lan", default
	// "local"); Host is the derived actual listen address per AccessMode and
	// never takes direct user input.
	AccessMode string `json:"accessMode"`
	Host       string `json:"host"`
	Port       int    `json:"port"`
	MaxModels  int    `json:"maxModels"`
	CacheRAM   int    `json:"cacheRam"`
}

// effectiveHost derives the actual listen address from the access scope:
// lan → "0.0.0.0", any other value (including empty and invalid) →
// "127.0.0.1". Pure function shared by SaveServerConfig normalization,
// loadConfig compatibility, and buildServerCommand, keeping Host consistent
// everywhere.
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
	CPUMoe        bool    `json:"cpuMoe"`           // keep all MoE experts on CPU
	NCpuMoe       int     `json:"nCpuMoe"`          // keep first N MoE layers on CPU, 0=disabled
	SplitMode     string  `json:"splitMode"`        // "", none, layer, row, tensor
	TensorSplit   string  `json:"tensorSplit"`      // e.g. "3,1"
	MainGPU       int     `json:"mainGpu"`          // default 0
	RopeScaling   string  `json:"ropeScaling"`      // "", none, linear, yarn
	RopeScale     float64 `json:"ropeScale"`        // 0=disabled
	MMProj        string  `json:"mmproj"`           // explicit mmproj path override, empty=auto-detect
	Reasoning     bool    `json:"reasoning"`        // disable thinking (writes reasoning = off)
	SpecType      string  `json:"specType"`         // "", draft-mtp
	SpecDraftNMax int     `json:"specDraftNMax"`    // >0 writes spec-draft-n-max
	MLock         bool    `json:"mlock,omitempty"`  // deprecated, kept only to migrate old configs
	NoMMap        bool    `json:"noMmap,omitempty"` // deprecated, kept only to migrate old configs
}

// migrateLegacyConfig copies the legacy llama-gui-era config file content to
// the new filename: only when the new file does not exist and the old one
// does, it reads the old content and writes it to the new file (0644, matching
// the saveConfig write convention) — no deletion or renaming. The old file
// stays in place, unchanged.
// Copy instead of rename because: wails dev's file watcher watches the project
// root, and deleting/renaming root files during startup triggers a
// GetFileAttributesEx race in the Wails CLI that crashes the run; copying does
// not delete the source, the new file's existence short-circuits, and a
// leftover old file has no side effects — migration re-triggers only if the
// user deletes the new file. Failures only log a warning and fall back to
// loadConfig's defaults, never blocking startup.
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
	// Pre-populate defaults before Unmarshal: Go's zero value false cannot
	// distinguish "old config missing the field" from "explicitly set to
	// false". trayEnabled must default to true when absent (tray stays on
	// after historical config upgrades, matching 4aacac2's unconditional
	// tray); sidebarCollapsed must default to true when absent (sidebar
	// collapses by default with no saved preference).
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
	// llama.cpp download path: empty values fall back to the default
	// llama-cpp/ directory (no existence check — a fresh path is a valid
	// target for the next download).
	if cfg.LlamaCppDownloadDir != "" {
		llamaCppDownloadDirMu.Lock()
		llamaCppDownloadDirOverride = cfg.LlamaCppDownloadDir
		llamaCppDownloadDirMu.Unlock()
		log.Printf("[DIR] Loaded llama.cpp download dir from config: %s", cfg.LlamaCppDownloadDir)
	}
	// Model download path: empty values fall back to the default LLM-Models
	// directory (no existence check — a fresh path is a valid target for the
	// next model download).
	if cfg.ModelDownloadDir != "" {
		modelDownloadDirMu.Lock()
		modelDownloadDirOverride = cfg.ModelDownloadDir
		modelDownloadDirMu.Unlock()
		log.Printf("[DIR] Loaded model download dir from config: %s", cfg.ModelDownloadDir)
	}
	// Imported model directory: empty values or paths that do not exist / are
	// not directories are ignored and fall back to the default directory,
	// preventing scans/downloads from landing on invalid paths after config
	// corruption or directory deletion.
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
	// Migrate legacy mlock/noMmap to load-mode (both DEPRECATED since b10342):
	// if an old config has no explicit loadMode, derive it from the old boolean
	// combination and clear the compatibility fields; omitempty in saveConfig
	// guarantees the old keys are never written back (gradual cleanup).
	for k, c := range cachedModelConfigs {
		if c.LoadMode == "" && (c.MLock || c.NoMMap) {
			switch {
			case c.MLock && c.NoMMap:
				c.LoadMode = "mlock" // mlock semantics take priority
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
	// Access scope: empty values or anything outside the {local,lan} whitelist
	// fall back to local (no error when old configs lack accessMode or data is
	// corrupt). Host is always derived by effectiveHost from accessMode; a
	// possibly-invalid host value in old configs is never trusted (extending
	// the #5 defense).
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

	// Download source: empty or invalid values fall back to the default hf
	// (no error when old configs lack this field or data is corrupt).
	if cfg.DownloadSource != sourceHF && cfg.DownloadSource != sourceHuggingFace && cfg.DownloadSource != sourceModelScope {
		cfg.DownloadSource = defaultDownloadSource
	}
	downloadSourceMu.Lock()
	downloadSource = cfg.DownloadSource
	downloadSourceMu.Unlock()

	// Language preference: empty values or anything outside the zh/en/auto
	// whitelist fall back to auto (no error when old configs lack this field
	// or data is corrupt). Same strategy as downloadSource: invalid values are
	// always normalized back to the default.
	if cfg.Language != "zh" && cfg.Language != "en" && cfg.Language != "auto" {
		cfg.Language = "auto"
	}
	languageMu.Lock()
	currentLanguage = cfg.Language
	languageMu.Unlock()

	// System tray toggle: keep the pre-populated default true when the field is
	// missing; only an explicit false disables it (tray stays on after old
	// config upgrades, matching 4aacac2's unconditional tray behavior).
	// API-route mode: Go zero value false is already the intended default for
	// configs missing the field, no pre-population needed (unlike trayEnabled).
	configMu.Lock()
	trayEnabled = cfg.TrayEnabled
	apiRouteMode = cfg.ApiRouteMode
	configMu.Unlock()

	// Sidebar collapsed state: keep the pre-populated default true when the
	// field is missing (collapsed, see the appConfig pre-population above);
	// only an explicit false (user's expand preference) yields false, same
	// pattern as trayEnabled.
	configMu.Lock()
	currentSidebarCollapsed = cfg.SidebarCollapsed
	// Onboarding checklist: Go zero value false is already the intended
	// default for configs missing the field (checklist visible until the user
	// dismisses it or completes all steps), no pre-population needed.
	currentOnboardingDismissed = cfg.OnboardingDismissed
	configMu.Unlock()

	// Restore the download task queue (after a process restart there are no
	// active goroutines, so no task auto-starts its download): Source falls
	// back to hf; statuses outside the whitelist and downloading are all
	// normalized to paused (the downloading goroutine died with the process;
	// the frontend can offer resume/retry); URLs are rebuilt via
	// buildModelDownloadURL; resumeCh is a fresh buffered channel while
	// ctx/cancel stay nil (RetryDownloadTask rebuilds ctx before starting).
	// After restoring, bump dlTaskCounter to avoid id collisions with
	// existing tasks.
	restored := make([]*DlTask, 0, len(cfg.DownloadTasks))
	for _, pt := range cfg.DownloadTasks {
		src := pt.Source
		if src == "" {
			src = sourceHF
		}
		status := pt.Status
		switch status {
		case "done", "error", "cancelled", "queued", "paused":
			// terminal and controllable states stay as-is
		default:
			// empty, invalid, or downloading → paused
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
	// Bump the id counter to max restored sequence + 1 (parsing "dl-N") to
	// avoid id collisions with new tasks; keep the current value on parse
	// failure or when nothing was restored.
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

// trayEnabled indicates whether the Windows system tray is enabled (closing
// the window minimizes to tray), default true; guarded by configMu and
// persisted to the config file's trayEnabled field. When an old config lacks
// the field, loadConfig falls back to true (see the appConfig{TrayEnabled:
// true} pre-population in loadConfig).
var trayEnabled = true

// apiRouteMode indicates whether API-route (headless) mode is enabled:
// when true, the next app start skips the GUI (WebView2) and runs as the Go
// backend + system tray + llama-server only, keeping the OpenAI API alive
// with a much smaller footprint (Windows only, see core/headless.go).
// Default false; guarded by configMu and persisted to the config file's
// apiRouteMode field. Old configs missing the field fall back to false
// (Go zero value, no pre-population needed).
var apiRouteMode bool

// ApiRouteMode returns the current API-route (headless) mode preference
// (concurrency-safe, guarded by configMu). Used by ShouldRunHeadless when a
// process starts without an explicit --headless/--gui flag.
func ApiRouteMode() bool {
	configMu.Lock()
	defer configMu.Unlock()
	return apiRouteMode
}

// currentSidebarCollapsed indicates whether the sidebar is collapsed
// (icon-only rail), default true (collapsed); guarded by configMu and
// persisted to the config file's sidebarCollapsed field. When an old config
// lacks the field, loadConfig pre-populates the default true (see the
// appConfig pre-population in loadConfig), the same fallback pattern as
// trayEnabled.
var currentSidebarCollapsed = true

// currentOnboardingDismissed indicates whether the Home page quick-start
// checklist has been dismissed (manually closed or auto-completed); guarded
// by configMu and persisted to the config file's onboardingDismissed field.
// Default false: old configs lacking the field show the checklist.
var currentOnboardingDismissed = false

// TrayEnabled returns the current tray preference (concurrency-safe, guarded
// by configMu). Used by main.go's OnStartup to decide whether to start the
// tray per the persisted config.
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

	llamaCppDownloadDirMu.Lock()
	llamaDownloadDir := llamaCppDownloadDirOverride
	llamaCppDownloadDirMu.Unlock()

	modelDownloadDirMu.Lock()
	modelDownloadDir := modelDownloadDirOverride
	modelDownloadDirMu.Unlock()

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
	apiRoute := apiRouteMode
	onboardingDismissed := currentOnboardingDismissed
	configMu.Unlock()

	// Lock-ordering iron rule: inside saveConfig, dlTasksMu must be the last
	// lock acquired. No call site may call saveConfig while holding dlTasksMu —
	// callers must copy under the lock, unlock, then save (e.g.
	// CancelDownloadTask does not call saveConfig before its deferred Unlock).
	// Otherwise the global ordering between dlTasksMu and other locks
	// (configMu etc.) is violated, causing deadlock.
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
		LlamaCppDir:         dir,
		ModelDir:            modelDir,
		LlamaCppDownloadDir: llamaDownloadDir,
		ModelDownloadDir:    modelDownloadDir,
		Theme:               theme,
		ModelConfigs:        mcfgs,
		ServerConfig:        scfg,
		DownloadSource:      dlsrc,
		Language:            lang,
		TrayEnabled:         tray,
		SidebarCollapsed:    sidebarCollapsed,
		OnboardingDismissed: onboardingDismissed,
		ApiRouteMode:        apiRoute,
		DownloadTasks:       persistedTasks,
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
const hfDirectBase = "https://huggingface.co"

// activeHFBase returns the HF-compatible API base for the active non-ModelScope
// source: the official Hugging Face host for "huggingface", otherwise the
// hf-mirror.com mirror. Both expose identical Hub API paths, so the same
// request code serves either host.
func activeHFBase() string {
	if activeDownloadSource() == sourceHuggingFace {
		return hfDirectBase
	}
	return hfMirrorBase
}

// buildModelDownloadURL builds the model file download URL per download source:
//   - hf: {hfMirrorBase}/{modelID}/resolve/main/{fileName} (filename PathEscaped)
//   - huggingface: same path on the official Hugging Face host (hfDirectBase)
//   - modelscope: delegates to buildModelScopeDownloadURL (the legacy API repo endpoint)
//   - unknown source returns an error (defense in depth; callers must not pass invalid values)
func buildModelDownloadURL(source, modelID, fileName string) (string, error) {
	switch source {
	case sourceHF:
		return fmt.Sprintf("%s/%s/resolve/main/%s", hfMirrorBase, modelID, url.PathEscape(fileName)), nil
	case sourceHuggingFace:
		return fmt.Sprintf("%s/%s/resolve/main/%s", hfDirectBase, modelID, url.PathEscape(fileName)), nil
	case sourceModelScope:
		return buildModelScopeDownloadURL(modelscopeLegacyBase, modelID, fileName), nil
	default:
		return "", fmt.Errorf(tr("未知下载源 %q", "unknown download source %q"), source)
	}
}

// searchHFMirror queries the default HF Mirror endpoint.
func searchHFMirror(q string, filter string) ([]HFSearchResult, error) {
	return searchHFMirrorAt(activeHFBase(), q, filter)
}

// searchHFMirrorAt queries an HF-compatible API base for models matching q,
// filtering to models containing GGUF files. The filter parameter is
// deprecated, kept only for signature compatibility; no pipeline_tag type
// filtering happens anymore (embedding / llm classification was removed).
// The API supports neither library filtering nor pagination, so candidates
// are pulled with a large limit and then filtered for GGUF. To cover as many
// candidates as possible, three sorts — downloads / likes / lastModified —
// are requested in parallel (each limit=200&full=true), each filtered for
// GGUF and then merged and deduplicated by modelId in downloads → likes →
// lastModified order (already-seen entries skipped). A failed sort request
// only skips that route (with a [WARN]); an error is returned only when all
// three routes fail.
func searchHFMirrorAt(baseURL, q, filter string) ([]HFSearchResult, error) {
	sorts := []string{"downloads", "likes", "lastModified"}

	type routeResult struct {
		results []HFSearchResult
		err     error
	}
	routeResults := make([]routeResult, len(sorts))

	// Three sort routes fetched in parallel, each with its own result slice;
	// no shared writes
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
			log.Printf("[WARN] HF search sort %s request failed, skipping route: %v", sort, routeResults[i].err)
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

// searchHFMirrorSortAt fetches one page of candidates from an HF-compatible
// API with the given sort (limit=200&full=true), filtering for results with
// GGUF files. Request failures or non-200 statuses return an error;
// searchHFMirrorAt then decides to skip the whole route (other sorts are
// unaffected).
func searchHFMirrorSortAt(baseURL, q, sort string) ([]HFSearchResult, error) {
	apiURL := fmt.Sprintf("%s/api/models?search=%s&sort=%s&limit=200&full=true", baseURL, q, sort)

	req, err := http.NewRequest("GET", apiURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", appUserAgent())

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

// hasGGUF reports whether an HF search result contains a .gguf file (the GGUF
// filter for HF search candidates).
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
	return getModelDescriptionAt(activeHFBase(), modelID)
}

// getModelDescriptionAt fetches the README of a model on an HF-compatible base
// and extracts its natural-language description:
//   - GET {base}/{modelID}/raw/main/README.md (User-Agent via appUserAgent(), 30s timeout)
//   - non-200 returns an error; YAML front-matter (a block starting with ---) is skipped
//   - split by blank lines, take the first paragraph that is non-empty and does
//     not start with #, trim it and truncate to 200 runes
//   - when the README exists but has no description paragraph, return an empty
//     string and a nil error (silent)
func getModelDescriptionAt(baseURL, modelID string) (string, error) {
	readmeURL := fmt.Sprintf("%s/%s/raw/main/README.md", baseURL, modelID)

	req, err := http.NewRequest("GET", readmeURL, nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("User-Agent", appUserAgent())

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

// extractDescription extracts the natural-language description from a README
// body (shared by HF and ModelScope):
//   - skip YAML front-matter (a block whose first line trims to ---)
//   - split by blank lines, take the first paragraph that is non-empty and does
//     not start with #, trim it and truncate to 200 runes
//   - return an empty string when the body has no description paragraph
//     (silent, not treated as a failure)
func extractDescription(body string) string {
	lines := strings.Split(body, "\n")
	start := 0
	// Skip YAML front-matter: when the first line trims to ---, skip past the
	// next ---
	if len(lines) > 0 && strings.TrimSpace(lines[0]) == "---" {
		for i := 1; i < len(lines); i++ {
			if strings.TrimSpace(lines[i]) == "---" {
				start = i + 1
				break
			}
		}
	}

	// Split by blank lines, take the first paragraph that is non-empty and
	// does not start with #
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
		// Truncate to 200 runes, appending an ellipsis when exceeded
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
	return getHFModelFilesAt(activeHFBase(), modelID)
}

// getHFModelFilesAt lists the GGUF siblings of a model on an HF-compatible API base.
// blobs=true makes the API return real file sizes (HF search/detail APIs do
// not include size on siblings by default).
func getHFModelFilesAt(baseURL, modelID string) ([]HFFileOut, error) {
	apiURL := fmt.Sprintf("%s/api/models/%s?blobs=true", baseURL, modelID)

	req, err := http.NewRequest("GET", apiURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", appUserAgent())

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

// getHFModelMaxGGUFSize returns the size of the model's largest GGUF file
// (via the default mirror).
func getHFModelMaxGGUFSize(modelID string) (int64, error) {
	return getHFModelMaxGGUFSizeAt(activeHFBase(), modelID)
}

// getHFModelMaxGGUFSizeAt queries the model detail API (blobs=true is required
// for real sizes) and returns the size of the model's largest .gguf file; 0
// and nil when there is no GGUF. The HF search API's siblings carry no size
// (empirically all null), so model sizes on search cards can only be fetched
// one by one via the detail API by modelId; the largest file is used instead
// of the sum of all GGUFs to avoid the inflated totals of multi-quant models
// (dozens of quantized files) misleading users about model scale.
func getHFModelMaxGGUFSizeAt(baseURL, modelID string) (int64, error) {
	apiURL := fmt.Sprintf("%s/api/models/%s?blobs=true", baseURL, modelID)

	req, err := http.NewRequest("GET", apiURL, nil)
	if err != nil {
		return 0, err
	}
	req.Header.Set("User-Agent", appUserAgent())

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

// computeSpeed computes the download speed (bytes/sec) from the sampling
// interval (seconds) and the bytes downloaded within it. Pure function:
// returns 0 when elapsed or delta is non-positive (not computable or no
// valid progress).
func computeSpeed(elapsedSec float64, deltaBytes int64) float64 {
	if elapsedSec <= 0 || deltaBytes <= 0 {
		return 0
	}
	return float64(deltaBytes) / elapsedSec
}

// lastTaskPersist is the timestamp of the last download-queue persistence;
// lastTaskPersistMu guards its reads/writes. Progress-update paths use
// persistTasksThrottled: saves less than 5 seconds after the previous one are
// skipped, preventing high-frequency download progress from saturating config
// file writes (#12 queue persistence).
var lastTaskPersist time.Time
var lastTaskPersistMu sync.Mutex

// persistTasksNow persists the download task queue immediately (enqueue,
// status-change, and terminal-state paths). Callers must not hold dlTasksMu:
// saveConfig acquires dlTasksMu again at the end for its snapshot.
func persistTasksNow() {
	lastTaskPersistMu.Lock()
	lastTaskPersist = time.Now()
	lastTaskPersistMu.Unlock()
	saveConfig()
}

// persistTasksThrottled persists the download task queue with throttling
// (progress-update paths): skips saves less than 5 seconds after the last one
// (whether triggered by persistTasksNow or this function).
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

// retryDownloadTask rebuilds the task's download context and restarts the
// download goroutine. Once a task reaches a terminal error/cancelled/done
// state its ctx is already finished (the goroutine exited) and cannot be
// reused; a fresh context.WithCancel is required. downloadTask reads the
// .part file size at startup as the resume offset, naturally reusing
// resumable downloads. Callers must hold dlTasksMu.
func retryDownloadTask(task *DlTask) {
	task.ctx, task.cancel = context.WithCancel(context.Background())
	// Clear the error and stale progress display so the frontend stops
	// showing the previous red error box; downloadTask refills
	// Downloaded/Total/Progress from the .part resume offset.
	task.Error = ""
	task.Downloaded = 0
	task.Total = 0
	task.Progress = 0
	task.SizeHuman = ""
	task.Speed = 0
	task.Status = "queued"
	go downloadTask(task)
}

// idleReadTimeout is how long the download loop waits for the next body chunk
// before treating the stream as stalled (half-open TCP connection or a server
// / proxy that stopped sending) and reconnecting via Range at the current
// .part size. Injectable so tests can use a short value; the production window
// is generous so slow-but-alive streams are never cut off.
var idleReadTimeout = 60 * time.Second

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

	// Speed sampling state (exclusive to the downloadTask goroutine, no
	// locking needed): records the last sample time and byte count;
	// task.Speed is updated inside the read loop at intervals ≥1s.
	var lastSampleTime time.Time
	var lastSampleBytes int64

	client := &http.Client{Timeout: 30 * time.Minute}

	// Automatic transient-failure retries (see the download retry policy
	// block), shared by the connect / status / mid-stream error branches.
	retries := 0

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

		req, err := buildDownloadRequest(task.ctx, task.URL, offset)
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
			// Cancel-vs-network-error race defense: when ctx is already
			// cancelled (e.g. the user just clicked cancel), the task should
			// be marked cancelled rather than error, preventing the race from
			// pushing the task back to the error terminal state (e.g. the
			// network error from hf-mirror actively aborting the stream).
			if task.ctx.Err() != nil {
				task.Status = "cancelled"
				task.Speed = 0
				dlTasksMu.Unlock()
				persistTasksNow()
				return
			}
			dlTasksMu.Unlock()
			// Automatic transient-failure retry (see the download retry
			// policy block): network errors reconnect up to
			// downloadRetryCount times, resuming from the .part on disk; the
			// task stays in downloading state so the UI never flashes error.
			if retries < downloadRetryCount {
				retries++
				log.Printf("[WARN] task %s attempt failed (%v), retrying %d/%d", task.ID, err, retries, downloadRetryCount)
				if sleepDownloadRetry(task.ctx) {
					continue
				}
				dlTasksMu.Lock()
				task.Status = "cancelled"
				task.Speed = 0
				dlTasksMu.Unlock()
				persistTasksNow()
				return
			}
			dlTasksMu.Lock()
			task.Status = "error"
			task.Error = err.Error()
			task.Speed = 0
			dlTasksMu.Unlock()
			persistTasksNow()
			return
		}

		if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusPartialContent {
			status := resp.StatusCode
			resp.Body.Close()
			// Same automatic retry for transient HTTP statuses (429/5xx);
			// permanent 4xx (404/403/...) surfaces immediately.
			if retryableDownloadStatus(status) && retries < downloadRetryCount {
				retries++
				log.Printf("[WARN] task %s got HTTP %d, retrying %d/%d", task.ID, status, retries, downloadRetryCount)
				if sleepDownloadRetry(task.ctx) {
					continue
				}
				dlTasksMu.Lock()
				task.Status = "cancelled"
				task.Speed = 0
				dlTasksMu.Unlock()
				persistTasksNow()
				return
			}
			dlTasksMu.Lock()
			task.Status = "error"
			task.Error = fmt.Sprintf("HTTP %d", status)
			task.Speed = 0
			dlTasksMu.Unlock()
			persistTasksNow()
			return
		}

		// Robustness against servers ignoring Range (#B3): when offset>0 this
		// request carried a Range header, but some servers (e.g. the
		// ModelScope repo endpoint) do not guarantee Range support and ignore
		// the header, returning the full body with 200. Appending that full
		// body to the .part at offset would duplicate content onto the
		// existing partial file and corrupt it. Handling: close the response,
		// truncate .part to 0, zero the offset, clear the progress display,
		// then continue the outer loop to reconnect. With offset=0 the next
		// request carries no Range header; if the server keeps ignoring Range
		// and returns 200, writing still starts from zero — the content is
		// correct, the reconnect happens only this once, no infinite loop.
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
			// Reset the speed sampling baseline: after a full rewrite,
			// downloaded accumulates from 0 again.
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

	readLoop:
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
				// Reset the speed sampling baseline: elapsed would be inflated
				// by the pause duration; re-establish sampling from the new
				// offset after resume so the first segment's speed is not
				// dragged down by the paused time.
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
			readTimer := time.NewTimer(idleReadTimeout)
			select {
			case <-task.ctx.Done():
				readTimer.Stop()
				resp.Body.Close()
				out.Close()
				dlTasksMu.Lock()
				task.Status = "cancelled"
				task.Speed = 0
				dlTasksMu.Unlock()
				persistTasksNow()
				return
			case rr = <-ch:
				readTimer.Stop()
			case <-readTimer.C:
				// No data arrived within the idle window: the stream has
				// stalled (half-open connection / server or proxy stopped
				// sending). Close this attempt and reconnect with a Range
				// header at the current .part size — resuming from exactly
				// where the stalled read left off. The outer loop reopens the
				// .part file in append mode, so no bytes are lost or
				// duplicated.
				resp.Body.Close()
				out.Close()
				dlTasksMu.Lock()
				task.Speed = 0
				dlTasksMu.Unlock()
				if fi, err := os.Stat(tmpPath); err == nil {
					offset = fi.Size()
				} else {
					offset = downloaded
				}
				// Reset the speed sampling baseline: elapsed would otherwise
				// be inflated by the stall duration.
				lastSampleTime = time.Time{}
				lastSampleBytes = 0
				break readLoop
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
				// Speed sampling: update only at intervals ≥1s to avoid
				// high-frequency computation and jitter. After a pause/resume,
				// downloaded accumulates from the new offset; negative deltas
				// are treated as 0.
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
				// On move failure, mark the task as errored and return without
				// advancing to done (#10). moveFile internally uses the
				// injectable package-level variable renameFile so tests can
				// simulate failure; across devices (cross-drive on Windows) it
				// falls back to copy + delete source.
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
				// Cancel-vs-read-error race defense: when ctx is already
				// cancelled, mark cancelled rather than error (same strategy
				// as the client.Do error branch).
				if task.ctx.Err() != nil {
					task.Status = "cancelled"
					task.Speed = 0
					dlTasksMu.Unlock()
					persistTasksNow()
					return
				}
				dlTasksMu.Unlock()
				// Mid-body stream failures are transient: reconnect and
				// resume from the .part size on disk (outer loop re-stats).
				if retries < downloadRetryCount {
					retries++
					log.Printf("[WARN] task %s stream failed (%v), retrying %d/%d", task.ID, rr.err, retries, downloadRetryCount)
					if sleepDownloadRetry(task.ctx) {
						break readLoop
					}
					dlTasksMu.Lock()
					task.Status = "cancelled"
					task.Speed = 0
					dlTasksMu.Unlock()
					persistTasksNow()
					return
				}
				dlTasksMu.Lock()
				task.Status = "error"
				task.Error = rr.err.Error()
				task.Speed = 0
				dlTasksMu.Unlock()
				persistTasksNow()
				return
			}
		}
	}
}

// buildDownloadRequest creates a GET request for a download URL with the
// appUserAgent() User-Agent, adding a Range header when resuming from an offset.
// The request is bound to the task's cancel context so cancelling the task
// aborts an in-flight transfer immediately.
func buildDownloadRequest(ctx context.Context, downloadURL string, offset int64) (*http.Request, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", downloadURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", appUserAgent())
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
