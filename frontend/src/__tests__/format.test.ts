import { describe, it, expect } from 'vitest'
import { formatSpeed, formatBytes } from '../lib/format'

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
