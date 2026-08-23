import { describe, it, expect } from 'vitest'
import { hasActiveTask, countActiveTasks, visibleTasks, activeTaskItems, finishedTaskItems } from '../lib/taskStatus'
import { MODEL_TASK_STATUSES } from '../lib/downloadStatus'
import type { ModelTaskStatus } from '../lib/downloadStatus'

// Tables keyed by ModelTaskStatus make an unlisted new backend status a compile error
// (vue-tsc fails on the missing Record key), and iterating the canonical
// MODEL_TASK_STATUSES list makes a wrongly-listed status a runtime test failure.

// "Active" semantics shared by hasActiveTask/countActiveTasks (same predicate as
// lib/taskStatus.ts): only downloading / paused / queued count; fetching / extracting
// and the terminal done / error / cancelled do not (mirrors current behavior).
const activeTable: Record<ModelTaskStatus, boolean> = {
  queued: true,
  fetching: false,
  downloading: true,
  paused: true,
  extracting: false,
  done: false,
  error: false,
  cancelled: false,
}

// visibleTasks semantics: every canonical status is shown except cancelled.
const visibleTable: Record<ModelTaskStatus, boolean> = {
  queued: true,
  fetching: true,
  downloading: true,
  paused: true,
  extracting: true,
  done: true,
  error: true,
  cancelled: false,
}

describe('hasActiveTask', () => {
  it('single-task arrays map per canonical status', () => {
    for (const status of MODEL_TASK_STATUSES) {
      expect(hasActiveTask([{ status }]), `status=${status}`).toBe(activeTable[status])
    }
  })

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
  it('single-task arrays count per canonical status', () => {
    for (const status of MODEL_TASK_STATUSES) {
      expect(countActiveTasks([{ status }]), `status=${status}`).toBe(activeTable[status] ? 1 : 0)
    }
  })

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
  it('single-task arrays map per canonical status (all kept except cancelled)', () => {
    for (const status of MODEL_TASK_STATUSES) {
      const out = visibleTasks([{ status }])
      if (visibleTable[status]) {
        expect(out, `status=${status} should be kept`).toEqual([{ status }])
      } else {
        expect(out, `status=${status} should be filtered out`).toEqual([])
      }
    }
  })

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

describe('activeTaskItems', () => {
  it('keeps only in-flight statuses (queued/downloading/paused)', () => {
    const tasks = [
      { id: '1', status: 'downloading' },
      { id: '2', status: 'done' },
      { id: '3', status: 'paused' },
      { id: '4', status: 'error' },
      { id: '5', status: 'queued' },
      { id: '6', status: 'cancelled' }
    ]
    expect(activeTaskItems(tasks).map(t => t.id)).toEqual(['1', '3', '5'])
  })

  it('returns empty for an empty list', () => {
    expect(activeTaskItems([])).toEqual([])
  })
})

describe('finishedTaskItems', () => {
  it('keeps only terminal done/error for the history group', () => {
    const tasks = [
      { id: '1', status: 'downloading' },
      { id: '2', status: 'done' },
      { id: '3', status: 'error' },
      { id: '4', status: 'cancelled' }
    ]
    expect(finishedTaskItems(tasks).map(t => t.id)).toEqual(['2', '3'])
  })

  it('returns empty for an empty list', () => {
    expect(finishedTaskItems([])).toEqual([])
  })
})
