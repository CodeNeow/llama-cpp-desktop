import { describe, it, expect } from 'vitest'
import { hasActiveTask } from '../lib/taskStatus'

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
