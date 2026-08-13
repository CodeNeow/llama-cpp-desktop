/**
 * 格式化工具：字节速率与字节大小。全部为纯函数，便于单测。
 */

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
