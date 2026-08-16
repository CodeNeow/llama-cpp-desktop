import { reactive } from 'vue'
import {
  checkForUpdate as checkForUpdateBackend,
  startUpdateDownload as startUpdateDownloadBackend,
  stopUpdateDownload as stopUpdateDownloadBackend,
  getUpdateDownloadStatus,
} from '../wails'
import { t } from './i18n'

// Auto-check throttle: skip auto-check within 48 hours of the last check (local time).
// Manual checks are not throttled.
export const CHECK_INTERVAL_MS = 48 * 60 * 60 * 1000
const CHECK_KEY = 'llama-desktop-last-update-check'
// Legacy key from before the llama-gui → llama-desktop rename: read-only fallback
// preserving the throttle timestamp of older installs
const LEGACY_CHECK_KEY = 'llama-gui-last-update-check'

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
  kind: string // Artifact kind of this download: setup (installer) / portable
}

export const updateState = reactive({
  checking: false,
  result: null as UpdateResult | null,
  download: null as UpdateDownloadState | null,
  showModal: false,
  error: '',
})

let downloadTimer: ReturnType<typeof setInterval> | null = null

/** Whether more than 48 hours have passed since the last check (or it never happened). */
export function shouldAutoCheck(now = Date.now()): boolean {
  // Fall back to the legacy key when the new key is missing (rename migration), so
  // older installs do not get their throttle window reset into a duplicate check prompt
  const last = Number(localStorage.getItem(CHECK_KEY) || localStorage.getItem(LEGACY_CHECK_KEY) || 0)
  if (!last) return true
  return now - last > CHECK_INTERVAL_MS
}

function writeCheckTime(now = Date.now()) {
  localStorage.setItem(CHECK_KEY, String(now))
}

/**
 * Check for a new version. The last-check time is refreshed once the check
 * completes (regardless of outcome or failure), avoiding "just checked manually,
 * auto-check fires again on next launch". Shows the update modal when a new
 * version is found.
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
    // Version comparison is authoritative on the backend currentVersion; here it is display only
    updateState.showModal = result.hasUpdate
  } catch {
    writeCheckTime()
    updateState.error = t('update.checkFailed')
  } finally {
    updateState.checking = false
  }
}

/** User accepted the update: start downloading the new version and poll progress. */
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
    kind: '', // Artifact kind unknown before the download starts; filled from the backend status once polled
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

/** Fetch the download status once; stop polling when finished. */
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

/**
 * Close the update modal. While still downloading, polling keeps running so the
 * background progress row (TaskDock) stays live. Otherwise polling stops and a
 * terminal download state (done/error) is cleared so the dock row does not linger.
 */
export function closeUpdateModal(): void {
  updateState.showModal = false
  if (updateState.download?.status === 'downloading') return
  stopPolling()
  const st = updateState.download?.status
  if (st === 'done' || st === 'error') updateState.download = null
}

/**
 * User cancelled the download: stop the backend download and the polling, clear
 * the download state, and keep the modal open so the user lands back on the
 * confirm view (updateState.result is untouched). Backend failures are swallowed:
 * the UI has already reset, so there is nothing meaningful left to surface.
 */
export async function cancelUpdateDownload(): Promise<void> {
  stopPolling()
  updateState.download = null
  try {
    await stopUpdateDownloadBackend()
  } catch {
    // backend already idle or unavailable: ignore
  }
}
