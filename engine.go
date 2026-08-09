package main

import (
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
	TagName string         `json:"tag_name"`
	Name    string         `json:"name"`
	Assets  []GitHubAsset  `json:"assets"`
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
	ID           string   `json:"id"`
	ModelID      string   `json:"modelId"`
	Author       string   `json:"author"`
	Downloads    int      `json:"downloads"`
	Likes        int      `json:"likes"`
	PipelineTag  string   `json:"pipelineTag"`
	Tags         []string `json:"tags"`
	Siblings     []HFFile `json:"siblings"`
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
	URL        string  `json:"-"`
	Status     string  `json:"status"`
	Progress   int     `json:"progress"`
	Total      int64   `json:"total"`
	Downloaded int64   `json:"downloaded"`
	SizeHuman  string  `json:"sizeHuman"`
	Error      string  `json:"error"`
	ctx        context.Context
	cancel     context.CancelFunc
	pauseCh    chan struct{}
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
var modelsOnce sync.Once

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
	Status      string `json:"status"` // idle, fetching, downloading, paused, extracting, done, error
	Paused      bool   `json:"paused"`
	Progress    int    `json:"progress"`
	Total       int64  `json:"total"`
	Downloaded  int64  `json:"downloaded"`
	FileName    string `json:"fileName"`
	Version     string `json:"version"`
	Error       string `json:"error"`
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

func getLlamaCppInfo() LlamaCppInfo {
	info := LlamaCppInfo{}

	// Search for common llama.cpp binaries in PATH
	binaryNames := []string{
		"llama-cli",
		"llama.cpp",
		"llama",
		"llama-server",
	}

	// Also check custom directory if set
	customLlamaCppMu.Lock()
	customDir := customLlamaCppDir
	customLlamaCppMu.Unlock()

	dirsToCheck := []string{""} // empty means PATH
	if customDir != "" {
		dirsToCheck = append([]string{customDir}, dirsToCheck...)
	}

	for _, bin := range binaryNames {
		for _, dir := range dirsToCheck {
			var path string
			var err error
			if dir == "" {
				path, err = exec.LookPath(bin)
			} else {
				candidate := filepath.Join(dir, bin)
				if runtime.GOOS == "windows" {
					candidate += ".exe"
				}
				if _, statErr := os.Stat(candidate); statErr == nil {
					path = candidate
					err = nil
				} else {
					// Also check without .exe on Windows
					if runtime.GOOS == "windows" {
						candidateNoExt := filepath.Join(dir, bin)
						if _, statErr := os.Stat(candidateNoExt); statErr == nil {
							path = candidateNoExt
							err = nil
						} else {
							err = statErr
						}
					}
				}
			}
			if err != nil {
				continue
			}

			info.Installed = true
			info.Path = path

			// Try to get version
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
			} else {
				versionOut = runCmd(path, "-v")
				if versionOut != "" {
					info.Version = strings.TrimSpace(versionOut)
				}
			}
			return info
		}
	}

	return info
}

// ─── Command helpers ─────────────────────────────────────────────

func runCmd(name string, args ...string) string {
	cmd := exec.Command(name, args...)
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
		// Remove trailing colon from key
		if strings.TrimRight(fields[0], ":") == key {
			val, err := strconv.ParseUint(fields[1], 10, 64)
			if err == nil {
				return val
			}
		}
	}
	return 0
}

// ─── Model scanning ──────────────────────────────────────────────

const modelsDir = "LLM-Models"

func scanModels() []ModelInfo {
	models := make([]ModelInfo, 0)

	if err := os.MkdirAll(modelsDir, 0755); err != nil {
		log.Printf("[WARN] Failed to create %s dir: %v", modelsDir, err)
		return models
	}

	// Top-level: author directories
	authors, err := os.ReadDir(modelsDir)
	if err != nil {
		log.Printf("[WARN] Failed to read %s dir: %v", modelsDir, err)
		return models
	}

	for _, authorEntry := range authors {
		if !authorEntry.IsDir() {
			continue
		}
		author := authorEntry.Name()
		authorDir := filepath.Join(modelsDir, author)

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

const githubReleasesAPI = "https://api.github.com/repos/ggml-org/llama.cpp/releases/latest"
const downloadDir = "llama-cpp"
const configFile = "llama-gui-config.json"

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
	modelsOnce = sync.Once{}

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

func fetchLatestRelease() (*GitHubRelease, error) {
	req, err := http.NewRequest("GET", githubReleasesAPI, nil)
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

func pickBestAsset(assets []GitHubAsset) *GitHubAsset {
	if len(assets) == 0 {
		return nil
	}

	platform := runtime.GOOS
	arch := runtime.GOARCH

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

	// On Windows with CUDA, prefer CUDA builds
	hasCUDA := len(getGPUInfo()) > 0

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
			cudaInfo := getCUDAInfo()
			if cudaInfo.ToolkitVersion != "" {
				// Extract major.minor from toolkit version like "12.8"
				parts := strings.Split(cudaInfo.ToolkitVersion, ".")
				if len(parts) >= 2 {
					cudaVer := parts[0] + "." + parts[1]
					if strings.Contains(name, "cuda-"+cudaVer) {
						score += 50 // Exact version match
					}
				}
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

func extractZip(src, dest string) error {
	r, err := zip.OpenReader(src)
	if err != nil {
		return err
	}
	defer r.Close()

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

		_, err = io.Copy(outFile, rc)
		rc.Close()
		outFile.Close()
		if err != nil {
			return err
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
			if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
				return err
			}
			outFile, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, os.FileMode(header.Mode))
			if err != nil {
				return err
			}
			if _, err := io.Copy(outFile, tarReader); err != nil {
				outFile.Close()
				return err
			}
			outFile.Close()
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

// ─── llama-server manager ────────────────────────────────────────

var serverCmd *exec.Cmd
var serverLogs []string
var serverLogsMu sync.Mutex
var serverRunning bool





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

func generateModelsPreset() (string, error) {
	// Scan LLM-Models and create an INI preset file
	models := scanModels()
	if len(models) == 0 {
		return "", fmt.Errorf("LLM-Models 目录中没有模型")
	}

	var buf bytes.Buffer
	modelConfigsMu.Lock()
	cfgs := cachedModelConfigs
	modelConfigsMu.Unlock()

	for _, m := range models {
		alias := sanitizeAlias(m.Name)
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
				buf.WriteString(fmt.Sprintf("gpu-layers = %s\n", cfg.GPULayers))
			}
			if cfg.FlashAttn {
				buf.WriteString("flash-attn = on\n")
			}
			if cfg.CacheTypeK != "" {
				buf.WriteString(fmt.Sprintf("cache-type-k = %s\n", cfg.CacheTypeK))
			}
			if cfg.CacheTypeV != "" {
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
	if cfg.Theme == "" {
		cfg.Theme = "dark"
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

var currentTheme = "dark"
var configMu sync.Mutex

func saveConfig() {
	customLlamaCppMu.Lock()
	dir := customLlamaCppDir
	customLlamaCppMu.Unlock()

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

	cfg := appConfig{LlamaCppDir: dir, Theme: theme, ModelConfigs: mcfgs, ServerConfig: scfg}
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

func searchHFMirror(q string, filter string) ([]HFSearchResult, error) {
	apiURL := fmt.Sprintf("%s/api/models?search=%s&sort=downloads&limit=20&full=true", hfMirrorBase, q)

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
		hasGGUF := false
		for _, s := range r.Siblings {
			if strings.HasSuffix(strings.ToLower(s.Filename), ".gguf") {
				hasGGUF = true
				break
			}
		}
		if !hasGGUF {
			continue
		}

		// Apply type filter
		if filter != "" && filter != "all" {
			switch filter {
			case "embedding":
				if r.PipelineTag != "sentence-similarity" && r.PipelineTag != "feature-extraction" {
					isEmbedding := false
					for _, t := range r.Tags {
						if strings.Contains(strings.ToLower(t), "embedding") ||
							strings.Contains(strings.ToLower(t), "sentence-transformers") ||
							strings.Contains(strings.ToLower(t), "text-embeddings") {
							isEmbedding = true
							break
						}
					}
					if !isEmbedding {
						continue
					}
				}
			case "llm":
				if r.PipelineTag != "text-generation" {
					continue
				}
			}
		}

		results = append(results, result)
	}

	return results, nil
}

func getHFModelFiles(modelID string) ([]HFFileOut, error) {
	apiURL := fmt.Sprintf("%s/api/models/%s", hfMirrorBase, modelID)

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

		req, err := http.NewRequest("GET", task.URL, nil)
		if err != nil {
			dlTasksMu.Lock()
			task.Status = "error"
			task.Error = err.Error()
			dlTasksMu.Unlock()
			return
		}
		req.Header.Set("User-Agent", "llama-gui")
		if offset > 0 {
			req.Header.Set("Range", fmt.Sprintf("bytes=%d-", offset))
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
			type readRes struct{ n int; err error }
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
				os.Rename(tmpPath, destPath)
				dlTasksMu.Lock()
				task.Status = "done"
				task.Progress = 100
				dlTasksMu.Unlock()
				modelsOnce = sync.Once{}
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

func waitForTaskResume(task *DlTask, resumeCh chan struct{}) {
	select {
	case <-resumeCh:
	case <-task.ctx.Done():
	}
}
