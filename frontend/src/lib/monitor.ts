/**
 * Monitor data pure-function utilities: used by Api.vue to maintain generation-speed
 * chart history, compute SVG polyline coordinates, and format uptime (the former
 * standalone Monitor.vue page was merged into Api.vue). All pure functions for easy
 * unit testing; the MonitorStatus interface maps one-to-one to the backend
 * GetMonitorStatus JSON contract.
 */

import type { Locale } from './i18n'

export interface MonitorStatus {
  cpuPercent: number
  memUsed: number
  memTotal: number
  gpus: { index: number; name: string; utilPercent: number; memUsed: number; memTotal: number }[]
  serverRunning: boolean
  /** Prompt processing speed in tokens/s (prefill): refreshed per batch during prefill (prompt processing line, newer llama.cpp), set to the final value when the request ends (prompt eval time line) */
  promptTps: number
  /** Generation speed in tokens/s (live decode): during generation refreshed roughly every 3s by tg_3s log lines, with the eval time line as fallback when the request ends */
  decodeTps: number
  uptimeSeconds: number
  /** Disk usage of the volume holding the models directory; null when sampling fails/is unavailable and the frontend hides the disk row */
  disk?: { path: string; used: number; total: number } | null
}

/** Append one sample to the history array; drop the oldest beyond cap, return a new array (input not mutated). */
export function appendHistory(history: number[], value: number, cap = 60): number[] {
  const next = [...history, value]
  if (next.length > cap) next.splice(0, next.length - cap)
  return next
}

/**
 * Convert a history sequence into an SVG polyline points string:
 * - x evenly distributed (single point horizontally centered);
 * - y bottom-aligned with a 2px margin, larger values higher;
 * - max defaults to the history maximum with a floor of 1, so an all-zero sequence never divides by zero.
 */
export function chartPoints(history: number[], width: number, height: number, max?: number): string {
  if (history.length === 0) return ''
  const n = history.length
  const scale = (max ?? Math.max(...history)) || 1
  const usable = height - 4
  return history
    .map((v, i) => {
      const x = n === 1 ? width / 2 : (i / (n - 1)) * width
      const normalized = Math.max(0, Math.min(1, v / scale))
      const y = height - 2 - normalized * usable
      return `${x.toFixed(1)},${y.toFixed(1)}`
    })
    .join(' ')
}

/**
 * Display rule for prompt processing speed: while the service is running but no
 * measurement exists yet (tps <= 0, including NaN), show the "—" placeholder; once
 * measured, keep one decimal. Distinct from generation speed (a live 0.0 still shows 0.0).
 */
export function formatPromptTps(tps: number): string {
  return tps > 0 ? tps.toFixed(1) : '—'
}

/**
 * Format seconds as an uptime string in the given locale (zh/en); 0 seconds
 * shows 0 in both languages. The locale parameter is passed explicitly, keeping
 * the function pure and easy to unit-test per UI language.
 */
export function formatUptime(seconds: number, locale: Locale): string {
  const s = Math.max(0, Math.floor(seconds))
  if (locale === 'en') {
    if (s < 60) return `${s}s`
    if (s < 3600) return `${Math.floor(s / 60)}m`
    const h = Math.floor(s / 3600)
    const m = Math.floor((s % 3600) / 60)
    return `${h}h ${m}m`
  }
  if (s < 60) return `${s} 秒`
  if (s < 3600) return `${Math.floor(s / 60)} 分钟`
  const h = Math.floor(s / 3600)
  const m = Math.floor((s % 3600) / 60)
  return `${h} 小时 ${m} 分`
}
