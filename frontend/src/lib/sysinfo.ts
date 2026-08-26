/**
 * System Info page (Home.vue) pure-function helpers: GPU display merging,
 * VRAM aggregation and CUDA compatibility classification. No side effects,
 * no backend calls — easy to unit-test in isolation.
 */

import type { MonitorStatus } from './monitor'

/** Static per-GPU info as returned by the backend GetGPU binding (MB units). */
export interface GpuStaticInfo {
  name: string
  memoryMb: number
  memoryUsedMb: number
  driverVersion: string
  computeCapability: number
}

/** Per-GPU view model rendered by the Home page: static identity + live metrics when available. */
export interface GpuDisplay {
  name: string
  driverVersion: string
  computeCapability: number
  totalMb: number
  usedMb: number
  /** Live GPU utilization percent; null when no monitor sample exists yet. */
  utilPercent: number | null
}

/** Summed VRAM across all GPUs (what matters for multi-GPU model offloading). */
export interface VramTotals {
  totalMb: number
  usedMb: number
}

/**
 * Aggregate VRAM across GPUs. Unknown values (<= 0) are skipped so one
 * mis-reporting card cannot zero out the sum; returns null for an empty list.
 */
export function aggregateVram(items: Pick<GpuDisplay, 'totalMb' | 'usedMb'>[]): VramTotals | null {
  if (items.length === 0) return null
  let totalMb = 0
  let usedMb = 0
  for (const item of items) {
    if (item.totalMb > 0) totalMb += item.totalMb
    if (item.usedMb > 0) usedMb += item.usedMb
  }
  return { totalMb, usedMb }
}

export type CudaCompatLevel = 'ok' | 'blackwell'

/**
 * CUDA compatibility class from the GPU compute capability: Blackwell cards
 * (compute capability >= 12.0) require CUDA >= 12.8 runtime builds of llama.cpp;
 * anything older runs on the standard builds.
 */
export function cudaCompatLevel(computeCapability: number): CudaCompatLevel {
  return computeCapability >= 12.0 ? 'blackwell' : 'ok'
}

/**
 * Merge the static GPU snapshot with live monitor samples into display models.
 * Monitor samples (byte units) win over the snapshot whenever present; the
 * lists are matched by the monitor's index field, falling back to the snapshot
 * value when a GPU has no sample. Utilization only comes from the monitor.
 */
export function buildGpuDisplays(
  statics: GpuStaticInfo[],
  monitorGpus?: MonitorStatus['gpus'] | null
): GpuDisplay[] {
  const byIndex = new Map((monitorGpus ?? []).map(g => [g.index, g]))
  return statics.map((gpu, i) => {
    const m = byIndex.get(i)
    return {
      name: gpu.name,
      driverVersion: gpu.driverVersion,
      computeCapability: gpu.computeCapability,
      totalMb: m && m.memTotal > 0 ? Math.round(m.memTotal / (1024 * 1024)) : gpu.memoryMb,
      usedMb: m && m.memUsed > 0 ? Math.round(m.memUsed / (1024 * 1024)) : gpu.memoryUsedMb,
      utilPercent: m && Number.isFinite(m.utilPercent) && m.utilPercent >= 0
        ? Math.round(m.utilPercent)
        : null
    }
  })
}
