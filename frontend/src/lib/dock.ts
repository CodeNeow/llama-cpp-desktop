/**
 * TaskDock 纯函数：判断下载/模型相关 UI 的显示条件与任务过滤。
 * - 无外部依赖，纯输入输出，便于 vitest 覆盖。
 */

/**
 * activeLlamaCppDownload 判断 llama.cpp 下载是否处于「活跃」状态：
 * fetching / downloading / paused / extracting / error 均算活跃（idle / done 隐藏）。
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
 * activeModelTasks 过滤下载任务列表中需要展示的「活跃」任务：
 * queued / fetching / downloading / paused / extracting 展示；
 * done / error / cancelled 隐藏。
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
 * shouldShowDock 判断 TaskDock 卡片是否需要显示：
 * llama.cpp 下载活跃、或有模型下载任务、或有内存中的模型时显示。
 */
export function shouldShowDock(
  llamaActive: boolean,
  activeTaskCount: number,
  loadedCount: number
): boolean {
  return llamaActive || activeTaskCount > 0 || loadedCount > 0
}
