/**
 * Download section display conditions: drives the visibility logic of Home.vue's
 * download area's three blocks (button group / custom path info / progress area),
 * preventing a recurrence of v-if/v-else-if mutual exclusion swallowing the
 * progress area (previously, when a custom path existed the progress area never
 * rendered: after clicking download the buttons vanished with no feedback).
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
