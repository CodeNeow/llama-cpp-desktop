import { describe, it, expect } from 'vitest'
import { appendLogEntries, applyFullLogFetch, SERVER_LOG_WINDOW, type ServerLogEntry } from '../lib/serverLog'

// Pure helpers for the incremental server-log view (Api.vue cursor polling);
// assert the merge / trim / reset-on-gap behavior directly, no mocks needed.
function entries(...pairs: [number, string][]): ServerLogEntry[] {
  return pairs.map(([seq, text]) => ({ seq, text }))
}

describe('appendLogEntries', () => {
  it('appends new entries and advances the cursor', () => {
    const r = appendLogEntries(['a'], 1, entries([1, 'b'], [2, 'c']), 3)
    expect(r.lines).toEqual(['a', 'b', 'c'])
    expect(r.cursor).toBe(3)
    expect(r.reset).toBe(false)
  })

  it('keeps the view untouched when nothing new arrived', () => {
    const r = appendLogEntries(['a'], 5, [], 5)
    expect(r.lines).toEqual(['a'])
    expect(r.cursor).toBe(5)
    expect(r.reset).toBe(false)
  })

  it('trims the local view to the cap, keeping the newest lines', () => {
    const lines = Array.from({ length: 10 }, (_, i) => `old-${i}`)
    const r = appendLogEntries(lines, 19, entries([20, 'new-0'], [21, 'new-1']), 22, 10)
    expect(r.lines).toHaveLength(10)
    expect(r.lines[0]).toBe('old-2')
    expect(r.lines[9]).toBe('new-1')
    expect(r.reset).toBe(false)
  })

  it('flags a reset when the cursor fell out of the retention window', () => {
    // next - cursor = 3000 > cap: lines were evicted between polls, the view
    // cannot be patched — the caller must refetch from 0 and replace it.
    const r = appendLogEntries(['stale'], 0, entries([2500, 'x']), 3000, SERVER_LOG_WINDOW)
    expect(r.reset).toBe(true)
    expect(r.lines).toEqual(['stale'])
    expect(r.cursor).toBe(0)
  })

  it('does not flag a reset at the exact retention boundary', () => {
    // next - cursor == cap: everything since the cursor is still retained.
    const e = entries(...(Array.from({ length: SERVER_LOG_WINDOW }, (_, i) => [i, `l${i}`] as [number, string])))
    const r = appendLogEntries([], 0, e, SERVER_LOG_WINDOW, SERVER_LOG_WINDOW)
    expect(r.reset).toBe(false)
    expect(r.lines).toHaveLength(SERVER_LOG_WINDOW)
    expect(r.lines[0]).toBe('l0')
    expect(r.cursor).toBe(SERVER_LOG_WINDOW)
  })
})

describe('applyFullLogFetch', () => {
  it('replaces the view and adopts the next cursor', () => {
    const r = applyFullLogFetch(entries([5, 'a'], [6, 'b']), 7)
    expect(r.lines).toEqual(['a', 'b'])
    expect(r.cursor).toBe(7)
  })

  it('trims a full fetch that exceeds the cap, keeping the newest lines', () => {
    const e = entries(...(Array.from({ length: SERVER_LOG_WINDOW + 500 }, (_, i) => [i, `l${i}`] as [number, string])))
    const r = applyFullLogFetch(e, SERVER_LOG_WINDOW + 500, SERVER_LOG_WINDOW)
    expect(r.lines).toHaveLength(SERVER_LOG_WINDOW)
    expect(r.lines[0]).toBe('l500')
    expect(r.cursor).toBe(SERVER_LOG_WINDOW + 500)
  })
})
