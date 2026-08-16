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
 * shouldShowDock: whether the TaskDock card needs to be shown:
 * shown when the llama.cpp download is active, or there are model download tasks,
 * or there are in-memory models.
 */
export function shouldShowDock(
  llamaActive: boolean,
  activeTaskCount: number,
  loadedCount: number
): boolean {
  return llamaActive || activeTaskCount > 0 || loadedCount > 0
}
