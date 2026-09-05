/**
 * TaskDock pure functions: display conditions for download/model-related UI and task filtering.
 * - No external dependencies, pure input/output, easy to cover with vitest.
 */

/**
 * activeLlamaCppDownload: whether the llama.cpp download is "active":
 * fetching / downloading / paused / extracting / error all count (idle / done hidden).
 */
export function activeLlamaCppDownload(status: string): boolean {
  switch (status) {
  case 'fetching':
  case 'downloading':
  case 'paused':
  case 'extracting':
  case 'error':
    return true
  default:
    return false // idle / done
  }
}

/**
 * activeModelTasks filters the download task list down to the "active" tasks to show:
 * queued / fetching / downloading / paused / extracting are shown;
 * done / error / cancelled are hidden.
 */
export function activeModelTasks<T extends { status: string }>(tasks: T[]): T[] {
  const out: T[] = []
  for (const t of tasks) {
    switch (t.status) {
    case 'queued':
    case 'fetching':
    case 'downloading':
    case 'paused':
    case 'extracting':
      out.push(t)
    }
  }
  return out
}

/**
 * activeUpdateDownload: whether the app self-update download row is shown in the
 * dock: downloading / installing / done / error count; idle / empty / unknown
 * are hidden. After backgrounding, the outcome stays visible until the user
 * reopens the modal and closes it (which clears the terminal state).
 */
export function activeUpdateDownload(status: string): boolean {
  switch (status) {
  case 'downloading':
  case 'installing':
  case 'done':
  case 'error':
    return true
  default:
    return false // idle / empty / unknown
  }
}

/**
 * shouldShowDock: whether the TaskDock card needs to be shown:
 * shown when the llama.cpp download is active, or there are model download tasks,
 * or there are in-memory models, or the app self-update download is active.
 */
export function shouldShowDock(
  llamaActive: boolean,
  activeTaskCount: number,
  loadedCount: number,
  updateActive: boolean
): boolean {
  return llamaActive || activeTaskCount > 0 || loadedCount > 0 || updateActive
}

/** How a resident model leaves memory on the current platform. */
export type ResidentReleaseMode = 'router-unload' | 'stop-server'

/**
 * residentReleaseMode: which binding serves "unload this model" for the
 * given platform. Desktop router mode has POST /models/unload per model;
 * direct-mode Android runs one resident per server process with no unload
 * route, so releasing memory means stopping the service (the next chat send
 * auto-restarts it). Pure: the callers fetch no state for this decision.
 */
export function residentReleaseMode(isAndroid: boolean): ResidentReleaseMode {
  return isAndroid ? 'stop-server' : 'router-unload'
}

/**
 * homeResidentShowsUnload: whether the Home resident-model card offers the
 * unload action. Android shows STATUS ONLY (design-draft platform crop, frames
 * A②/B②): direct mode swaps models automatically on the next chat, so a
 * manual unload button has nothing to do — the card renders the "auto switch"
 * pill in its place instead.
 */
export function homeResidentShowsUnload(isAndroid: boolean): boolean {
  return !isAndroid
}

/**
 * dockShowsCompactSkin: whether the TaskDock renders the design-draft capsule
 * skin (frame ⑲: dark edge-hugging capsule + expanded glass task card) instead
 * of the desktop segments pill. True on the phone tier AND on the whole tablet
 * tier — the draft gives tablets the same capsule interaction in BOTH
 * orientations ("双方向同一交互"), and the phase 0 classifier folds the Android
 * tablet-landscape band into isTablet, so one flag covers the portrait band
 * (768..1099) and the landscape band (Android, 1100..1360) alike. Desktop OS
 * windows above the portrait band are never tablet, so they keep the desktop
 * pill unchanged. Pure: the caller reads the platform state.
 */
export function dockShowsCompactSkin(isMobile: boolean, isTablet: boolean): boolean {
  return isMobile || isTablet
}
