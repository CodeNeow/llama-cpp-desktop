package main

import (
	"context"
	"log"
	"runtime"
	"sync"

	wailsRuntime "github.com/wailsapp/wails/v2/pkg/runtime"
)

// App is the Wails application struct whose methods are bound to the frontend.
type App struct {
	ctx context.Context
}

func NewApp() *App {
	return &App{}
}

func (a *App) startup(ctx context.Context) {
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

func (a *App) shutdown(ctx context.Context) {
	// Stop running llama-server if any
	serverLogsMu.Lock()
	running := serverRunning
	cmd := serverCmd
	serverLogsMu.Unlock()
	if running && cmd != nil {
		addServerLog("[INFO] Stopping llama-server on shutdown...")
		cmd.Process.Signal(osInterrupt)
	}

	// Cancel ongoing llama.cpp download
	if downloadCancel != nil {
		downloadCancel()
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
	modelsOnce.Do(func() {
		log.Println("[INFO] Scanning LLM-Models (first time)...")
		cachedModels = scanModels()
		log.Printf("[OK] Found %d models", len(cachedModels))
	})
	return cachedModels
}

func (a *App) RefreshModels() []ModelInfo {
	modelsOnce = sync.Once{}
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

func (a *App) SaveServerConfig(cfg ServerConfig) {
	serverConfigMu.Lock()
	cachedServerConfig = cfg
	serverConfigMu.Unlock()
	saveConfig()
}

func (a *App) GetServerStatus() map[string]interface{} {
	serverLogsMu.Lock()
	logs := make([]string, len(serverLogs))
	copy(logs, serverLogs)
	running := serverRunning
	serverLogsMu.Unlock()

	return map[string]interface{}{
		"running": running,
		"log":     logs,
	}
}

func (a *App) StartServer() error {
	serverLogsMu.Lock()
	if serverRunning {
		serverLogsMu.Unlock()
		return nil // already running
	}
	serverLogsMu.Unlock()

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
	if downloadCancel != nil {
		downloadCancel()
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
