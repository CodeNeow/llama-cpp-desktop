import { describe, it, expect } from 'vitest'
import { LimitedQueue } from '../lib/limitedQueue'

// 并发受限队列是纯逻辑模块，直接断言调度行为，不 mock 任何依赖。
function deferred<T>(): { promise: Promise<T>; resolve: (v: T) => void; reject: (e: unknown) => void } {
  let resolve!: (v: T) => void
  let reject!: (e: unknown) => void
  const promise = new Promise<T>((res, rej) => { resolve = res; reject = rej })
  return { promise, resolve, reject }
}

describe('LimitedQueue', () => {
  it('并发不超过上限，且任务按入队顺序执行', async () => {
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

    // 前两个任务应已开始执行（并发上限 2），其余排队
    await new Promise(r => setTimeout(r, 0))
    expect(active).toBe(2)
    expect(maxActive).toBe(2)
    expect(queue.pending()).toBe(5)

    // 依次放行，任务应逐个补位并按序完成
    for (let i = 0; i < gates.length; i++) {
      gates[i].resolve()
      await new Promise(r => setTimeout(r, 0))
    }
    expect(maxActive).toBe(2)
    expect(order).toEqual([0, 1, 2, 3, 4])
    expect(queue.pending()).toBe(0)
  })

  it('任务失败不影响后续任务执行', async () => {
    const queue = new LimitedQueue(1)
    const done: string[] = []
    const d1 = deferred<void>()

    queue.push(async () => { await d1.promise; throw new Error('boom') })
    queue.push(async () => { done.push('second'); await Promise.resolve() })

    await new Promise(r => setTimeout(r, 0))
    expect(done).toEqual([]) // 第二个还在排队

    d1.resolve()
    await new Promise(r => setTimeout(r, 0))
    expect(done).toEqual(['second']) // 第一个失败后第二个照常执行
  })

  it('max 非法时抛错', () => {
    expect(() => new LimitedQueue(0)).toThrow()
  })
})
