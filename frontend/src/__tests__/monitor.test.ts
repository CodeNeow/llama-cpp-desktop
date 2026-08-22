import { describe, it, expect } from 'vitest'
import { formatPromptTps, formatUptime } from '../lib/monitor'

describe('formatPromptTps', () => {
  it('no measurement (0 or negative) returns "—" placeholder', () => {
    // while service is running, promptTps is 0 before request ends; negative treated as no measurement
    expect(formatPromptTps(0)).toBe('—')
    expect(formatPromptTps(-1)).toBe('—')
  })

  it('NaN returns "—" placeholder', () => {
    expect(formatPromptTps(NaN)).toBe('—')
  })

  it('normal values keep 1 decimal place', () => {
    // toFixed(1) rounds: 128.36 -> 128.4
    expect(formatPromptTps(128.36)).toBe('128.4')
    expect(formatPromptTps(12.04)).toBe('12.0')
  })

  it('tiny positive shows 0.0 at 1 decimal (distinct from no-measurement "—")', () => {
    expect(formatPromptTps(0.004)).toBe('0.0')
  })
})

describe('formatUptime', () => {
  it('Chinese: under 1 minute shows seconds', () => {
    expect(formatUptime(0, 'zh')).toBe('0 秒')
    expect(formatUptime(59, 'zh')).toBe('59 秒')
  })

  it('Chinese: exactly 1 minute and under 1 hour shows minutes', () => {
    expect(formatUptime(60, 'zh')).toBe('1 分钟')
    expect(formatUptime(3599, 'zh')).toBe('59 分钟')
  })

  it('Chinese: over 1 hour shows hours + minutes', () => {
    expect(formatUptime(3600, 'zh')).toBe('1 小时 0 分')
    expect(formatUptime(3600 + 23 * 60, 'zh')).toBe('1 小时 23 分')
    expect(formatUptime(7200 + 45, 'zh')).toBe('2 小时 0 分')
  })

  it('Chinese: negative and decimal treated as 0', () => {
    expect(formatUptime(-5, 'zh')).toBe('0 秒')
    expect(formatUptime(45.9, 'zh')).toBe('45 秒')
  })

  it('English: under 1 minute shows {n}s', () => {
    expect(formatUptime(0, 'en')).toBe('0s')
    expect(formatUptime(59, 'en')).toBe('59s')
  })

  it('English: under 1 hour shows {n}m', () => {
    expect(formatUptime(60, 'en')).toBe('1m')
    expect(formatUptime(3599, 'en')).toBe('59m')
  })

  it('English: over 1 hour shows {h}h {m}m', () => {
    expect(formatUptime(3600, 'en')).toBe('1h 0m')
    expect(formatUptime(3600 + 23 * 60, 'en')).toBe('1h 23m')
    expect(formatUptime(7200 + 45, 'en')).toBe('2h 0m')
  })
})
