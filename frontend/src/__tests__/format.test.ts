import { describe, it, expect } from 'vitest'
import { formatSpeed, formatBytes, usagePercent } from '../lib/format'

describe('formatSpeed', () => {
  it('<=0 返回空串', () => {
    expect(formatSpeed(0)).toBe('')
    expect(formatSpeed(-1)).toBe('')
  })

  it('< 1024 显示字节数（B/s）', () => {
    expect(formatSpeed(512)).toBe('512 B/s')
    expect(formatSpeed(1023)).toBe('1023 B/s')
  })

  it('KB/s 一位小数', () => {
    expect(formatSpeed(1024)).toBe('1.0 KB/s')
    expect(formatSpeed(2048)).toBe('2.0 KB/s')
    expect(formatSpeed(1536)).toBe('1.5 KB/s')
  })

  it('MB/s 一位小数', () => {
    expect(formatSpeed(1024 * 1024)).toBe('1.0 MB/s')
    expect(formatSpeed(1.5 * 1024 * 1024)).toBe('1.5 MB/s')
  })

  it('GB/s 一位小数', () => {
    expect(formatSpeed(1024 * 1024 * 1024)).toBe('1.0 GB/s')
    expect(formatSpeed(2.5 * 1024 * 1024 * 1024)).toBe('2.5 GB/s')
  })

  it('边界：刚好等于各量级阈值', () => {
    expect(formatSpeed(1024 * 1024 * 1024)).toBe('1.0 GB/s')
  })
})

describe('usagePercent', () => {
  // total<=0 或 used<0 时返回 0
  it('total=0 返回 0', () => {
    expect(usagePercent(10, 0)).toBe(0)
    expect(usagePercent(0, 0)).toBe(0)
  })

  it('used<0 返回 0', () => {
    expect(usagePercent(-1, 100)).toBe(0)
  })

  // 正常值保留一位小数
  it('正常值保留一位小数', () => {
    expect(usagePercent(14.2, 31.2)).toBeCloseTo(45.5, 1)
    expect(usagePercent(0, 100)).toBe(0)
    expect(usagePercent(100, 100)).toBe(100)
  })

  // used>total 截断为 100
  it('used>total 截断为 100', () => {
    expect(usagePercent(200, 100)).toBe(100)
  })

  // 一位小数舍入
  it('一位小数四舍五入', () => {
    expect(usagePercent(1, 3)).toBeCloseTo(33.3, 1)
    expect(usagePercent(2, 3)).toBeCloseTo(66.7, 1)
  })
})


describe('formatBytes', () => {
  it('<=0 返回空串', () => {
    expect(formatBytes(0)).toBe('')
    expect(formatBytes(-100)).toBe('')
  })

  it('各量级格式化与下载页历史行为一致', () => {
    expect(formatBytes(1024)).toBe('1 KB')
    expect(formatBytes(1.5 * 1024 * 1024)).toBe('1.5 MB')
    expect(formatBytes(2 * 1024 * 1024 * 1024)).toBe('2.00 GB')
  })
})
