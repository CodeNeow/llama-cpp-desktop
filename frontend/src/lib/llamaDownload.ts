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
