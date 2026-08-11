import { describe, it, expect } from 'vitest'
import { hasActiveTask, countActiveTasks } from '../lib/taskStatus'

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
