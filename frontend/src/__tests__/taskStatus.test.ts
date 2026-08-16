import { describe, it, expect } from 'vitest'
import { hasActiveTask, countActiveTasks, visibleTasks } from '../lib/taskStatus'

describe('hasActiveTask', () => {
  it('all terminal tasks (done/error/cancelled any combination) return false', () => {
    expect(hasActiveTask([{ status: 'done' }, { status: 'error' }, { status: 'cancelled' }])).toBe(false)
    expect(hasActiveTask([{ status: 'done' }])).toBe(false)
    expect(hasActiveTask([{ status: 'error' }])).toBe(false)
    expect(hasActiveTask([{ status: 'cancelled' }])).toBe(false)
    expect(hasActiveTask([{ status: 'done' }, { status: 'cancelled' }])).toBe(false)
  })

  it('contains downloading/paused/queued returns true', () => {
    expect(hasActiveTask([{ status: 'downloading' }])).toBe(true)
    expect(hasActiveTask([{ status: 'paused' }])).toBe(true)
    expect(hasActiveTask([{ status: 'queued' }])).toBe(true)
    expect(hasActiveTask([{ status: 'downloading' }, { status: 'done' }])).toBe(true)
    expect(hasActiveTask([{ status: 'queued' }, { status: 'error' }])).toBe(true)
  })

  it('empty array returns false', () => {
    expect(hasActiveTask([])).toBe(false)
  })
})

describe('countActiveTasks', () => {
  it('empty array returns 0', () => {
    expect(countActiveTasks([])).toBe(0)
  })

  it('all terminal tasks (done/error/cancelled any combination) return 0', () => {
    expect(countActiveTasks([{ status: 'done' }, { status: 'error' }, { status: 'cancelled' }])).toBe(0)
    expect(countActiveTasks([{ status: 'done' }])).toBe(0)
    expect(countActiveTasks([{ status: 'error' }])).toBe(0)
    expect(countActiveTasks([{ status: 'cancelled' }])).toBe(0)
    expect(countActiveTasks([{ status: 'done' }, { status: 'cancelled' }])).toBe(0)
  })

  it('mixed states (downloading/paused/queued/terminal) count correctly', () => {
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

  it('queued counts as active', () => {
    expect(countActiveTasks([{ status: 'queued' }])).toBe(1)
    expect(countActiveTasks([{ status: 'queued' }, { status: 'queued' }])).toBe(2)
    expect(countActiveTasks([{ status: 'queued' }, { status: 'done' }])).toBe(1)
  })
})

describe('visibleTasks', () => {
  it('filter out cancelled, keep other statuses', () => {
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

  it('empty array returns empty array', () => {
    expect(visibleTasks([])).toEqual([])
  })

  it('all cancelled returns empty array', () => {
    expect(visibleTasks([{ status: 'cancelled' }, { status: 'cancelled' }])).toEqual([])
  })

  it('without cancelled, content unchanged (filter returns new array with same content)', () => {
    const tasks = [{ status: 'downloading' }, { status: 'error' }]
    const result = visibleTasks(tasks)
    expect(result).toHaveLength(2)
    expect(result).toStrictEqual(tasks)
  })
})
