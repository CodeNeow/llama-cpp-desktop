/**
 * Wails v2 bridge — replaces all fetch('http://localhost:18080/api/...') calls
 * with direct Go method bindings via window.go.core.App.*.
 * The `core` namespace comes from the Go package core/ that hosts the App struct.
 */

import type { MonitorStatus } from './lib/monitor'

// Wails injects window.go when running in the Wails runtime.
// When running standalone (vite dev only), we fall back to a mock/error.

function app(): any {
  const w = window as any
  if (w.go?.core?.App) return w.go.core.App
  throw new Error('Wails backend not available. Run with `wails dev` instead of `vite`.')
}

// ─── Config ─────────────────────────────────────────────────────

export async function getConfig(): Promise<{ theme: string; llamaCppDir: string; modelsDir: string; llamaCppDownloadDir?: string; modelDownloadDir?: string; downloadSource: string; language: string; resolvedLanguage: 'zh' | 'en'; trayEnabled: boolean; apiRouteMode?: boolean; sidebarCollapsed?: boolean }> {
  return app().GetConfig()
}

export async function setTheme(theme: string): Promise<void> {
  return app().SetTheme(theme)
}

// Set the sidebar collapsed/expanded preference (pure UI state, no window-related side effects)
export async function setSidebarCollapsed(collapsed: boolean): Promise<void> {
  return app().SetSidebarCollapsed(collapsed)
}

// Set the Windows system tray toggle (settings item rendered on Windows only; on other
// platforms the backend just persists without starting/stopping anything). systray cannot
// be started twice in one process: once disabled it stays disabled until the app restarts.
export async function setTrayEnabled(enabled: boolean): Promise<void> {
  return app().SetTrayEnabled(enabled)
}

// Toggle API-route (headless) mode (settings item rendered on Windows only). Enabling
// persists the preference, hands the running llama-server over, relaunches the app in
// headless mode (tray + llama-server only, no GUI/WebView2) and quits — the service
// keeps serving the OpenAI API without interruption. From headless, the tray "Show Main
// Window" item returns to GUI mode. On success this call never resolves visibly in the
// GUI (the app quits); failures reject for inline error display.
export async function setApiRouteMode(enabled: boolean): Promise<void> {
  return app().SetApiRouteMode(enabled)
}

// Set the UI language preference ("zh" | "en" | "auto"); the backend returns the effective
// resolvedLanguage (zh/en; auto is resolved from system detection), which store uses to
// refresh the UI copy
export async function setLanguage(language: string): Promise<string> {
  return app().SetLanguage(language)
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

// ─── Monitor ─────────────────────────────────────────────────────

// Real-time monitoring (CPU / memory / GPU / inference prompt processing and generation
// decode speed); return shape matches MonitorStatus in lib/monitor.ts
export async function getMonitorStatus(): Promise<MonitorStatus> {
  return app().GetMonitorStatus()
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

// ServerConfig mirrors the backend core.ServerConfig: accessMode is the service access
// scope ("local" | "lan"), host is the actual listen address derived from accessMode.
export interface ServerConfig {
  accessMode: string
  host: string
  port: number
  maxModels: number
  cacheRam: number
}

export async function getServerConfig(): Promise<ServerConfig> {
  return app().GetServerConfig()
}

export async function saveServerConfig(cfg: ServerConfig): Promise<void> {
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

// Choose the llama.cpp download path: opens the system folder picker, persists the
// choice, and returns the selected directory string (empty string when cancelled)
export async function browseLlamaCppDownloadDir(): Promise<string> {
  return app().BrowseLlamaCppDownloadDir()
}

// Choose the models directory: opens the system folder picker and returns the selected
// directory string (empty string when the dialog is cancelled)
export async function browseModelsDir(): Promise<string> {
  return app().BrowseModelsDir()
}

// Choose the model download path: opens the system folder picker, persists the
// choice, and returns the selected directory string (empty string when cancelled)
export async function browseModelDownloadDir(): Promise<string> {
  return app().BrowseModelDownloadDir()
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

export async function getUpdateDownloadStatus(): Promise<{ status: string; progress: number; total: number; downloaded: number; version: string; filePath: string; error: string; kind: string; installer: boolean }> {
  return app().GetUpdateDownloadStatus()
}

export async function stopUpdateDownload(): Promise<void> {
  return app().StopUpdateDownload()
}

// Launch the downloaded setup installer and exit the app (user confirmed
// "install now"); rejects when the download is not a finished installer
export async function installUpdate(): Promise<void> {
  return app().InstallUpdate()
}

// ─── Downloads (HF Mirror) ───────────────────────────────────────

export async function searchDownloads(query: string, filter: string): Promise<any[]> {
  return app().SearchDownloads(query, filter)
}

export async function getModelFiles(modelID: string): Promise<any[]> {
  return app().GetModelFiles(modelID)
}

// Get the model's largest GGUF file size (bytes) for the search card display;
// 0 means no GGUF or query failure, handled silently by the caller
export async function getModelMaxFileSize(modelID: string): Promise<number> {
  return app().GetModelMaxFileSize(modelID)
}

// Get the first paragraph of the model README (<=200 chars); returns an error when
// there is no README or the read fails, handled silently by the caller
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

export async function retryDownloadTask(id: string): Promise<void> {
  return app().RetryDownloadTask(id)
}

export async function pauseDownloadTask(id: string): Promise<void> {
  return app().PauseDownloadTask(id)
}

export async function resumeDownloadTask(id: string): Promise<void> {
  return app().ResumeDownloadTask(id)
}

// ─── Download Source ─────────────────────────────────────────────

// Current model download source ("hf" | "modelscope"); decides which mirror backend search and download use
export async function getDownloadSource(): Promise<string> {
  return app().GetDownloadSource()
}

export async function setDownloadSource(source: string): Promise<void> {
  return app().SetDownloadSource(source)
}

// ─── Router Models (TaskDock in-memory model list + unload) ─────────────────

// LoadedModel mirrors the backend core.LoadedModel: id / type (chat|audio|image|video) / status (loaded|loading|sleeping)
export interface LoadedModel { id: string; type: string; status: string }

// getLoadedModels queries the llama-server router for models currently loaded /
// loading / sleeping; returns an empty array when the service is not running or the
// query fails (the frontend polls and retries).
export async function getLoadedModels(): Promise<LoadedModel[]> {
  return (await app().GetLoadedModels()) ?? []
}

// unloadModel sends a model unload request to llama-server; on failure the frontend shows the error inline.
export async function unloadModel(id: string): Promise<void> {
  return app().UnloadModel(id)
}
