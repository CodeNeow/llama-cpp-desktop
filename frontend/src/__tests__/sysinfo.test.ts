import { describe, it, expect } from 'vitest'
import { aggregateVram, buildGpuDisplays, cudaCompatLevel } from '../lib/sysinfo'
import type { GpuStaticInfo } from '../lib/sysinfo'
import type { MonitorStatus } from '../lib/monitor'

const staticGpu = (overrides: Partial<GpuStaticInfo> = {}): GpuStaticInfo => ({
  name: 'Test GPU',
  memoryMb: 8192,
  memoryUsedMb: 2048,
  driverVersion: '566.36',
  computeCapability: 8.9,
  ...overrides
})

describe('aggregateVram', () => {
  it('empty list returns null (no GPU card)', () => {
    expect(aggregateVram([])).toBeNull()
  })

  it('sums totals and used across GPUs', () => {
    const r = aggregateVram([
      { totalMb: 8192, usedMb: 2048 },
      { totalMb: 4096, usedMb: 1024 }
    ])
    expect(r).toEqual({ totalMb: 12288, usedMb: 3072 })
  })

  it('skips unknown (<=0) values so one mis-report cannot zero the sum', () => {
    const r = aggregateVram([
      { totalMb: 8192, usedMb: 2048 },
      { totalMb: 0, usedMb: -1 }
    ])
    expect(r).toEqual({ totalMb: 8192, usedMb: 2048 })
  })

  it('all-unknown list still returns zeroed totals (not null)', () => {
    expect(aggregateVram([{ totalMb: 0, usedMb: 0 }])).toEqual({ totalMb: 0, usedMb: 0 })
  })
})

describe('cudaCompatLevel', () => {
  it('compute capability >= 12.0 classifies as blackwell', () => {
    expect(cudaCompatLevel(12.0)).toBe('blackwell')
    expect(cudaCompatLevel(12.9)).toBe('blackwell')
  })

  it('older cards classify as ok', () => {
    expect(cudaCompatLevel(11.9)).toBe('ok')
    expect(cudaCompatLevel(8.9)).toBe('ok')
    expect(cudaCompatLevel(0)).toBe('ok')
  })
})

describe('buildGpuDisplays', () => {
  it('without monitor samples falls back to static snapshot values and null utilization', () => {
    const displays = buildGpuDisplays([staticGpu()])
    expect(displays).toEqual([
      {
        name: 'Test GPU',
        driverVersion: '566.36',
        computeCapability: 8.9,
        totalMb: 8192,
        usedMb: 2048,
        utilPercent: null
      }
    ])
  })

  it('live monitor samples (bytes) win over the snapshot', () => {
    const monitorGpus: MonitorStatus['gpus'] = [
      { index: 0, name: 'Test GPU', utilPercent: 37.4, memUsed: 1024 * 1024 * 1024, memTotal: 4096 * 1024 * 1024 }
    ]
    const displays = buildGpuDisplays([staticGpu()], monitorGpus)
    // bytes → MiB rounding
    expect(displays[0].totalMb).toBe(4096)
    expect(displays[0].usedMb).toBe(1024)
    // utilization is rounded to an integer percent
    expect(displays[0].utilPercent).toBe(37)
  })

  it('matches GPUs by monitor index, not position in a filtered list', () => {
    const monitorGpus: MonitorStatus['gpus'] = [
      { index: 1, name: 'Second GPU', utilPercent: 5, memUsed: 512 * 1024 * 1024, memTotal: 2048 * 1024 * 1024 }
    ]
    const displays = buildGpuDisplays([staticGpu()], monitorGpus)
    // index 1 does not match display position 0: keep snapshot values
    expect(displays[0].totalMb).toBe(8192)
    expect(displays[0].usedMb).toBe(2048)
    expect(displays[0].utilPercent).toBeNull()
  })

  it('zero/invalid monitor values fall back to the snapshot field-by-field', () => {
    const monitorGpus: MonitorStatus['gpus'] = [
      { index: 0, name: 'Test GPU', utilPercent: Number.NaN, memUsed: 0, memTotal: 0 }
    ]
    const displays = buildGpuDisplays([staticGpu()], monitorGpus)
    expect(displays[0].totalMb).toBe(8192)
    expect(displays[0].usedMb).toBe(2048)
    expect(displays[0].utilPercent).toBeNull()
  })

  it('handles a missing monitor array entirely', () => {
    const displays = buildGpuDisplays([staticGpu(), staticGpu({ name: 'GPU B' })], null)
    expect(displays).toHaveLength(2)
    expect(displays[1].name).toBe('GPU B')
  })
})
