/**
 * Download section display conditions: drives the visibility logic of the
 * Runtime page's download area's three blocks (button group / custom path
 * info / progress area), preventing a recurrence of v-if/v-else-if mutual
 * exclusion swallowing the progress area (previously, when a custom path
 * existed the progress area never rendered: after clicking download the
 * buttons vanished with no feedback).
 *
 * - showButtons: show the "download llama.cpp / custom" button group on idle / error;
 * - showProgress: show the progress area for non-idle states (fetching /
 *   downloading / paused / extracting / error / done) — depends only on the
 *   download status, not on whether a custom path is set.
 */
export function downloadVisibility(status: string): { showButtons: boolean; showProgress: boolean } {
  return {
    showButtons: status === 'idle' || status === 'error',
    showProgress: status !== 'idle',
  }
}

/**
 * Initial download-status action when the home page mounts (pure-function source
 * behind checkInitialDownloadStatus).
 *
 * The llama.cpp download runs in a backend Go goroutine and survives page
 * switches; on returning to the home page, onMounted must decide how to resume
 * from the backend's current download status:
 *
 * - 'poll': downloading / fetching / paused / extracting — the download is
 *   still in flight (paused included; the backend goroutine keeps the state
 *   and the user can resume), resume 500ms polling to keep progress updated
 *   (independent of install state; a download in flight cannot have finished
 *   installing).
 * - 'refresh': done but install not detected — the download finished while system
 *   info is stale; refresh the system info.
 * - 'showError': error — the download failed while away; previously this branch
 *   was missing, so returning to the home page fell back to idle, losing the
 *   error message ("download failed: ...") and leaving only the download button.
 *   In the error state both downloadVisibility showButtons / showProgress cover
 *   it, and the UI automatically shows the button group plus the dl-error
 *   message and retry button.
 * - 'none': idle (no download) or done and already installed (onMounted's
 *   fetchSystemInfo already covers the installed case); nothing extra to do.
 */
export type InitialDownloadAction = 'poll' | 'refresh' | 'showError' | 'none'

export function initialDownloadAction(status: string, installed: boolean): InitialDownloadAction {
  if (status === 'downloading' || status === 'fetching' || status === 'paused' || status === 'extracting') {
    return 'poll'
  }
  if (status === 'done' && !installed) {
    return 'refresh'
  }
  if (status === 'error') {
    return 'showError'
  }
  return 'none'
}

/**
 * Whether a llama.cpp release asset name is the cudart runtime package
 * (e.g. "cudart-llama-bin-win-cuda-12.4-x64.zip"): on Windows CUDA builds the
 * runtime ships as a separate asset downloaded after the main program and
 * extracted into the same directory.
 */
export function isCudartAsset(name: string): boolean {
  return name.toLowerCase().includes('cudart')
}

export interface DownloadPackageRow {
  id: 'main' | 'cudart'
  /** Row currently downloading (exactly one while a file name is present) */
  active: boolean
  /** Package fully downloaded (the main program finishes before cudart starts) */
  done: boolean
  /** 0–100 clamped; a row without a measurable byte share reports 0 */
  progress: number
}

/**
 * Split the combined llama.cpp download progress into per-package rows for the
 * Runtime page. The backend downloads assets sequentially (main program first,
 * then the cudart runtime) with Downloaded/Total accumulating across both and
 * FileName naming the asset currently in flight:
 * - while the main asset downloads there is no way to know whether a cudart
 *   asset will follow (CPU/Vulkan builds never ship one), so only one row is
 *   returned — a perpetually "waiting" cudart row would be misleading there;
 * - once the cudart asset becomes current, the main package is complete by
 *   construction; mainBytes is the cumulative byte count the caller snapshotted
 *   when the file name switched, so the cudart row's share is
 *   (downloaded − mainBytes) / (total − mainBytes).
 */
export function packageRows(fileName: string, downloaded: number, total: number, mainBytes: number): DownloadPackageRow[] {
  const pct = (part: number, whole: number): number =>
    whole > 0 ? Math.min(100, Math.max(0, Math.round((part / whole) * 100))) : 0

  if (!isCudartAsset(fileName)) {
    return [{ id: 'main', active: true, done: false, progress: pct(downloaded, total) }]
  }
  return [
    { id: 'main', active: false, done: true, progress: 100 },
    { id: 'cudart', active: true, done: false, progress: pct(downloaded - mainBytes, total - mainBytes) },
  ]
}
