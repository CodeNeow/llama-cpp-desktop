package core

import (
	"context"
	"fmt"
	"log"
	"runtime"

	wailsRuntime "github.com/wailsapp/wails/v2/pkg/runtime"
)

// App is the Wails application struct whose methods are bound to the frontend.
type App struct {
	ctx context.Context
}

func NewApp() *App {
	return &App{}
}

// Startup is called by Wails after the runtime is ready; it loads persisted
// config and applies the saved theme to the window background.
func (a *App) Startup(ctx context.Context) {
	a.ctx = ctx

	// Load persisted config on startup
	loadConfig()

	// Set initial window background from saved theme
	configMu.Lock()
	theme := currentTheme
	configMu.Unlock()
	if theme == "light" {
		wailsRuntime.WindowSetBackgroundColour(ctx, 248, 250, 252, 255)
	} else {
		wailsRuntime.WindowSetBackgroundColour(ctx, 15, 15, 20, 255)
	}

	log.Println("[INFO] Llama GUI started")
}

// Shutdown is called by Wails on application exit; it stops llama-server and
// cancels all in-flight downloads.
func (a *App) Shutdown(ctx context.Context) {
	// Stop running llama-server if any（#3）：在 serverMu 锁内取局部副本，
	// 锁外对副本 Signal，避免与应用退出期间并发启停的数据竞争。
	serverMu.Lock()
	running := serverRunning
	cmd := serverCmd
	serverMu.Unlock()
	if running && cmd != nil {
		addServerLog("[INFO] Stopping llama-server on shutdown...")
		cmd.Process.Signal(osInterrupt)
	}

	// Cancel ongoing llama.cpp download（#3）：downloadCancel 受 downloadMu
	// 保护，先锁内取副本，锁外调用。
	downloadMu.Lock()
	cancelDownload := downloadCancel
	downloadMu.Unlock()
	if cancelDownload != nil {
		cancelDownload()
	}

	// Cancel all HF model download tasks
	dlTasksMu.Lock()
	for _, t := range dlTasks {
		if t.cancel != nil {
			t.cancel()
		}
	}
	dlTasksMu.Unlock()

	log.Println("[INFO] Llama GUI stopped")
}

// ─── Config ─────────────────────────────────────────────────────

func (a *App) GetConfig() map[string]interface{} {
	customLlamaCppMu.Lock()
	dir := customLlamaCppDir
	customLlamaCppMu.Unlock()

	configMu.Lock()
	theme := currentTheme
	configMu.Unlock()

	return map[string]interface{}{
		"theme":       theme,
		"llamaCppDir": dir,
	}
}

func (a *App) SetTheme(theme string) {
	configMu.Lock()
	currentTheme = theme
	configMu.Unlock()
	saveConfig()

	// Update window background to match theme
	if a.ctx != nil {
		if theme == "light" {
			wailsRuntime.WindowSetBackgroundColour(a.ctx, 248, 250, 252, 255)
		} else {
			wailsRuntime.WindowSetBackgroundColour(a.ctx, 15, 15, 20, 255)
		}
	}
}

// ─── System Info ─────────────────────────────────────────────────

func (a *App) GetSystemInfo() *SystemInfo {
	systemInfoOnce.Do(func() {
		cachedSystemInfo = collectSystemInfo()
	})
	return &cachedSystemInfo
}

func (a *App) GetCPU() *CPUInfo {
	cpuOnce.Do(func() { cachedCPU = getCPUInfo() })
	return &cachedCPU
}

func (a *App) GetMemory() *MemoryInfo {
	memOnce.Do(func() {
		cachedMemory = MemoryInfo{TotalGB: getTotalMemoryGB(), FreeGB: getFreeMemoryGB()}
	})
	return &cachedMemory
}

func (a *App) GetGPU() []GPUInfo {
	gpuOnce.Do(func() { cachedGPU = getGPUInfo() })
	return cachedGPU
}

func (a *App) GetCUDA() *CUDAInfo {
	cudaOnce.Do(func() { cachedCUDA = getCUDAInfo() })
	return &cachedCUDA
}

func (a *App) GetLlamaCpp() *LlamaCppInfo {
	if !llamaCacheValid.Load() {
		cachedLlamaCpp = getLlamaCppInfo()
		llamaCacheValid.Store(true)
	}
	return &cachedLlamaCpp
}

func (a *App) GetOS() map[string]string {
	return map[string]string{"os": runtime.GOOS, "arch": runtime.GOARCH}
}

// ─── Models ──────────────────────────────────────────────────────

func (a *App) GetModels() []ModelInfo {
	// 缓存无效时重扫（#4）：快速路径先读 atomic 标记，慢速路径在
	// modelsMu 锁内复核，避免并发首次调用重复扫描；cachedModels 的写入
	// 只发生在锁内。
	if !modelsCacheValid.Load() {
		modelsMu.Lock()
		if !modelsCacheValid.Load() {
			log.Println("[INFO] Scanning LLM-Models ...")
			cachedModels = scanModels()
			modelsCacheValid.Store(true)
			log.Printf("[OK] Found %d models", len(cachedModels))
		}
		modelsMu.Unlock()
	}
	// 返回前在 modelsMu 锁内拷贝切片副本，避免调用方与后续 RefreshModels
	// 重扫（写入 cachedModels）并发读写同一底层数组（#4）。
	modelsMu.Lock()
	out := make([]ModelInfo, len(cachedModels))
	copy(out, cachedModels)
	modelsMu.Unlock()
	return out
}

func (a *App) RefreshModels() []ModelInfo {
	invalidateModelCache()
	return a.GetModels()
}

func (a *App) GetModelConfig(modelID string) ModelConfig {
	modelConfigsMu.Lock()
	cfg, ok := cachedModelConfigs[modelID]
	modelConfigsMu.Unlock()
	if !ok {
		return defaultModelConfig()
	}
	return cfg
}

func (a *App) SaveModelConfig(modelID string, config ModelConfig) error {
	// 校验字符串字段白名单，防止非法取值（含 INI 注入 payload）进入
	// 配置并被预设生成直接写入 INI（#9 第一层防御）。
	if !validGPULayersValue(config.GPULayers) {
		return fmt.Errorf("非法 GPULayers %q：仅允许 auto/all/0 或正整数", config.GPULayers)
	}
	if !validCacheTypeValue(config.CacheTypeK) {
		return fmt.Errorf("非法 CacheTypeK %q：仅允许 q8_0/q4_0/f16/bf16", config.CacheTypeK)
	}
	if !validCacheTypeValue(config.CacheTypeV) {
		return fmt.Errorf("非法 CacheTypeV %q：仅允许 q8_0/q4_0/f16/bf16", config.CacheTypeV)
	}

	modelConfigsMu.Lock()
	cachedModelConfigs[modelID] = config
	modelConfigsMu.Unlock()
	saveConfig()
	return nil
}

// ─── Server ──────────────────────────────────────────────────────

func (a *App) GetServerConfig() ServerConfig {
	serverConfigMu.Lock()
	cfg := cachedServerConfig
	serverConfigMu.Unlock()
	return cfg
}

func (a *App) SaveServerConfig(cfg ServerConfig) error {
	// Host 白名单：仅允许环回地址，避免把推理服务暴露到局域网/公网（#5）。
	switch cfg.Host {
	case "127.0.0.1", "localhost", "::1":
	default:
		return fmt.Errorf("非法 Host %q：仅允许环回地址，避免将推理服务暴露到局域网", cfg.Host)
	}
	if cfg.Port < 1024 || cfg.Port > 65535 {
		return fmt.Errorf("非法 Port %d：端口范围应为 1024-65535", cfg.Port)
	}
	if cfg.MaxModels < 1 {
		return fmt.Errorf("非法 MaxModels %d：至少为 1", cfg.MaxModels)
	}
	if cfg.CacheRAM < 0 {
		return fmt.Errorf("非法 CacheRAM %d：不能为负数", cfg.CacheRAM)
	}

	serverConfigMu.Lock()
	cachedServerConfig = cfg
	serverConfigMu.Unlock()
	saveConfig()
	return nil
}

func (a *App) GetServerStatus() map[string]interface{} {
	// serverRunning 由 serverMu 保护（#3）；serverLogs 由 serverLogsMu 保护。
	serverMu.Lock()
	running := serverRunning
	serverMu.Unlock()
	serverLogsMu.Lock()
	logs := make([]string, len(serverLogs))
	copy(logs, serverLogs)
	serverLogsMu.Unlock()

	return map[string]interface{}{
		"running": running,
		"log":     logs,
	}
}

func (a *App) StartServer() error {
	// 已运行则幂等返回 nil（#3）：serverRunning 由 serverMu 保护。
	serverMu.Lock()
	running := serverRunning
	serverMu.Unlock()
	if running {
		return nil
	}

	return startServerInternal()
}

func (a *App) StopServer() error {
	return stopServerInternal()
}

// ─── LlamaCpp Download ───────────────────────────────────────────

func (a *App) GetLlamaCppDownloadStatus() *DownloadState {
	downloadMu.Lock()
	ds := *downloadState
	downloadMu.Unlock()
	return &ds
}

func (a *App) StartLlamaCppDownload() error {
	startLlamaCppDownload()
	return nil
}

func (a *App) PauseLlamaCppDownload() {
	downloadMu.Lock()
	downloadState.Status = "paused"
	downloadState.Paused = true
	downloadResumeCh = make(chan struct{}, 1) // fresh channel for resume signal
	downloadMu.Unlock()
}

func (a *App) ResumeLlamaCppDownload() {
	downloadMu.Lock()
	downloadState.Paused = false
	if downloadState.Status == "paused" {
		downloadState.Status = "downloading"
	}
	// Signal resume (non-blocking send to buffered channel)
	select {
	case downloadResumeCh <- struct{}{}:
	default:
	}
	downloadMu.Unlock()
}

func (a *App) StopLlamaCppDownload() {
	// downloadCancel 受 downloadMu 保护：锁内取副本，锁外调用（#3）。
	downloadMu.Lock()
	cancelDownload := downloadCancel
	downloadMu.Unlock()
	if cancelDownload != nil {
		cancelDownload()
	}
}

func (a *App) BrowseLlamaCppDir() (string, error) {
	if a.ctx == nil {
		return "", nil
	}
	dir, err := wailsRuntime.OpenDirectoryDialog(a.ctx, wailsRuntime.OpenDialogOptions{
		Title: "选择 llama.cpp 目录",
	})
	if err != nil || dir == "" {
		return "", err
	}
	customLlamaCppMu.Lock()
	customLlamaCppDir = dir
	customLlamaCppMu.Unlock()
	llamaCacheValid.Store(false)
	saveConfig()
	return dir, nil
}

// ─── Downloads (HF Mirror) ───────────────────────────────────────

func (a *App) SearchDownloads(query, filter string) ([]HFSearchResult, error) {
	return searchHFMirror(query, filter)
}

func (a *App) GetModelFiles(modelID string) ([]HFFileOut, error) {
	return getHFModelFiles(modelID)
}

func (a *App) StartDownload(modelID string, files []string) error {
	return startHFDownload(modelID, files)
}

func (a *App) GetDownloadTasks() []DlTask {
	dlTasksMu.Lock()
	defer dlTasksMu.Unlock()
	tasks := make([]DlTask, 0, len(dlTasks))
	for _, t := range dlTasks {
		tasks = append(tasks, *t)
	}
	return tasks
}

func (a *App) CancelDownloadTask(id string) error {
	dlTasksMu.Lock()
	defer dlTasksMu.Unlock()
	for _, t := range dlTasks {
		if t.ID == id {
			t.cancel()
			return nil
		}
	}
	return nil
}

func (a *App) PauseDownloadTask(id string) error {
	dlTasksMu.Lock()
	defer dlTasksMu.Unlock()
	for _, t := range dlTasks {
		if t.ID == id && t.Status == "downloading" {
			t.Status = "paused"
			t.resumeCh = make(chan struct{}, 1) // fresh channel for resume signal
			return nil
		}
	}
	return nil
}

func (a *App) ResumeDownloadTask(id string) error {
	dlTasksMu.Lock()
	defer dlTasksMu.Unlock()
	for _, t := range dlTasks {
		if t.ID == id && t.Status == "paused" {
			t.Status = "downloading"
			// Signal resume (non-blocking send to buffered channel)
			select {
			case t.resumeCh <- struct{}{}:
			default:
			}
			return nil
		}
	}
	return nil
}
