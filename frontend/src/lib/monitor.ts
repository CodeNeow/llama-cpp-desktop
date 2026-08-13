/**
 * 监控数据纯函数工具：供 Monitor.vue 维护折线图历史、计算 SVG polyline
 * 坐标与格式化运行时长。全部为纯函数，便于单测；MonitorStatus 接口与后端
 * GetMonitorStatus 的 JSON 契约一一对应。
 */

export interface MonitorStatus {
  cpuPercent: number
  memUsed: number
  memTotal: number
  gpus: { index: number; name: string; utilPercent: number; memUsed: number; memTotal: number }[]
  serverRunning: boolean
  /** 提示词处理速度 tokens/s（提示词预填充 prefill）：预填充期间按批实时刷新（prompt processing 行，新 llama.cpp），请求结束时更新为最终值（prompt eval time 行） */
  promptTps: number
  /** 生成速度 tokens/s（实时解码 decode）：生成期间由 tg_3s 日志行每约 3 秒刷新，请求结束时以 eval time 行兜底 */
  decodeTps: number
  uptimeSeconds: number
}

/** 追加一个采样值到历史数组；超出 cap 时丢弃最旧值，返回新数组（不修改入参）。 */
export function appendHistory(history: number[], value: number, cap = 60): number[] {
  const next = [...history, value]
  if (next.length > cap) next.splice(0, next.length - cap)
  return next
}

/**
 * 将历史序列转换为 SVG polyline 的 points 串：
 * - x 均匀分布（单点水平居中）；
 * - y 底部对齐并留 2px 边距，值越大越靠上；
 * - max 缺省取历史最大值，且下限为 1，全 0 序列也不会除零。
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
 * 提示词处理速度的显示规则：服务运行中但尚无测量值（tps <= 0，含 NaN）时
 * 显示「—」占位，有测量值时保留 1 位小数。区别于生成速度（实时 0.0 仍显示 0.0）。
 */
export function formatPromptTps(tps: number): string {
  return tps > 0 ? tps.toFixed(1) : '—'
}

/** 把秒数格式化为人类可读的中文运行时长，如 "45 秒"、"1 小时 23 分"。 */
export function formatUptime(seconds: number): string {
  const s = Math.max(0, Math.floor(seconds))
  if (s < 60) return `${s} 秒`
  if (s < 3600) return `${Math.floor(s / 60)} 分钟`
  const h = Math.floor(s / 3600)
  const m = Math.floor((s % 3600) / 60)
  return `${h} 小时 ${m} 分`
}
