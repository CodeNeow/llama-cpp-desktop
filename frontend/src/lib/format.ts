/**
 * 格式化工具：字节速率与字节大小。全部为纯函数，便于单测。
 */

/** 使用率百分比：used/total 保留一位小数；total<=0 或 used<0 返回 0，>100 截断为 100。 */
export function usagePercent(used: number, total: number): number {
  if (total <= 0 || used < 0) return 0
  const pct = (used / total) * 100
  if (pct > 100) return 100
  return Math.round(pct * 10) / 10
}

/** 字节速率格式化：<1024 → "X B/s"，KB/s / MB/s / GB/s 一位小数；<=0 返回空串。 */
export function formatSpeed(bytesPerSec: number): string {
  if (!(bytesPerSec > 0)) return ''
  if (bytesPerSec < 1024) return `${bytesPerSec} B/s`
  if (bytesPerSec < 1024 * 1024) return `${(bytesPerSec / 1024).toFixed(1)} KB/s`
  if (bytesPerSec < 1024 * 1024 * 1024) return `${(bytesPerSec / (1024 * 1024)).toFixed(1)} MB/s`
  return `${(bytesPerSec / (1024 * 1024 * 1024)).toFixed(1)} GB/s`
}

/** 字节大小格式化（与下载页原有 formatSize 行为一致）：<=0 返回空串。 */
export function formatBytes(bytes: number): string {
  if (!(bytes > 0)) return ''
  if (bytes < 1024 * 1024) return `${(bytes / 1024).toFixed(0)} KB`
  if (bytes < 1024 * 1024 * 1024) return `${(bytes / (1024 * 1024)).toFixed(1)} MB`
  return `${(bytes / (1024 * 1024 * 1024)).toFixed(2)} GB`
}
