import { describe, it, expect } from 'vitest'
import { LimitedQueue } from '../lib/limitedQueue'

// concurrency-limited queue is a pure logic module; assert scheduling behavior directly without mocking dependencies.
function deferred<T>(): { promise: Promise<T>; resolve: (v: T) => void; reject: (e: unknown) => void } {
  let resolve!: (v: T) => void
  let reject!: (e: unknown) => void
  const promise = new Promise<T>((res, rej) => { resolve = res; reject = rej })
  return { promise, resolve, reject }
}

describe('LimitedQueue', () => {
  it('concurrency does not exceed limit, tasks execute in enqueue order', async () => {
    const queue = new LimitedQueue(2)
    let active = 0
    let maxActive = 0
    const order: number[] = []
    const gates = [deferred<void>(), deferred<void>(), deferred<void>(), deferred<void>(), deferred<void>()]

    for (let i = 0; i < gates.length; i++) {
      const idx = i
      queue.push(async () => {
        active++
        maxActive = Math.max(maxActive, active)
        order.push(idx)
        await gates[idx].promise
        active--
      })
    }

    // first two tasks should have started (concurrency limit 2), rest queued
    await new Promise(r => setTimeout(r, 0))
    expect(active).toBe(2)
    expect(maxActive).toBe(2)
    expect(queue.pending()).toBe(5)

    // release in order, tasks should backfill and complete sequentially
    for (let i = 0; i < gates.length; i++) {
      gates[i].resolve()
      await new Promise(r => setTimeout(r, 0))
    }
    expect(maxActive).toBe(2)
    expect(order).toEqual([0, 1, 2, 3, 4])
    expect(queue.pending()).toBe(0)
  })

  it('task failure does not affect subsequent task execution', async () => {
    const queue = new LimitedQueue(1)
    const done: string[] = []
    const d1 = deferred<void>()

    queue.push(async () => { await d1.promise; throw new Error('boom') })
    queue.push(async () => { done.push('second'); await Promise.resolve() })

    await new Promise(r => setTimeout(r, 0))
    expect(done).toEqual([]) // second still queued

    d1.resolve()
    await new Promise(r => setTimeout(r, 0))
    expect(done).toEqual(['second']) // second runs normally after first fails
  })

  it('invalid max throws error', () => {
    expect(() => new LimitedQueue(0)).toThrow()
  })
})
