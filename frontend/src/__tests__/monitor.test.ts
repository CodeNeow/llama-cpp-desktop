import { describe, it, expect } from 'vitest'
import { apiSpeedPlacement, appendHistory, chartPoints, formatPromptTps, formatUptime, showDirectModeTag } from '../lib/monitor'

describe('appendHistory', () => {
  it('append new value and return new array (does not modify input)', () => {
    const history = [1, 2]
    const next = appendHistory(history, 3)
    expect(next).toEqual([1, 2, 3])
    expect(history).toEqual([1, 2])
  })

  it('when exceeding cap, drop oldest value and maintain capacity limit', () => {
    const history = [1, 2, 3]
    const next = appendHistory(history, 4, 3)
    expect(next).toEqual([2, 3, 4])
    expect(next.length).toBe(3)
  })

  it('default cap is 60', () => {
    let history: number[] = []
    for (let i = 0; i < 65; i++) history = appendHistory(history, i)
    expect(history.length).toBe(60)
    expect(history[0]).toBe(5) // oldest 5 have been discarded
    expect(history[59]).toBe(64)
  })
})

describe('chartPoints', () => {
  it('empty sequence returns empty string', () => {
    expect(chartPoints([], 100, 50)).toBe('')
  })

  it('x evenly distributed, y bottom-aligned with 2px margin', () => {
    // two-point sequence: 0 and 100, width=100 height=100
    // point1: x=0, normalized 0 => y=100-2-0*96=98; point2: x=100, normalized 1 => y=100-2-96=2
    const points = chartPoints([0, 100], 100, 100)
    expect(points).toBe('0.0,98.0 100.0,2.0')
  })

  it('max defaults to history max; explicit max scales by provided value', () => {
    // default max=20: history [10, 20], width=100 height=50 (usable=46)
    // point1 normalized 0.5 => y=50-2-0.5*46=25; point2 normalized 1 => y=2
    const auto = chartPoints([10, 20], 100, 50)
    expect(auto).toBe('0.0,25.0 100.0,2.0')
    // explicit max=100: normalized 0.1 / 0.2 => y=43.4 / 38.8
    const fixed = chartPoints([10, 20], 100, 50, 100)
    expect(fixed).toBe('0.0,43.4 100.0,38.8')
  })

  it('all-zero sequence avoids div-by-zero (max floor 1), line hugs bottom', () => {
    const points = chartPoints([0, 0, 0], 100, 50)
    // normalized 0 => y constant 48 (height-2)
    expect(points).toBe('0.0,48.0 50.0,48.0 100.0,48.0')
  })

  it('single point centered horizontally, at top when value equals max', () => {
    const points = chartPoints([5], 100, 50)
    expect(points).toBe('50.0,2.0')
  })
})

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

describe('apiSpeedPlacement', () => {
  it('phone and desktop tiers keep the chart embedded in the hero card', () => {
    expect(apiSpeedPlacement(false)).toBe('hero')
  })

  it('tablet tier (portrait band) moves the chart to its own island', () => {
    expect(apiSpeedPlacement(true)).toBe('island')
  })
})

describe('showDirectModeTag', () => {
  it('shows only on Android tablet tiers while the server is running', () => {
    expect(showDirectModeTag(true, true, true)).toBe(true)
  })

  it('never shows on desktop OSes (router mode, not direct mode)', () => {
    expect(showDirectModeTag(false, true, true)).toBe(false)
  })

  it('never shows on the phone tier (phone page rendering is unchanged)', () => {
    expect(showDirectModeTag(true, false, true)).toBe(false)
  })

  it('drops the annotation while the server is stopped (draft frame ⑭)', () => {
    expect(showDirectModeTag(true, true, false)).toBe(false)
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
