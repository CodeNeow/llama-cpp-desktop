import { describe, it, expect } from 'vitest'
import { hasActiveTask, countActiveTasks, visibleTasks } from '../lib/taskStatus'

describe('hasActiveTask', () => {
  it('全终态任务（done/error/cancelled 任意组合）返回 false', () => {
    expect(hasActiveTask([{ status: 'done' }, { status: 'error' }, { status: 'cancelled' }])).toBe(false)
    expect(hasActiveTask([{ status: 'done' }])).toBe(false)
    expect(hasActiveTask([{ status: 'error' }])).toBe(false)
    expect(hasActiveTask([{ status: 'cancelled' }])).toBe(false)
    expect(hasActiveTask([{ status: 'done' }, { status: 'cancelled' }])).toBe(false)
  })

  it('含 downloading/paused/queued 时返回 true', () => {
    expect(hasActiveTask([{ status: 'downloading' }])).toBe(true)
    expect(hasActiveTask([{ status: 'paused' }])).toBe(true)
    expect(hasActiveTask([{ status: 'queued' }])).toBe(true)
    expect(hasActiveTask([{ status: 'downloading' }, { status: 'done' }])).toBe(true)
    expect(hasActiveTask([{ status: 'queued' }, { status: 'error' }])).toBe(true)
  })

  it('空数组返回 false', () => {
    expect(hasActiveTask([])).toBe(false)
  })
})

describe('countActiveTasks', () => {
  it('空数组返回 0', () => {
    expect(countActiveTasks([])).toBe(0)
  })

  it('全终态任务（done/error/cancelled 任意组合）返回 0', () => {
    expect(countActiveTasks([{ status: 'done' }, { status: 'error' }, { status: 'cancelled' }])).toBe(0)
    expect(countActiveTasks([{ status: 'done' }])).toBe(0)
    expect(countActiveTasks([{ status: 'error' }])).toBe(0)
    expect(countActiveTasks([{ status: 'cancelled' }])).toBe(0)
    expect(countActiveTasks([{ status: 'done' }, { status: 'cancelled' }])).toBe(0)
  })

  it('混合状态（downloading/paused/queued/终态）正确计数', () => {
    const tasks = [
      { status: 'downloading' },
      { status: 'paused' },
      { status: 'queued' },
      { status: 'done' },
      { status: 'error' },
      { status: 'cancelled' },
    ]
    expect(countActiveTasks(tasks)).toBe(3)
  })

  it('queued 计入活跃数', () => {
    expect(countActiveTasks([{ status: 'queued' }])).toBe(1)
    expect(countActiveTasks([{ status: 'queued' }, { status: 'queued' }])).toBe(2)
    expect(countActiveTasks([{ status: 'queued' }, { status: 'done' }])).toBe(1)
  })
})

describe('visibleTasks', () => {
  it('过滤掉 cancelled，保留其他状态', () => {
    const tasks = [
      { status: 'queued' },
      { status: 'downloading' },
      { status: 'paused' },
      { status: 'fetching' },
      { status: 'extracting' },
      { status: 'done' },
      { status: 'error' },
      { status: 'cancelled' },
    ]
    const result = visibleTasks(tasks)
    expect(result).toHaveLength(7)
    expect(result.map(t => t.status)).not.toContain('cancelled')
    expect(result.map(t => t.status)).toEqual(
      expect.arrayContaining(['queued', 'downloading', 'paused', 'fetching', 'extracting', 'done', 'error'])
    )
  })

  it('空数组返回空数组', () => {
    expect(visibleTasks([])).toEqual([])
  })

  it('全 cancelled 返回空数组', () => {
    expect(visibleTasks([{ status: 'cancelled' }, { status: 'cancelled' }])).toEqual([])
  })

  it('无 cancelled 时内容不变（filter 返回新数组，内容与原数组一致）', () => {
    const tasks = [{ status: 'downloading' }, { status: 'error' }]
    const result = visibleTasks(tasks)
    expect(result).toHaveLength(2)
    expect(result).toStrictEqual(tasks)
  })
})
