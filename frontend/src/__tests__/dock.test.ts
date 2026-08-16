import { describe, it, expect } from 'vitest'
import { activeLlamaCppDownload, activeModelTasks, shouldShowDock } from '../lib/dock'

describe('activeLlamaCppDownload', () => {
  it('idle/done 返回 false', () => {
    expect(activeLlamaCppDownload('idle')).toBe(false)
    expect(activeLlamaCppDownload('done')).toBe(false)
  })

  it('fetching/downloading/paused/extracting/error 返回 true', () => {
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

  it('过滤出活跃任务（queued/fetching/downloading/paused/extracting）', () => {
    const out = activeModelTasks(tasks)
    expect(out.map(t => t.id)).toEqual(['1', '2', '3'])
  })

  it('空数组返回空数组', () => {
    expect(activeModelTasks([])).toEqual([])
  })
})

describe('shouldShowDock', () => {
  it('全部为零时不显示', () => {
    expect(shouldShowDock(false, 0, 0)).toBe(false)
  })

  it('llamaActive=true 时显示', () => {
    expect(shouldShowDock(true, 0, 0)).toBe(true)
  })

  it('activeTaskCount>0 时显示', () => {
    expect(shouldShowDock(false, 1, 0)).toBe(true)
  })

  it('loadedCount>0 时显示', () => {
    expect(shouldShowDock(false, 0, 1)).toBe(true)
  })
})
