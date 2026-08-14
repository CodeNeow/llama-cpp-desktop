import { reactive } from 'vue'
import {
  checkForUpdate as checkForUpdateBackend,
  startUpdateDownload as startUpdateDownloadBackend,
  getUpdateDownloadStatus,
} from '../wails'
import { t } from './i18n'

// 自动检查节流：距上次检查不足 48 小时不再自动检查（本地时间）。
// 手动检查不受此限制。
export const CHECK_INTERVAL_MS = 48 * 60 * 60 * 1000
const CHECK_KEY = 'llama-gui-last-update-check'

export interface UpdateResult {
  hasUpdate: boolean
  version: string
  notes: string
  published: string
}

export interface UpdateDownloadState {
  status: string // idle / downloading / done / error
  progress: number
  total: number
  downloaded: number
  version: string
  filePath: string
  error: string
}

export const updateState = reactive({
  checking: false,
  result: null as UpdateResult | null,
  download: null as UpdateDownloadState | null,
  showModal: false,
  error: '',
})

let downloadTimer: ReturnType<typeof setInterval> | null = null

/** 距上次检查是否已超过 48 小时（或从未检查过）。 */
export function shouldAutoCheck(now = Date.now()): boolean {
  const last = Number(localStorage.getItem(CHECK_KEY) || 0)
  if (!last) return true
  return now - last > CHECK_INTERVAL_MS
}

function writeCheckTime(now = Date.now()) {
  localStorage.setItem(CHECK_KEY, String(now))
}

/**
 * 检查是否有新版本。检查完成（无论结果或失败）都刷新最近检查时间，
 * 避免「刚手动检查完，下次启动又自动检查」。发现新版本时弹出更新窗口。
 */
export async function checkForUpdate(): Promise<void> {
  updateState.checking = true
  updateState.error = ''
  try {
    const result = await checkForUpdateBackend()
    writeCheckTime()
    updateState.result = {
      hasUpdate: result.hasUpdate,
      version: result.version,
      notes: result.notes || '',
      published: result.published || '',
    }
    // 版本比较以后端 currentVersion 为准，这里仅弹窗展示
    updateState.showModal = result.hasUpdate
  } catch {
    writeCheckTime()
    updateState.error = t('update.checkFailed')
  } finally {
    updateState.checking = false
  }
}

/** 用户同意更新，开始下载新版本并轮询进度。 */
export function startUpdateDownload(): void {
  const version = updateState.result?.version
  if (!version) return
  updateState.download = {
    status: 'downloading',
    progress: 0,
    total: 0,
    downloaded: 0,
    version,
    filePath: '',
    error: '',
  }
  startUpdateDownloadBackend(version).catch(() => {
    if (updateState.download) {
      updateState.download.status = 'error'
      updateState.download.error = t('update.startFailed')
    }
    stopPolling()
  })
  if (downloadTimer) clearInterval(downloadTimer)
  downloadTimer = setInterval(() => pollUpdateDownload(), 1000)
}

/** 拉取一次下载状态；结束后停止轮询。 */
export async function pollUpdateDownload(): Promise<void> {
  try {
    const st = await getUpdateDownloadStatus()
    updateState.download = st
    if (st.status === 'done' || st.status === 'error') stopPolling()
  } catch {
    stopPolling()
  }
}

export function stopPolling(): void {
  if (downloadTimer) {
    clearInterval(downloadTimer)
    downloadTimer = null
  }
}

export function closeUpdateModal(): void {
  updateState.showModal = false
  stopPolling()
}
