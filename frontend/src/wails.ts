/**
 * Wails v2 bridge — replaces all fetch('http://localhost:18080/api/...') calls
 * with direct Go method bindings via window.go.core.App.*.
 * The `core` namespace comes from the Go package core/ that hosts the App struct.
 */

// Wails injects window.go when running in the Wails runtime.
// When running standalone (vite dev only), we fall back to a mock/error.

function app(): any {
  const w = window as any
  if (w.go?.core?.App) return w.go.core.App
  throw new Error('Wails backend not available. Run with `wails dev` instead of `vite`.')
}

// ─── Config ─────────────────────────────────────────────────────

export async function getConfig(): Promise<{ theme: string; llamaCppDir: string; modelsDir: string }> {
  return app().GetConfig()
}

export async function setTheme(theme: string): Promise<void> {
  return app().SetTheme(theme)
}

// ─── System Info ─────────────────────────────────────────────────

export async function getSystemInfo(): Promise<any> {
  return app().GetSystemInfo()
}

export async function getCPU(): Promise<any> {
  return app().GetCPU()
}

export async function getMemory(): Promise<any> {
  return app().GetMemory()
}

export async function getGPU(): Promise<any> {
  return app().GetGPU()
}

export async function getCUDA(): Promise<any> {
  return app().GetCUDA()
}

export async function getLlamaCpp(): Promise<any> {
  return app().GetLlamaCpp()
}

export async function getOS(): Promise<{ os: string; arch: string }> {
  return app().GetOS()
}

// ─── Models ──────────────────────────────────────────────────────

export async function getModels(): Promise<any[]> {
  return app().GetModels()
}

export async function refreshModels(): Promise<any[]> {
  return app().RefreshModels()
}

export async function getModelConfig(modelID: string): Promise<any> {
  return app().GetModelConfig(modelID)
}

export async function saveModelConfig(modelID: string, config: any): Promise<void> {
  return app().SaveModelConfig(modelID, config)
}

// ─── Server ──────────────────────────────────────────────────────

export async function getServerConfig(): Promise<any> {
  return app().GetServerConfig()
}

export async function saveServerConfig(cfg: any): Promise<void> {
  return app().SaveServerConfig(cfg)
}

export async function getServerStatus(): Promise<{ running: boolean; log: string[] }> {
  return app().GetServerStatus()
}

export async function startServer(): Promise<void> {
  return app().StartServer()
}

export async function stopServer(): Promise<void> {
  return app().StopServer()
}

// ─── llama.cpp Download ──────────────────────────────────────────

export async function getLlamaCppDownloadStatus(): Promise<any> {
  return app().GetLlamaCppDownloadStatus()
}

export async function startLlamaCppDownload(): Promise<void> {
  return app().StartLlamaCppDownload()
}

export async function pauseLlamaCppDownload(): Promise<void> {
  return app().PauseLlamaCppDownload()
}

export async function resumeLlamaCppDownload(): Promise<void> {
  return app().ResumeLlamaCppDownload()
}

export async function stopLlamaCppDownload(): Promise<void> {
  return app().StopLlamaCppDownload()
}

export async function browseLlamaCppDir(): Promise<string> {
  return app().BrowseLlamaCppDir()
}

// 选择模型目录：弹出系统文件夹选择框，返回所选目录字符串（对话框取消返回空串）
export async function browseModelsDir(): Promise<string> {
  return app().BrowseModelsDir()
}

// ─── App Update ──────────────────────────────────────────────────

export async function getAppVersion(): Promise<string> {
  return app().GetAppVersion()
}

export async function checkForUpdate(): Promise<{ hasUpdate: boolean; version: string; notes: string; published: string }> {
  return app().CheckForUpdate()
}

export async function startUpdateDownload(version: string): Promise<void> {
  return app().StartUpdateDownload(version)
}

export async function getUpdateDownloadStatus(): Promise<{ status: string; progress: number; total: number; downloaded: number; version: string; filePath: string; error: string }> {
  return app().GetUpdateDownloadStatus()
}

export async function stopUpdateDownload(): Promise<void> {
  return app().StopUpdateDownload()
}

// ─── Downloads (HF Mirror) ───────────────────────────────────────

export async function searchDownloads(query: string, filter: string): Promise<any[]> {
  return app().SearchDownloads(query, filter)
}

export async function getModelFiles(modelID: string): Promise<any[]> {
  return app().GetModelFiles(modelID)
}

// 获取模型最大的 GGUF 文件大小（字节数），供搜索卡片展示模型大小；
// 返回 0 表示无 GGUF 或查询失败，由调用方静默处理
export async function getModelMaxFileSize(modelID: string): Promise<number> {
  return app().GetModelMaxFileSize(modelID)
}

// 获取模型 README 首段描述（≤200 字）；无 README/读取失败时返回错误，由调用方静默兜底
export async function getModelDescription(modelID: string): Promise<string> {
  return app().GetModelDescription(modelID)
}

export async function startDownload(modelID: string, files: string[]): Promise<void> {
  return app().StartDownload(modelID, files)
}

export async function getDownloadTasks(): Promise<any[]> {
  return app().GetDownloadTasks()
}

export async function cancelDownloadTask(id: string): Promise<void> {
  return app().CancelDownloadTask(id)
}

export async function pauseDownloadTask(id: string): Promise<void> {
  return app().PauseDownloadTask(id)
}

export async function resumeDownloadTask(id: string): Promise<void> {
  return app().ResumeDownloadTask(id)
}
