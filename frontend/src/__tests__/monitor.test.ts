import { describe, it, expect } from 'vitest'
import { appendHistory, chartPoints, formatUptime } from '../lib/monitor'

describe('appendHistory', () => {
  it('追加新值并返回新数组（不修改入参）', () => {
    const history = [1, 2]
    const next = appendHistory(history, 3)
    expect(next).toEqual([1, 2, 3])
    expect(history).toEqual([1, 2])
  })

  it('超出 cap 时丢弃最旧值，保持容量上限', () => {
    const history = [1, 2, 3]
    const next = appendHistory(history, 4, 3)
    expect(next).toEqual([2, 3, 4])
    expect(next.length).toBe(3)
  })

  it('默认 cap 为 60', () => {
    let history: number[] = []
    for (let i = 0; i < 65; i++) history = appendHistory(history, i)
    expect(history.length).toBe(60)
    expect(history[0]).toBe(5) // 最旧的 5 个已被丢弃
    expect(history[59]).toBe(64)
  })
})

describe('chartPoints', () => {
  it('空序列返回空串', () => {
    expect(chartPoints([], 100, 50)).toBe('')
  })

  it('x 均匀分布、y 底部对齐并留 2px 边距', () => {
    // 两点序列：0 与 100，width=100 height=100 时
    // 点1：x=0, 归一化 0 => y=100-2-0*96=98；点2：x=100, 归一化 1 => y=100-2-96=2
    const points = chartPoints([0, 100], 100, 100)
    expect(points).toBe('0.0,98.0 100.0,2.0')
  })

  it('max 缺省取历史最大值；显式 max 时按传入值缩放', () => {
    // 缺省 max=20：历史 [10, 20]，width=100 height=50（usable=46）
    // 点1 归一化 0.5 => y=50-2-0.5*46=25；点2 归一化 1 => y=2
    const auto = chartPoints([10, 20], 100, 50)
    expect(auto).toBe('0.0,25.0 100.0,2.0')
    // 显式 max=100：归一化 0.1 / 0.2 => y=43.4 / 38.8
    const fixed = chartPoints([10, 20], 100, 50, 100)
    expect(fixed).toBe('0.0,43.4 100.0,38.8')
  })

  it('全 0 序列不除零（max 下限为 1），折线贴底部', () => {
    const points = chartPoints([0, 0, 0], 100, 50)
    // 归一化 0 => y 恒为 48（height-2）
    expect(points).toBe('0.0,48.0 50.0,48.0 100.0,48.0')
  })

  it('单点时水平居中，且数值即最大值时位于顶部', () => {
    const points = chartPoints([5], 100, 50)
    expect(points).toBe('50.0,2.0')
  })
})

describe('formatUptime', () => {
  it('不足 1 分钟显示秒', () => {
    expect(formatUptime(0)).toBe('0 秒')
    expect(formatUptime(59)).toBe('59 秒')
  })

  it('1 分钟内含秒数按秒显示（60 秒 = 1 分钟）', () => {
    expect(formatUptime(60)).toBe('1 分钟')
    expect(formatUptime(45)).toBe('45 秒')
  })

  it('不足 1 小时显示分钟', () => {
    expect(formatUptime(3599)).toBe('59 分钟')
  })

  it('超过 1 小时显示小时 + 分钟', () => {
    expect(formatUptime(3600)).toBe('1 小时 0 分')
    expect(formatUptime(3600 + 23 * 60)).toBe('1 小时 23 分')
    expect(formatUptime(7200 + 45)).toBe('2 小时 0 分')
  })

  it('负数与小数按 0 处理', () => {
    expect(formatUptime(-5)).toBe('0 秒')
    expect(formatUptime(45.9)).toBe('45 秒')
  })
})
