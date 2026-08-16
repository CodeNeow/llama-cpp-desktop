import { describe, it, expect } from 'vitest'
import { activeLlamaCppDownload, activeModelTasks, shouldShowDock } from '../lib/dock'

describe('activeLlamaCppDownload', () => {
  it('idle/done returns false', () => {
    expect(activeLlamaCppDownload('idle')).toBe(false)
    expect(activeLlamaCppDownload('done')).toBe(false)
  })

  it('fetching/downloading/paused/extracting/error returns true', () => {
    expect(activeLlamaCppDownload('fetching')).toBe(true)
    expect(activeLlamaCppDownload('downloading')).toBe(true)
    expect(activeLlamaCppDownload('paused')).toBe(true)
    expect(activeLlamaCppDownload('extracting')).toBe(true)
    expect(activeLlamaCppDownload('error')).toBe(true)
  })
})

describe('activeModelTasks', () => {
  const tasks = [
    { id: '1', status: 'queued' },
    { id: '2', status: 'downloading' },
    { id: '3', status: 'paused' },
    { id: '4', status: 'done' },
    { id: '5', status: 'error' },
    { id: '6', status: 'cancelled' },
  ]

  it('filter active tasks (queued/fetching/downloading/paused/extracting)', () => {
    const out = activeModelTasks(tasks)
    expect(out.map(t => t.id)).toEqual(['1', '2', '3'])
  })

  it('empty array returns empty array', () => {
    expect(activeModelTasks([])).toEqual([])
  })
})

describe('shouldShowDock', () => {
  it('do not show when all zero', () => {
    expect(shouldShowDock(false, 0, 0)).toBe(false)
  })

  it('show when llamaActive is true', () => {
    expect(shouldShowDock(true, 0, 0)).toBe(true)
  })

  it('show when activeTaskCount > 0', () => {
    expect(shouldShowDock(false, 1, 0)).toBe(true)
  })

  it('show when loadedCount > 0', () => {
    expect(shouldShowDock(false, 0, 1)).toBe(true)
  })
})
