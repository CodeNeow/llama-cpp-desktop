import { describe, it, expect, vi } from 'vitest'
import { watch, nextTick } from 'vue'
import { nudgeDock, dockNudgeCounter } from '../lib/dockNudge'

// lib/dockNudge keeps one module-level counter per test file; assertions are
// relative to the value observed at test start so ordering never matters.
describe('lib/dockNudge', () => {
  // nudgeDock() increments the counter exactly once per call
  it('increments the counter once per nudge', () => {
    const before = dockNudgeCounter.value
    nudgeDock()
    expect(dockNudgeCounter.value).toBe(before + 1)
    nudgeDock()
    expect(dockNudgeCounter.value).toBe(before + 2)
  })

  // watchers fire on each increment: the TaskDock re-poll wiring relies on this
  it('is observable via watch (fires per increment)', async () => {
    const before = dockNudgeCounter.value
    const seen: number[] = []
    const stop = watch(dockNudgeCounter, (v) => seen.push(v))
    nudgeDock()
    await nextTick()
    nudgeDock()
    await nextTick()
    expect(seen).toEqual([before + 1, before + 2])
    expect(dockNudgeCounter.value).toBe(before + 2)
    stop()
  })

  // the exposed counter is read-only: consumers can watch but not write it
  it('exposes a readonly counter', () => {
    const before = dockNudgeCounter.value
    // Dev-mode Vue warns (rather than throws) on readonly writes; silence the
    // expected warning so the test output stays clean.
    const warnSpy = vi.spyOn(console, 'warn').mockImplementation(() => {})
    try {
      ;(dockNudgeCounter as unknown as { value: number }).value = before + 100
    } finally {
      warnSpy.mockRestore()
    }
    expect(dockNudgeCounter.value).toBe(before)
  })
})
