package core

import (
	"context"
	"errors"
	"fmt"
	"log"
	"os"
	"runtime"

	wailsRuntime "github.com/wailsapp/wails/v2/pkg/runtime"
) // App is the Wails application struct whose methods are bound to the frontend.
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

	// Start the background monitor sampler (CPU / memory / GPU / TPS)
	StartMonitorSampler()

	// Set initial window background from saved theme
	configMu.Lock()
	theme := currentTheme
	configMu.Unlock()
	if theme == "light" {
		wailsRuntime.WindowSetBackgroundColour(ctx, 248, 250, 252, 255)
	} else {
		wailsRuntime.WindowSetBackgroundColour(ctx, 15, 15, 20, 255)
	}

	log.Println("[INFO] Llama Desktop started")
}

// Shutdown is called by Wails on application exit; it persists the download
// queue, stops llama-server and cancels all in-flight downloads.
func (a *App) Shutdown(ctx context.Context) {
	// 退出前先持久化下载任务队列（#12）：在取消任何下载之前保存，保证重启后
	// 恢复的是最新状态。saveConfig 末尾会获取 dlTasksMu，此处未持有它。
	saveConfig()

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

	log.Println("[INFO] Llama Desktop stopped")
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
	tray := trayEnabled
	sidebarCollapsed := currentSidebarCollapsed
	configMu.Unlock()

	downloadSourceMu.Lock()
	dsrc := downloadSource
	downloadSourceMu.Unlock()

	languageMu.Lock()
	lang := currentLanguage
	languageMu.Unlock()

	return map[string]interface{}{
		"theme":            theme,
		"llamaCppDir":      dir,
		"modelsDir":        modelDir,
		"downloadSource":   dsrc,
		"language":         lang,                // 语言原始偏好: zh / en / auto
		"resolvedLanguage": effectiveLanguage(), // 生效语言: zh / en（auto 按系统检测结果）
		"trayEnabled":      tray,                // Windows 系统托盘开关
		"sidebarCollapsed": sidebarCollapsed,    // 侧边栏收起状态（默认 false=展开）
	}
}

// GetDownloadSource 返回当前模型下载源（"hf" | "modelscope"）。
func (a *App) GetDownloadSource() string {
	return activeDownloadSource()
}

// SetDownloadSource 设置模型下载源：仅允许 hf / modelscope 白名单值，非法值
// 返回中文错误且不改写状态；合法值写入全局并持久化。
func (a *App) SetDownloadSource(source string) error {
	if source != sourceHF && source != sourceModelScope {
		return fmt.Errorf(tr("非法下载源 %q：仅允许 hf 或 modelscope", "invalid download source %q: only hf or modelscope are allowed"), source)
	}
	downloadSourceMu.Lock()
	downloadSource = source
	downloadSourceMu.Unlock()
	saveConfig()
	return nil
}

// SetLanguage 设置界面语言：仅允许 zh / en / auto 白名单值，非法值返回错误且
// 不改写状态；合法值写入全局并持久化，并返回生效语言（zh/en，auto 时按系统
// 检测结果解析）。前端用返回值立即刷新界面文案，避免依赖 GetConfig 二次读取。
func (a *App) SetLanguage(language string) (string, error) {
	switch language {
	case "zh", "en", "auto":
	default:
		return "", fmt.Errorf(tr("非法语言 %q：仅允许 zh/en/auto", "invalid language %q: only zh/en/auto"), language)
	}
	languageMu.Lock()
	currentLanguage = language
	languageMu.Unlock()
	saveConfig()
	return effectiveLanguage(), nil
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

// SetSidebarCollapsed 设置侧边栏收起/展开偏好：写入全局状态并持久化到配置文件，
// 无窗口背景等副作用（纯 UI 偏好，镜像 SetTheme 的极简形态）。
func (a *App) SetSidebarCollapsed(collapsed bool) {
	configMu.Lock()
	currentSidebarCollapsed = collapsed
	configMu.Unlock()
	saveConfig()
}

// SetTrayEnabled 设置 Windows 系统托盘开关：持久化偏好到配置文件；Windows 上
// 按持久化值启动或退出托盘，其他平台只持久化不启停（InitTray/QuitTray 为
// no-op 存根）。并发安全（configMu 与 trayMu 保护全局状态），由前端设置页的
// 系统托盘开关调用。
//
// 注意：fyne.io/systray 的包级 quitOnce（sync.Once）使得同进程内一次 Run 退出
// 后无法再次 Run——因此启用托盘时仅调用 InitTray 启动尚未启动的托盘（幂等），
// 绝不重复调用 Run；禁用托盘时 QuitTray 摘除图标后，本次进程内不再重启（见
// core/tray_windows.go 的 trayStarted 注释与 systray 源码 quitOnce）。再次启用
// 在重启应用后生效。
func (a *App) SetTrayEnabled(enabled bool) error {
	configMu.Lock()
	trayEnabled = enabled
	configMu.Unlock()

	if enabled {
		// 启用：systray 同进程不可二次 Run，仅当托盘从未启动过时启动（幂等）
		if runtime.GOOS == "windows" {
			InitTray(a.ctx, TrayIcon)
		}
	} else {
		// 禁用：摘除托盘图标（systray.Quit 由 quitOnce 保证只生效一次）
		if runtime.GOOS == "windows" {
			QuitTray()
		}
	}
	saveConfig()
	log.Printf("[INFO] trayEnabled set to %v", enabled)
	return nil
}

// SetModelsDir 设置自定义模型目录：合法目录写入全局状态、持久化配置并使
// 模型缓存失效（下次 GetModels 用新目录重扫）；非法输入返回中文错误且
// 不改写状态。
func (a *App) SetModelsDir(dir string) error {
	if dir == "" {
		return errors.New(tr("模型目录不能为空", "models directory cannot be empty"))
	}
	fi, err := os.Stat(dir)
	if err != nil {
		return fmt.Errorf(tr("模型目录不存在: %s", "models directory does not exist: %s"), dir)
	}
	if !fi.IsDir() {
		return fmt.Errorf(tr("模型目录不是有效目录: %s", "models directory is not a valid directory: %s"), dir)
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
		Title: tr("选择模型目录", "Select Models Directory"),
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
		return fmt.Errorf(tr("非法 GPULayers %q：仅允许 auto/all/0 或正整数", "invalid GPULayers %q: only auto/all/0 or a positive integer"), config.GPULayers)
	}
	if !validCacheTypeValue(config.CacheTypeK) {
		return fmt.Errorf(tr("非法 CacheTypeK %q：仅允许 f32/f16/bf16/q8_0/q4_0/q4_1/iq4_nl/q5_0/q5_1", "invalid CacheTypeK %q: only f32/f16/bf16/q8_0/q4_0/q4_1/iq4_nl/q5_0/q5_1"), config.CacheTypeK)
	}
	if !validCacheTypeValue(config.CacheTypeV) {
		return fmt.Errorf(tr("非法 CacheTypeV %q：仅允许 f32/f16/bf16/q8_0/q4_0/q4_1/iq4_nl/q5_0/q5_1", "invalid CacheTypeV %q: only f32/f16/bf16/q8_0/q4_0/q4_1/iq4_nl/q5_0/q5_1"), config.CacheTypeV)
	}
	// b10342 新参数校验：LoadMode/SplitMode/RopeScaling 走白名单，
	// TensorSplit 走 INI 注入防御（含换行/首尾空白即拒绝）。
	if !validLoadModeValue(config.LoadMode) {
		return fmt.Errorf(tr("非法 LoadMode %q：仅允许 none/mmap/mlock/mmap+mlock/dio", "invalid LoadMode %q: only none/mmap/mlock/mmap+mlock/dio"), config.LoadMode)
	}
	if !validSplitModeValue(config.SplitMode) {
		return fmt.Errorf(tr("非法 SplitMode %q：仅允许 none/layer/row/tensor", "invalid SplitMode %q: only none/layer/row/tensor"), config.SplitMode)
	}
	if !validRopeScalingValue(config.RopeScaling) {
		return fmt.Errorf(tr("非法 RopeScaling %q：仅允许 none/linear/yarn", "invalid RopeScaling %q: only none/linear/yarn"), config.RopeScaling)
	}
	if !validIniValue(config.TensorSplit) {
		return fmt.Errorf(tr("非法 TensorSplit %q：不能包含换行或首尾空白", "invalid TensorSplit %q: must not contain newlines or leading/trailing whitespace"), config.TensorSplit)
	}
	// mmproj 显式路径覆盖：非空时必须通过 INI 注入校验（含换行/首尾空白即拒绝）；
	// 不要求文件存在（模型可能移动）。空值表示保持自动检测。
	if !validIniValue(config.MMProj) {
		return fmt.Errorf(tr("非法 MMProj %q：不能包含换行或首尾空白", "invalid MMProj %q: must not contain newlines or leading/trailing whitespace"), config.MMProj)
	}
	// spec-type 白名单：仅允许空或 draft-mtp。
	if !validSpecTypeValue(config.SpecType) {
		return fmt.Errorf(tr("非法 SpecType %q：仅允许 draft-mtp", "invalid SpecType %q: only draft-mtp"), config.SpecType)
	}
	if config.SpecDraftNMax < 0 {
		return fmt.Errorf(tr("非法 SpecDraftNMax %d：不能为负数", "invalid SpecDraftNMax %d: must not be negative"), config.SpecDraftNMax)
	}
	if config.MainGPU < 0 {
		return fmt.Errorf(tr("非法 MainGPU %d：不能为负数", "invalid MainGPU %d: must not be negative"), config.MainGPU)
	}
	if config.NCpuMoe < 0 {
		return fmt.Errorf(tr("非法 NCpuMoe %d：不能为负数", "invalid NCpuMoe %d: must not be negative"), config.NCpuMoe)
	}
	if config.RopeScale < 0 {
		return fmt.Errorf(tr("非法 RopeScale %g：不能为负数", "invalid RopeScale %g: must not be negative"), config.RopeScale)
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
	// 访问范围白名单：仅允许 local/lan 两档，避免任意 host 值把推理服务
	// 暴露到局域网/公网（#5）。通过后 Host 由 effectiveHost 按 AccessMode
	// 强制派生，不信任前端传入的 host。
	if cfg.AccessMode != accessLocal && cfg.AccessMode != accessLAN {
		return fmt.Errorf(tr("非法访问范围 %q：仅允许 local/lan", "invalid access mode %q: only local/lan"), cfg.AccessMode)
	}
	cfg.Host = effectiveHost(cfg.AccessMode)
	if cfg.Port < 1024 || cfg.Port > 65535 {
		return fmt.Errorf(tr("非法 Port %d：端口范围应为 1024-65535", "invalid Port %d: port must be in range 1024-65535"), cfg.Port)
	}
	if cfg.MaxModels < 1 {
		return fmt.Errorf(tr("非法 MaxModels %d：至少为 1", "invalid MaxModels %d: must be at least 1"), cfg.MaxModels)
	}
	if cfg.CacheRAM < 0 {
		return fmt.Errorf(tr("非法 CacheRAM %d：不能为负数", "invalid CacheRAM %d: must not be negative"), cfg.CacheRAM)
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
		Title: tr("选择 llama.cpp 目录", "Select llama.cpp Directory"),
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
		return fmt.Errorf(tr("没有可更新的版本: %s", "no update available for version %s"), version)
	}
	updateDownloadMu.Lock()
	inProgress := updateDownloadState.Status == "downloading"
	updateDownloadMu.Unlock()
	if inProgress {
		return errors.New(tr("更新下载已在进行中", "update download already in progress"))
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

// ─── Downloads (HF Mirror / ModelScope) ──────────────────────────

// SearchDownloads 按当前下载源路由搜索：modelscope → searchModelScopeAt，
// 其余（默认 hf）→ searchHFMirror。
func (a *App) SearchDownloads(query, filter string) ([]HFSearchResult, error) {
	if activeDownloadSource() == sourceModelScope {
		return searchModelScope(query)
	}
	return searchHFMirror(query, filter)
}

// GetModelFiles 按当前下载源路由文件列表：modelscope → listModelScopeFiles，
// 其余（默认 hf）→ getHFModelFiles。
func (a *App) GetModelFiles(modelID string) ([]HFFileOut, error) {
	if activeDownloadSource() == sourceModelScope {
		return listModelScopeFiles(modelID)
	}
	return getHFModelFiles(modelID)
}

// GetModelMaxFileSize 返回模型最大的 GGUF 文件大小（供搜索卡片展示模型大小）。
// modelscope 源用文件列表取最大；hf 源走详情接口（blobs=true 才有真实 size）。
func (a *App) GetModelMaxFileSize(modelID string) (int64, error) {
	if activeDownloadSource() == sourceModelScope {
		files, err := listModelScopeFiles(modelID)
		if err != nil {
			return 0, err
		}
		var max int64
		for _, f := range files {
			if f.Size > max {
				max = f.Size
			}
		}
		return max, nil
	}
	return getHFModelMaxGGUFSize(modelID)
}

// GetModelDescription 获取模型 README 中的自然语言描述（供下载页展示模型说明）。
// 按当前下载源路由到 HF 或 ModelScope 的 README 端点。
func (a *App) GetModelDescription(modelID string) (string, error) {
	if activeDownloadSource() == sourceModelScope {
		return getModelScopeDescription(modelID)
	}
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
	var found bool
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
			found = true
			break
		}
	}
	dlTasksMu.Unlock()
	// 状态变更后持久化：必须在解锁后调用（saveConfig 末尾会获取 dlTasksMu）。
	if found {
		persistTasksNow()
	}
	return nil
}

func (a *App) PauseDownloadTask(id string) error {
	dlTasksMu.Lock()
	var found bool
	for _, t := range dlTasks {
		if t.ID == id && t.Status == "downloading" {
			t.Status = "paused"
			t.resumeCh = make(chan struct{}, 1) // fresh channel for resume signal
			found = true
			break
		}
	}
	dlTasksMu.Unlock()
	if found {
		persistTasksNow()
	}
	return nil
}

func (a *App) ResumeDownloadTask(id string) error {
	dlTasksMu.Lock()
	var found bool
	for _, t := range dlTasks {
		if t.ID == id && t.Status == "paused" {
			t.Status = "downloading"
			// Signal resume (non-blocking send to buffered channel)
			select {
			case t.resumeCh <- struct{}{}:
			default:
			}
			found = true
			break
		}
	}
	dlTasksMu.Unlock()
	if found {
		persistTasksNow()
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
	var found bool
	for _, t := range dlTasks {
		if t.ID == id {
			if t.Status == "downloading" || t.Status == "paused" {
				found = true // 不重试，但该分支无状态变更，无需持久化
				break
			}
			retryDownloadTask(t)
			found = true
			break
		}
	}
	dlTasksMu.Unlock()
	// 状态变更后持久化（retryDownloadTask 置 Status=queued 并启动 goroutine）。
	if found {
		persistTasksNow()
	}
	return nil
}

// GetMonitorStatus 返回实时监控采样快照（CPU/内存/GPU 由后台采样器缓存，
// ServerRunning / TPS / UptimeSeconds 现取）。
func (a *App) GetMonitorStatus() *MonitorStatus {
	return GetMonitorStatus()
}
