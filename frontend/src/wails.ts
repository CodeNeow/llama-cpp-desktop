/**
 * Wails v2 bridge — replaces all fetch('http://localhost:18080/api/...') calls
 * with direct Go method bindings via window.go.main.App.*.
 */

// Wails injects window.go when running in the Wails runtime.
// When running standalone (vite dev only), we fall back to a mock/error.

function app(): any {
  const w = window as any
  if (w.go?.main?.App) return w.go.main.App
  throw new Error('Wails backend not available. Run with `wails dev` instead of `vite`.')
}

// ─── Config ─────────────────────────────────────────────────────

export async function getConfig(): Promise<{ theme: string; llamaCppDir: string }> {
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

// ─── Downloads (HF Mirror) ───────────────────────────────────────

export async function searchDownloads(query: string, filter: string): Promise<any[]> {
  return app().SearchDownloads(query, filter)
}

export async function getModelFiles(modelID: string): Promise<any[]> {
  return app().GetModelFiles(modelID)
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
