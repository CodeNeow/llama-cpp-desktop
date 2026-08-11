package core

import (
	"context"
	"fmt"
	"log"
	"os"
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

	// Cancel ongoing app update download
	updateDownloadMu.Lock()
	cancelUpdate := updateDownloadCancel
	updateDownloadMu.Unlock()
	if cancelUpdate != nil {
		cancelUpdate()
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

	modelsDirMu.Lock()
	modelDir := customModelsDir
	modelsDirMu.Unlock()

	configMu.Lock()
	theme := currentTheme
	configMu.Unlock()

	return map[string]interface{}{
		"theme":       theme,
		"llamaCppDir": dir,
		"modelsDir":   modelDir,
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

// SetModelsDir 设置自定义模型目录：合法目录写入全局状态、持久化配置并使
// 模型缓存失效（下次 GetModels 用新目录重扫）；非法输入返回中文错误且
// 不改写状态。
func (a *App) SetModelsDir(dir string) error {
	if dir == "" {
		return fmt.Errorf("模型目录不能为空")
	}
	fi, err := os.Stat(dir)
	if err != nil {
		return fmt.Errorf("模型目录不存在: %s", dir)
	}
	if !fi.IsDir() {
		return fmt.Errorf("模型目录不是有效目录: %s", dir)
	}
	modelsDirMu.Lock()
	customModelsDir = dir
	modelsDirMu.Unlock()
	invalidateModelCache()
	saveConfig()
	return nil
}

// BrowseModelsDir 弹出系统目录选择对话框选择模型目录；取消时返回空串与
// nil，选择成功后执行与 SetModelsDir 相同的校验与写入并返回所选目录。
func (a *App) BrowseModelsDir() (string, error) {
	if a.ctx == nil {
		return "", nil
	}
	dir, err := wailsRuntime.OpenDirectoryDialog(a.ctx, wailsRuntime.OpenDialogOptions{
		Title: "选择模型目录",
	})
	if err != nil || dir == "" {
		return "", err
	}
	if err := a.SetModelsDir(dir); err != nil {
		return "", err
	}
	return dir, nil
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

// ─── App Update ──────────────────────────────────────────────────

// GetAppVersion 返回应用当前版本号（与 GitHub 发布 tag 对齐）。
func (a *App) GetAppVersion() string {
	return currentVersion
}

// CheckForUpdate 查询远程仓库 latest release，返回是否有新版本及版本信息。
func (a *App) CheckForUpdate() (*UpdateCheckResult, error) {
	return CheckForUpdateAt(updateRepoAPI)
}

// StartUpdateDownload 开始下载新版本 exe 到可执行文件同目录。
// 版本号由调用方传入（来自 CheckForUpdate 结果），用于目标文件命名。
func (a *App) StartUpdateDownload(version string) error {
	if compareVersions(version, currentVersion) <= 0 {
		return fmt.Errorf("没有可更新的版本: %s", version)
	}
	updateDownloadMu.Lock()
	inProgress := updateDownloadState.Status == "downloading"
	updateDownloadMu.Unlock()
	if inProgress {
		return fmt.Errorf("更新下载已在进行中")
	}
	go downloadUpdateRelease(version)
	return nil
}

// GetUpdateDownloadStatus 返回更新下载的当前状态与进度。
func (a *App) GetUpdateDownloadStatus() *UpdateDownloadState {
	updateDownloadMu.Lock()
	ds := *updateDownloadState
	updateDownloadMu.Unlock()
	return &ds
}

// StopUpdateDownload 取消在途的更新下载（用户主动停止/退出应用）。
func (a *App) StopUpdateDownload() {
	updateDownloadMu.Lock()
	cancel := updateDownloadCancel
	updateDownloadMu.Unlock()
	if cancel != nil {
		cancel()
	}
}

// ─── Downloads (HF Mirror) ───────────────────────────────────────

func (a *App) SearchDownloads(query, filter string) ([]HFSearchResult, error) {
	return searchHFMirror(query, filter)
}

func (a *App) GetModelFiles(modelID string) ([]HFFileOut, error) {
	return getHFModelFiles(modelID)
}

// GetModelMaxFileSize 返回模型最大的 GGUF 文件大小（供搜索卡片展示模型大小）。
func (a *App) GetModelMaxFileSize(modelID string) (int64, error) {
	return getHFModelMaxGGUFSize(modelID)
}

// GetModelDescription 获取模型 README 中的自然语言描述（供下载页展示模型说明）。
func (a *App) GetModelDescription(modelID string) (string, error) {
	return getModelDescription(modelID)
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
			// 锁内置 cancelled 状态再 cancel()：对 error/cancelled 等终态任务
			// （goroutine 已退出，cancel() 无效果）点取消也能立即生效，前端
			// 轮询即可看到取消状态；对 downloading 任务，downloadTask 的
			// ctx.Done() 分支同样会置 cancelled，二者一致。
			t.Status = "cancelled"
			if t.cancel != nil {
				t.cancel()
			}
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

// RetryDownloadTask 重试下载任务：对已结束（error/cancelled/done）或排队中
// （queued）的任务重建 ctx 并重新启动下载 goroutine，downloadTask 会检查
// .part 文件大小作为续传 offset，天然复用断点续传。仍在下载中（downloading）
// 或已暂停（paused）的任务存在活跃 goroutine，不允许重试避免并发写同一
// .part 文件；找不到 id 时静默返回 nil，与 CancelDownloadTask 语义一致。
func (a *App) RetryDownloadTask(id string) error {
	dlTasksMu.Lock()
	defer dlTasksMu.Unlock()
	for _, t := range dlTasks {
		if t.ID == id {
			if t.Status == "downloading" || t.Status == "paused" {
				return nil
			}
			retryDownloadTask(t)
			return nil
		}
	}
	return nil
}
