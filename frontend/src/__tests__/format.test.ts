import { describe, it, expect } from 'vitest'
import { formatSpeed, formatBytes, usagePercent } from '../lib/format'

describe('formatSpeed', () => {
  it('<=0 returns empty string', () => {
    expect(formatSpeed(0)).toBe('')
    expect(formatSpeed(-1)).toBe('')
  })

  it('< 1024 shows byte count (B/s)', () => {
    expect(formatSpeed(512)).toBe('512 B/s')
    expect(formatSpeed(1023)).toBe('1023 B/s')
  })

  it('KB/s one decimal', () => {
    expect(formatSpeed(1024)).toBe('1.0 KB/s')
    expect(formatSpeed(2048)).toBe('2.0 KB/s')
    expect(formatSpeed(1536)).toBe('1.5 KB/s')
  })

  it('MB/s one decimal', () => {
    expect(formatSpeed(1024 * 1024)).toBe('1.0 MB/s')
    expect(formatSpeed(1.5 * 1024 * 1024)).toBe('1.5 MB/s')
  })

  it('GB/s one decimal', () => {
    expect(formatSpeed(1024 * 1024 * 1024)).toBe('1.0 GB/s')
    expect(formatSpeed(2.5 * 1024 * 1024 * 1024)).toBe('2.5 GB/s')
  })

  it('boundary: exactly at each magnitude threshold', () => {
    expect(formatSpeed(1024 * 1024 * 1024)).toBe('1.0 GB/s')
  })
})

describe('usagePercent', () => {
  // return 0 when total <= 0 or used < 0
  it('total=0 returns 0', () => {
    expect(usagePercent(10, 0)).toBe(0)
    expect(usagePercent(0, 0)).toBe(0)
  })

  it('used<0 returns 0', () => {
    expect(usagePercent(-1, 100)).toBe(0)
  })

  // normal values keep one decimal place
  it('normal values keep one decimal place', () => {
    expect(usagePercent(14.2, 31.2)).toBeCloseTo(45.5, 1)
    expect(usagePercent(0, 100)).toBe(0)
    expect(usagePercent(100, 100)).toBe(100)
  })

  // clamp used > total to 100
  it('used>total clamps to 100', () => {
    expect(usagePercent(200, 100)).toBe(100)
  })

  // round to one decimal place
  it('round to one decimal place', () => {
    expect(usagePercent(1, 3)).toBeCloseTo(33.3, 1)
    expect(usagePercent(2, 3)).toBeCloseTo(66.7, 1)
  })
})


describe('formatBytes', () => {
  it('<=0 returns empty string', () => {
    expect(formatBytes(0)).toBe('')
    expect(formatBytes(-100)).toBe('')
  })

  it('each magnitude format matches download page historical behavior', () => {
    expect(formatBytes(1024)).toBe('1 KB')
    expect(formatBytes(1.5 * 1024 * 1024)).toBe('1.5 MB')
    expect(formatBytes(2 * 1024 * 1024 * 1024)).toBe('2.00 GB')
  })
})
