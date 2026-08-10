/**
 * 下载区显示条件：承载 Home.vue 下载区三个区块（按钮组 / 自定义路径信息 /
 * 进度区）的显示逻辑，防止再次出现 v-if/v-else-if 互斥导致进度区被吞
 * （此前自定义路径存在时进度区永远不渲染，点击下载后按钮消失且无任何反馈）。
 *
 * - showButtons：idle / error 时显示"下载 llama.cpp / 自定义"按钮组；
 * - showProgress：非 idle 状态（fetching / downloading / paused / extracting /
 *   error / done）显示进度区——只依赖下载状态，与是否设置自定义路径无关。
 */
export function downloadVisibility(status: string): { showButtons: boolean; showProgress: boolean } {
  return {
    showButtons: status === 'idle' || status === 'error',
    showProgress: status !== 'idle',
  }
}

/**
 * 主页挂载时对下载状态的初始处理动作（checkInitialDownloadStatus 的纯函数驱动源）。
 *
 * llama.cpp 下载由后端 Go goroutine 执行，切页不会中断；返回主页时
 * onMounted 需根据后端当前下载状态决定恢复方式：
 *
 * - 'poll'：downloading / fetching / extracting——下载仍在进行，恢复 500ms 轮询
 *   以持续更新进度（与是否已安装无关，下载中必定未完成安装）。
 * - 'refresh'：done 且未检测到安装——下载已完成但系统信息仍是旧值，刷新系统信息。
 * - 'showError'：error——切页期间下载失败；此前缺少该分支导致返回主页时
 *   状态回落为 idle，错误信息（"下载失败: ..."）丢失、只剩"下载"按钮。
 *   error 状态下 downloadVisibility 的 showButtons / showProgress 均覆盖，
 *   UI 会自动显示按钮组与 dl-error 错误信息 + 重试按钮。
 * - 'none'：idle（无下载）或 done 且已安装（onMounted 的 fetchSystemInfo
 *   已覆盖已安装场景），无需额外处理。
 */
export type InitialDownloadAction = 'poll' | 'refresh' | 'showError' | 'none'

export function initialDownloadAction(status: string, installed: boolean): InitialDownloadAction {
  if (status === 'downloading' || status === 'fetching' || status === 'extracting') {
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
