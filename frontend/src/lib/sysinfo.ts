/**
 * System Info page (Home.vue) pure-function helpers: GPU display merging,
 * VRAM aggregation and CUDA compatibility classification. No side effects,
 * no backend calls — easy to unit-test in isolation.
 */

import type { MonitorStatus } from './monitor'

/** Static per-GPU info as returned by the backend GetGPU binding (MB units). */
export interface GpuStaticInfo {
  name: string
  /** Vendor classification: 'nvidia' | 'intel' | 'amd' | 'apple' | '' (unknown). */
  vendor: string
  memoryMb: number
  memoryUsedMb: number
  driverVersion: string
  computeCapability: number
}

/** Per-GPU view model rendered by the Home page: static identity + live metrics when available. */
export interface GpuDisplay {
  name: string
  vendor: string
  driverVersion: string
  computeCapability: number
  totalMb: number
  usedMb: number
  /** Live GPU utilization percent; null when no monitor sample exists yet. */
  utilPercent: number | null
}

/** Summed VRAM across GPUs (what matters for multi-GPU model offloading). */
export interface VramTotals {
  totalMb: number
  usedMb: number
}

/**
 * Whether a GPU reports discrete VRAM worth rendering: NVIDIA always (the
 * nvidia-smi probe samples it), Apple Silicon never (unified memory — no
 * VRAM bar), and every other vendor only when the probe returned a positive
 * total (Linux PCI display controllers carry no VRAM fields).
 */
export function gpuHasVram(gpu: Pick<GpuDisplay, 'vendor' | 'totalMb'>): boolean {
  if (gpu.vendor === 'apple') return false
  if (gpu.vendor === 'nvidia') return true
  return gpu.totalMb > 0
}

/**
 * Aggregate VRAM across NVIDIA GPUs. Non-NVIDIA vendors are skipped: NVIDIA
 * is the only vendor whose VRAM is sampled (monitor + probe), and mixing in
 * unreported/zero values from PCI-classified or unified-memory cards would
 * distort the offloading picture. Unknown values (<= 0) are skipped so one
 * mis-reporting card cannot zero out the sum; returns null for an empty list.
 */
export function aggregateVram(items: Pick<GpuDisplay, 'vendor' | 'totalMb' | 'usedMb'>[]): VramTotals | null {
  let totalMb = 0
  let usedMb = 0
  let counted = 0
  for (const item of items) {
    if (item.vendor !== 'nvidia') continue
    counted++
    if (item.totalMb > 0) totalMb += item.totalMb
    if (item.usedMb > 0) usedMb += item.usedMb
  }
  if (counted === 0) return null
  return { totalMb, usedMb }
}

export type CudaCompatLevel = 'ok' | 'need' | 'satisfied'

/**
 * The Blackwell CUDA floor (12.8) for compute capability >= 12.0, duplicated
 * from the backend's cudaFloorForComputeCap (core/sysinfo.go) — the backend is
 * authoritative; keep this literal in sync when changing one side.
 */
const BLACKWELL_CUDA_FLOOR = 12.8

/**
 * Whether an installed cudart version satisfies the given CUDA floor.
 * Parses "13" / "12.8" / "12.4" (major[.minor]); empty or unparsable input
 * never satisfies the floor. Deliberately conservative: the cudart DLL file
 * name only reveals the CUDA major family (cudart64_13.dll = CUDA 13), so a
 * bare major that merely equals the floor's major cannot prove the minor
 * requirement is met — "12" must NOT satisfy a 12.8 floor, only "12.8" or
 * higher with an explicit minor part does. A higher major ("13") always
 * satisfies the floor.
 */
export function cudartVersionSatisfiesFloor(version: string, floor: number): boolean {
  const m = /^(\d+)(?:\.(\d+))?$/.exec(version.trim())
  if (!m) return false
  const major = Number(m[1])
  const minor = m[2] === undefined ? null : Number(m[2])
  const floorMajor = Math.floor(floor)
  // Scale the fractional part (0.8 → 8); rounding absorbs float noise (12.8 − 12)
  const floorMinor = Math.round((floor - floorMajor) * 10)
  if (major > floorMajor) return true
  if (major < floorMajor) return false
  // Equal majors: an absent minor cannot prove the floor's minor is met
  if (minor === null) return false
  return minor >= floorMinor
}

/**
 * CUDA compatibility class from the GPU compute capability plus the installed
 * cudart runtime: pre-Blackwell cards (compute capability < 12.0) run on the
 * standard builds ('ok'); Blackwell cards (>= 12.0) with a cudart runtime
 * family that provably satisfies the 12.8 floor are 'satisfied'; Blackwell
 * cards without one (or with an unverifiable 12.x runtime) stay at 'need'.
 */
export function cudaCompatLevel(computeCapability: number, cudartVersion?: string): CudaCompatLevel {
  if (computeCapability < 12.0) return 'ok'
  return cudartVersionSatisfiesFloor(cudartVersion ?? '', BLACKWELL_CUDA_FLOOR) ? 'satisfied' : 'need'
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
    // Monitor samples exist only for NVIDIA (nvidia-smi is the sampler):
    // never attach one to a non-NVIDIA entry, or a PCI/apple card could
    // pick up an unrelated card's VRAM/utilization by index.
    const m = gpu.vendor === 'nvidia' ? byIndex.get(i) : undefined
    return {
      name: gpu.name,
      vendor: gpu.vendor,
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
