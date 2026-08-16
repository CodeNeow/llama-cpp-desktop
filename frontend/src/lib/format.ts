/**
 * Formatting utilities: byte rates and byte sizes. All pure functions, easy to unit-test.
 */

/** Usage percentage: used/total with one decimal; returns 0 when total<=0 or used<0, clamps >100 to 100. */
export function usagePercent(used: number, total: number): number {
  if (total <= 0 || used < 0) return 0
  const pct = (used / total) * 100
  if (pct > 100) return 100
  return Math.round(pct * 10) / 10
}

/** Byte rate format: <1024 -> "X B/s", KB/s / MB/s / GB/s with one decimal; <=0 returns an empty string. */
export function formatSpeed(bytesPerSec: number): string {
  if (!(bytesPerSec > 0)) return ''
  if (bytesPerSec < 1024) return `${bytesPerSec} B/s`
  if (bytesPerSec < 1024 * 1024) return `${(bytesPerSec / 1024).toFixed(1)} KB/s`
  if (bytesPerSec < 1024 * 1024 * 1024) return `${(bytesPerSec / (1024 * 1024)).toFixed(1)} MB/s`
  return `${(bytesPerSec / (1024 * 1024 * 1024)).toFixed(1)} GB/s`
}

/** Byte size format (same behavior as the downloads page's original formatSize): <=0 returns an empty string. */
export function formatBytes(bytes: number): string {
  if (!(bytes > 0)) return ''
  if (bytes < 1024 * 1024) return `${(bytes / 1024).toFixed(0)} KB`
  if (bytes < 1024 * 1024 * 1024) return `${(bytes / (1024 * 1024)).toFixed(1)} MB`
  return `${(bytes / (1024 * 1024 * 1024)).toFixed(2)} GB`
}
